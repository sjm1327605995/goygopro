package scenes

import (
	"strings"
	"testing"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol/network"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// 猜拳和先后攻发生在**大厅**：服务端在双方准备好之后立刻发 STOC_SELECT_HAND /
// STOC_SELECT_TP，而客户端要等 MSG_START 才切到决斗盘。
//
// 对话框曾经只挂在决斗盘里，于是这两步的弹窗无处显示 —— 玩家点不了，决斗永远开不了局。
// 现在它挂在 App 根上，这个测试钉住「任何场景都能弹出对话框」。
func TestDialogShowsOutsideDuelField(t *testing.T) {
	for _, scene := range []string{"mainMenu", "lobby", "lanWindow", "duelField"} {
		t.Run(scene, func(t *testing.T) {
			client.MainGame.DField = client.NewClientField()
			// CurMsg 决定选项的文案：猜拳时是石头/剪刀/布，否则只是「选项 N」
			client.MainGame.DInfo = client.DuelInfo{
				DuelRule: 5, CurMsg: network.MSG_ROCK_PAPER_SCISSORS,
			}
			client.MainGame.Dialog = client.NewDialogState()
			client.MainGame.SceneSignal.Set(scene)

			// 猜拳：石头/剪刀/布
			client.MainGame.Dialog.ShowOption([]int32{1, 2, 3})

			h := ui.Mount(ui.Use(App, struct{}{}),
				client.GameWindowWidth, client.GameWindowHeight)

			if !dialogVisible(h, "石头") {
				t.Errorf("%s 场景下猜拳对话框没有显示 —— 玩家无法猜拳，决斗开不了局", scene)
			}
		})
	}
	client.MainGame.SceneSignal.Set("mainMenu")
	client.MainGame.Dialog.Hide()
}

// 对话框在 Portal 里（顶层浮层），要从 Overlays 查而不是主树。
func dialogVisible(h *ui.Harness, want string) bool {
	for _, q := range h.Overlays() {
		if strings.Contains(q.AllText(), want) {
			return true
		}
	}
	return false
}
