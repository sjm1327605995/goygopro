package scenes

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol/network"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// 这些测试走 tenon 的无头 harness：真实的 reconcile → 布局 → 命中 → 事件路径，不开窗口。
// 决斗盘的点击最终会走到 DuelClient.SendResponse，未连接时 send() 里 conn==nil 直接返回，
// 所以测试里点击是安全的。

func mountField(t *testing.T) *ui.Harness {
	t.Helper()
	resetField()
	return ui.Mount(ui.Use(DuelFieldScene, struct{}{}),
		client.GameWindowWidth, client.GameWindowHeight)
}

func resetField() {
	client.MainGame.DField = client.NewClientField()
	client.MainGame.DInfo = client.DuelInfo{DuelRule: 5}
	client.MainGame.Dialog = client.NewDialogState()
	seedDemoField()
}

// 悬停一张卡应当在左侧弹出大图预览。预览面板画在 3D 场景之外，靠卡把悬停事件报上来，
// 这条链路（卡的 OnHover → 决斗盘的 UseState → cardPreview）容易在重构里断掉。
//
// 场上只摆一张卡：否则「第一个可点击节点」是卡组顶张（Code=0，本就没有预览）。
func TestHoverCardShowsPreview(t *testing.T) {
	client.MainGame.DField = client.NewClientField()
	client.MainGame.DInfo = client.DuelInfo{DuelRule: 5}
	client.MainGame.Dialog = client.NewDialogState()

	df := client.MainGame.DField
	df.MZone[0] = make([]*client.ClientCard, 7)
	card := client.NewClientCard()
	card.Code, card.Controler, card.Location, card.Sequence = 12345, 0, 0x04, 0
	card.Position, card.Type = 0x01, 0x1 // 表侧攻击的怪兽
	card.Attack, card.Defense, card.Level = 2500, 2100, 7
	df.MZone[0][0] = card

	h := ui.Mount(ui.Use(DuelFieldScene, struct{}{}),
		client.GameWindowWidth, client.GameWindowHeight)

	want := fmt.Sprintf("卡密: %d", card.Code)
	if hasText(h, want) {
		t.Fatal("还没悬停就出现了预览")
	}

	cardNode := h.Root().Find(func(q *ui.Query) bool { return q.Clickable() })
	if !cardNode.Exists() {
		t.Fatal("场上找不到可点击的卡")
	}
	cardNode.Hover(true)

	if !hasText(h, want) {
		t.Errorf("悬停后没有出现预览（期望文本 %q）", want)
	}
	if !hasText(h, "攻 2500 / 守 2100") {
		t.Error("预览里没有攻守数值")
	}

	cardNode.Hover(false)
	if hasText(h, want) {
		t.Error("移开后预览没有消失")
	}
}

// 盖着的对手卡不该被预览泄底。
func TestFaceDownOpponentCardHasNoPreview(t *testing.T) {
	resetField()
	card := client.MainGame.DField.MZone[1][0]
	card.Position = 0x08 // 里侧
	h := ui.Mount(ui.Use(DuelFieldScene, struct{}{}),
		client.GameWindowWidth, client.GameWindowHeight)

	if node := preview(h, card); node {
		t.Fatal("前提不成立：初始就有预览")
	}
	// 直接驱动预览组件本身，避免依赖具体是哪个节点被悬停
	if cardPreview(card) != nil {
		t.Error("对手的里侧卡被预览了 —— 等于开挂")
	}
}

// 「选择位置」时点空格子应当把选择发回服务器，并清掉可选标记。
func TestClickSelectablePlaceSendsResponse(t *testing.T) {
	resetField()
	client.MainGame.DInfo.CurMsg = network.MSG_SELECT_PLACE
	// 怪兽区 5 号位（p0）可选：低 5 位对应 p0 的 MZONE
	client.MainGame.DField.SelectableField = 1 << 4

	if !isPlaceSelectable(0, 0x04, 4) {
		t.Fatal("前提不成立：该位置应当可选")
	}
	onEmptySlotClick(0, 0x04, 4)

	if client.MainGame.DField.SelectableField != 0 {
		t.Error("回应之后没有清掉 SelectableField，格子还会亮着")
	}
}

// 卡在区域之间移动时必须是同一个节点（key 跟着 UID 走），否则 FLIP 动画无从谈起 ——
// tenon 会把它当成旧节点消失、新节点出现。
func TestCardKeepsIdentityAcrossMove(t *testing.T) {
	resetField()
	df := client.MainGame.DField

	card := df.Hand[0][0]
	uid := card.UID
	before := fmt.Sprintf("card-%d", uid)

	// 手牌 -> 怪兽区，和 event_handler 的 MSG_MOVE 一样复用同一个对象
	df.RemoveCard(0, 0x02, 0)
	df.AddCard(card, 0, 0x04, 5)

	if card.UID != uid {
		t.Fatalf("移动后 UID 变了：%d -> %d", uid, card.UID)
	}
	after := fmt.Sprintf("card-%d", card.UID)
	if before != after {
		t.Errorf("移动后节点 key 变了：%s -> %s", before, after)
	}
}

// 卡必须正好摆在它那个格子的中心 —— 这是整个伪 3D 决斗盘的地基：格子、地板贴图
// （PlaneImage 按同一投影预变形）和卡三者共用一套桌面坐标，任何一处漂了都会立刻看出来
// 「卡没落在格子里」。摆位走 GetCardLocation、格子走 ZoneCenter，是两条独立代码路径，
// 容易在改动中悄悄分叉，故在此钉死。
func TestCardSitsOnItsZone(t *testing.T) {
	resetField()
	rule := client.FieldRule()

	for _, tc := range []struct {
		name string
		loc  uint8
		list func(p int) []*client.ClientCard
	}{
		{"怪兽区", 0x04, func(p int) []*client.ClientCard { return client.MainGame.DField.MZone[p] }},
		{"魔陷区", 0x08, func(p int) []*client.ClientCard { return client.MainGame.DField.SZone[p] }},
	} {
		for p := 0; p < 2; p++ {
			for seq, card := range tc.list(p) {
				if card == nil {
					continue
				}
				cx, cy, _, _ := client.MainGame.DField.GetCardLocation(card)
				zx, zy := client.ZoneCenter(p, tc.loc, seq, rule)
				if cx != zx || cy != zy {
					t.Errorf("%s p%d seq%d: 卡在 (%.1f,%.1f)，格子中心在 (%.1f,%.1f)",
						tc.name, p, seq, cx, cy, zx, zy)
				}
			}
		}
	}
}

func hasText(h *ui.Harness, want string) bool {
	return h.Root().Find(func(q *ui.Query) bool {
		return strings.Contains(q.Text(), want)
	}).Exists()
}

func preview(h *ui.Harness, card *client.ClientCard) bool {
	return hasText(h, fmt.Sprintf("卡密: %d", card.Code))
}
