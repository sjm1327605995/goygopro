package scenes

import (
	"fmt"
	"net"
	"strconv"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// LanWindowScene 是联机模式界面：主机列表 / 建立主机 / 加入主机三态。
//
// 旧实现里这些「输入框」是白底 Box 里塞一行 Text —— 看着像输入框，其实敲不进字，
// 昵称和 IP 只能改配置文件。这里用真的 Input。
func LanWindowScene(_ struct{}) *ui.Node {
	mode, setMode := ui.UseState("list")

	switch mode {
	case "create":
		return ui.Use(createHostView, modeProps{SetMode: setMode})
	case "join":
		return ui.Use(joinHostView, modeProps{SetMode: setMode})
	default:
		return ui.Use(hostListView, modeProps{SetMode: setMode})
	}
}

type modeProps struct{ SetMode func(string) }

func hostListView(p modeProps) *ui.Node {
	cfg := &client.MainGame.Config
	nickname, setNickname := ui.UseState(cfg.Nickname)
	host, setHost := ui.UseState(cfg.LastHost)
	port, setPort := ui.UseState(cfg.LastPort)
	hosts, setHosts := ui.UseState([]protocol.HostPacket(nil))
	errMsg, setErr := ui.UseState("")

	join := func() {
		cfg.Nickname, cfg.LastHost, cfg.LastPort = nickname, host, port
		if err := joinHost(host, port); err != nil {
			setErr(err.Error())
		}
	}

	return bg("textures/bg_menu.jpg", []ui.StyleOpt{ui.ItemsCenter, ui.JustifyCenter},
		window("联机模式", 580,
			ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(6), ui.ItemsCenter, ui.Padding(8)},
				fieldLabel("昵称:", 40),
				textInput(nickname, setNickname, "", 400),
				lobbyButton("建立主机", func() { p.SetMode("create") }),
			),
			hostList(hosts, func(h protocol.HostPacket) {
				setHost(hostIP(h).String())
				setPort(strconv.Itoa(int(h.Port)))
			}),
			ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(8), ui.Padding(8), ui.ItemsCenter},
				lobbyButton("刷新主机", func() { setHosts(client.Client.DiscoverHosts()) }),
				fieldLabel("主机:", 40),
				textInputSubmit(host, setHost, join, "127.0.0.1", 180),
				textInputSubmit(port, setPort, join, "7911", 60),
				lobbyButton("加入游戏", join),
				lobbyButton("取消", func() { client.PopScene() }),
			),
			errorLabel(errMsg),
		),
	)
}

func hostList(hosts []protocol.HostPacket, onPick func(protocol.HostPacket)) *ui.Node {
	body := []*ui.Node{}
	if len(hosts) == 0 {
		body = append(body, ui.Text("（未发现主机，点「刷新主机」搜索局域网）",
			ui.FontSize(12), ui.TextColor(gray)))
	}
	for i, h := range hosts {
		h := h
		name := client.Utf16ToString(protocol.PackGameMsg(h.Name))
		info := fmt.Sprintf("%s @ %s:%d | LP:%d 手牌:%d 抽卡:%d",
			name, hostIP(h).String(), h.Port, h.Host.StartLp, h.Host.StartHand, h.Host.DrawCount)
		body = append(body, ui.Keyed(strconv.Itoa(i), ui.Use(hostRow, hostRowProps{
			Info:   info,
			OnPick: func() { onPick(h) },
		})))
	}
	return ui.ScrollView(
		ui.Style(ui.Height(220), ui.MarginXY(8, 0), ui.Padding(8), ui.Column, ui.Gap(2),
			ui.Bg(ui.Hex("#f5f5f5")), ui.Border(1, ui.Hex("#ababab"))),
		ui.Fragment(body...),
	)
}

type hostRowProps struct {
	Info   string
	OnPick func()
}

// hostRow 点一下把该主机填进下方的地址栏 —— 旧实现里列表纯是摆设，看到了也只能手抄 IP。
func hostRow(p hostRowProps) *ui.Node {
	hovered, _, ia := ui.UseInteraction()
	face := ui.Hex("#00000000")
	if hovered {
		face = ui.Hex("#c9dcf5")
	}
	return ui.Button(
		ui.Style(ui.Padding(4), ui.Bg(face), ui.Radius(2)),
		ui.OnClick(p.OnPick), ia,
		ui.Text(p.Info, ui.FontSize(11), ui.TextColor(black)),
	)
}

func createHostView(p modeProps) *ui.Node {
	cfg := &client.MainGame.Config
	startLP, setStartLP := ui.UseState("8000")
	startHand, setStartHand := ui.UseState("5")
	drawCount, setDrawCount := ui.UseState("1")
	timeLimit, setTimeLimit := ui.UseState("180")
	gameName, setGameName := ui.UseState(cfg.GameName)
	roomPass, setRoomPass := ui.UseState(cfg.RoomPass)
	noCheck, setNoCheck := ui.UseState(false)
	noShuffle, setNoShuffle := ui.UseState(false)
	errMsg, setErr := ui.UseState("")

	create := func() {
		client.MainGame.HostInfo = protocol.HostInfo{
			StartLp:       int32(atoiOr(startLP, 8000)),
			StartHand:     uint8(atoiOr(startHand, 5)),
			DrawCount:     uint8(atoiOr(drawCount, 1)),
			TimeLimit:     uint16(atoiOr(timeLimit, 180)),
			NoCheckDeck:   boolToUint8(noCheck),
			NoShuffleDeck: boolToUint8(noShuffle),
		}
		cfg.GameName, cfg.RoomPass = gameName, roomPass
		if err := createHost(cfg.ServerPort); err != nil {
			setErr(err.Error())
			return
		}
	}

	return bg("textures/bg_menu.jpg", []ui.StyleOpt{ui.ItemsCenter, ui.JustifyCenter},
		window("建立主机", 400,
			ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(4), ui.Padding(10)},
				formRow("禁限卡表:", staticField("2025.10", 200)),
				formRow("卡片允许:", staticField("OCG", 200)),
				formRow("决斗模式:", staticField("单局模式", 200)),
				formRow("每回合时间:", textInput(timeLimit, setTimeLimit, "180", 60)),
				formRow("规则:", staticField("Master Rule 2020", 200)),
				ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(16), ui.Padding(4), ui.ItemsCenter},
					checkRow("不检查卡组", noCheck, setNoCheck),
					checkRow("不洗卡组", noShuffle, setNoShuffle),
				),
				formRow("初始 LP:", textInput(startLP, setStartLP, "8000", 60)),
				formRow("初始手牌:", textInput(startHand, setStartHand, "5", 60)),
				formRow("每回合抽卡:", textInput(drawCount, setDrawCount, "1", 60)),
				formRow("房间名:", textInput(gameName, setGameName, "", 180)),
				formRow("房间密码:", textInput(roomPass, setRoomPass, "", 180)),
			),
			ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(12), ui.Padding(10), ui.JustifyCenter},
				lobbyButton("确定", create),
				lobbyButton("取消", func() { p.SetMode("list") }),
			),
			errorLabel(errMsg),
		),
	)
}

func joinHostView(p modeProps) *ui.Node {
	cfg := &client.MainGame.Config
	host, setHost := ui.UseState(cfg.LastHost)
	port, setPort := ui.UseState(cfg.LastPort)
	errMsg, setErr := ui.UseState("")

	join := func() {
		cfg.LastHost, cfg.LastPort = host, port
		if err := joinHost(host, port); err != nil {
			setErr(err.Error())
		}
	}

	return bg("textures/bg_menu.jpg", []ui.StyleOpt{ui.ItemsCenter, ui.JustifyCenter},
		window("加入主机", 300,
			ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(8), ui.Padding(16)},
				formRow("主机 IP:", textInputSubmit(host, setHost, join, "127.0.0.1", 180)),
				formRow("端口:", textInputSubmit(port, setPort, join, "7911", 100)),
				ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(12), ui.JustifyCenter},
					lobbyButton("加入", join),
					lobbyButton("取消", func() { p.SetMode("list") }),
				),
				errorLabel(errMsg),
			),
		),
	)
}

// ---- 动作 ----

func createHost(port uint16) error {
	if err := client.StartLocalServer(port); err != nil {
		return fmt.Errorf("创建主机失败: %w", err)
	}
	if !client.Client.StartClient("127.0.0.1", port, true) {
		return fmt.Errorf("无法连接到本地服务器")
	}
	client.PushScene("lobby")
	return nil
}

func joinHost(host, port string) error {
	p := atoiOr(port, 7911)
	if !client.Client.StartClient(host, uint16(p), false) {
		return fmt.Errorf("连接 %s:%d 失败", host, p)
	}
	client.PushScene("lobby")
	return nil
}

// ---- 通用小部件 ----

// window 是 ygopro 那种带蓝色标题栏的灰色窗口。
func window(title string, width float32, kids ...*ui.Node) *ui.Node {
	return ui.Box([]ui.StyleOpt{ui.Width(width), ui.Column, ui.Bg(windowBg)},
		ui.Box([]ui.StyleOpt{ui.Height(24), ui.Bg(titleBarBlue), ui.JustifyCenter, ui.PaddingXY(8, 0)},
			ui.Text(title, ui.FontSize(13), ui.TextColor(white)),
		),
		ui.Fragment(kids...),
	)
}

func fieldLabel(text string, width float32) *ui.Node {
	return ui.Box([]ui.StyleOpt{ui.Width(width), ui.JustifyCenter},
		ui.Text(text, ui.FontSize(12), ui.TextColor(black)),
	)
}

func formRow(label string, field *ui.Node) *ui.Node {
	return ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(6), ui.ItemsCenter},
		fieldLabel(label, 90), field,
	)
}

func textInput(value string, onChange func(string), placeholder string, width float32) *ui.Node {
	return textInputSubmit(value, onChange, nil, placeholder, width)
}

// textInputSubmit 是带回车提交的输入框。onSubmit 为 nil 时回车无动作。
func textInputSubmit(value string, onChange func(string), onSubmit func(), placeholder string, width float32) *ui.Node {
	attrs := []*ui.Node{
		ui.Value(value), ui.OnChange(onChange), ui.Placeholder(placeholder),
		ui.Style(ui.Width(width), ui.Height(22), ui.PaddingXY(4, 0), ui.Bg(white),
			ui.Border(1, ui.Hex("#8c8c8c")), ui.FontSize(12), ui.TextColor(black)),
	}
	if onSubmit != nil {
		attrs = append(attrs, ui.OnSubmit(func(string) { onSubmit() }))
	}
	return ui.Input(attrs...)
}

// staticField 是尚未做成下拉框的只读格子（禁卡表、规则等需要选项列表，等做到再换）。
func staticField(text string, width float32) *ui.Node {
	return ui.Box([]ui.StyleOpt{ui.Width(width), ui.Height(22), ui.PaddingXY(4, 0),
		ui.JustifyCenter, ui.Bg(ui.Hex("#ebebeb")), ui.Border(1, ui.Hex("#b0b0b0"))},
		ui.Text(text, ui.FontSize(12), ui.TextColor(black)),
	)
}

// checkRow 是一行复选框。没用 pkg/shadcn 的 Checkbox：那套是现代扁平风，
// 跟 ygopro 的 Win32 味界面放一起很突兀。
func checkRow(label string, checked bool, onChange func(bool)) *ui.Node {
	mark := ""
	if checked {
		mark = "✓"
	}
	return ui.Button(
		ui.Style(ui.Row, ui.Gap(6), ui.ItemsCenter),
		ui.OnClick(func() { onChange(!checked) }),
		ui.Box([]ui.StyleOpt{
			ui.Width(14), ui.Height(14), ui.ItemsCenter, ui.JustifyCenter,
			ui.Bg(white), ui.Border(1, ui.Hex("#6e6e6e")),
		},
			ui.Text(mark, ui.FontSize(11), ui.TextColor(black)),
		),
		ui.Text(label, ui.FontSize(12), ui.TextColor(black)),
	)
}

func errorLabel(msg string) *ui.Node {
	return ui.If(msg != "", ui.Box([]ui.StyleOpt{ui.Padding(4)},
		ui.Text(msg, ui.FontSize(11), ui.TextColor(targetColor)),
	))
}

// ---- 杂项 ----

func hostIP(h protocol.HostPacket) net.IP {
	return net.IP{byte(h.IPAddr), byte(h.IPAddr >> 8), byte(h.IPAddr >> 16), byte(h.IPAddr >> 24)}
}

func atoiOr(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v == 0 {
		return def
	}
	return v
}

func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
