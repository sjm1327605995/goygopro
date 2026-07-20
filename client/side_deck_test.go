package client

import (
	"encoding/binary"
	"testing"

	"github.com/sjm1327605995/goygopro/protocol"
)

// 换副卡组只能在三个区之间挪牌，不能凭空增删 —— 卡片总数必须守恒。
func TestSideMoveConservesCards(t *testing.T) {
	dm := &DeckManager{CurrentDeck: Deck{
		Main:  []uint32{1, 2, 3},
		Extra: []uint32{10},
		Side:  []uint32{20, 21},
	}}
	total := func() int {
		d := &dm.CurrentDeck
		return len(d.Main) + len(d.Extra) + len(d.Side)
	}
	before := total()

	if !dm.SideMove(0, 2, 1) { // 主卡组第 2 张 -> 副卡组
		t.Fatal("挪牌应当成功")
	}
	if total() != before {
		t.Errorf("挪牌后总数 = %d, 换牌前 %d —— 卡片凭空增减了", total(), before)
	}
	if len(dm.CurrentDeck.Main) != 2 || len(dm.CurrentDeck.Side) != 3 {
		t.Errorf("主/副张数 = %d/%d, want 2/3", len(dm.CurrentDeck.Main), len(dm.CurrentDeck.Side))
	}
	// 挪的必须是指定那张
	for _, c := range dm.CurrentDeck.Main {
		if c == 2 {
			t.Error("被挪走的卡还留在主卡组里")
		}
	}
	if dm.CurrentDeck.Side[2] != 2 {
		t.Errorf("副卡组末尾 = %d, want 2", dm.CurrentDeck.Side[2])
	}
}

func TestSideMoveRejectsBadInput(t *testing.T) {
	dm := &DeckManager{CurrentDeck: Deck{Main: []uint32{1}}}
	for _, tc := range []struct {
		name            string
		from, to, index int
	}{
		{"同区自挪", 0, 0, 0},
		{"下标越界", 0, 2, 5},
		{"下标为负", 0, 2, -1},
		{"区号非法", 9, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if dm.SideMove(tc.from, tc.to, tc.index) {
				t.Error("非法输入不该挪成功")
			}
			if len(dm.CurrentDeck.Main) != 1 {
				t.Error("非法输入把卡组改坏了")
			}
		})
	}
}

// 三边张数与换牌前不一致时不能提交 —— 服务端 LoadSide 会拒绝，提前挡住并说清楚哪边不对。
func TestSideCountsMatchAndHint(t *testing.T) {
	g := &Game{DeckMgr: &DeckManager{CurrentDeck: Deck{
		Main:  []uint32{1, 2, 3},
		Extra: []uint32{10},
		Side:  []uint32{20},
	}}}
	g.SidePreMain, g.SidePreExtra, g.SidePreSide = 3, 1, 1

	if !g.SideCountsMatch() {
		t.Fatal("张数与换牌前一致，应当可以提交")
	}
	if h := g.SideCountHint(); h != "" {
		t.Errorf("一致时不该有提示，得到 %q", h)
	}

	// 把一张从主卡组挪进副卡组：两边都不对了
	g.DeckMgr.SideMove(0, 2, 0)
	if g.SideCountsMatch() {
		t.Error("主 2/3、副 2/1，不该允许提交")
	}
	hint := g.SideCountHint()
	if hint == "" {
		t.Fatal("张数不对时应当给出提示")
	}
	if want := "主卡组少了 1 张"; hint != want {
		t.Errorf("提示 = %q, want %q", hint, want)
	}
}

// SendUpdateDeck 发出去的字节必须能被服务端的解包器读回来 —— 这是联机的必经一步，
// 格式错了就是「点了准备开不了局」。这里用 protocol 自己的 Unpack 做往返验证。
func TestSendUpdateDeckWireFormat(t *testing.T) {
	deck := &Deck{
		Main:  []uint32{100, 101, 102},
		Extra: []uint32{200},
		Side:  []uint32{300, 301},
	}

	payload := buildUpdateDeckPayload(deck)

	var got protocol.CTOSDeckData
	if _, err := got.Unpack(payload, binary.LittleEndian); err != nil {
		t.Fatalf("服务端解包失败: %v", err)
	}

	// mainc 是「主 + 额外」，服务端再按卡片类型拆回去
	if got.MainC != 4 {
		t.Errorf("MainC = %d, want 4（3 主 + 1 额外）", got.MainC)
	}
	if got.SideC != 2 {
		t.Errorf("SideC = %d, want 2", got.SideC)
	}
	want := []uint32{100, 101, 102, 200, 300, 301}
	if len(got.List) != len(want) {
		t.Fatalf("卡号数量 = %d, want %d", len(got.List), len(want))
	}
	for i := range want {
		if got.List[i] != want[i] {
			t.Errorf("List[%d] = %d, want %d —— 三段顺序应为 主/额外/副", i, got.List[i], want[i])
		}
	}
}

func TestSendUpdateDeckHandlesNil(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("空卡组导致 panic: %v", r)
		}
	}()
	Client.SendUpdateDeck(nil)
}
