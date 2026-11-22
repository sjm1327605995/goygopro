package tenon

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	appservice "github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/backend/g3n"
	"github.com/sjm1327605995/tenon/pkg/ui"
	"github.com/sjm1327605995/tenon/pkg/value"
)

type pageID string

const (
	pageMenu   pageID = "menu"
	pageLobby  pageID = "lobby"
	pageDeck   pageID = "deck"
	pageSingle pageID = "single"
	pageReplay pageID = "replay"
	pageDuel   pageID = "duel"
)

type Desktop struct {
	service         *appservice.App
	events          *EventBus
	page            value.Bind[pageID]
	search          value.Bind[string]
	filterAttack    value.Bind[string]
	filterDefense   value.Bind[string]
	filterLevel     value.Bind[string]
	filterScale     value.Bind[string]
	filterKind      value.Bind[string]
	filterSubtype   value.Bind[string]
	filterAttribute value.Bind[string]
	filterRace      value.Bind[string]
	cards           value.Bind[[]appservice.CardInfo]
	singles         value.Bind[[]appservice.SingleInfo]
	singleName      value.Bind[string]
	singleReturn    value.Bind[bool]
	replays         value.Bind[[]string]
	replayName      value.Bind[string]
	replayTurn      value.Bind[string]
	replayRename    value.Bind[string]
	replayDetails   value.Bind[string]
	replayDelete    value.Bind[string]
	deck            value.Bind[appservice.DeckData]
	deckName        value.Bind[string]
	selected        value.Bind[uint32]
	message         value.Bind[string]
	address         value.Bind[string]
	hostPort        value.Bind[string]
	hostCardRule    value.Bind[string]
	hostMode        value.Bind[string]
	hostDuelRule    value.Bind[string]
	localPort       value.Bind[string]
	password        value.Bind[string]
	username        value.Bind[string]
	hosts           value.Bind[[]appservice.HostEntry]
	createOpen      value.Bind[bool]
	roomName        value.Bind[string]
	roomPass        value.Bind[string]
	scene           *g3n.Scene
	duelModel       *DuelModel
	duelController  *DuelController
	duelVersion     value.Bind[int]
	actionPopup     value.Bind[*ActionPopupProps]
}

func NewDesktop(service *appservice.App) *Desktop {
	desktop := &Desktop{
		service:         service,
		events:          NewEventBus(),
		page:            value.New(pageMenu),
		search:          value.New(""),
		filterAttack:    value.New(""),
		filterDefense:   value.New(""),
		filterLevel:     value.New(""),
		filterScale:     value.New(""),
		filterKind:      value.New("0"),
		filterSubtype:   value.New("0"),
		filterAttribute: value.New("0"),
		filterRace:      value.New("0"),
		cards:           value.New([]appservice.CardInfo{}),
		singles:         value.New([]appservice.SingleInfo{}),
		singleName:      value.New(""),
		singleReturn:    value.New(false),
		replays:         value.New([]string{}),
		replayName:      value.New(""),
		replayTurn:      value.New("1"),
		replayRename:    value.New(""),
		replayDetails:   value.New("（未选择录像）"),
		replayDelete:    value.New(""),
		deck:            value.New(appservice.DeckData{Name: "Blue-Eyes Beatdown"}),
		deckName:        value.New("Blue-Eyes Beatdown"),
		selected:        value.New(uint32(89631139)),
		message:         value.New(""),
		address:         value.New("127.0.0.1"),
		hostPort:        value.New("7911"),
		hostCardRule:    value.New("0"),
		hostMode:        value.New("0"),
		hostDuelRule:    value.New("5"),
		localPort:       value.New("7911"),
		password:        value.New(""),
		username:        value.New("Duelist"),
		hosts:           value.New([]appservice.HostEntry{}),
		createOpen:      value.New(false),
		roomName:        value.New("Championship Match"),
		roomPass:        value.New(""),
		duelVersion:     value.New(0),
		actionPopup:     value.New[*ActionPopupProps](nil),
	}
	desktop.duelModel = NewDuelModel(func() {
		desktop.duelVersion.Set(desktop.duelVersion.Get() + 1)
	})
	desktop.duelController = NewDuelController(service, desktop.duelModel)
	return desktop
}

func Run() error {
	if err := appservice.UseResourceDir(); err != nil {
		return err
	}
	service := appservice.NewApp()
	if err := service.Startup(context.Background()); err != nil {
		return err
	}
	defer service.ServiceShutdown()
	desktop := NewDesktop(service)
	desktop.duelModel.Attach(desktop.events)
	appservice.AppEventSink = desktop.events.Emit
	defer func() { appservice.AppEventSink = nil }()
	appservice.SetDesktopHooks(g3n.Exit, g3n.Minimise)
	g3n.Use(g3n.Options{
		Launch: func(game *g3n.Game) {
			desktop.scene = g3n.Scene3D(newDemoScene(desktop))
			desktop.scene.Launch(game)
		},
		Update: func(game *g3n.Game, dt time.Duration) {
			if desktop.scene != nil {
				desktop.scene.Frame(game, dt)
			}
		},
		ClearColor: &g3n.Color{R: 8.0 / 255, G: 15.0 / 255, B: 24.0 / 255},
		SceneViewport: func(width, height int) (int, int, int, int) {
			rect := arenaViewport(float32(width), float32(height))
			return int(rect.X), int(rect.Y), int(rect.W), int(rect.H)
		},
	})
	ui.WindowTitle("GoYGOPro — Tenon")
	ui.WindowSize(1280, 720)
	ui.WindowMinSize(1024, 640)
	ui.Run(desktop.Root())
	return nil
}

func (d *Desktop) Root() *ui.Node { return ui.Use(d.render, struct{}{}) }

func (d *Desktop) render(_ struct{}) *ui.Node {
	page := ui.UseValue(d.page)
	if d.scene != nil && d.scene.Root() != nil {
		d.scene.Root().SetVisible(page == pageDuel)
	}
	var content *ui.Node
	switch page {
	case pageLobby:
		content = ui.Use(d.lobbyComponent, struct{}{})
	case pageDeck:
		content = ui.Use(d.deckComponent, struct{}{})
	case pageSingle:
		content = ui.Use(d.singleComponent, struct{}{})
	case pageReplay:
		content = ui.Use(d.replayComponent, struct{}{})
	case pageDuel:
		content = ui.Use(d.duelComponent, struct{}{})
	default:
		content = ui.Use(d.menuComponent, struct{}{})
	}
	content = ui.Fragment(ui.Use(pageEnter, pageEnterProps{content: content}), ui.Key(string(page)))
	background := colorBackground
	if page == pageDuel {
		background = ui.Transparent
	}
	return ui.ThemeProvider(ui.DarkTheme, ui.Div(ui.Style(ui.Fill, ui.Bg(background)), content))
}

type pageEnterProps struct{ content *ui.Node }

func pageEnter(p pageEnterProps) *ui.Node {
	progress := ui.UseEnterTransition(140)
	return ui.Div(ui.Style(ui.Fill, ui.Opacity(progress), ui.TranslateXY(0, 8*(1-progress))), p.content)
}

func (d *Desktop) menuComponent(_ struct{}) *ui.Node   { return d.menuPage() }
func (d *Desktop) lobbyComponent(_ struct{}) *ui.Node  { return d.lobbyPage() }
func (d *Desktop) deckComponent(_ struct{}) *ui.Node   { return d.deckPage() }
func (d *Desktop) singleComponent(_ struct{}) *ui.Node { return d.singlePage() }
func (d *Desktop) replayComponent(_ struct{}) *ui.Node { return d.replayPage() }
func (d *Desktop) duelComponent(_ struct{}) *ui.Node   { return d.duelPage() }

func button(label string, onClick func()) *ui.Node {
	return ui.Button(
		ui.Style(ui.Width(240), ui.Height(38), ui.Bg(ui.Blue)),
		ui.OnClick(onClick),
		ui.Text(label).TextColor(ui.White),
	)
}

func (d *Desktop) menuPage() *ui.Node {
	return designFrame(1024, 640,
		pos(0, 0, 1024, 640, ui.Style(ui.LinearGradient(ui.Hex("#153443"), ui.Hex("#060b12"), 35))),
		pos(520, -160, 220, 950, ui.Style(ui.Bg(ui.Hex("#98c6e005")), ui.Border(1, ui.Hex("#a9c9e019")), ui.Rotate(35))),
		pos(880, -160, 90, 950, ui.Style(ui.Bg(ui.Hex("#c8ee4a04")), ui.Rotate(35))),
		pos(604, 130, 460, 1, ui.Style(ui.Bg(ui.Hex("#afc7d82b")), ui.Rotate(-45))),
		pos(804, 510, 140, 1, ui.Style(ui.Bg(ui.Hex("#c8ee4a65")), ui.Rotate(-45))),
		pos(498, 108, 430, 430, ui.Style(ui.Border(1, ui.Hex("#a5d0d92b")), ui.Radius(215), ui.Rotate(-25), ui.ScaleXY(1, 0.84))),
		pos(524, 135, 378, 378, ui.Style(ui.Border(1, ui.Hex("#c8ee4a24")), ui.Radius(189), ui.Rotate(-25), ui.ScaleXY(1, 0.84))),
		pos(36, 26, 42, 42,
			ui.Style(ui.LinearGradient(ui.Hex("#263c45"), ui.Hex("#0d161e"), 45), ui.Border(1, ui.Hex("#bddb7180")), ui.CutCorner(8)),
			textAt("D", 14, 8, 20, 26, 23, colorLime),
		),
		textAt("DUELIST", 90, 31, 240, 20, 14, colorText),
		textAt("决斗者 / DUELIST", 90, 56, 240, 12, 7, colorMuted),
		textAt("GOYGOPRO", 633, 40, 150, 22, 16, colorText),
		textAt("●  本地客户端  |  MR2020  |  LOCAL", 760, 45, 210, 14, 7, colorMuted),
		pos(36, 88, 952, 1, ui.Style(ui.Bg(ui.Hex("#b5c9d51c")))),
		textAt("THE NEXT TURN IS YOURS", 48, 123, 300, 14, 8, ui.Hex("#bbd976")),
		textAt("每一张卡，", 48, 151, 390, 48, 35, colorText),
		textAt("开启无限可能。", 48, 197, 430, 48, 35, colorText),
		textAt("构筑你的战术，迎接下一场决斗。", 48, 254, 360, 16, 10, ui.Hex("#8a9ca9")),
		pos(420, 95, 600, 468,
			ui.Img(ui.SrcImage("blue-eyes-menu", blueEyesMenuArt()), ui.Style(ui.Fill, ui.Opacity(0.62)), ui.Fit(ui.FitCover)),
		),
		pos(528, 497, 2, 74, ui.Style(ui.Bg(colorLime))),
		textAt("BLUE-EYES WHITE DRAGON", 543, 497, 280, 14, 8, ui.Hex("#bbd976")),
		textAt("青眼白龙", 543, 519, 260, 36, 25, colorText),
		textAt("LIGHT · DRAGON  —  ATK 3000 / DEF 2500", 543, 558, 320, 14, 8, ui.Hex("#8dabb6")),
		panelButton("联机模式", "DUEL  ↗", 48, 285, 330, 58, true, func() {
			d.message.Set("")
			d.page.Set(pageLobby)
		}),
		panelButton("单人模式", "SOLO / 02", 48, 353, 330, 40, false, func() {
			d.message.Set("")
			items := d.service.ListSingles()
			d.singles.Set(items)
			if len(items) > 0 {
				d.singleName.Set(items[0].Name)
			}
			d.page.Set(pageSingle)
		}),
		panelButton("观看录像", "REPLAY / 03", 48, 399, 330, 40, false, func() {
			d.message.Set("")
			d.replays.Set(d.service.ListReplays())
			d.page.Set(pageReplay)
		}),
		panelButton("编辑卡组", "DECK / 04", 48, 445, 330, 40, false, d.openDeck),
		panelButton("退出", "", 48, 491, 110, 28, false, d.service.Quit),
		pos(48, 522, 330, 1, ui.Style(ui.Bg(ui.Hex("#a8bdca30")))),
		textAt("决斗指南", 48, 532, 180, 12, 7, ui.Hex("#92a5ae")),
		textAt("GET STARTED / 01", 248, 532, 130, 12, 6, ui.Hex("#596e7b")),
		textAt("从你的第一套卡组开始", 48, 554, 330, 18, 11, ui.Hex("#c7d4dc")),
		textAt("编辑卡组 → 单人练习 → 联机决斗", 48, 577, 330, 14, 8, ui.Hex("#718794")),
		pos(36, 601, 952, 1, ui.Style(ui.Bg(ui.Hex("#a8bdca1c")))),
		textAt("GOYGOPRO / DUEL TERMINAL", 36, 613, 300, 10, 6, ui.Hex("#5e7380")),
		textAt("© goygopro · ocgcore", 820, 613, 168, 10, 6, ui.Hex("#5e7380")),
		textAt("YGOPro Version:1.036.2", 840, 624, 148, 10, 7, ui.Hex("#657585")),
	)
}

func (d *Desktop) goMenu() {
	d.message.Set("")
	d.page.Set(pageMenu)
}

func (d *Desktop) backButton() *ui.Node {
	return button("返回主菜单", func() { d.goMenu() })
}

func (d *Desktop) statusText() *ui.Node {
	return ui.Text(ui.UseValue(d.message)).TextColor(ui.LightGray)
}

func (d *Desktop) deckPage() *ui.Node { return d.deckEditorPage() }

func (d *Desktop) currentCardFilter() appservice.CardFilter {
	return appservice.CardFilter{
		Keyword: d.search.Get(), Limit: 60,
		Type:      optionUint(d.filterKind.Get()) | optionUint(d.filterSubtype.Get()),
		Attribute: optionUint(d.filterAttribute.Get()), Race: optionUint(d.filterRace.Get()),
		AtkFilter: d.filterAttack.Get(), DefFilter: d.filterDefense.Get(),
		LevelFilter: d.filterLevel.Get(), ScaleFilter: d.filterScale.Get(),
	}
}

func (d *Desktop) searchCards() {
	d.message.Set("搜索中…")
	filter := d.currentCardFilter()
	go func() {
		cards := d.service.SearchCards(filter)
		ui.Post(func() {
			d.cards.Set(cards)
			d.message.Set(fmt.Sprintf("找到 %d 张卡片（最多显示 60 张）", len(cards)))
		})
	}()
}

func (d *Desktop) hostInfoRequest() appservice.HostInfoReq {
	return appservice.HostInfoReq{
		Rule: uint8(optionUint(d.hostCardRule.Get())), Mode: uint8(optionUint(d.hostMode.Get())),
		DuelRule: uint8(optionUint(d.hostDuelRule.Get(), "5")), StartLp: 8000, StartHand: 5, DrawCount: 1, TimeLimit: 180,
	}
}

func (d *Desktop) lobbyPage() *ui.Node {
	hosts := ui.UseValue(d.hosts)
	createOpen := ui.UseValue(d.createOpen)
	rows := make([]*ui.Node, 0, len(hosts))
	for _, host := range hosts {
		host := host
		rows = append(rows, ui.HStack(0,
			ui.Style(ui.Width(580), ui.Height(20), ui.Bg(ui.Transparent)),
			ui.OnClick(func() {
				d.address.Set(host.IP)
				d.hostPort.Set(strconv.Itoa(int(host.Port)))
			}),
			ui.Text(fmt.Sprintf("%s:%d  %s  LP %d  MR%d", host.IP, host.Port, host.Name, host.StartLp, host.DuelRule)).
				FontSize(12).TextColor(ui.Hex("#c2ccdb")).Style(ui.PaddingXY(6, 2)),
		))
	}
	if len(rows) == 0 {
		rows = append(rows, ui.Text("未发现主机——可建立主机，或输入主机信息加入游戏").FontSize(12).TextColor(colorMuted).Style(ui.Padding(8)))
	}
	refresh := func() {
		d.message.Set("正在搜索局域网主机…")
		go func() {
			hosts := d.service.RefreshHosts(1500)
			ui.Post(func() {
				d.hosts.Set(hosts)
				d.message.Set(fmt.Sprintf("发现 %d 台主机", len(hosts)))
			})
		}()
	}
	connect := func() {
		address := d.address.Get() + ":" + d.hostPort.Get()
		username, password := d.username.Get(), d.password.Get()
		d.message.Set("连接中…")
		go func() {
			result := d.service.ConnectServer(address, username, password)
			ui.Post(func() { d.message.Set(resultMessage(result)) })
		}()
	}
	children := []*ui.Node{
		pos(0, 0, 1024, 640, ui.Style(ui.LinearGradient(ui.Hex("#1d3540"), ui.Hex("#060b12"), 35))),
		screenHeading("联机大厅", "ONLINE DUEL / 寻找对手，开启决斗"),
		windowPanel("联机模式", 202, 85, 620, 470,
			textAt("昵称：", 12, 1, 88, 25, 12, colorMuted),
			pos(100, 1, 360, 22, darkTextField(d.username)),
			windowButton("建立主机", 472, 1, 110, func() { d.createOpen.Set(true) }),
			pos(12, 38, 596, 300,
				ui.Style(ui.Bg(ui.Hex("#08111b")), ui.Border(1, ui.Hex("#a8bdca2b"))),
				ui.ScrollView(ui.Style(ui.Fill), ui.VStack(0, rows...)),
			),
			windowButton("刷新主机", 260, 343, 100, refresh),
			textAt("本地端口：", 370, 343, 85, 25, 12, colorMuted),
			pos(450, 344, 50, 22, darkTextField(d.localPort)),
			windowButton("启动服务器", 506, 343, 90, func() {
				port, err := strconv.Atoi(d.localPort.Get())
				if err != nil {
					d.message.Set("本地端口无效")
					return
				}
				d.message.Set(resultMessage(d.service.StartLocalServer(port)))
			}),
			textAt("主机信息：", 12, 376, 90, 25, 12, colorMuted),
			pos(102, 376, 296, 22, darkTextField(d.address)),
			pos(406, 376, 60, 22, darkTextField(d.hostPort)),
			textAt("主机密码：", 12, 407, 90, 25, 12, colorMuted),
			pos(102, 407, 364, 22, darkTextField(d.password)),
			windowButton("加入游戏", 486, 376, 110, connect),
			windowButton("取消", 486, 407, 110, func() { d.goMenu() }),
		),
		pos(215, 570, 594, 24, d.statusText()),
	}
	if createOpen {
		children = append(children, windowPanel("建立主机", 312, 85, 400, 470,
			textAt("禁限卡表：", 20, 20, 110, 22, 12, colorMuted),
			pos(130, 20, 240, 22, selectBox("N/A", 0, 0, 240, 22, "N/A")),
			textAt("卡片允许：", 20, 55, 110, 22, 12, colorMuted),
			pos(130, 55, 240, 22, selectItemBox(d.hostCardRule.Get(), 0, 0, 240, 22, hostCardRuleOptions, d.hostCardRule.Set)),
			textAt("决斗模式：", 20, 90, 110, 22, 12, colorMuted),
			pos(130, 90, 240, 22, selectItemBox(d.hostMode.Get(), 0, 0, 240, 22, hostModeOptions, d.hostMode.Set)),
			textAt("决斗规则：", 20, 125, 110, 22, 12, colorMuted),
			pos(130, 125, 240, 22, selectItemBox(d.hostDuelRule.Get(), 0, 0, 240, 22, hostDuelRuleOptions, d.hostDuelRule.Set)),
			textAt("初始基本分：8000", 20, 170, 330, 22, 12, colorMuted),
			textAt("初始手卡：5    每回合抽卡：1", 20, 200, 330, 22, 12, colorMuted),
			textAt("每回合时间：180 秒", 20, 230, 330, 22, 12, colorMuted),
			textAt("主机名称：", 20, 280, 110, 22, 12, colorMuted),
			pos(130, 280, 240, 22, darkTextField(d.roomName)),
			textAt("主机密码：", 20, 315, 110, 22, 12, colorMuted),
			pos(130, 315, 240, 22, darkTextField(d.roomPass)),
			windowButton("确定", 20, 390, 110, func() {
				req := d.hostInfoRequest()
				result := d.service.CreateGame(req, d.roomName.Get(), d.roomPass.Get())
				d.message.Set(resultMessage(result))
				if success, _ := result["success"].(bool); success {
					d.createOpen.Set(false)
				}
			}),
			windowButton("取消", 260, 390, 110, func() { d.createOpen.Set(false) }),
		))
	}
	return designFrame(1024, 640, children...)
}

func (d *Desktop) singlePage() *ui.Node {
	items := ui.UseValue(d.singles)
	selected := ui.UseValue(d.singleName)
	returnDeckTop := ui.UseValue(d.singleReturn)
	rows := make([]*ui.Node, 0, len(items))
	selectedInfo := "选择左侧谜题查看说明。"
	for _, item := range items {
		item := item
		if item.Name == selected {
			selectedInfo = item.Message
		}
		background := ui.Transparent
		foreground := ui.Hex("#c2ccdb")
		if item.Name == selected {
			background = ui.Hex("#c8ee4a20")
			foreground = ui.Hex("#e4f4c1")
		}
		rows = append(rows, ui.HStack(0,
			ui.Style(ui.Width(384), ui.Height(30), ui.Bg(background)),
			ui.OnClick(func() { d.singleName.Set(item.Name) }),
			ui.Text(item.Name).FontSize(12).TextColor(foreground).Style(ui.PaddingXY(12, 7)),
		))
	}
	if len(items) == 0 {
		rows = append(rows, ui.Text("没有找到谜题脚本，请检查 single/ 目录。").FontSize(12).TextColor(colorMuted).Style(ui.PaddingXY(12, 14)))
	}
	returnLabel := "[ ] 不洗切时回卡组改为回顶端"
	if returnDeckTop {
		returnLabel = "[✓] 不洗切时回卡组改为回顶端"
	}
	start := func() {
		name := d.singleName.Get()
		if name == "" {
			d.message.Set("请先选择残局脚本")
			return
		}
		d.message.Set("正在加载残局…")
		go func() {
			result := d.service.StartSingle(name, d.singleReturn.Get())
			ui.Post(func() {
				if success, _ := result["success"].(bool); success {
					d.message.Set("残局已启动")
					d.page.Set(pageDuel)
				} else {
					d.message.Set(resultMessage(result))
				}
			})
		}()
	}
	return designFrame(1024, 640,
		pos(0, 0, 1024, 640, ui.Style(ui.LinearGradient(ui.Hex("#1d3540"), ui.Hex("#060b12"), 35))),
		screenHeading("单人模式", "SOLO / 磨练战术，解开残局"),
		windowPanel("单人模式", 182, 85, 660, 470,
			pos(14, 10, 400, 420,
				ui.Style(ui.Bg(ui.Hex("#08111b")), ui.Border(1, ui.Hex("#a8bdca2b"))),
				ui.ScrollView(ui.Style(ui.Fill), ui.VStack(2, rows...)),
			),
			textAt("主要信息", 430, 10, 210, 22, 12, colorMuted),
			pos(430, 36, 210, 200,
				ui.Style(ui.Bg(ui.Hex("#08111b")), ui.Border(1, ui.Hex("#a8bdca2b")), ui.Radius(10)),
				ui.ScrollView(ui.Style(ui.Fill), ui.Text(selectedInfo).FontSize(13).TextColor(colorText).Width(186).Style(ui.PaddingXY(0, 12))),
			),
			windowButton(returnLabel, 430, 252, 210, func() { d.singleReturn.Set(!d.singleReturn.Get()) }),
			pos(430, 292, 210, 70, d.statusText()),
			windowButton("确定", 430, 390, 105, start),
			windowButton("退出", 545, 390, 100, func() { d.goMenu() }),
		),
	)
}

func (d *Desktop) replayPage() *ui.Node {
	files := ui.UseValue(d.replays)
	selected := ui.UseValue(d.replayName)
	details := ui.UseValue(d.replayDetails)
	armedDelete := ui.UseValue(d.replayDelete)
	rows := make([]*ui.Node, 0, len(files))
	selectReplay := func(name string) {
		d.replayName.Set(name)
		d.replayDelete.Set("")
		d.replayRename.Set(strings.TrimSuffix(name, ".yrp"))
		info := d.service.ReplayInfo(name)
		if success, _ := info["success"].(bool); !success {
			d.replayDetails.Set(resultMessage(info))
			return
		}
		d.replayDetails.Set(fmt.Sprintf("录像：%s\n日期：%v\n玩家：%v\n初始基本分：%v\n决斗规则：%v\nTAG：%v",
			name, info["date"], info["players"], info["startLp"], info["duelRule"], info["isTag"]))
	}
	for _, file := range files {
		file := file
		background := ui.Transparent
		foreground := ui.Hex("#c2ccdb")
		if file == selected {
			background = ui.Hex("#c8ee4a20")
			foreground = ui.Hex("#e4f4c1")
		}
		rows = append(rows, ui.HStack(0,
			ui.Style(ui.Width(326), ui.Height(24), ui.Bg(background)),
			ui.OnClick(func() { selectReplay(file) }),
			ui.Text(file).FontSize(12).TextColor(foreground).Style(ui.PaddingXY(6, 4)),
		))
	}
	if len(files) == 0 {
		rows = append(rows, ui.Text("（没有录像文件）").FontSize(12).TextColor(colorMuted).Style(ui.Padding(10)))
	}
	deleteLabel := "删除录像"
	if armedDelete == selected && selected != "" {
		deleteLabel = "确认删除"
	}
	requireSelection := func() bool {
		if d.replayName.Get() != "" {
			return true
		}
		d.message.Set("请先选择录像")
		return false
	}
	return designFrame(1024, 640,
		pos(0, 0, 1024, 640, ui.Style(ui.LinearGradient(ui.Hex("#1d3540"), ui.Hex("#060b12"), 35))),
		screenHeading("决斗回放", "REPLAY / 回顾每一个关键回合"),
		windowPanel("观看录像", 220, 100, 580, 420,
			pos(10, 6, 340, 370,
				ui.Style(ui.Bg(ui.Hex("#08111b")), ui.Border(1, ui.Hex("#a8bdca2b"))),
				ui.ScrollView(ui.Style(ui.Fill), ui.VStack(0, rows...)),
			),
			textAt("录像信息：", 360, 6, 210, 22, 12, colorMuted),
			pos(360, 36, 210, 205,
				ui.Style(ui.Bg(ui.Hex("#08111be6")), ui.Border(1, ui.Hex("#94a3b833")), ui.Radius(8)),
				textAt(details, 10, 10, 190, 185, 12, ui.Hex("#c2ccdb")),
			),
			textAt("播放起始于回合：", 360, 251, 210, 20, 12, colorMuted),
			pos(360, 276, 100, 22, darkTextField(d.replayTurn)),
			pos(470, 251, 100, 22, darkTextField(d.replayRename).Placeholder("新名称")),
			windowButton("提取卡组", 470, 301, 100, func() {
				if !requireSelection() {
					return
				}
				result := d.service.ExportReplayDeck(d.replayName.Get())
				d.message.Set(resultMessage(result))
			}),
			windowButton(deleteLabel, 360, 331, 100, func() {
				if !requireSelection() {
					return
				}
				name := d.replayName.Get()
				if d.replayDelete.Get() != name {
					d.replayDelete.Set(name)
					return
				}
				if err := d.service.DeleteReplay(name); err != nil {
					d.message.Set(err.Error())
				}
				d.replayDelete.Set("")
				d.replayName.Set("")
				d.replayDetails.Set("（未选择录像）")
				d.replays.Set(d.service.ListReplays())
			}),
			windowButton("载入录像", 470, 331, 100, func() {
				if !requireSelection() {
					return
				}
				d.message.Set("回放解析中…")
				go func() {
					result := d.service.PlayReplay(d.replayName.Get())
					ui.Post(func() {
						d.message.Set(resultMessage(result))
						if success, _ := result["success"].(bool); success {
							d.page.Set(pageDuel)
						}
					})
				}()
			}),
			windowButton("重命名", 360, 361, 100, func() {
				if !requireSelection() {
					return
				}
				oldName := d.replayName.Get()
				if err := d.service.RenameReplay(oldName, d.replayRename.Get()); err != nil {
					d.message.Set(err.Error())
					return
				}
				d.replayName.Set("")
				d.replayDetails.Set("（未选择录像）")
				d.replays.Set(d.service.ListReplays())
			}),
			windowButton("退出", 470, 361, 100, func() { d.goMenu() }),
		),
		pos(230, 535, 560, 28, d.statusText()),
	)
}

func (d *Desktop) duelPage() *ui.Node { return d.duelHUD() }

func resultMessage(result map[string]interface{}) string {
	if success, _ := result["success"].(bool); success {
		return "操作成功"
	}
	if err, ok := result["error"].(string); ok {
		return err
	}
	return fmt.Sprintf("%v", result)
}
