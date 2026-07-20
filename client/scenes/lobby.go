package scenes

import (
	"fmt"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol/network"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// LobbyScene 是决斗准备房间。玩家列表由服务器推送更新，所以订阅 FieldRev：
// 旧实现只在进场景时构建一次，别人进房间/准备好了界面都不会动。
func LobbyScene(_ struct{}) *ui.Node {
	_ = UseRevision(client.MainGame.FieldRev)
	host := client.MainGame.HostInfo

	info := []string{
		fmt.Sprintf("禁卡表: %d", host.LFList),
		"卡池: OCG",
		fmt.Sprintf("决斗模式: %d", host.Mode),
		fmt.Sprintf("时间限制: %d", host.TimeLimit),
		"==========",
		fmt.Sprintf("初始 LP: %d", host.StartLp),
		fmt.Sprintf("初始手牌: %d", host.StartHand),
		fmt.Sprintf("每回合抽卡: %d", host.DrawCount),
	}
	infoLines := make([]*ui.Node, 0, len(info))
	for _, line := range info {
		infoLines = append(infoLines, ui.Text(line, ui.FontSize(12), ui.TextColor(black)))
	}

	return bg("textures/bg_menu.jpg", []ui.StyleOpt{ui.ItemsCenter, ui.JustifyCenter},
		ui.Box([]ui.StyleOpt{ui.Width(480), ui.Height(360), ui.Column, ui.Bg(windowBg)},
			ui.Box([]ui.StyleOpt{ui.Height(24), ui.Bg(titleBarBlue), ui.JustifyCenter, ui.PaddingXY(8, 0)},
				ui.Text("决斗准备", ui.FontSize(13), ui.TextColor(white)),
			),
			ui.Box([]ui.StyleOpt{ui.Row, ui.Grow(1)},
				ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(4), ui.Padding(8), ui.Grow(1)},
					ui.Text("决斗者", ui.FontSize(12), ui.TextColor(black)),
					playerRows(),
					lobbyButton("旁观", func() {
						client.Client.SendPacketToServer(network.CTOS_HS_TOOBSERVER)
					}),
				),
				ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(2), ui.Padding(8), ui.Width(200)},
					append(infoLines,
						ui.Box([]ui.StyleOpt{ui.Height(8)}),
						lobbyButton("准备", onReady),
					)...,
				),
			),
			ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(12), ui.Padding(8), ui.JustifyCenter},
				lobbyButton("开始决斗", func() {
					client.Client.SendPacketToServer(network.CTOS_HS_START)
				}),
				lobbyButton("退出", func() {
					client.Client.StopClient()
					client.PopScene()
				}),
			),
		),
	)
}

// onReady 点「准备」：先把卡组发给服务器，再发 READY。
//
// 顺序不能反，也不能只发 READY —— 服务器要靠 CTOS_UPDATE_DECK 拿到卡组才能校验并开局
// （core/duel 的 LoadDeck + CheckDeck）。这一步此前完全缺失，点了准备也开不了局。
// 对应 C++ menu_handler.cpp：UpdateDeck() 紧接着 SendPacketToServer(CTOS_HS_READY)。
func onReady() {
	client.Client.SendUpdateDeck(&client.MainGame.DeckMgr.CurrentDeck)
	client.Client.SendPacketToServer(network.CTOS_HS_READY)
}

func playerRows() *ui.Node {
	rows := make([]*ui.Node, 0, 4)
	for i := 0; i < 4; i++ {
		name := client.MainGame.GetHostPrepName(i)
		ready := client.MainGame.GetHostPrepReady(i)
		readyMark, readyColor := "", cardEmptyBg
		if ready {
			readyMark, readyColor = "✓", cmdSummonColor
		}
		rows = append(rows, ui.Keyed(fmt.Sprint(i), ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(4), ui.ItemsCenter},
			ui.Box([]ui.StyleOpt{
				ui.Width(140), ui.Height(20), ui.Bg(white), ui.PaddingXY(4, 0), ui.JustifyCenter,
			},
				ui.Text(name, ui.FontSize(12), ui.TextColor(black)),
			),
			ui.Box([]ui.StyleOpt{
				ui.Width(20), ui.Height(20), ui.Bg(readyColor), ui.ItemsCenter, ui.JustifyCenter,
			},
				ui.Text(readyMark, ui.FontSize(12), ui.TextColor(white)),
			),
		)))
	}
	return ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(4)}, rows...)
}

type lobbyButtonProps struct {
	Label    string
	OnClick  func()
	Disabled bool
}

func lobbyButton(label string, onClick func()) *ui.Node {
	return ui.Use(lobbyButtonC, lobbyButtonProps{Label: label, OnClick: onClick})
}

// lobbyButtonDisabled 是带禁用态的版本：disabled 时变灰且点不动。
// 用在「当前选择还不合法」这类地方 —— 让按钮点了没反应，不如直接告诉玩家不能点。
func lobbyButtonDisabled(label string, disabled bool, onClick func()) *ui.Node {
	return ui.Use(lobbyButtonC, lobbyButtonProps{Label: label, OnClick: onClick, Disabled: disabled})
}

func lobbyButtonC(p lobbyButtonProps) *ui.Node {
	hovered, pressed, ia := ui.UseInteraction()
	face, fg := ui.Hex("#b4b4b4"), black
	switch {
	case p.Disabled:
		face, fg = ui.Hex("#a8a8a8"), gray
	case pressed:
		face = ui.Hex("#8f8f8f")
	case hovered:
		face = ui.Hex("#c9c9c9")
	}
	style := ui.Style(ui.Height(24), ui.PaddingXY(12, 0), ui.ItemsCenter, ui.JustifyCenter,
		ui.Bg(face), ui.Border(1, ui.Hex("#6e6e6e")), ui.Radius(2))
	label := ui.Text(p.Label, ui.FontSize(12), ui.TextColor(fg))
	if p.Disabled {
		return ui.Box([]ui.StyleOpt{ui.Height(24), ui.PaddingXY(12, 0), ui.ItemsCenter,
			ui.JustifyCenter, ui.Bg(face), ui.Border(1, ui.Hex("#6e6e6e")), ui.Radius(2)}, label)
	}
	return ui.Button(style, ui.OnClick(p.OnClick), ia, label)
}
