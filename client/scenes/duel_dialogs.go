package scenes

import (
	"fmt"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol/network"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// 决斗中的各种选择框。它们是模态的，走 Portal 渲染到顶层浮层 —— 否则会被卡牌的
// 3D 变换与 ZIndex 影响，压在桌面下面。

func duelDialog() *ui.Node {
	d := client.MainGame.Dialog
	mounted, p := ui.UseTransition(d.Visible, 160)
	if !mounted {
		return nil
	}

	return ui.Portal(ui.TrapFocus(),
		ui.Box([]ui.StyleOpt{
			ui.Fill, ui.ItemsCenter, ui.JustifyCenter,
			ui.Bg(modalOverlay.Alpha(p)),
		},
			ui.Box([]ui.StyleOpt{
				ui.Column, ui.ItemsCenter, ui.Gap(12), ui.Padding(24),
				ui.Bg(ui.Hex("#2b2b2b")), ui.Border(1, ui.Hex("#5a5a5a")), ui.Radius(12),
				ui.MaxWidth(640), ui.Opacity(p), ui.Scale(0.94 + 0.06*p),
			},
				dialogContent(d),
			),
		),
	)
}

func dialogContent(d *client.DialogState) *ui.Node {
	switch d.Type {
	case client.DialogYesNo:
		return yesNoDialog(d)
	case client.DialogOption:
		return optionDialog(d)
	case client.DialogCardSelect:
		return cardSelectDialog(d)
	case client.DialogCmdSelect:
		return cmdSelectDialog(d)
	case client.DialogPosition:
		return positionDialog(d)
	case client.DialogSort:
		return sortDialog(d)
	case client.DialogRace:
		return bitChoiceDialog("选择种族", d.AnnounceRaceAvail, raceNames)
	case client.DialogAttrib:
		return bitChoiceDialog("选择属性", d.AnnounceAttribAvail, attribNames)
	case client.DialogCard:
		return announceCardDialog()
	default:
		return nil
	}
}

func yesNoDialog(d *client.DialogState) *ui.Node {
	return ui.Fragment(
		dialogTitle(fmt.Sprintf("效果 #%d", d.YesNoDesc)),
		ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(16), ui.JustifyCenter},
			lobbyButton("是", func() { respondI(1) }),
			lobbyButton("否", func() { respondI(0) }),
		),
	)
}

func optionDialog(d *client.DialogState) *ui.Node {
	title := "选择要发动的效果"
	switch client.MainGame.DInfo.CurMsg {
	case network.MSG_ROCK_PAPER_SCISSORS:
		title = "猜拳"
	case network.MSG_ANNOUNCE_NUMBER:
		title = "选择数字"
	}

	btns := make([]*ui.Node, 0, len(d.Options))
	for i, opt := range d.Options {
		idx, label := i, fmt.Sprintf("选项 %d", opt)
		switch client.MainGame.DInfo.CurMsg {
		case network.MSG_ROCK_PAPER_SCISSORS:
			switch opt {
			case 1:
				label = "石头"
			case 2:
				label = "剪刀"
			case 3:
				label = "布"
			}
		case network.MSG_ANNOUNCE_NUMBER:
			label = fmt.Sprint(opt)
		}
		btns = append(btns, ui.Keyed(fmt.Sprint(i), lobbyButton(label, func() {
			if d.OnOptionSelected != nil {
				d.OnOptionSelected(idx)
				d.Hide()
				client.MainGame.FieldRev.Bump()
				return
			}
			respondI(int32(idx))
		})))
	}
	return ui.Fragment(dialogTitle(title), ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(8)}, btns...))
}

func cardSelectDialog(d *client.DialogState) *ui.Node {
	title := d.SelectHint
	if title == "" {
		title = "选择卡片"
	}
	cards := d.SelectCards

	picks := make([]*ui.Node, 0, len(cards))
	for i, card := range cards {
		idx := i
		picks = append(picks, ui.Keyed(fmt.Sprint(i), ui.Use(dialogCard, dialogCardProps{
			Card:    card,
			Marked:  card.IsSelected,
			// 走 selectCardClick 而不是直接翻转：凑数值的两种选择每点一下都要重算
			// 剩下哪些卡还凑得出来，绕过它就等于没有可选性判定。
			OnClick: func() { selectCardClick(cards[idx]) },
			Overlay: "",
		})))
	}

	var btns []*ui.Node
	if d.SelectCancelable {
		btns = append(btns, lobbyButton("取消", func() {
			client.Client.SetResponseI(-1)
			client.Client.SendResponse()
			client.MainGame.DField.ClearSelect()
			d.Hide()
			client.MainGame.FieldRev.Bump()
		}))
	}
	// 凑数值的选择要等组合合法（SelectReady）才能提交 —— 否则点了也只会被服务器拒绝。
	// 其余选择没有这个约束。
	sumMode := client.MainGame.DInfo.CurMsg == network.MSG_SELECT_SUM ||
		client.MainGame.DInfo.CurMsg == network.MSG_SELECT_TRIBUTE
	confirmDisabled := sumMode && !client.MainGame.DField.SelectReady

	btns = append(btns, lobbyButtonDisabled("确定", confirmDisabled, func() {
		count := 0
		for _, c := range cards {
			if c.IsSelected {
				count++
			}
		}
		resp := []byte{byte(count)}
		// MSG_SELECT_SUM 回的是 SelectSeq，其余回的是卡在列表里的下标。
		if client.MainGame.DInfo.CurMsg == network.MSG_SELECT_SUM {
			for _, c := range cards {
				if c.IsSelected {
					resp = append(resp, byte(c.SelectSeq))
				}
			}
		} else {
			for i, c := range cards {
				if c.IsSelected {
					resp = append(resp, byte(i))
				}
			}
		}
		client.Client.SetResponseB(resp)
		client.Client.SendResponse()
		client.MainGame.DField.ClearSelect()
		d.Hide()
		client.MainGame.FieldRev.Bump()
	}))

	return ui.Fragment(
		dialogTitle(title),
		ui.ScrollView(ui.Style(ui.MaxHeight(200), ui.Row, ui.Gap(4), ui.Wrap), ui.Fragment(picks...)),
		ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(12), ui.JustifyCenter}, btns...),
	)
}

func cmdSelectDialog(d *client.DialogState) *ui.Node {
	btns := make([]*ui.Node, 0, len(d.CmdSelectOptions))
	for i, opt := range d.CmdSelectOptions {
		idx := i
		btns = append(btns, ui.Keyed(fmt.Sprint(i), lobbyButton(opt, func() {
			client.MainGame.DField.ClearCommandFlag()
			client.Client.SetResponseI(d.CmdSelectResp[idx])
			client.Client.SendResponse()
			d.Hide()
			client.MainGame.FieldRev.Bump()
		})))
	}
	return ui.Fragment(dialogTitle("选择操作"), ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(8)}, btns...))
}

func positionDialog(d *client.DialogState) *ui.Node {
	positions := []struct {
		flag  uint8
		label string
	}{
		{0x01, "表侧攻击表示"},
		{0x02, "里侧攻击表示"},
		{0x04, "表侧守备表示"},
		{0x08, "里侧守备表示"},
	}
	var btns []*ui.Node
	for _, pos := range positions {
		if d.PositionOptions&pos.flag == 0 {
			continue
		}
		val := int32(pos.flag)
		btns = append(btns, ui.Keyed(pos.label, lobbyButton(pos.label, func() { respondI(val) })))
	}
	return ui.Fragment(
		dialogTitle(fmt.Sprintf("选择 %d 的表示形式", d.PositionCode)),
		ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(8)}, btns...),
	)
}

func sortDialog(d *client.DialogState) *ui.Node {
	cards, order := d.SortCards, d.SortOrder

	picks := make([]*ui.Node, 0, len(cards))
	for i, card := range cards {
		idx := i
		mark := ""
		if order[idx] > 0 {
			mark = fmt.Sprintf("#%d", order[idx])
		}
		picks = append(picks, ui.Keyed(fmt.Sprint(i), ui.Use(dialogCard, dialogCardProps{
			Card:    card,
			Marked:  order[idx] > 0,
			Overlay: mark,
			OnClick: func() { toggleSortPick(d, idx) },
		})))
	}

	return ui.Fragment(
		dialogTitle("按顺序点击卡片排序"),
		ui.ScrollView(ui.Style(ui.MaxHeight(200), ui.Row, ui.Gap(4), ui.Wrap), ui.Fragment(picks...)),
		lobbyButton("取消", func() {
			client.Client.SetResponseI(-1)
			client.Client.SendResponse()
			d.Hide()
			client.MainGame.FieldRev.Bump()
		}),
	)
}

// toggleSortPick 点一下给卡编号、再点取消编号（后面的号往前补）。全部编完自动回应。
func toggleSortPick(d *client.DialogState, idx int) {
	order := d.SortOrder
	if order[idx] > 0 {
		removed := order[idx]
		order[idx] = 0
		d.SortCur--
		for j := range order {
			if order[j] > removed {
				order[j]--
			}
		}
		client.MainGame.FieldRev.Bump()
		return
	}

	order[idx] = d.SortCur
	d.SortCur++
	if d.SortCur > len(d.SortCards)+1 {
		d.SortCur = len(d.SortCards) + 1
	}
	assigned := 0
	for _, v := range order {
		if v > 0 {
			assigned++
		}
	}
	if assigned == len(d.SortCards) {
		resp := make([]byte, len(d.SortCards))
		for j, v := range order {
			resp[j] = byte(v - 1)
		}
		client.Client.SetResponseB(resp)
		client.Client.SendResponse()
		d.Hide()
	}
	client.MainGame.FieldRev.Bump()
}

// bitChoiceDialog 是「从一个位掩码里挑一项」的通用框（种族、属性）。
func bitChoiceDialog(title string, avail uint32, names []string) *ui.Node {
	var btns []*ui.Node
	for i, name := range names {
		bit := uint32(1) << i
		if avail&bit == 0 {
			continue
		}
		btns = append(btns, ui.Keyed(name, lobbyButton(name, func() { respondI(int32(bit)) })))
	}
	return ui.Fragment(
		dialogTitle(title),
		ui.ScrollView(ui.Style(ui.MaxHeight(320), ui.Column, ui.Gap(4)), ui.Fragment(btns...)),
	)
}

func announceCardDialog() *ui.Node {
	return ui.Fragment(
		dialogTitle("宣言卡名（未实现）"),
		ui.Text("缺少卡片数据库，无法按条件筛选卡名。", ui.FontSize(12), ui.TextColor(gray)),
		lobbyButton("取消", func() { respondI(0) }),
	)
}

// ---- 小工具 ----

type dialogCardProps struct {
	Card    *client.ClientCard
	Marked  bool
	Overlay string
	OnClick func()
}

func dialogCard(p dialogCardProps) *ui.Node {
	hovered, _, ia := ui.UseInteraction()
	border, bw := ui.Hex("#00000000"), float32(2)
	switch {
	case p.Marked:
		border = selectedGold
	case hovered:
		border = selectableCyan
	}

	kids := []*ui.Node{ui.Style(
		ui.Width(60), ui.Height(80), ui.Bg(cardGray),
		ui.Border(bw, border), ui.Radius(4), ui.Clip,
	), ia, ui.OnClick(p.OnClick)}

	if img := client.ImageMgr.GetTexture(int(p.Card.Code)); img != nil {
		kids = append(kids, ui.Img(ui.SrcImage(fmt.Sprintf("card:%d", p.Card.Code), img),
			ui.Fit(ui.FitCover), ui.Style(ui.Absolute, ui.Left(0), ui.Top(0), ui.Fill)))
	} else {
		kids = append(kids, ui.Text(fmt.Sprint(p.Card.Code), ui.FontSize(10), ui.TextColor(black)))
	}
	if p.Overlay != "" {
		kids = append(kids, ui.Box([]ui.StyleOpt{
			ui.Absolute, ui.Left(2), ui.Bottom(2), ui.Bg(blackTransparent), ui.Padding(2), ui.Radius(2),
		}, ui.Text(p.Overlay, ui.FontSize(12), ui.TextColor(white))))
	}
	return ui.Button(kids...)
}

func dialogTitle(s string) *ui.Node {
	return ui.Text(s, ui.FontSize(16), ui.TextColor(white))
}

// respondI 回一个整数并关掉对话框 —— 大多数选择框的收尾动作。
func respondI(v int32) {
	client.Client.SetResponseI(v)
	client.Client.SendResponse()
	client.MainGame.Dialog.Hide()
	client.MainGame.FieldRev.Bump()
}

var raceNames = []string{
	"战士", "魔法师", "天使", "恶魔", "不死", "机械", "水", "炎",
	"岩石", "鸟兽", "植物", "昆虫", "雷", "龙", "兽", "兽战士",
	"恐龙", "鱼", "海龙", "爬虫类", "念动力", "幻神兽", "创造神", "幻龙", "电子界",
}

var attribNames = []string{"地", "水", "炎", "风", "光", "暗", "神"}
