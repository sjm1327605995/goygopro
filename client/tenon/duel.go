package tenon

import (
	"fmt"

	"github.com/sjm1327605995/tenon/pkg/ui"
)

func arenaViewport(width, height float32) ui.Rect {
	return ui.Rect{
		X: width * 0.234375,
		Y: height * 0.1375,
		W: width * 0.765625,
		H: height * 0.74375,
	}
}

type PlayerPanelProps struct {
	Name  string
	Frame ui.Rect
	LP    int32
	Piles DuelPiles
	Turn  int
}

func PlayerPanel(p PlayerPanelProps) *ui.Node {
	turnText := ""
	if p.Turn > 0 {
		turnText = fmt.Sprintf("回合 %d", p.Turn)
	}
	w := p.Frame.W
	return pos(p.Frame.X, p.Frame.Y, w, 66,
		ui.Style(ui.LinearGradient(ui.Hex("#15232ff5"), ui.Hex("#0b131ef5"), 55), ui.Border(1, ui.Hex("#9cb9cd30")), ui.Radius(2)),
		textAt(p.Name, 12, 7, 200, 16, 11, ui.Hex("#cadbdf")),
		textAt(turnText, w-78, 7, 65, 16, 8, ui.Hex("#d4e8a2")),
		pos(12, 27, w-24, 24, ui.Style(ui.Bg(ui.Hex("#2b101780")), ui.Border(1, ui.Hex("#8aabba30")))),
		textCenterAt(fmt.Sprintf("%d", p.LP), 12, 27, w-24, 24, 19, colorText),
		textAt(fmt.Sprintf("卡组 %d     墓地 %d     除外 %d     额外 %d", p.Piles.Deck, p.Piles.Grave, p.Piles.Banish, p.Piles.Extra), 12, 54, w-24, 10, 7, colorMuted),
	)
}

func (d *Desktop) duelHUD() *ui.Node {
	_ = ui.UseValue(d.duelVersion)
	popup := ui.UseValue(d.actionPopup)
	closePopup := func() { d.actionPopup.Set(nil) }
	model := d.duelModel
	ui.UseEffect(func() ui.Cleanup {
		closePopup()
		return nil
	}, model.Revision)
	nextPhaseLabel := "战斗阶段"
	if model.Prompt.Mode == "battle" {
		nextPhaseLabel = "主要阶段 2"
	}
	handHighlight := func(code uint32, sequence int) int {
		selection := model.CardSelection
		if selection == nil {
			return 0
		}
		for index, choice := range selection.Choices {
			if choice.Code != code || choice.Seat != 0 || choice.Location != locationHand || choice.Sequence != sequence {
				continue
			}
			for _, selected := range selection.Selected {
				if selected == index {
					return 2
				}
			}
			return 1
		}
		return 0
	}
	selected := ui.UseValue(d.selected)
	info := d.service.GetCard(selected)
	name := fmt.Sprintf("Blue-Eyes White Dragon[%d]", selected)
	description := "选择场上的卡片查看效果与数值。"
	level := "[★8] 3000/2500"
	if info != nil {
		name = fmt.Sprintf("%s[%d]", info.Name, info.Code)
		description = info.Desc
		level = fmt.Sprintf("[★%d] %d/%d", info.Level, info.Attack, info.Defense)
	}
	status := ui.UseValue(d.message)
	if model.Message != "" {
		status = model.Message
	}
	selfTurn, opponentTurn := 0, 0
	if model.TurnPlayer == 0 {
		selfTurn = model.Turn
	} else {
		opponentTurn = model.Turn
	}
	children := []*ui.Node{
		pos(0, 0, 240, 640, ui.Style(ui.LinearGradient(ui.Hex("#162632"), ui.Hex("#0b131e"), 35), ui.Border(1, ui.Hex("#a9c2d32b")))),
		textAt("DUEL FIELD", 120, 17, 108, 18, 12, ui.Hex("#cadbdf")),
		textAt("决斗终端", 120, 40, 108, 12, 8, ui.Hex("#768d9c")),
		pos(12, 58, 216, 244, ui.Style(ui.LinearGradient(ui.Hex("#29424d"), ui.Hex("#0a131e"), 90), ui.Border(1, ui.Hex("#9cb9cd30")), ui.Radius(2))),
		CardArt(CardArtProps{Service: d.service, Code: selected, Frame: ui.Rect{X: 20, Y: 66, W: 200, H: 228}}),
		pos(12, 311, 216, 213, ui.Style(ui.Bg(ui.Hex("#0a141fc9")), ui.Border(1, ui.Hex("#9cb9cd30")), ui.Radius(2))),
		textAt("卡片信息", 20, 319, 96, 22, 10, ui.Hex("#def2ae")),
		textAt("消息记录", 125, 319, 96, 22, 10, ui.Hex("#8ea6b5")),
		textAt(name, 20, 349, 200, 34, 12, ui.Hex("#e7eedb")),
		textAt("[怪兽|通常] 鸟兽/光", 20, 385, 200, 18, 10, colorLime),
		textAt(level, 20, 406, 200, 18, 10, colorText),
		pos(20, 430, 200, 86, ui.ScrollView(ui.Style(ui.Fill), ui.Text(description).FontSize(11).TextColor(ui.Hex("#b7c9d4")).Width(188))),
		PlayerPanel(PlayerPanelProps{Name: model.Names[0], Frame: ui.Rect{X: 254, Y: 14, W: 310}, LP: model.LP[0], Piles: model.Piles[0], Turn: selfTurn}),
		PlayerPanel(PlayerPanelProps{Name: model.Names[1], Frame: ui.Rect{X: 700, Y: 14, W: 310}, LP: model.LP[1], Piles: model.Piles[1], Turn: opponentTurn}),
		pos(586, 23, 92, 46, ui.Style(ui.Bg(ui.Hex("#11202c")), ui.Border(1, ui.Hex("#9cb9cd30")), ui.Radius(2))),
		textCenterAt("TURN", 586, 28, 92, 10, 7, ui.Hex("#a8c481")),
		textCenterAt(fmt.Sprint(max(1, model.Turn)), 586, 43, 92, 24, 22, colorText),
		pos(402, 91, 460, 26, ui.Style(ui.Bg(ui.Hex("#111e2aec")), ui.Border(1, ui.Hex("#c8ee4a50"))), textCenterAt(status, 8, 4, 444, 18, 11, colorText)),
		smallButton("忽略时点", 12, 540, 69, 27, func() { d.message.Set("已忽略时点") }),
		smallButton("显示时点", 85, 540, 69, 27, func() { d.message.Set("显示可用时点") }),
		smallButton("可用时点", 158, 540, 69, 27, func() { d.message.Set("可用时点") }),
		smallButton("洗切手牌", 12, 571, 216, 23, func() {
			closePopup()
			if !d.duelController.ShuffleHand() {
				d.message.Set("当前不能洗切手牌")
			}
		}),
		pos(12, 600, 216, 27,
			ui.Style(ui.Bg(ui.Hex("#221d26")), ui.Border(1, ui.Hex("#ad6e743d"))),
			ui.OnClick(func() {
				d.service.StopSingle()
				d.goMenu()
			}),
			textCenterAt("投降 / 返回主菜单", 6, 6, 204, 15, 10, ui.Hex("#ba9398")),
		),
		pos(254, 598, 358, 28, ui.Style(ui.Bg(ui.Hex("#0f1c28")), ui.Border(1, ui.Hex("#9cb9cd30"))), textAt("聊天...", 8, 6, 342, 16, 10, colorMuted)),
		pos(628, 586, 382, 40, ui.Style(ui.Bg(ui.Hex("#101c28ed")), ui.Border(1, ui.Hex("#9cb9cd30")))),
		smallButton("M1", 633, 591, 118, 28, closePopup),
		smallButton(nextPhaseLabel, 759, 591, 118, 28, func() {
			closePopup()
			if model.Prompt.Mode == "battle" {
				d.duelController.ToMain2()
			} else {
				d.duelController.ToBattle()
			}
		}),
		smallButton("结束回合", 885, 591, 118, 28, func() {
			closePopup()
			d.duelController.ToEnd()
		}),
		ui.Use(HandFan, HandFanProps{
			Service: d.service, Cards: model.Hands[1], X: 240, Y: 88, Width: 784, Height: 462, ShuffleRevision: model.Shuffle[1],
			OnHover: func(code uint32) {
				if code != 0 {
					d.selected.Set(code)
				}
			},
			OnSelect: func(selection HandSelection) {
				if selection.Code != 0 {
					d.selected.Set(selection.Code)
				}
			},
		}),
		ui.Use(HandFan, HandFanProps{
			Service: d.service, Cards: model.Hands[0], X: 240, Y: 88, Width: 784, Height: 462, Near: true, ShuffleRevision: model.Shuffle[0], Highlight: handHighlight,
			OnHover: func(code uint32) {
				if code != 0 {
					d.selected.Set(code)
				}
			},
			OnSelect: func(selection HandSelection) {
				if selection.Code == 0 {
					return
				}
				d.selected.Set(selection.Code)
				if model.CardSelection != nil {
					d.duelController.ChooseCard(selection.Code, 0, locationHand, selection.Sequence)
					closePopup()
					return
				}
				options := d.duelController.HandOptions(selection.Code, selection.Sequence)
				if len(options) == 0 {
					closePopup()
					return
				}
				d.actionPopup.Set(&ActionPopupProps{X: selection.X, Y: selection.Y, Options: options})
			},
		}),
	}
	if selection := model.CardSelection; selection != nil {
		ready := len(selection.Selected) >= selection.Min && len(selection.Selected) <= selection.Max && (selection.Kind != "unselect" || len(selection.Selected) == 1)
		if ready || selection.Cancelable || len(selection.Selected) > 0 {
			label := "取消"
			if ready {
				label = "完成"
			} else if len(selection.Selected) > 0 {
				label = "撤销"
			}
			children = append(children, smallButton(label, 462, 598, 140, 28, func() {
				if ready {
					d.duelController.FinishCardSelection()
				} else {
					d.duelController.CancelOrUndoCardSelection()
				}
			}))
		}
	}
	if popup != nil {
		props := *popup
		props.OnClose = closePopup
		props.OnSelect = func(option ActionOption) {
			closePopup()
			d.duelController.Execute(option)
		}
		children = append(children, ui.Use(ActionPopup, props))
	}
	return frame(children...)
}
