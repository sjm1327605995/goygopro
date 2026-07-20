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
					seatButton(),
				),
				ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(2), ui.Padding(8), ui.Width(200)},
					append(infoLines,
						ui.Box([]ui.StyleOpt{ui.Height(8)}),
						ui.Use(deckPicker, struct{}{}),
						readyButton(),
					)...,
				),
			),
			ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(12), ui.Padding(8), ui.JustifyCenter},
				// 只有房主能开始决斗；其他人点了服务端也不认，不如直接禁用。
				lobbyButtonDisabled("开始决斗", !client.Client.IsHost(), func() {
					client.Client.SendPacketToServer(network.CTOS_HS_START)
				}),
				lobbyButton("退出", leaveRoom),
			),
		),
	)
}

// readyButton 是「准备 / 取消准备」的切换。
// 准备之后想换卡组必须能反悔 —— 此前只有单向的准备，按下去就没法回头了。
func readyButton() *ui.Node {
	if client.Client.IsObserver() {
		return nil // 观众没有准备一说
	}
	if selfReady() {
		return lobbyButton("取消准备", func() {
			client.Client.NotReady()
		})
	}
	return lobbyButton("准备", onReady)
}

// seatButton 在决斗席与观众席之间切换。
// 此前只有「旁观」这一个方向，转成观众后就再也回不到决斗席。
func seatButton() *ui.Node {
	if client.Client.IsObserver() {
		return lobbyButton("回到决斗席", func() {
			client.Client.ToDuelist()
		})
	}
	return lobbyButton("旁观", func() {
		client.Client.SendPacketToServer(network.CTOS_HS_TOOBSERVER)
	})
}

// selfReady 是自己当前是否已准备。服务端用 STOC_HS_PLAYER_CHANGE 广播各位置的状态，
// 客户端记在 HostPrepReady 里。
func selfReady() bool {
	pos := int(client.MainGame.DInfo.PlayerType)
	if pos < 0 || pos >= 4 {
		return false
	}
	return client.MainGame.GetHostPrepReady(pos)
}

// leaveRoom 正常离开房间：先告诉服务端，再断连。
//
// 直接 StopClient 是粗暴断连 —— 服务端只能等超时或 TCP 出错才发现，
// 对局中途还会被判成掉线。
func leaveRoom() {
	client.Client.LeaveGame()
	client.Client.StopClient()
	client.PopScene()
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
		pos := uint8(i)
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
			// 踢人只有房主能用，且空位无人可踢。
			ui.If(client.Client.IsHost() && name != "",
				ui.Use(kickButton, kickButtonProps{Pos: pos})),
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

type kickButtonProps struct{ Pos uint8 }

// kickButton 是玩家行末尾的小叉：房主用来把某个位置上的人请出房间。
func kickButton(p kickButtonProps) *ui.Node {
	hovered, _, ia := ui.UseInteraction()
	face := ui.Hex("#c0c0c0")
	if hovered {
		face = cmdAttackColor
	}
	return ui.Button(
		ui.Style(ui.Width(20), ui.Height(20), ui.ItemsCenter, ui.JustifyCenter,
			ui.Bg(face), ui.Border(1, ui.Hex("#6e6e6e")), ui.Radius(2)),
		ui.OnClick(func() { client.Client.Kick(p.Pos) }), ia,
		ui.Text("×", ui.FontSize(12), ui.TextColor(black)),
	)
}

// deckPicker 让玩家在自己的卡组之间切换。
//
// 此前客户端只认死路径 deck/default.ydk —— 玩家没法用自己的牌联机。
// 「准备」会把当前卡组发给服务器（见 onReady），所以这个选择必须在准备之前生效。
func deckPicker(_ struct{}) *ui.Node {
	dm := client.MainGame.DeckMgr
	cfg := &client.MainGame.Config

	// 目录内容在进入大厅时读一次就够，不必每帧扫盘。
	names := ui.UseMemo(func() []string {
		return dm.DeckNames(cfg.LastCategory)
	}, cfg.LastCategory)

	current, setCurrent := ui.UseState(cfg.LastDeck)
	errMsg, setErr := ui.UseState("")

	if len(names) == 0 {
		return ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(2)},
			ui.Text("卡组", ui.FontSize(12), ui.TextColor(black)),
			ui.Text("deck 目录里没有 .ydk", ui.FontSize(11), ui.TextColor(cmdAttackColor)),
		)
	}

	// 配置里记的卡组可能已经被删掉了，回退到第一个。
	idx := 0
	for i, n := range names {
		if n == current {
			idx = i
			break
		}
	}

	pick := func(delta int) {
		next := names[((idx+delta)%len(names)+len(names))%len(names)]
		if err := dm.LoadCurrentDeck(cfg.LastCategory, next); err != nil {
			setErr("载入失败: " + err.Error())
			return
		}
		setErr("")
		setCurrent(next)
	}

	return ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(2)},
		ui.Text("卡组", ui.FontSize(12), ui.TextColor(black)),
		ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(4), ui.ItemsCenter},
			lobbyButton("‹", func() { pick(-1) }),
			ui.Box([]ui.StyleOpt{
				ui.Width(110), ui.Height(22), ui.Bg(white), ui.PaddingXY(4, 0), ui.JustifyCenter,
			},
				ui.Text(names[idx], ui.FontSize(12), ui.TextColor(black)),
			),
			lobbyButton("›", func() { pick(1) }),
		),
		ui.If(errMsg != "", ui.Text(errMsg, ui.FontSize(10), ui.TextColor(cmdAttackColor))),
	)
}
