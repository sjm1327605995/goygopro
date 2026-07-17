package scenes

import (
	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// App 是根组件：订阅场景名，切换到对应场景。
//
// 这里没有 cmd/client 里那套手写的 rootWidget —— 挂载/卸载/失效重绘由 tenon 的
// reconcile 负责，场景切换就是换一个返回值。
func App(_ struct{}) *ui.Node {
	scene := UseStore(client.MainGame.SceneSignal)

	switch scene {
	case "lanWindow":
		return ui.Use(LanWindowScene, struct{}{})
	case "lobby":
		return ui.Use(LobbyScene, struct{}{})
	case "duelField":
		return ui.Use(DuelFieldScene, struct{}{})
	case "deckEdit":
		return ui.Use(DeckEditScene, struct{}{})
	default:
		return ui.Use(MainMenuScene, struct{}{})
	}
}

// bg 铺一张整屏背景图，内容压在它上面 —— ygopro 每个界面都是这个结构。
// opts 是内容层的样式（排布方式由各场景自己定）。
func bg(path string, opts []ui.StyleOpt, kids ...*ui.Node) *ui.Node {
	return ui.Box([]ui.StyleOpt{ui.Fill},
		ui.Img(ui.Src(path), ui.Fit(ui.FitCover),
			ui.Style(ui.Absolute, ui.Top(0), ui.Left(0), ui.Fill)),
		ui.Box(append([]ui.StyleOpt{ui.Fill}, opts...), kids...),
	)
}
