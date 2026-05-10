package scenes

import (
	"fmt"

	"github.com/sjm1327605995/tenon"
	"github.com/sjm1327605995/tenon/pkg/engine"
	"github.com/sjm1327605995/tenon/pkg/widgets"
	"github.com/sjm1327605995/tenon/yoga"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// LobbyScene holds the duel room state.
var lobbyScene = &LobbyScene{}

// LobbyScene corresponds to C++ handshake lobby (wHostPrepare)
type LobbyScene struct{}

// LobbyRoute is the tenon RouteBuilder for the lobby.
func LobbyRoute(ctx engine.BuildContext, params engine.RouteParams) engine.Widget {
	return lobbyScene.build(ctx)
}

func (s *LobbyScene) build(ctx engine.BuildContext) engine.Widget {
	nav := tenon.GetNavigator(ctx)

	// Room info strings
	hostInfo := client.MainGame.HostInfo
	roomInfo := []string{
		fmt.Sprintf("禁限卡表: %d", hostInfo.LFList),
		fmt.Sprintf("卡片允许: O C G"),
		fmt.Sprintf("决斗模式: %d", hostInfo.Mode),
		fmt.Sprintf("每回合时间: %d", hostInfo.TimeLimit),
		"==========",
		fmt.Sprintf("初始基本分: %d", hostInfo.StartLp),
		fmt.Sprintf("初始手卡数: %d", hostInfo.StartHand),
		fmt.Sprintf("每回合抽卡: %d", hostInfo.DrawCount),
	}

	// Deck categories (placeholder)
	deckCategories := []widgets.SelectOption{{Value: "none", Label: "未分类卡组"}}
	deckOptions := []widgets.SelectOption{{Value: "starter", Label: "A Starter Deck"}}

	return tenon.Stack(
		// Background
			 tenon.Positioned(
					 tenon.Image(mainMenuScene.bg).Fit(tenon.ObjectFitCover),
			).L(0).T(0).R(0).B(0),
		// Centered window
			 tenon.Positioned(
					 tenon.Container(
						 tenon.VStack(
							 // Title bar
							 tenon.Container(
								 tenon.Text("决斗准备").FontSize(13).Color(white),
							 ).Height(24).Background(titleBarBlue).Padding(0).Width(480),
							 // Top area: left (players) + right (info)
							 tenon.HStack(
								 // Left: duelist list
								 tenon.VStack(
									 tenon.Text("决斗者").FontSize(12).Color(black),
									 s.buildPlayerRows(ctx),
									 tenon.Button("观战").Style(tenon.ButtonDefault).OnClick(func() {
										 client.Client.SendPacketToServer(network.CTOS_HS_TOOBSERVER)
									 }),
								 ).Gap(4).Padding(8),
								 // Right: room info
								 tenon.VStack(
									 s.buildInfoLines(roomInfo),
									 tenon.Container(tenon.Spacer()).Height(8),
									 tenon.Button("准备").Style(tenon.ButtonDefault).OnClick(func() {
										 client.Client.SendPacketToServer(network.CTOS_HS_READY)
									 }),
								 ).Gap(2).Padding(8),
							 ).Gap(0).AlignItems(tenon.AlignStretch),
							 // Deck selection
							 tenon.HStack(
								 tenon.Container(tenon.Text("卡组选择:").FontSize(12).Color(black)).Width(60),
								 tenon.Select(deckCategories).Width(120).WithValue("none"),
								 tenon.Select(deckOptions).Width(160).WithValue("starter"),
							 ).Gap(6).Padding(8).AlignItems(tenon.AlignCenter),
							 // Bottom buttons
							 tenon.HStack(
								 tenon.Button("开始").Style(tenon.ButtonDefault).OnClick(func() {
									 client.Client.SendPacketToServer(network.CTOS_HS_START)
								 }),
								 tenon.Button("退出").Style(tenon.ButtonDefault).OnClick(func() {
									 client.Client.StopClient()
									 if nav != nil { nav.Pop() }
								 }),
							 ).Gap(12).Justify(yoga.JustifyCenter).Padding(8),
						 ).Gap(0).AlignItems(tenon.AlignStretch),
					 ).Background(windowBg).Width(480).Height(360),
			).Center(),
	).Width(client.GameWindowWidth).Height(client.GameWindowHeight)
}

func (s *LobbyScene) buildPlayerRows(ctx engine.BuildContext) tenon.Widget {
	rows := make([]tenon.Widget, 0, 4)
	for i := 0; i < 4; i++ {
		name := client.MainGame.GetHostPrepName(i)
		if name == "" {
			name = ""
		}
		ready := client.MainGame.GetHostPrepReady(i)
		_ = ready
		rows = append(rows, tenon.HStack(
			 tenon.Button("X").Style(tenon.ButtonGhost).OnClick(func() {
				 // TODO: kick player
			 }),
			 tenon.Input(name).Width(140).Height(20),
			 tenon.Input("").Width(20).Height(20),
		).Gap(4).AlignItems(tenon.AlignCenter))
	}
	return tenon.VStack(rows...).Gap(4).AlignItems(tenon.AlignStretch)
}

func (s *LobbyScene) buildInfoLines(lines []string) tenon.Widget {
	widgets_ := make([]tenon.Widget, 0, len(lines))
	for _, line := range lines {
		widgets_ = append(widgets_, tenon.Text(line).FontSize(12).Color(black))
	}
	return tenon.VStack(widgets_...).Gap(2).AlignItems(tenon.AlignCenter)
}
