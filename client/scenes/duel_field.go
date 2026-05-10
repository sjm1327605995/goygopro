package scenes

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/sjm1327605995/tenon"
	"github.com/sjm1327605995/tenon/pkg/engine"
	"github.com/sjm1327605995/tenon/yoga"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

func cardImage(code int) *ebiten.Image {
	img := client.ImageMgr.GetTexture(code)
	if img == nil {
		return nil
	}
	ebImg, ok := img.(*ebiten.Image)
	if ok {
		return ebImg
	}
	return ebiten.NewImageFromImage(img)
}

// DuelFieldScene corresponds to C++ duel field rendering in game.cpp.
// Kept as a singleton so background image cache survives route changes.
type DuelFieldScene struct {
	bg          *ebiten.Image
	chatInput   string
}

var duelFieldScene = &DuelFieldScene{}

func (s *DuelFieldScene) loadBackground() {
	if s.bg != nil {
		return
	}
	f, err := os.Open("textures/bg.jpg")
	if err != nil {
		return
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return
	}
	s.bg = ebiten.NewImageFromImage(img)
}

// DuelFieldRoute is the tenon RouteBuilder for the duel field.
// The framework invokes this automatically — no explicit Build() call needed.
func DuelFieldRoute(ctx engine.BuildContext, params engine.RouteParams) engine.Widget {
	return duelFieldScene.build()
}

func (s *DuelFieldScene) build() engine.Widget {
	s.loadBackground()

	return tenon.Stack(
		// Background - absolute positioned, fills stack
		tenon.Positioned(
			tenon.Image(s.bg).Fit(tenon.ObjectFitCover),
		).L(0).T(0).R(0).B(0),

		// Game UI overlay - absolute positioned, fills stack
		tenon.Positioned(
			tenon.VStack(
				// Opponent info bar
				s.buildPlayerInfoBar(1),
				// Field grid
				s.buildFieldGrid(),
				// Own info bar
				s.buildPlayerInfoBar(0),
				// Command / phase buttons
				s.buildCmdBar(),
			).Gap(2),
		).L(0).T(0).R(0).B(0),

		// Chat overlay - bottom left
		tenon.Positioned(
			s.buildChatOverlay(),
		).L(8).B(80).W(300).H(200),

		// Dialog overlay - absolute positioned, fills stack
		tenon.Positioned(
			s.buildDialog(),
		).L(0).T(0).R(0).B(0),
	).Width(client.GameWindowWidth).Height(client.GameWindowHeight)
}

func (s *DuelFieldScene) buildPlayerInfoBar(player int) engine.Widget {
	lp := client.MainGame.DInfo.LP[player]
	lpStr := client.MainGame.DInfo.StrLP[player]
	if lpStr == "" {
		lpStr = fmt.Sprintf("%d", lp)
	}

	deckCount := len(client.MainGame.DField.Deck[player])
	handCount := len(client.MainGame.DField.Hand[player])
	graveCount := len(client.MainGame.DField.Grave[player])
	extraCount := len(client.MainGame.DField.Extra[player])
	removeCount := len(client.MainGame.DField.Remove[player])

	phaseStr := phaseName(client.MainGame.DInfo.Phase)
	return tenon.Container(
		tenon.HStack(
			tenon.Container(tenon.Text(fmt.Sprintf("LP: %s", lpStr)).FontSize(16).Color(white)).W(120),
			tenon.HStack(
				tenon.Text(fmt.Sprintf("D:%d", deckCount)).FontSize(11).Color(white),
				tenon.Text(fmt.Sprintf("H:%d", handCount)).FontSize(11).Color(white),
				tenon.Text(fmt.Sprintf("G:%d", graveCount)).FontSize(11).Color(white),
				tenon.Text(fmt.Sprintf("E:%d", extraCount)).FontSize(11).Color(white),
				tenon.Text(fmt.Sprintf("R:%d", removeCount)).FontSize(11).Color(white),
			).Gap(6),
			tenon.Container(
				tenon.Text(fmt.Sprintf("Turn %d", client.MainGame.DInfo.Turn)).FontSize(12).Color(white),
			).W(70),
			tenon.Container(
				tenon.Text(phaseStr).FontSize(12).Color(white),
			).W(90),
		).Gap(8).Padding(4).AlignItems(tenon.AlignCenter),
	).Background(blackTransparent)
}

// buildFieldGrid builds the full duel field with both players' zones.
func (s *DuelFieldScene) buildFieldGrid() engine.Widget {
	return tenon.VStack(
		// Opponent side (flipped visually)
		s.buildPlayerField(1),
		// Spacer between fields
		tenon.Container(tenon.Spacer()).H(8),
		// Own side
		s.buildPlayerField(0),
	).Gap(2).AlignItems(tenon.AlignCenter)
}

// buildPlayerField builds one player's field side:
// Side bar | SZone (back row) | MZone (front row)
func (s *DuelFieldScene) buildPlayerField(player int) engine.Widget {
	return tenon.HStack(
		// Side bar: Extra / Deck / Grave / Removed / Hand counts
		s.buildSideBar(player),
		// Main field: back row (SZone) + front row (MZone)
		tenon.VStack(
			s.buildZoneRow(player, 0x08, 8), // SZone: 5 spell/trap + 1 field
			tenon.Container(tenon.Spacer()).H(4),
			s.buildZoneRow(player, 0x04, 7), // MZone: 5 monster + 2 extra for alignment
		).Gap(2),
	).Gap(4).AlignItems(tenon.AlignCenter)
}

func (s *DuelFieldScene) buildSideBar(player int) engine.Widget {
	extraCount := len(client.MainGame.DField.Extra[player])
	deckCount := len(client.MainGame.DField.Deck[player])
	graveCount := len(client.MainGame.DField.Grave[player])
	removeCount := len(client.MainGame.DField.Remove[player])
	handCount := len(client.MainGame.DField.Hand[player])

	items := []tenon.Widget{
		s.sideBarItem("Extra", extraCount),
		s.sideBarItem("Deck", deckCount),
		s.sideBarItem("Grave", graveCount),
		s.sideBarItem("Removed", removeCount),
		s.sideBarItem("Hand", handCount),
	}
	return tenon.VStack(items...).Gap(2).AlignItems(tenon.AlignCenter)
}

func (s *DuelFieldScene) sideBarItem(label string, count int) tenon.Widget {
	return tenon.Container(
		tenon.VStack(
			tenon.Text(label).FontSize(9).Color(white),
			tenon.Text(fmt.Sprintf("%d", count)).FontSize(10).Color(white),
		).Gap(0).AlignItems(tenon.AlignCenter),
	).W(40).H(36).Background(blackTransparent).CornerRadius(4)
}

// buildZoneRow builds a row of card slots for a zone (MZone or SZone).
func (s *DuelFieldScene) buildZoneRow(player int, location uint8, slots int) tenon.Widget {
	cells := make([]tenon.Widget, slots)
	for i := 0; i < slots; i++ {
		seq := i
		if location == 0x08 && i == 5 {
			// Field spell zone at the end of SZone row
			seq = 5
		}
		card := client.MainGame.DField.GetCard(player, location, seq)
		cells[i] = s.buildCardSlot(card, player, location, seq)
	}
	return tenon.HStack(cells...).Gap(4).Justify(yoga.JustifyCenter)
}

func (s *DuelFieldScene) buildCardSlot(card *client.ClientCard, player int, location uint8, sequence int) tenon.Widget {
	placeSelectable := s.isPlaceSelectable(player, location, sequence)

	if card == nil || card.Code == 0 {
		return s.buildEmptySlot(player, location, sequence, placeSelectable)
	}

	isSet := card.Position == 0x08 || card.Position == 0x02 // face-down DEF or face-down ATK
	bg, borderColor, borderW := s.cardColors(card, location, isSet, placeSelectable)

	// Build card content
	var content tenon.Widget
	if isSet {
		content = s.buildSetCardContent(card, placeSelectable)
	} else {
		content = s.buildFaceUpCardContent(card, location, placeSelectable)
	}

	overlays := []tenon.Widget{content}
	if card.IsShowEquip {
		overlays = append(overlays, tenon.Positioned(tenon.Container(tenon.Text("E").FontSize(8).Color(white)).Background(equipColor).Padding(1).CornerRadius(2)).R(2).T(2))
	}
	if card.IsShowTarget {
		overlays = append(overlays, tenon.Positioned(tenon.Container(tenon.Text("T").FontSize(8).Color(white)).Background(targetColor).Padding(1).CornerRadius(2)).L(2).T(2))
	}
	if card.IsShowChainTarget {
		overlays = append(overlays, tenon.Positioned(tenon.Container(tenon.Text("C").FontSize(8).Color(white)).Background(chainColor).Padding(1).CornerRadius(2)).L(2).B(2))
	}

	if len(overlays) > 1 {
		return tenon.Container(tenon.Stack(overlays...)).
			W(58).H(78).
			Background(bg).
			Border(borderColor, borderW).
			CornerRadius(6).
			OnClick(func() {
				s.onCardClick(card, player, location, sequence)
			})
	}
	return tenon.Container(content).
		W(58).H(78).
		Background(bg).
		Border(borderColor, borderW).
		CornerRadius(6).
		OnClick(func() {
			s.onCardClick(card, player, location, sequence)
		})
}

func (s *DuelFieldScene) buildEmptySlot(player int, location uint8, sequence int, placeSelectable bool) tenon.Widget {
	bg := cardEmptyBg
	borderColor := color.RGBA{R: 255, G: 255, B: 255, A: 40}
	borderW := float32(1)
	if placeSelectable {
		bg = placePurple
		borderColor = white
		borderW = 2
	}
	return tenon.Container(tenon.Spacer()).
		W(58).H(78).
		Background(bg).
		Border(borderColor, borderW).
		CornerRadius(6).
		OnClick(func() {
			s.onEmptySlotClick(player, location, sequence)
		})
}

func (s *DuelFieldScene) cardColors(card *client.ClientCard, location uint8, isSet, placeSelectable bool) (bg color.Color, borderColor color.Color, borderW float32) {
	borderW = 1
	borderColor = color.RGBA{R: 255, G: 255, B: 255, A: 80}

	if card.IsSelected {
		borderColor = selectedGold
		borderW = 3
	} else if card.IsSelectable {
		borderColor = selectableCyan
		borderW = 2
	} else if placeSelectable {
		borderColor = placePurple
		borderW = 2
	}

	if isSet {
		bg = cardSetBg
		return
	}

	switch location {
	case 0x04: // MZONE
		bg = cardMonsterBg
	case 0x08: // SZONE
		if card.Type&0x4 != 0 { // TYPE_TRAP
			bg = cardTrapBg
		} else {
			bg = cardSpellBg
		}
	default:
		bg = cardMonsterBg
	}
	return
}


func (s *DuelFieldScene) buildSetCardContent(card *client.ClientCard, placeSelectable bool) tenon.Widget {
	return tenon.VStack(
		tenon.Text("???").FontSize(12).Color(white),
	).AlignItems(tenon.AlignCenter).Justify(yoga.JustifyCenter)
}

func (s *DuelFieldScene) buildFaceUpCardContent(card *client.ClientCard, location uint8, placeSelectable bool) tenon.Widget {
	posText := ""
	switch card.Position {
	case 0x01:
		posText = "ATK"
	case 0x04:
		posText = "DEF"
	}

	// Try to show card image
	cImg := cardImage(int(card.Code))
	if cImg != nil {
		overlay := []tenon.Widget{
			tenon.Image(cImg).Fit(tenon.ObjectFitCover),
		}
		// Overlay position text and badges
		if posText != "" {
			overlay = append(overlay, tenon.Positioned(tenon.Container(tenon.Text(posText).FontSize(9).Color(white)).Background(blackTransparent).Padding(1)).L(2).T(2))
		}
		if card.CmdFlag != 0 {
			badges := []tenon.Widget{}
			if card.CmdFlag&client.CommandSummon != 0 {
				badges = append(badges, tenon.Container(tenon.Text("SUM").FontSize(7).Color(white)).Background(cmdSummonColor).CornerRadius(2).Padding(1))
			}
			if card.CmdFlag&client.CommandAttack != 0 {
				badges = append(badges, tenon.Container(tenon.Text("ATK").FontSize(7).Color(white)).Background(cmdAttackColor).CornerRadius(2).Padding(1))
			}
			if card.CmdFlag&client.CommandActivate != 0 {
				badges = append(badges, tenon.Container(tenon.Text("ACT").FontSize(7).Color(white)).Background(cmdActivateColor).CornerRadius(2).Padding(1))
			}
			if len(badges) > 0 {
				overlay = append(overlay, tenon.Positioned(tenon.HStack(badges...).Gap(1)).R(2).B(2))
			}
		}
		return tenon.Stack(overlay...)
	}

	// Fallback: text-only display
	badges := []tenon.Widget{}
	if card.CmdFlag != 0 {
		if card.CmdFlag&client.CommandSummon != 0 {
			badges = append(badges, tenon.Container(tenon.Text("SUM").FontSize(7).Color(white)).Background(cmdSummonColor).CornerRadius(2).Padding(1))
		}
		if card.CmdFlag&client.CommandAttack != 0 {
			badges = append(badges, tenon.Container(tenon.Text("ATK").FontSize(7).Color(white)).Background(cmdAttackColor).CornerRadius(2).Padding(1))
		}
		if card.CmdFlag&client.CommandActivate != 0 {
			badges = append(badges, tenon.Container(tenon.Text("ACT").FontSize(7).Color(white)).Background(cmdActivateColor).CornerRadius(2).Padding(1))
		}
	}

	// Counter text
	counterText := ""
	if client.MainGame.DInfo.CurMsg == network.MSG_SELECT_COUNTER && card.IsSelectable {
		counterText = fmt.Sprintf("C:%d", card.OpParam&0xffff)
	}

	// Code display (truncated)
	codeText := fmt.Sprintf("%d", card.Code)
	if len(codeText) > 8 {
		codeText = codeText[:8]
	}

	widgets := []tenon.Widget{
		tenon.Text(codeText).FontSize(8).Color(white),
	}
	if posText != "" {
		widgets = append(widgets, tenon.Text(posText).FontSize(9).Color(white))
	}
	if len(badges) > 0 {
		widgets = append(widgets, tenon.HStack(badges...).Gap(2))
	}
	if counterText != "" {
		widgets = append(widgets, tenon.Text(counterText).FontSize(8).Color(white))
	}

	return tenon.VStack(widgets...).Gap(2).AlignItems(tenon.AlignCenter).Justify(yoga.JustifyCenter)
}

func (s *DuelFieldScene) isPlaceSelectable(player int, location uint8, sequence int) bool {
	curMsg := client.MainGame.DInfo.CurMsg
	if curMsg != network.MSG_SELECT_PLACE && curMsg != network.MSG_SELECT_DISFIELD {
		return false
	}
	field := client.MainGame.DField.SelectableField
	var bit uint32
	switch {
	case player == 0 && location == 0x04: // MZONE
		bit = 1 << sequence
	case player == 0 && location == 0x08 && sequence < 6: // SZONE
		bit = 1 << (sequence + 8)
	case player == 0 && location == 0x08 && sequence >= 6: // SZONE PZONE
		bit = 1 << (sequence + 8)
	case player == 1 && location == 0x04: // MZONE
		bit = 1 << (sequence + 16)
	case player == 1 && location == 0x08 && sequence < 6: // SZONE
		bit = 1 << (sequence + 24)
	case player == 1 && location == 0x08 && sequence >= 6: // SZONE PZONE
		bit = 1 << (sequence + 24)
	}
	return field&bit != 0
}

func (s *DuelFieldScene) onCardClick(card *client.ClientCard, player int, location uint8, sequence int) {
	// Handle selection during dialog
	dialog := client.MainGame.Dialog
	if dialog.Visible && dialog.Type == client.DialogCardSelect {
		if card.IsSelectable {
			card.IsSelected = !card.IsSelected
		}
		return
	}

	// Handle place/disfield selection
	curMsg := client.MainGame.DInfo.CurMsg
	if curMsg == network.MSG_SELECT_PLACE || curMsg == network.MSG_SELECT_DISFIELD {
		if s.isPlaceSelectable(player, location, sequence) {
			resp := []byte{uint8(player), location, uint8(sequence)}
			client.MainGame.DField.SelectableField = 0
			client.Client.SetResponseB(resp)
			client.Client.SendResponse()
		}
		return
	}

	// Handle counter selection
	if curMsg == network.MSG_SELECT_COUNTER {
		if card.IsSelectable {
			card.OpParam--
			if card.OpParam&0xffff == 0 {
				card.IsSelectable = false
			}
			client.MainGame.DField.SelectCounterCount--
			if client.MainGame.DField.SelectCounterCount == 0 {
				// Build response
				resp := make([]byte, len(client.MainGame.DField.SelectableCards)*2)
				for i, c := range client.MainGame.DField.SelectableCards {
					val := uint16((c.OpParam >> 16) - (c.OpParam & 0xffff))
					binary.LittleEndian.PutUint16(resp[i*2:], val)
				}
				client.Client.SetResponseB(resp)
				client.Client.SendResponse()
				client.MainGame.DField.ClearSelect()
			}
		}
		return
	}

	// Handle command selection during IDLECMD / BATTLECMD
	if curMsg == network.MSG_SELECT_IDLECMD || curMsg == network.MSG_SELECT_BATTLECMD {
		if card.CmdFlag != 0 {
			s.handleCmdClick(card, curMsg)
			return
		}
	}

	fmt.Printf("Clicked card code=%d loc=%d seq=%d\n", card.Code, location, sequence)
}

func (s *DuelFieldScene) onEmptySlotClick(player int, location uint8, sequence int) {
	curMsg := client.MainGame.DInfo.CurMsg
	if curMsg == network.MSG_SELECT_PLACE || curMsg == network.MSG_SELECT_DISFIELD {
		if s.isPlaceSelectable(player, location, sequence) {
			resp := []byte{uint8(player), location, uint8(sequence)}
			client.MainGame.DField.SelectableField = 0
			client.Client.SetResponseB(resp)
			client.Client.SendResponse()
		}
	}
}

func (s *DuelFieldScene) handleCmdClick(card *client.ClientCard, curMsg int16) {
	df := client.MainGame.DField
	options := []string{}
	resps := []int32{}

	if curMsg == network.MSG_SELECT_IDLECMD {
		for i, c := range df.SummonableCards {
			if c == card {
				options = append(options, "Summon")
				resps = append(resps, int32(i<<16))
			}
		}
		for i, c := range df.SPSummonableCards {
			if c == card {
				options = append(options, "Special Summon")
				resps = append(resps, int32((i<<16)+1))
			}
		}
		for i, c := range df.ReposableCards {
			if c == card {
				options = append(options, "Repos")
				resps = append(resps, int32((i<<16)+2))
			}
		}
		for i, c := range df.MSetableCards {
			if c == card {
				options = append(options, "Set")
				resps = append(resps, int32((i<<16)+3))
			}
		}
		for i, c := range df.SSetableCards {
			if c == card {
				options = append(options, "Set Spell/Trap")
				resps = append(resps, int32((i<<16)+4))
			}
		}
		for i, c := range df.ActivatableCards {
			if c == card {
				flag := df.ActivatableDescs[i][1]
				if flag&client.EDESCOperation != 0 {
					continue
				}
				options = append(options, "Activate")
				resps = append(resps, int32((i<<16)+5))
			}
		}
	} else if curMsg == network.MSG_SELECT_BATTLECMD {
		for i, c := range df.ActivatableCards {
			if c == card {
				flag := df.ActivatableDescs[i][1]
				if flag&client.EDESCOperation != 0 {
					continue
				}
				options = append(options, "Activate")
				resps = append(resps, int32(i<<16))
			}
		}
		for i, c := range df.AttackableCards {
			if c == card {
				options = append(options, "Attack")
				resps = append(resps, int32((i<<16)+1))
			}
		}
	}

	if len(options) == 1 {
		client.MainGame.DField.ClearCommandFlag()
		client.Client.SetResponseI(resps[0])
		client.Client.SendResponse()
	} else if len(options) > 1 {
		client.MainGame.Dialog.ShowCmdSelect(options, resps)
	}
}

// ----- Dialog rendering -----

func (s *DuelFieldScene) buildDialog() engine.Widget {
	dialog := client.MainGame.Dialog
	if !dialog.Visible {
		return tenon.Container(tenon.Spacer())
	}

	return tenon.Container(
		tenon.Stack(
			// Dim background - absolute positioned, fills stack
			tenon.Positioned(
				tenon.Container(tenon.Spacer()).Background(modalOverlay),
			).L(0).T(0).R(0).B(0),
			// Dialog content - absolute positioned, fills stack
			tenon.Positioned(
				tenon.VStack(
					tenon.Container(tenon.Spacer()).H(80),
					s.buildDialogContent(),
					tenon.Container(tenon.Spacer()).H(80),
				).AlignItems(tenon.AlignCenter).Justify(yoga.JustifyCenter),
			).L(0).T(0).R(0).B(0),
		),
	)
}

func (s *DuelFieldScene) buildDialogContent() engine.Widget {
	dialog := client.MainGame.Dialog
	switch dialog.Type {
	case client.DialogYesNo:
		return s.buildYesNoDialog()
	case client.DialogOption:
		return s.buildOptionDialog()
	case client.DialogCardSelect:
		return s.buildCardSelectDialog()
	case client.DialogCmdSelect:
		return s.buildCmdSelectDialog()
	case client.DialogPosition:
		return s.buildPositionDialog()
	case client.DialogSort:
		return s.buildSortDialog()
	case client.DialogRace:
		return s.buildRaceDialog()
	case client.DialogAttrib:
		return s.buildAttribDialog()
	case client.DialogCard:
		return s.buildAnnounceCardDialog()
	default:
		return tenon.Container(tenon.Spacer())
	}
}

func (s *DuelFieldScene) animatedDialogContent(content tenon.Widget, w, h float32) tenon.Widget {
	return tenon.NewAnimatedContainer().
		WithChild(content).
		WithSize(w, h).
		WithDuration(200 * time.Millisecond).
		WithCurve(tenon.EaseInOutCurve{})
}

func (s *DuelFieldScene) buildYesNoDialog() engine.Widget {
	desc := fmt.Sprintf("Effect #%d", client.MainGame.Dialog.YesNoDesc)
	content := tenon.Container(
		tenon.VStack(
			tenon.Text(desc).FontSize(18).Color(white),
			tenon.Container(tenon.Spacer()).H(20),
			tenon.HStack(
				tenon.Button("Yes").OnClick(func() {
					client.Client.SetResponseI(1)
					client.Client.SendResponse()
					client.MainGame.Dialog.Hide()
				}),
				tenon.Button("No").Style(tenon.ButtonOutline).OnClick(func() {
					client.Client.SetResponseI(0)
					client.Client.SendResponse()
					client.MainGame.Dialog.Hide()
				}),
			).Gap(16).Justify(yoga.JustifyCenter),
		).Gap(12).Padding(24).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(400).H(180)
	return s.animatedDialogContent(content, 400, 180)
}

func (s *DuelFieldScene) buildOptionDialog() engine.Widget {
	options := client.MainGame.Dialog.Options
	buttons := make([]tenon.Widget, 0, len(options))
	for i, opt := range options {
		idx := i
		label := fmt.Sprintf("Option %d", opt)
		switch client.MainGame.DInfo.CurMsg {
		case network.MSG_ROCK_PAPER_SCISSORS:
			switch opt {
			case 1:
				label = "Rock"
			case 2:
				label = "Paper"
			case 3:
				label = "Scissors"
			}
		case network.MSG_ANNOUNCE_NUMBER:
			label = fmt.Sprintf("%d", opt)
		}
		buttons = append(buttons,
			tenon.Button(label).OnClick(func() {
				if client.MainGame.Dialog.OnOptionSelected != nil {
					client.MainGame.Dialog.OnOptionSelected(idx)
				} else {
					client.Client.SetResponseI(int32(idx))
					client.Client.SendResponse()
				}
				client.MainGame.Dialog.Hide()
			}),
		)
	}
	title := "Select an effect to activate"
	switch client.MainGame.DInfo.CurMsg {
	case network.MSG_ROCK_PAPER_SCISSORS:
		title = "Rock Paper Scissors"
	case network.MSG_ANNOUNCE_NUMBER:
		title = "Select a number"
	}
	content := tenon.Container(
		tenon.VStack(
			tenon.Text(title).FontSize(18).Color(white),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.VStack(buttons...).Gap(8).AlignItems(tenon.AlignStretch),
		).Gap(12).Padding(24).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(400)
	return s.animatedDialogContent(content, 400, 300)
}

func (s *DuelFieldScene) buildCardSelectDialog() engine.Widget {
	dialog := client.MainGame.Dialog
	title := dialog.SelectHint
	if title == "" {
		title = "Select cards"
	}
	cards := dialog.SelectCards

	cardWidgets := make([]tenon.Widget, 0, len(cards))
	for i, card := range cards {
		idx := i
		bg := cardGray
		if card.IsSelected {
			bg = color.RGBA{R: 100, G: 200, B: 100, A: 255}
		}
		var cardContent tenon.Widget
		cImg := cardImage(int(card.Code))
		if cImg != nil {
			cardContent = tenon.Image(cImg).Fit(tenon.ObjectFitCover)
		} else {
			cardContent = tenon.VStack(
				tenon.Text(fmt.Sprintf("%d", card.Code)).FontSize(10).Color(black),
			).AlignItems(tenon.AlignCenter).Justify(yoga.JustifyCenter)
		}
		cardWidgets = append(cardWidgets,
			tenon.Container(cardContent).
				W(60).H(80).Background(bg).CornerRadius(4).OnClick(func() {
				cards[idx].IsSelected = !cards[idx].IsSelected
			}),
		)
	}

	buttons := []tenon.Widget{}
	if dialog.SelectCancelable {
		buttons = append(buttons, tenon.Button("Cancel").Style(tenon.ButtonOutline).OnClick(func() {
			client.Client.SetResponseI(-1)
			client.Client.SendResponse()
			client.MainGame.DField.ClearSelect()
			client.MainGame.Dialog.Hide()
		}))
	}
	buttons = append(buttons, tenon.Button("OK").OnClick(func() {
		var resp []byte
		count := 0
		for _, c := range cards {
			if c.IsSelected {
				count++
			}
		}
		resp = append(resp, byte(count))

		if client.MainGame.DInfo.CurMsg == network.MSG_SELECT_SUM {
			// MSG_SELECT_SUM: must_select cards have select_seq=0,
			// optional cards use their index in the optional list
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
		client.MainGame.Dialog.Hide()
	}))

	content := tenon.Container(
		tenon.VStack(
			tenon.Text(title).FontSize(16).Color(white),
			tenon.Container(tenon.Spacer()).H(8),
			tenon.HStack(cardWidgets...).Gap(4),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.HStack(buttons...).Gap(12).Justify(yoga.JustifyCenter),
		).Gap(8).Padding(16).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(600)
	return s.animatedDialogContent(content, 600, 200)
}

func (s *DuelFieldScene) buildCmdBar() engine.Widget {
	df := client.MainGame.DField
	curMsg := client.MainGame.DInfo.CurMsg
	buttons := []tenon.Widget{}

	if curMsg == network.MSG_SELECT_IDLECMD {
		if df.ShowBP {
			buttons = append(buttons, tenon.Button("BP").OnClick(func() {
				client.MainGame.DField.ClearCommandFlag()
				client.Client.SetResponseI(6)
				client.Client.SendResponse()
			}))
		}
		if df.ShowEP {
			buttons = append(buttons, tenon.Button("EP").OnClick(func() {
				client.MainGame.DField.ClearCommandFlag()
				client.Client.SetResponseI(7)
				client.Client.SendResponse()
			}))
		}
		if df.ShowShuffle {
			buttons = append(buttons, tenon.Button("Shuffle").OnClick(func() {
				client.MainGame.DField.ClearCommandFlag()
				client.Client.SetResponseI(8)
				client.Client.SendResponse()
			}))
		}
	} else if curMsg == network.MSG_SELECT_BATTLECMD {
		if df.ShowM2 {
			buttons = append(buttons, tenon.Button("M2").OnClick(func() {
				client.MainGame.DField.ClearCommandFlag()
				client.Client.SetResponseI(2)
				client.Client.SendResponse()
			}))
		}
		if df.ShowEP {
			buttons = append(buttons, tenon.Button("EP").OnClick(func() {
				client.MainGame.DField.ClearCommandFlag()
				client.Client.SetResponseI(3)
				client.Client.SendResponse()
			}))
		}
	}

	if len(buttons) == 0 {
		return tenon.Container(tenon.Spacer())
	}

	return tenon.Container(
		tenon.HStack(buttons...).Gap(8).Justify(yoga.JustifyCenter),
	).Background(blackTransparent).Padding(4)
}

func (s *DuelFieldScene) buildChatOverlay() engine.Widget {
	if client.MainGame.HideChat {
		// Minimized chat - just input box
		return tenon.Container(
			tenon.Input("Chat...").Value(s.chatInput).FontSize(12).Background(blackTransparent).Border(white, 1).CornerRadius(4).Padding(4).OnChange(func(v string) {
				s.chatInput = v
			}).OnSubmit(func(v string) {
				if v != "" {
					client.Client.SendChat(v)
					s.chatInput = ""
				}
			}),
		).Background(blackTransparent).Padding(4).CornerRadius(6)
	}

	msgs := make([]tenon.Widget, 0, 8)
	for i := 7; i >= 0; i-- {
		msg := client.MainGame.ChatMsg[i]
		if msg == "" {
			continue
		}
		playerType := client.MainGame.ChatType[i]
		prefix := ""
		if playerType < 4 {
			prefix = fmt.Sprintf("P%d: ", playerType)
		} else if playerType < 8 {
			prefix = "OBS: "
		}
		msgs = append(msgs, tenon.Text(prefix+msg).FontSize(11).Color(white))
	}
	if len(msgs) == 0 {
		msgs = append(msgs, tenon.Text("").FontSize(11))
	}

	return tenon.Container(
		tenon.VStack(
			tenon.VStack(msgs...).Gap(2).AlignItems(tenon.AlignFlexStart),
			tenon.Container(tenon.Spacer()).H(4),
			tenon.Input("Chat...").Value(s.chatInput).FontSize(12).Background(blackTransparent).Border(white, 1).CornerRadius(4).Padding(4).OnChange(func(v string) {
				s.chatInput = v
			}).OnSubmit(func(v string) {
				if v != "" {
					client.Client.SendChat(v)
					s.chatInput = ""
				}
			}),
		).Gap(4),
	).Background(blackTransparent).Padding(6).CornerRadius(6)
}

func (s *DuelFieldScene) buildCmdSelectDialog() engine.Widget {
	dialog := client.MainGame.Dialog
	buttons := make([]tenon.Widget, 0, len(dialog.CmdSelectOptions))
	for i, opt := range dialog.CmdSelectOptions {
		idx := i
		buttons = append(buttons,
			tenon.Button(opt).OnClick(func() {
				client.MainGame.DField.ClearCommandFlag()
				client.Client.SetResponseI(dialog.CmdSelectResp[idx])
				client.Client.SendResponse()
				client.MainGame.Dialog.Hide()
			}),
		)
	}
	content := tenon.Container(
		tenon.VStack(
			tenon.Text("Select Command").FontSize(18).Color(white),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.VStack(buttons...).Gap(8).AlignItems(tenon.AlignStretch),
		).Gap(12).Padding(24).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(300)
	return s.animatedDialogContent(content, 300, 250)
}

func (s *DuelFieldScene) buildPositionDialog() engine.Widget {
	dialog := client.MainGame.Dialog
	buttons := []tenon.Widget{}

	posMap := []struct {
		flag  uint8
		label string
	}{
		{0x01, "Face-up ATK"},
		{0x02, "Face-down ATK"},
		{0x04, "Face-up DEF"},
		{0x08, "Face-down DEF"},
	}

	for _, p := range posMap {
		if dialog.PositionOptions&p.flag != 0 {
			val := int32(p.flag)
			buttons = append(buttons,
				tenon.Button(p.label).OnClick(func() {
					client.Client.SetResponseI(val)
					client.Client.SendResponse()
					client.MainGame.Dialog.Hide()
				}),
			)
		}
	}

	content := tenon.Container(
		tenon.VStack(
			tenon.Text(fmt.Sprintf("Select position for %d", dialog.PositionCode)).FontSize(16).Color(white),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.VStack(buttons...).Gap(8).AlignItems(tenon.AlignStretch),
		).Gap(12).Padding(24).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(300)
	return s.animatedDialogContent(content, 300, 220)
}

func (s *DuelFieldScene) buildSortDialog() engine.Widget {
	dialog := client.MainGame.Dialog
	cards := dialog.SortCards
	order := dialog.SortOrder

	cardWidgets := make([]tenon.Widget, 0, len(cards))
	for i, card := range cards {
		idx := i
		bg := cardGray
		sortText := ""
		if order[idx] > 0 {
			bg = color.RGBA{R: 100, G: 200, B: 100, A: 255}
			sortText = fmt.Sprintf("#%d", order[idx])
		}
		var cardContent tenon.Widget
		cImg := cardImage(int(card.Code))
		if cImg != nil {
			cardContent = tenon.Stack(
				tenon.Image(cImg).Fit(tenon.ObjectFitCover),
				tenon.Positioned(tenon.Container(tenon.Text(sortText).FontSize(14).Color(white)).Background(blackTransparent).Padding(2)).L(2).B(2),
			)
		} else {
			cardContent = tenon.VStack(
				tenon.Text(fmt.Sprintf("%d", card.Code)).FontSize(10).Color(black),
				tenon.Text(sortText).FontSize(12).Color(black),
			).AlignItems(tenon.AlignCenter).Justify(yoga.JustifyCenter)
		}
		cardWidgets = append(cardWidgets,
			tenon.Container(cardContent).
				W(60).H(80).Background(bg).CornerRadius(4).OnClick(func() {
				if order[idx] > 0 {
					// Unassign
					removed := order[idx]
					order[idx] = 0
					dialog.SortCur--
					for j := range order {
						if order[j] > removed {
							order[j]--
						}
					}
				} else {
					// Assign next number
					order[idx] = dialog.SortCur
					dialog.SortCur++
					if dialog.SortCur > len(cards)+1 {
						dialog.SortCur = len(cards) + 1
					}
					// Auto-send when all assigned
					assigned := 0
					for _, v := range order {
						if v > 0 {
							assigned++
						}
					}
					if assigned == len(cards) {
						resp := make([]byte, len(cards))
						for j, v := range order {
							resp[j] = byte(v - 1)
						}
						client.Client.SetResponseB(resp)
						client.Client.SendResponse()
						client.MainGame.Dialog.Hide()
					}
				}
			}),
		)
	}

	content := tenon.Container(
		tenon.VStack(
			tenon.Text("Sort cards (click in order)").FontSize(16).Color(white),
			tenon.Container(tenon.Spacer()).H(8),
			tenon.HStack(cardWidgets...).Gap(4),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.Button("Cancel").Style(tenon.ButtonOutline).OnClick(func() {
				client.Client.SetResponseI(-1)
				client.Client.SendResponse()
				client.MainGame.Dialog.Hide()
			}),
		).Gap(8).Padding(16).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(600)
	return s.animatedDialogContent(content, 600, 200)
}

func (s *DuelFieldScene) buildRaceDialog() engine.Widget {
	avail := client.MainGame.Dialog.AnnounceRaceAvail
	raceMap := []struct {
		bit   uint32
		label string
	}{
		{0x1, "Warrior"},
		{0x2, "Spellcaster"},
		{0x4, "Fairy"},
		{0x8, "Fiend"},
		{0x10, "Zombie"},
		{0x20, "Machine"},
		{0x40, "Aqua"},
		{0x80, "Pyro"},
		{0x100, "Rock"},
		{0x200, "Winged Beast"},
		{0x400, "Plant"},
		{0x800, "Insect"},
		{0x1000, "Thunder"},
		{0x2000, "Dragon"},
		{0x4000, "Beast"},
		{0x8000, "Beast-Warrior"},
		{0x10000, "Dinosaur"},
		{0x20000, "Fish"},
		{0x40000, "Sea Serpent"},
		{0x80000, "Reptile"},
		{0x100000, "Psychic"},
		{0x200000, "Divine"},
		{0x400000, "Creator God"},
		{0x800000, "Wyrm"},
		{0x1000000, "Cyberse"},
	}

	selected := uint32(0)
	buttons := make([]tenon.Widget, 0)
	for _, r := range raceMap {
		if avail&r.bit == 0 {
			continue
		}
		bit := r.bit
		buttons = append(buttons,
			tenon.Button(r.label).OnClick(func() {
				selected |= bit
				client.Client.SetResponseI(int32(selected))
				client.Client.SendResponse()
				client.MainGame.Dialog.Hide()
			}),
		)
	}
	content := tenon.Container(
		tenon.VStack(
			tenon.Text("Select Race").FontSize(16).Color(white),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.VStack(buttons...).Gap(4).AlignItems(tenon.AlignStretch),
		).Gap(12).Padding(24).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(300)
	return s.animatedDialogContent(content, 300, 350)
}

func (s *DuelFieldScene) buildAttribDialog() engine.Widget {
	avail := client.MainGame.Dialog.AnnounceAttribAvail
	attribMap := []struct {
		bit   uint32
		label string
	}{
		{0x01, "EARTH"},
		{0x02, "WATER"},
		{0x04, "FIRE"},
		{0x08, "WIND"},
		{0x10, "LIGHT"},
		{0x20, "DARK"},
		{0x40, "DIVINE"},
	}

	selected := uint32(0)
	buttons := make([]tenon.Widget, 0)
	for _, a := range attribMap {
		if avail&a.bit == 0 {
			continue
		}
		bit := a.bit
		buttons = append(buttons,
			tenon.Button(a.label).OnClick(func() {
				selected |= bit
				client.Client.SetResponseI(int32(selected))
				client.Client.SendResponse()
				client.MainGame.Dialog.Hide()
			}),
		)
	}
	content := tenon.Container(
		tenon.VStack(
			tenon.Text("Select Attribute").FontSize(16).Color(white),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.VStack(buttons...).Gap(4).AlignItems(tenon.AlignStretch),
		).Gap(12).Padding(24).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(300)
	return s.animatedDialogContent(content, 300, 280)
}

func (s *DuelFieldScene) buildAnnounceCardDialog() engine.Widget {
	// Without card database, we can't filter by opcodes.
	// Show a simple input placeholder.
	content := tenon.Container(
		tenon.VStack(
			tenon.Text("Announce Card (placeholder)").FontSize(16).Color(white),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.Text("Card database not available").FontSize(12).Color(gray),
			tenon.Container(tenon.Spacer()).H(12),
			tenon.Button("Cancel").Style(tenon.ButtonOutline).OnClick(func() {
				client.Client.SetResponseI(0)
				client.Client.SendResponse()
				client.MainGame.Dialog.Hide()
			}),
		).Gap(12).Padding(24).AlignItems(tenon.AlignCenter),
	).Background(white).CornerRadius(12).W(300)
	return s.animatedDialogContent(content, 300, 180)
}

func phaseName(phase uint16) string {
	switch phase {
	case 0x01:
		return "DP"
	case 0x02:
		return "SP"
	case 0x04:
		return "M1"
	case 0x08:
		return "BP Start"
	case 0x10:
		return "BP Step"
	case 0x20:
		return "Damage"
	case 0x40:
		return "Damage Calc"
	case 0x80:
		return "BP"
	case 0x100:
		return "M2"
	case 0x200:
		return "EP"
	default:
		return ""
	}
}
