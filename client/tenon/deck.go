package tenon

import (
	"fmt"
	"image"
	"math/rand"
	"sort"
	"strings"

	appservice "github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/shadcn"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

func (d *Desktop) openDeck() {
	d.message.Set("")
	if d.deck.Get().Main == nil {
		if names := d.service.ListDecks(); len(names) > 0 {
			if deck, err := d.service.LoadDeck(names[0]); err == nil {
				d.deck.Set(*deck)
				d.deckName.Set(deck.Name)
			}
		}
	}
	d.searchCards()
	d.page.Set(pageDeck)
}

func smallButton(label string, x, y, w, h float32, onClick func()) *ui.Node {
	return pos(x, y, w, h,
		ui.Style(ui.LinearGradient(ui.Hex("#253842"), ui.Hex("#182a34"), 90), ui.Border(1, ui.Hex("#9fb6bf50")), ui.Radius(3)),
		ui.OnClick(onClick),
		textAt(label, 5, h/2-6, w-10, 14, 11, colorText),
	)
}

func selectBox(value string, x, y, w, h float32, options ...string) *ui.Node {
	return pos(x, y, w, h, shadcn.Select(shadcn.SelectProps{
		Value: value, Options: options, Placeholder: "（无）",
		Style: []ui.StyleOpt{ui.Fill, ui.MinWidth(0), ui.Height(h), ui.PaddingXY(4, 0), ui.Radius(1), ui.Bg(ui.Hex("#08111b")), ui.Border(1, ui.Hex("#a8bdca2b"))},
	}))
}

type CardArtProps struct {
	Service      *appservice.App
	Code         uint32
	Frame        ui.Rect
	Selected     bool
	Accent       ui.Color
	OnClick      func()
	OnRightClick func()
}

func CardArt(p CardArtProps) *ui.Node { return ui.Use(cardArt, p) }

func cardArt(p CardArtProps) *ui.Node {
	img, setImg := ui.UseState(cachedCardImage(p.Code))
	ui.UseEffect(func() ui.Cleanup {
		code, service := p.Code, p.Service
		cancelled := false
		setImg(cachedCardImage(code))
		requestCardImage(service, code, func(loaded image.Image) {
			if !cancelled {
				setImg(loaded)
			}
		})
		return func() { cancelled = true }
	}, p.Code, p.Service)

	border := colorBorder
	if p.Selected {
		border = colorLime
	}
	if p.Accent.A != 0 {
		border = p.Accent
	}
	children := []*ui.Node{ui.Style(ui.Border(1, border)), ui.OnClick(p.OnClick)}
	if p.OnRightClick != nil {
		children = append(children, ui.OnContextMenu(func(_, _ float32) { p.OnRightClick() }))
	}
	if img != nil {
		children = append(children, ui.Img(ui.SrcImage(fmt.Sprintf("%d", p.Code), img), ui.Style(ui.Fill), ui.Fit(ui.FitContain)))
	} else {
		children = append(children, textAt(fmt.Sprintf("%d", p.Code), 4, p.Frame.H/2-8, p.Frame.W-8, 18, 10, colorMuted))
	}
	return pos(p.Frame.X, p.Frame.Y, p.Frame.W, p.Frame.H, children...)
}

func selectItemBox(value string, x, y, w, h float32, items []shadcn.SelectOption, onChange func(string)) *ui.Node {
	return pos(x, y, w, h, shadcn.Select(shadcn.SelectProps{
		Value: value, Items: items, OnChange: onChange, Placeholder: "（无）",
		Style: []ui.StyleOpt{ui.Fill, ui.MinWidth(0), ui.Height(h), ui.PaddingXY(4, 0), ui.Radius(1), ui.Bg(ui.Hex("#08111b")), ui.Border(1, ui.Hex("#a8bdca2b"))},
	}))
}

func (d *Desktop) addCardToDeck(code uint32, target deckTarget) {
	cardType := uint32(0)
	if info := d.service.GetCard(code); info != nil {
		cardType = info.Type
	}
	deck, message := addDeckCard(d.deck.Get(), code, cardType, target)
	if strings.HasPrefix(message, "已") {
		d.deck.Set(deck)
	}
	d.message.Set(message)
}

func (d *Desktop) removeCardFromDeck(code uint32, target deckTarget) {
	deck, message := removeDeckCard(d.deck.Get(), code, target)
	if strings.HasPrefix(message, "已") {
		d.deck.Set(deck)
	}
	d.message.Set(message)
}

func (d *Desktop) deckEditorPage() *ui.Node {
	selected := ui.UseValue(d.selected)
	deck := ui.UseValue(d.deck)
	results := ui.UseValue(d.cards)
	info := d.service.GetCard(selected)
	panelStyle := []ui.StyleOpt{ui.LinearGradient(ui.Hex("#15232ded"), ui.Hex("#0a131cf5"), 45), ui.Border(1, ui.Hex("#a8bdca30")), ui.Radius(8)}
	children := []*ui.Node{
		pos(0, 0, 1024, 640, ui.Style(ui.LinearGradient(ui.Hex("#15232ded"), ui.Hex("#060b12f5"), 45))),
		pos(1, 1, 197, 272, ui.Style(panelStyle...)),
		CardArt(CardArtProps{Service: d.service, Code: selected, Frame: ui.Rect{X: 11, Y: 10, W: 177, H: 254}, Selected: true}),
		pos(1, 275, 300, 364, ui.Style(panelStyle...)),
		panelButton("退出编辑", "", 205, 5, 90, 75, false, func() { d.goMenu() }),
		textAt("卡片信息", 12, 287, 135, 26, 14, colorText),
		textAt("日志", 165, 287, 120, 26, 13, colorMuted),
	}
	if info != nil {
		children = append(children,
			pos(12, 323, 276, 25, ui.Style(ui.Bg(ui.Hex("#08111b")), ui.Border(1, colorBorder)), textAt(fmt.Sprintf("%s[%d]", info.Name, info.Code), 6, 4, 264, 18, 12, colorText)),
			textAt("[怪兽|通常] 鸟兽/光", 16, 357, 260, 21, 11, ui.Hex("#7fb0ff")),
			textAt(fmt.Sprintf("[★%d] %d/%d", info.Level, info.Attack, info.Defense), 16, 383, 260, 21, 11, colorText),
			pos(16, 411, 268, 150, ui.ScrollView(ui.Style(ui.Fill), ui.Text(info.Desc).FontSize(11).TextColor(colorText).Width(254))),
		)
	}
	children = append(children,
		smallButton("+主", 16, 574, 56, 26, func() { d.addCardToDeck(selected, deckTargetMain) }),
		smallButton("+额外", 78, 574, 68, 26, func() { d.addCardToDeck(selected, deckTargetExtra) }),
		smallButton("+副", 152, 574, 56, 26, func() { d.addCardToDeck(selected, deckTargetSide) }),
		smallButton("移除", 214, 574, 70, 26, func() { d.removeCardFromDeck(selected, deckTargetAny) }),
		textAt("右键搜索结果加入；右键卡组卡片移除", 16, 608, 268, 12, 8, colorMuted),
		pos(309, 5, 296, 125, ui.Style(panelStyle...)),
		pos(610, 5, 410, 125, ui.Style(panelStyle...)),
		textAt("卡组分类", 314, 10, 65, 20, 11, colorMuted),
		selectBox("未分类卡组", 384, 10, 140, 20, "未分类卡组"),
		smallButton("卡组管理", 529, 10, 65, 20, func() {}),
		textAt("卡组", 314, 40, 65, 20, 11, colorMuted),
		selectBox(deck.Name, 384, 40, 140, 20, deck.Name),
		smallButton("保存", 529, 40, 65, 20, func() {
			deck.Name = d.deckName.Get()
			if err := d.service.SaveDeck(deck); err != nil {
				d.message.Set(err.Error())
			}
		}),
		pos(384, 70, 140, 20, darkTextField(d.deckName).Placeholder("卡组名")),
		smallButton("另存为", 529, 70, 65, 20, func() {
			deck.Name = d.deckName.Get()
			if err := d.service.SaveDeck(deck); err != nil {
				d.message.Set(err.Error())
			}
		}),
		smallButton("打乱", 314, 100, 50, 20, func() {
			rand.Shuffle(len(deck.Main), func(i, j int) { deck.Main[i], deck.Main[j] = deck.Main[j], deck.Main[i] })
			d.deck.Set(deck)
		}),
		smallButton("排序", 369, 100, 50, 20, func() {
			sort.Slice(deck.Main, func(i, j int) bool { return deck.Main[i] < deck.Main[j] })
			d.deck.Set(deck)
		}),
		smallButton("清空", 424, 100, 50, 20, func() {
			deck.Main, deck.Extra, deck.Side = nil, nil, nil
			d.deck.Set(deck)
		}),
		smallButton("删除", 529, 100, 65, 20, func() {
			if err := d.service.DeleteDeck(deck.Name); err != nil {
				d.message.Set(err.Error())
			}
		}),
	)
	children = append(children, d.deckFilters()...)
	children = append(children,
		d.deckSection("主卡组", deck.Main, selected),
		d.deckSection("额外卡组", deck.Extra, selected),
		d.deckSection("副卡组", deck.Side, selected),
		d.searchResults(results, selected),
		pos(314, 606, 260, 20, d.statusText()),
	)
	return frame(children...)
}

func (d *Desktop) deckFilters() []*ui.Node {
	const labelSize = float32(11)
	return []*ui.Node{
		textAt("种类", 618, 10, 40, 20, labelSize, colorMuted),
		selectItemBox(d.filterKind.Get(), 661, 10, 62, 20, filterKindOptions, d.filterKind.Set),
		selectItemBox(d.filterSubtype.Get(), 726, 10, 68, 20, filterSubtypeOptions, d.filterSubtype.Set),
		textAt("禁限", 797, 10, 44, 20, labelSize, colorMuted),
		selectBox("（无）", 844, 10, 125, 20, "（无）"),
		textAt("属性", 618, 34, 40, 20, labelSize, colorMuted),
		selectItemBox(d.filterAttribute.Get(), 661, 34, 133, 20, filterAttributeOptions, d.filterAttribute.Set),
		textAt("攻击", 797, 34, 44, 20, labelSize, colorMuted),
		pos(844, 34, 78, 20, darkTextField(d.filterAttack)),
		smallButton("效果", 925, 34, 44, 44, func() {}),
		textAt("种族", 618, 58, 40, 20, labelSize, colorMuted),
		selectItemBox(d.filterRace.Get(), 661, 58, 133, 20, filterRaceOptions, d.filterRace.Set),
		textAt("守备", 797, 58, 44, 20, labelSize, colorMuted),
		pos(844, 58, 78, 20, darkTextField(d.filterDefense)),
		textAt("星数", 618, 82, 40, 20, labelSize, colorMuted),
		pos(661, 82, 40, 20, darkTextField(d.filterLevel)),
		pos(726, 82, 40, 20, darkTextField(d.filterScale).Placeholder("刻度")),
		textAt("卡名", 797, 82, 44, 20, labelSize, colorMuted),
		pos(844, 82, 125, 20, darkTextField(d.search).Placeholder("名称/编号")),
		smallButton("连接标记", 661, 106, 133, 20, func() {}),
		smallButton("清空", 797, 106, 44, 20, func() {
			d.search.Set("")
			d.filterAttack.Set("")
			d.filterDefense.Set("")
			d.filterLevel.Set("")
			d.filterScale.Set("")
			d.filterKind.Set("0")
			d.filterSubtype.Set("0")
			d.filterAttribute.Set("0")
			d.filterRace.Set("0")
			d.message.Set("已清空筛选条件")
		}),
		smallButton("搜索", 844, 106, 125, 20, d.searchCards),
	}
}

func (d *Desktop) deckSection(label string, codes []uint32, selected uint32) *ui.Node {
	titleY, gridY, gridH := float32(136), float32(159), float32(278)
	target, limit := deckTargetMain, 60
	switch label {
	case "额外卡组":
		titleY, gridY, gridH = 439, 462, 72
		target, limit = deckTargetExtra, 15
	case "副卡组":
		titleY, gridY, gridH = 536, 559, 72
		target, limit = deckTargetSide, 15
	}
	children := []*ui.Node{
		pos(309, titleY, 489, 22,
			ui.Style(ui.LinearGradient(ui.Hex("#293c42"), ui.Hex("#14242f"), 0), ui.Border(1, ui.Hex("#a8bdca3d")), ui.Radius(3)),
			textAt(label+"：", 10, 3, 240, 16, 11, ui.Hex("#d7e5d0")),
			textAt(fmt.Sprintf("%d/%d", len(codes), limit), 438, 3, 40, 16, 11, colorText)),
		pos(309, gridY, 489, gridH, ui.Style(ui.LinearGradient(ui.Hex("#15232d9e"), ui.Hex("#0a131c9e"), 90), ui.Border(1, ui.Hex("#a8bdca30")), ui.Radius(3))),
	}
	columns := len(codes)
	if label == "主卡组" {
		columns = 10
		if len(codes) > 40 {
			columns = (len(codes)-41)/4 + 11
		}
	}
	if columns < 1 {
		columns = 1
	}
	step := float32(436) / float32(max(1, columns-1))
	for i, code := range codes {
		code := code
		x := 314 + float32(i%columns)*step
		y := gridY + 5 + float32(i/columns)*68
		children = append(children, CardArt(CardArtProps{Service: d.service, Code: code, Frame: ui.Rect{X: x, Y: y, W: 44, H: 64}, Selected: code == selected, OnClick: func() { d.selected.Set(code) }, OnRightClick: func() { d.removeCardFromDeck(code, target) }}))
	}
	return ui.Div(children...)
}

func (d *Desktop) searchResults(cards []appservice.CardInfo, selected uint32) *ui.Node {
	rows := make([]*ui.Node, 0, len(cards))
	for _, card := range cards {
		card := card
		rows = append(rows, ui.HStack(0,
			ui.Style(ui.Width(206), ui.Height(66)),
			CardArt(CardArtProps{Service: d.service, Code: card.Code, Frame: ui.Rect{X: 5, W: 44, H: 64}, Selected: card.Code == selected, OnClick: func() { d.selected.Set(card.Code) }, OnRightClick: func() {
				d.selected.Set(card.Code)
				d.addCardToDeck(card.Code, deckTargetMain)
			}}),
			textAt(card.Name, 54, 2, 148, 22, 12, colorText),
			textAt(fmt.Sprintf("%d/%d", card.Attack, card.Defense), 54, 34, 148, 18, 11, colorMuted),
		))
	}
	if len(cards) == 0 {
		rows = append(rows, ui.Text("没有匹配结果").FontSize(11).TextColor(colorMuted).Style(ui.Padding(8)))
	}
	return ui.Div(
		pos(804, 136, 216, 494, ui.Style(ui.LinearGradient(ui.Hex("#15232ded"), ui.Hex("#0a131cf5"), 45), ui.Border(1, ui.Hex("#a8bdca30")), ui.Radius(4))),
		pos(804, 136, 122, 21, ui.Style(ui.Bg(colorPanelAlt)), textAt(fmt.Sprintf("搜索结果：%d", len(cards)), 6, 3, 112, 16, 11, colorText)),
		pos(804, 159, 216, 471, ui.ScrollView(ui.Style(ui.Fill), ui.VStack(0, rows...))),
	)
}
