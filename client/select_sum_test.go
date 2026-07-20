package client

import (
	"testing"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 这套算法决定「凑数值选卡」时哪些卡还能点（同调/超量素材、仪式与上级召唤的祭品）。
// 此前 Go 侧是 "Simplified: skip CheckSelectSum"，只按张数卡 min/max —— 玩家能选出
// 凑不成的组合、界面还显示合法。这些测试拿具体牌面验证判定。

// mkSum 造一张候选卡，opParam 里塞它能提供的数值。
func mkSum(op1, op2 int) *ClientCard {
	c := NewClientCard()
	c.OpParam = uint32(op1) | uint32(op2)<<16
	return c
}

// 布置一个「凑数值」的局面：sumVal 是目标，pool 是可选的卡。
func setupSum(sumVal, min, max int, mode int, pool []*ClientCard) *ClientField {
	cf := NewClientField()
	cf.SelectSumVal = sumVal
	cf.SelectMin = min
	cf.SelectMax = max
	cf.SelectMode = mode
	cf.SelectSumAll = pool
	cf.SelectedCards = nil
	cf.MustSelectCount = 0
	return cf
}

func selectable(cf *ClientField, c *ClientCard) bool {
	_, ok := cf.SelectSumCards[c]
	return ok
}

// 目标 8，手里有 4/4/3：两张 4 能凑出来，3 不能参与任何可行组合（4+3=7、3+3 没有第二张）。
// 这是最基本的一条：凑不出来的卡不该可点。
func TestCheckSelectSumMarksOnlyUsableCards(t *testing.T) {
	a, b, c := mkSum(4, 0), mkSum(4, 0), mkSum(3, 0)
	cf := setupSum(8, 1, 99, 0, []*ClientCard{a, b, c})

	cf.CheckSelectSum()

	if !selectable(cf, a) || !selectable(cf, b) {
		t.Error("两张 4 能凑出 8，应当可选")
	}
	if selectable(cf, c) {
		t.Error("3 参与不了任何凑出 8 的组合，不该可选")
	}
}

// 选了一张之后，剩下的可选集合要跟着收窄 —— 这是「边选边算」的关键。
func TestCheckSelectSumNarrowsAfterPick(t *testing.T) {
	four, three, five := mkSum(4, 0), mkSum(3, 0), mkSum(5, 0)
	cf := setupSum(8, 1, 99, 0, []*ClientCard{four, three, five})

	// 先选 3，还差 5。注意 SelectSumAll 保持全集不变 —— CheckSelectSum 自己会把
	// 已选的扣掉（对应 C++ 的 selectsum_all，它在整个选择过程中不变）。
	cf.SelectedCards = []*ClientCard{three}
	cf.CheckSelectSum()

	if !selectable(cf, five) {
		t.Error("已选 3、还差 5，那张 5 应当可选")
	}
	if selectable(cf, four) {
		t.Error("已选 3、还差 5，4 凑不出来，不该可选")
	}
}

// opParam 高低位是「二选一」的数值（如可当 4 级也可当 8 级）。
// 只认低位就会漏掉一半可行解。
func TestCheckSelectSumUsesBothParams(t *testing.T) {
	dual := mkSum(3, 8) // 这张可以算 3 或算 8
	cf := setupSum(8, 1, 99, 0, []*ClientCard{dual})

	cf.CheckSelectSum()

	if !selectable(cf, dual) {
		t.Error("这张卡的第二个候选值正好是 8，应当可选 —— 漏读高 16 位了")
	}
}

// 高位带 0x8000 时，整个 opParam 是一个大数值，不能拆成两个 16 位读。
func TestSumParamsSingleLargeValue(t *testing.T) {
	c := NewClientCard()
	c.OpParam = 0x80000000 | 12345

	op1, op2 := sumParams(c.OpParam)

	if op2 != 0 {
		t.Errorf("op2 = %d, 带 0x8000 标记时应为 0", op2)
	}
	if op1 != 0x80000000|12345&0x7fffffff {
		// 具体值按 C++ 公式：op1 = opParam & 0x7fffffff
		want := int((0x80000000 | 12345) & 0x7fffffff)
		if op1 != want {
			t.Errorf("op1 = %d, want %d", op1, want)
		}
	}
}

// 张数上限必须生效：目标 8，池子里只有 2/2/2/2 —— 凑够 8 恰好要 4 张，
// 而 max=3，所以谁都不该可选。
func TestCheckSelectSumRespectsMaxCount(t *testing.T) {
	pool := []*ClientCard{mkSum(2, 0), mkSum(2, 0), mkSum(2, 0), mkSum(2, 0)}
	cf := setupSum(8, 1, 3, 0, pool)

	cf.CheckSelectSum()

	for i, c := range pool {
		if selectable(cf, c) {
			t.Errorf("第 %d 张：凑够 8 需要 4 张，超过 max=3，不该可选", i)
		}
	}
}

// 张数**下限**同样要生效，而且这条要能挡住「和已经对了但张数不够」的情形：
// 目标 8、池子里有一张 8 和两张 4。min=2 时那张单独的 8 不该可选（只有 1 张），
// 两张 4 才合法。只看数值不看张数的话，8 会被误判为可选。
func TestCheckSelectSumRespectsMinCount(t *testing.T) {
	eight := mkSum(8, 0)
	f1, f2 := mkSum(4, 0), mkSum(4, 0)
	cf := setupSum(8, 2, 2, 0, []*ClientCard{eight, f1, f2})

	cf.CheckSelectSum()

	if selectable(cf, eight) {
		t.Error("单独一张 8 只有 1 张，不满足 min=2，不该可选")
	}
	if !selectable(cf, f1) || !selectable(cf, f2) {
		t.Error("两张 4 既凑出 8 又满足张数 2，应当可选")
	}
}

// 已选组合的和对了、但张数不够时，不能判定为「可以提交」。
func TestCheckSelectSumNotReadyWhenTooFewCards(t *testing.T) {
	eight := mkSum(8, 0)
	cf := setupSum(8, 2, 3, 0, nil)
	cf.SelectedCards = []*ClientCard{eight}

	if cf.CheckSelectSum() {
		t.Error("和虽然等于 8，但只选了 1 张、不满足 min=2，不该判定为可提交")
	}
}

// 已选组合正好命中目标且张数合规时，CheckSelectSum 返回 true（可以提交）。
func TestCheckSelectSumReadyWhenExact(t *testing.T) {
	a, b := mkSum(4, 0), mkSum(4, 0)
	cf := setupSum(8, 2, 2, 0, nil)
	cf.SelectedCards = []*ClientCard{a, b}

	if !cf.CheckSelectSum() {
		t.Error("4+4=8 且张数 2 落在 [2,2]，应当可以提交")
	}
}

// mode != 0 是「达到即可，但不能多余」：目标 5，已选 3，再加一张 4 → 7 ≥ 5，
// 且去掉那张 4 就只剩 3 < 5，属于「刚好够」，可选。
func TestCheckSelectSumGreaterMode(t *testing.T) {
	picked := mkSum(3, 0)
	four, one := mkSum(4, 0), mkSum(1, 0)
	cf := setupSum(5, 1, 99, 1, []*ClientCard{four, one})
	cf.SelectedCards = []*ClientCard{picked}

	cf.CheckSelectSum()

	if !selectable(cf, four) {
		t.Error("3+4=7 达到目标 5 且去掉它就不够，应当可选")
	}
}

// greater 模式的核心约束是「不能多余」：加上这一张达到目标后，去掉当前最小的那张
// 就必须不够 —— 否则这张是多余的。
//
// 局面：目标 10，已选一张 3（还差 7）。
//   - 加 7 → 10，去掉最小的 3 剩 7 < 10，刚好够 ✅
//   - 加 20 → 23，去掉最小的 3 仍有 20 ≥ 10，那张 3 就成了多余 ❌
//
// 注意已选的和必须**不足**目标，否则 CheckSelectSum 在 SelectSumVal <= sumc 处
// 就提前返回了，根本走不到这段判断（我第一版测试正是栽在这里，破坏了代码也照样绿）。
func TestCheckSelectSumGreaterRejectsRedundant(t *testing.T) {
	picked := mkSum(3, 0)
	just, tooMuch := mkSum(7, 0), mkSum(20, 0)
	cf := setupSum(10, 1, 99, 1, []*ClientCard{just, tooMuch})
	cf.SelectedCards = []*ClientCard{picked}

	cf.CheckSelectSum()

	if !selectable(cf, just) {
		t.Error("3+7=10 刚好达标，应当可选")
	}
	if selectable(cf, tooMuch) {
		t.Error("3+20=23，去掉那张 3 仍有 20 够用 —— 这张 3 成了多余，20 不该可选")
	}
}

// 祭品版：目标是「祭品数落在 [min,max]」而不是等于某值。
func TestCheckSelectTribute(t *testing.T) {
	// 每张提供 1 个祭品，需要 2 个
	a, b, c := mkSum(1, 0), mkSum(1, 0), mkSum(1, 0)
	cf := NewClientField()
	cf.SelectMin, cf.SelectMax = 2, 2
	cf.SelectSumAll = []*ClientCard{a, b, c}

	if cf.CheckSelectTribute() {
		t.Error("一张没选时不该判定为已就绪")
	}
	for _, x := range []*ClientCard{a, b, c} {
		if !selectable(cf, x) {
			t.Error("每张都能参与凑出 2 个祭品，应当可选")
		}
	}

	// 选够 2 张之后应当就绪
	cf2 := NewClientField()
	cf2.SelectMin, cf2.SelectMax = 2, 2
	cf2.SelectSumAll = []*ClientCard{c}
	cf2.SelectedCards = []*ClientCard{a, b}
	if !cf2.CheckSelectTribute() {
		t.Error("已选 2 张各提供 1 个祭品，正好落在 [2,2]，应当就绪")
	}
}

// 强制选中的卡（must_select）不能被取消。
func TestMustSelectCardsNotDeselectable(t *testing.T) {
	must, opt := mkSum(4, 0), mkSum(4, 0)
	cf := setupSum(8, 1, 99, 0, []*ClientCard{opt})
	cf.SelectedCards = []*ClientCard{must}
	cf.MustSelectCount = 1

	cf.CheckSelectSum()

	if must.IsSelectable {
		t.Error("must_select 的卡被标成可点，玩家能把它取消掉")
	}
	if !must.IsSelected {
		t.Error("must_select 的卡应当是选中状态")
	}
}

// select_sum.go 为了不依赖网络包，自己复制了两个消息号。复制就会漂 ——
// 这里钉住它们与 protocol/network 的定义一致。
func TestSelectMsgConstantsMatchProtocol(t *testing.T) {
	if msgSelectTribute != network.MSG_SELECT_TRIBUTE {
		t.Errorf("msgSelectTribute = %d, protocol 里是 %d", msgSelectTribute, network.MSG_SELECT_TRIBUTE)
	}
	if msgSelectSum != network.MSG_SELECT_SUM {
		t.Errorf("msgSelectSum = %d, protocol 里是 %d", msgSelectSum, network.MSG_SELECT_SUM)
	}
}

// 点击要真的驱动重算：选掉一张之后，剩下的可选集合必须跟着变，取消之后又要恢复。
//
// 局面：目标 8，池子 4/4/3。唯一解是 4+4，那张 3 谁都配不上。
//   - 初始：两张 4 可选，3 不可选
//   - 选掉一张 4：还差 4，只剩另一张 4 可选
//   - 取消：回到初始
func TestToggleSumPickRecomputes(t *testing.T) {
	a, b, three := mkSum(4, 0), mkSum(4, 0), mkSum(3, 0)
	cf := setupSum(8, 1, 99, 0, []*ClientCard{a, b, three})
	cf.CheckSelectSum()

	if !selectable(cf, a) || !selectable(cf, b) || selectable(cf, three) {
		t.Fatal("前提不成立：两张 4 应当可选、3 不可选")
	}

	cf.ToggleSumPick(a, msgSelectSum)

	if !a.IsSelected {
		t.Fatal("点击后应当被选中")
	}
	if !selectable(cf, b) {
		t.Error("已选一张 4、还差 4，另一张 4 应当仍可选")
	}
	if selectable(cf, three) {
		t.Error("3 依旧凑不出来，不该可选")
	}

	// 再点一次取消，可选集合应当回到初始状态
	cf.ToggleSumPick(a, msgSelectSum)
	if a.IsSelected {
		t.Error("再次点击应当取消选中")
	}
	if !selectable(cf, a) || !selectable(cf, b) {
		t.Error("取消之后两张 4 都应当重新可选 —— 没有重算")
	}
}

// 强制选中的卡点了也不能取消。
func TestToggleSumPickKeepsMustSelect(t *testing.T) {
	must, opt := mkSum(4, 0), mkSum(4, 0)
	cf := setupSum(8, 1, 99, 0, []*ClientCard{opt})
	cf.SelectedCards = []*ClientCard{must}
	cf.MustSelectCount = 1
	cf.CheckSelectSum()

	cf.ToggleSumPick(must, msgSelectSum)

	if !must.IsSelected || len(cf.SelectedCards) != 1 {
		t.Error("must_select 的卡被点掉了")
	}
}
