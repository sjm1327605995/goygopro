package scenes

import (
	"fmt"
	"image/color"
	"net"
	"strconv"

	"github.com/sjm1327605995/tenon"
	"github.com/sjm1327605995/tenon/pkg/engine"
	"github.com/sjm1327605995/tenon/pkg/widgets"
	"github.com/sjm1327605995/tenon/yoga"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol"
)

// LanWindowScene holds the LAN window state (host list, create/join forms).
var lanWindowScene = &LanWindowScene{
	joinHost:        client.MainGame.Config.LastHost,
	joinPort:        client.MainGame.Config.LastPort,
	nickname:        client.MainGame.Config.Nickname,
	createStartLP:   "8000",
	createStartHand: "5",
	createDrawCount: "1",
	createTimeLimit: "180",
	createGameName:  client.MainGame.Config.GameName,
	createRoomPass:  client.MainGame.Config.RoomPass,
}

// LanWindowScene corresponds to C++ wLanWindow + wCreateHost + wJoinHost
type LanWindowScene struct {
	showCreate bool
	showJoin   bool

	joinHost string
	joinPort string
	nickname string

	createStartLP       string
	createStartHand     string
	createDrawCount     string
	createTimeLimit     string
	createNoCheckDeck   bool
	createNoShuffleDeck bool
	createGameName      string
	createRoomPass      string
	createError         string

	hosts []protocol.HostPacket
}

// LanWindowRoute is the tenon RouteBuilder for the LAN window.
func LanWindowRoute(ctx engine.BuildContext, params engine.RouteParams) engine.Widget {
	if lanWindowScene.showCreate {
		return lanWindowScene.buildCreateHost(ctx)
	}
	if lanWindowScene.showJoin {
		return lanWindowScene.buildJoinHost(ctx)
	}
	return lanWindowScene.buildLanWindow(ctx)
}

// ---------- 联机模式窗口 ----------

func (s *LanWindowScene) buildLanWindow(ctx engine.BuildContext) engine.Widget {
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
						 tenon.Text("联机模式").FontSize(13).Color(white),
					 ).Height(24).Background(titleBarBlue).Padding(0).Width(580),
					 // Nickname row
					 tenon.HStack(
						 tenon.Container(tenon.Text("昵称:").FontSize(12).Color(black)).Width(40),
						 tenon.Input(s.nickname).OnChange(func(v string) {
							 s.nickname = v
							 client.MainGame.Config.Nickname = v
						 }).Width(420).Height(22),
						 tenon.Button("建立主机").Style(tenon.ButtonDefault).H(24).OnClick(func() {
							 s.showCreate = true
						 }),
					 ).Gap(6).AlignItems(tenon.AlignCenter).Padding(8),
					 // Host list
					 s.buildHostList(),
					 // Refresh button
					 tenon.HStack(
						 tenon.Button("刷新主机").Style(tenon.ButtonDefault).H(24).OnClick(func() {
							 s.hosts = client.Client.DiscoverHosts()
						 }),
					 ).Justify(yoga.JustifyCenter).Padding(4),
					 // Bottom area
					 tenon.Container(
						 tenon.HStack(
							 tenon.VStack(
								 tenon.HStack(
									 tenon.Container(tenon.Text("主机信息:").FontSize(12).Color(black)).Width(60),
									 tenon.Input(s.joinHost).OnChange(func(v string) { s.joinHost = v }).Width(200).Height(22),
									 tenon.Input(s.joinPort).OnChange(func(v string) { s.joinPort = v }).Width(60).Height(22),
								 ).Gap(4).AlignItems(tenon.AlignCenter),
								 tenon.HStack(
									 tenon.Container(tenon.Text("主机密码:").FontSize(12).Color(black)).Width(60),
									 tenon.Input("").OnChange(func(v string) { /* TODO */ }).Width(270).Height(22),
								 ).Gap(4).AlignItems(tenon.AlignCenter),
							 ).Gap(4).AlignItems(tenon.AlignStretch),
							 tenon.VStack(
								 tenon.Button("加入游戏").Style(tenon.ButtonDefault).H(24).OnClick(func() { s.onJoinHost() }),
								 tenon.Button("取消").Style(tenon.ButtonDefault).H(24).OnClick(func() {
									 if n := tenon.GetNavigator(ctx); n != nil {
										 n.Pop()
									 }
								 }),
							 ).Gap(4).AlignItems(tenon.AlignStretch),
						 ).Gap(8).Padding(8).AlignItems(tenon.AlignCenter),
					 ).Margin(8),
				 ).Gap(4).AlignItems(tenon.AlignStretch),
			 ).Background(windowBg).Width(580).Height(420),
		).Center(),
	).Width(client.GameWindowWidth).Height(client.GameWindowHeight)
}

func (s *LanWindowScene) buildHostList() tenon.Widget {
	if len(s.hosts) == 0 {
		return tenon.Container(
			 tenon.VStack(
				 tenon.Text("(No hosts found)").FontSize(12).Color(gray),
			 ).Gap(4).AlignItems(tenon.AlignCenter),
		).Height(220).Background(color.RGBA{R: 245, G: 245, B: 245, A: 255}).
			 Border(color.RGBA{R: 170, G: 170, B: 170, A: 255}, 1).
			 Padding(8).Margin(8)
	}
	rows := make([]tenon.Widget, 0, len(s.hosts))
	for _, h := range s.hosts {
		name := client.Utf16ToString(protocol.PackGameMsg(h.Name))
		ip := net.IP{byte(h.IPAddr), byte(h.IPAddr >> 8), byte(h.IPAddr >> 16), byte(h.IPAddr >> 24)}
		info := fmt.Sprintf("%s @ %s:%d | LP:%d Hand:%d Draw:%d",
			name, ip.String(), h.Port, h.Host.StartLp, h.Host.StartHand, h.Host.DrawCount)
		rows = append(rows, tenon.Container(tenon.Text(info).FontSize(11).Color(black)).Padding(4))
	}
	return tenon.Container(
		 tenon.VStack(rows...).Gap(4).AlignItems(tenon.AlignStretch),
	).Height(220).Background(color.RGBA{R: 245, G: 245, B: 245, A: 255}).
		 Border(color.RGBA{R: 170, G: 170, B: 170, A: 255}, 1).
		 Padding(8).Margin(8)
}

// ---------- 建立主机窗口 ----------

func (s *LanWindowScene) buildCreateHost(ctx engine.BuildContext) engine.Widget {
	lfListOpts := []widgets.SelectOption{{Value: "0", Label: "2025.10"}}
	cardAllowOpts := []widgets.SelectOption{{Value: "0", Label: "O C G"}}
	duelModeOpts := []widgets.SelectOption{
		{Value: "0", Label: "单局模式"},
		{Value: "1", Label: "比赛模式"},
		{Value: "2", Label: "Tag模式"},
	}
	ruleOpts := []widgets.SelectOption{
		{Value: "5", Label: "大师规则（2020）"},
	}

	return tenon.Stack(
		 tenon.Positioned(
			 tenon.Image(mainMenuScene.bg).Fit(tenon.ObjectFitCover),
		).L(0).T(0).R(0).B(0),
		 tenon.Positioned(
			 tenon.Container(
				 tenon.VStack(
					 // Title
					 tenon.Container(
						 tenon.Text("建立主机").FontSize(13).Color(white),
					 ).Height(24).Background(titleBarBlue).Padding(0).Width(400),
					 // Form rows (scrollable)
					 tenon.Scroll(
						 tenon.VStack(
							 s.createRow("禁限卡表:", tenon.Select(lfListOpts).Width(200).WithValue("0")),
							 s.createRow("卡片允许:", tenon.Select(cardAllowOpts).Width(200).WithValue("0")),
							 s.createRow("决斗模式:", tenon.Select(duelModeOpts).Width(200).WithValue("0")),
							 s.createRow("每回合时间:", tenon.Input(s.createTimeLimit).OnChange(func(v string) { s.createTimeLimit = v }).Width(60).Height(22)),
							 tenon.Text("↓额外选项（无特殊要求请勿修改）").FontSize(11).Color(gray),
							 s.createRow("规则:", tenon.Select(ruleOpts).Width(200).WithValue("5")),
							 // Checkboxes
							 tenon.HStack(
								 tenon.Button("☐ 不检查卡组").Style(tenon.ButtonGhost).OnClick(func() {
									 s.createNoCheckDeck = !s.createNoCheckDeck
								 }),
								 tenon.Button("☐ 不洗切卡组").Style(tenon.ButtonGhost).OnClick(func() {
									 s.createNoShuffleDeck = !s.createNoShuffleDeck
								 }),
							 ).Gap(12).Padding(4),
							 s.createRow("初始基本分:", tenon.Input(s.createStartLP).OnChange(func(v string) { s.createStartLP = v }).Width(60).Height(22)),
							 s.createRow("初始手卡数:", tenon.Input(s.createStartHand).OnChange(func(v string) { s.createStartHand = v }).Width(60).Height(22)),
							 s.createRow("每回合抽卡:", tenon.Input(s.createDrawCount).OnChange(func(v string) { s.createDrawCount = v }).Width(60).Height(22)),
						 ).Gap(4).Padding(10).AlignItems(tenon.AlignStretch),
					 ).MaxHeight(280),
					 // Bottom: host name + password + buttons
					 tenon.HStack(
						 tenon.VStack(
							 s.createRow("主机名称:", tenon.Input(s.createGameName).OnChange(func(v string) { s.createGameName = v }).Width(180).Height(22)),
							 s.createRow("主机密码:", tenon.Input(s.createRoomPass).OnChange(func(v string) { s.createRoomPass = v }).Width(180).Height(22)),
						 ).Gap(4).AlignItems(tenon.AlignStretch),
						 tenon.VStack(
							 tenon.Button("确定").Style(tenon.ButtonDefault).OnClick(func() { s.onCreateHost() }),
							 tenon.Button("取消").Style(tenon.ButtonDefault).OnClick(func() { s.showCreate = false; s.createError = "" }),
						 ).Gap(4).AlignItems(tenon.AlignStretch),
					 ).Gap(12).AlignItems(tenon.AlignCenter).Padding(10),
					 s.buildErrorLabel(),
				 ).Gap(0).AlignItems(tenon.AlignStretch),
			 ).Background(windowBg).Width(400).Height(420),
		).Center(),
	).Width(client.GameWindowWidth).Height(client.GameWindowHeight)
}

func (s *LanWindowScene) buildErrorLabel() tenon.Widget {
	if s.createError != "" {
		return tenon.Container(
			tenon.Text(s.createError).FontSize(11).Color(color.RGBA{R: 200, G: 50, B: 50, A: 255}),
		).Padding(4)
	}
	return tenon.Container(tenon.Text("").FontSize(11)).Padding(4)
}

func (s *LanWindowScene) createRow(label string, widget tenon.Widget) tenon.Widget {
	return tenon.HStack(
		 tenon.Container(tenon.Text(label).FontSize(12).Color(black)).Width(90),
		 widget,
	).Gap(6).AlignItems(tenon.AlignCenter)
}

// ---------- 加入主机窗口 ----------

func (s *LanWindowScene) buildJoinHost(ctx engine.BuildContext) engine.Widget {
	return tenon.Stack(
		 tenon.Positioned(
			 tenon.Image(mainMenuScene.bg).Fit(tenon.ObjectFitCover),
		).L(0).T(0).R(0).B(0),
		 tenon.Positioned(
			 tenon.Container(
				 tenon.VStack(
					 tenon.Container(
						 tenon.Text("加入主机").FontSize(13).Color(white),
					 ).Height(24).Background(titleBarBlue).Padding(0).Width(300),
					 tenon.VStack(
						 s.createRow("主机IP:", tenon.Input(s.joinHost).OnChange(func(v string) { s.joinHost = v }).Width(180).Height(22)),
						 s.createRow("端口:", tenon.Input(s.joinPort).OnChange(func(v string) { s.joinPort = v }).Width(100).Height(22)),
						 tenon.HStack(
							 tenon.Button("加入").Style(tenon.ButtonDefault).OnClick(func() { s.onJoinHost() }),
							 tenon.Button("取消").Style(tenon.ButtonDefault).OnClick(func() { s.showJoin = false }),
						 ).Gap(12).Justify(yoga.JustifyCenter),
					 ).Gap(8).Padding(16).AlignItems(tenon.AlignStretch),
				 ).Gap(0).AlignItems(tenon.AlignStretch),
			 ).Background(windowBg).Width(300).Height(160),
		).Center(),
	).Width(client.GameWindowWidth).Height(client.GameWindowHeight)
}

// ---------- Actions ----------

func (s *LanWindowScene) onCreateHost() {
	lp, _ := strconv.Atoi(s.createStartLP)
	hand, _ := strconv.Atoi(s.createStartHand)
	draw, _ := strconv.Atoi(s.createDrawCount)
	time, _ := strconv.Atoi(s.createTimeLimit)

	info := protocol.HostInfo{
		StartLp:       int32(lp),
		StartHand:     uint8(hand),
		DrawCount:     uint8(draw),
		TimeLimit:     uint16(time),
		NoCheckDeck:   boolToInt8(s.createNoCheckDeck),
		NoShuffleDeck: boolToInt8(s.createNoShuffleDeck),
	}
	client.MainGame.HostInfo = info
	client.MainGame.Config.GameName = s.createGameName
	client.MainGame.Config.RoomPass = s.createRoomPass

	port := client.MainGame.Config.ServerPort
	if err := client.StartLocalServer(port); err != nil {
		s.createError = err.Error()
		fmt.Println("创建主机失败:", err)
		return
	}
	s.createError = ""
	if client.Client.StartClient("127.0.0.1", port, true) {
		client.PushScene("lobby")
	} else {
		s.createError = "无法连接到本地服务器"
		fmt.Println("StartClient 返回 false")
	}
}

func (s *LanWindowScene) onJoinHost() {
	client.MainGame.Config.LastHost = s.joinHost
	client.MainGame.Config.LastPort = s.joinPort

	port, _ := strconv.Atoi(s.joinPort)
	if port == 0 {
		port = 7911
	}
	if client.Client.StartClient(s.joinHost, uint16(port), false) {
		client.PushScene("lobby")
	} else {
		fmt.Println("Failed to connect")
	}
}

func boolToInt8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

var (
	titleBarBlue = color.RGBA{R: 42, G: 74, B: 122, A: 255}
	windowBg     = color.RGBA{R: 208, G: 208, B: 208, A: 255}
)
