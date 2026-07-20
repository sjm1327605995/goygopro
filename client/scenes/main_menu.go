package scenes

import (
	"os"

	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

const appVersion = "1.036.2"

// MainMenuScene 是主菜单：一张背景图 + 中间一块带标题栏的面板。
func MainMenuScene(_ struct{}) *ui.Node {
	return bg("textures/bg_menu.jpg", []ui.StyleOpt{ui.ItemsCenter, ui.JustifyCenter},
		ui.Box([]ui.StyleOpt{ui.Width(320), ui.Column},
			ui.Box([]ui.StyleOpt{
				ui.Height(24), ui.Bg(titleBarBlue), ui.JustifyCenter, ui.PaddingXY(8, 0),
			},
				ui.Text("YGOPro Version:"+appVersion, ui.FontSize(13), ui.TextColor(white)),
			),
			ui.Box([]ui.StyleOpt{
				ui.Bg(windowBg), ui.Padding(12), ui.Gap(5), ui.ItemsCenter,
			},
				menuButton("联机模式", func() { client.PushScene("lanWindow") }),
				menuButton("单人模式", nil),
				menuButton("观看录像", func() { client.PushScene("replayList") }),
				menuButton("编辑卡组", func() { client.PushScene("deckEdit") }),
				menuButton("退出", func() { os.Exit(0) }),
			),
		),
	)
}

type menuButtonProps struct {
	Label   string
	OnClick func()
}

// menuButton 是 ygopro 菜单里那种灰底窄按钮。onClick 为 nil 表示功能尚未实现：变灰且不可点。
func menuButton(label string, onClick func()) *ui.Node {
	return ui.Use(menuButtonC, menuButtonProps{Label: label, OnClick: onClick})
}

func menuButtonC(p menuButtonProps) *ui.Node {
	hovered, pressed, ia := ui.UseInteraction()
	enabled := p.OnClick != nil

	face, fg := ui.Hex("#b4b4b4"), black
	switch {
	case !enabled:
		face, fg = ui.Hex("#a8a8a8"), gray
	case pressed:
		face = ui.Hex("#8f8f8f")
	case hovered:
		face = ui.Hex("#c9c9c9")
	}

	style := []ui.StyleOpt{
		ui.Width(280), ui.Height(30), ui.ItemsCenter, ui.JustifyCenter,
		ui.Bg(face), ui.Border(1, ui.Hex("#6e6e6e")), ui.Radius(2),
	}
	label := ui.Text(p.Label, ui.FontSize(14), ui.TextColor(fg))
	if !enabled {
		return ui.Box(style, label)
	}
	return ui.Button(ui.Style(style...), ui.OnClick(p.OnClick), ia, label)
}
