package client

import (
	"encoding/binary"
	"math/rand"
)

// ChainInfo corresponds to C++ struct ChainInfo in client_field.h
type ChainInfo struct {
	ChainPos    [3]float32
	ChainCard   *ClientCard
	Code        int
	Desc        int
	Controler   int
	Location    int
	Sequence    int
	Solved      bool
	Target      map[*ClientCard]struct{}
}

// ClientField corresponds to C++ class ClientField in client_field.h
// Holds the complete state of both players' fields.
type ClientField struct {
	Deck    [2][]*ClientCard
	Hand    [2][]*ClientCard
	MZone   [2][]*ClientCard
	SZone   [2][]*ClientCard
	Grave   [2][]*ClientCard
	Remove  [2][]*ClientCard
	Extra   [2][]*ClientCard
	OverlayCards map[*ClientCard]struct{}

	SummonableCards    []*ClientCard
	SPSummonableCards  []*ClientCard
	MSetableCards      []*ClientCard
	SSetableCards      []*ClientCard
	ReposableCards     []*ClientCard
	ActivatableCards   []*ClientCard
	AttackableCards    []*ClientCard
	ContiCards         []*ClientCard
	ActivatableDescs   [][2]int
	SelectOptions      []int
	SelectOptionsIndex []int
	Chains             []ChainInfo
	ExtraPCount        [2]int

	SelectedOption     int
	Attacker           *ClientCard
	AttackTarget       *ClientCard
	DisabledField      uint32
	SelectableField    uint32
	SelectedField      uint32
	SelectMin          int
	SelectMax          int
	MustSelectCount    int
	SelectCurValL      int
	SelectCurValH      int
	SelectSumVal       int
	SelectMode         int
	SelectHint         int
	SelectCancelable   bool
	SelectPanelMode    bool
	SelectReady        bool
	AnnounceCount      int
	SelectCounterCount int
	SelectCounterType  int
	SelectableCards    []*ClientCard
	SelectedCards      []*ClientCard
	SelectSumCards     map[*ClientCard]struct{}
	SelectSumAll       []*ClientCard
	DeclareOpcodes     []uint32
	DisplayCards       []*ClientCard
	SortList           []int
	PlayerDescHints    [2]map[int]int

	GraveAct   [2]bool
	RemoveAct  [2]bool
	DeckAct    [2]bool
	ExtraAct   [2]bool
	PZoneAct   [2]bool
	ContiAct   bool
	ChainForced bool
	CurrentChain ChainInfo
	LastChain   bool
	DeckReversed bool
	SelectContinuous bool
	CantCheckGrave bool
	TagSurrender bool
	TagTeammateSurrender bool

	ShowBP     bool
	ShowM2     bool
	ShowEP     bool
	ShowShuffle bool

	rnd *rand.Rand
}

func NewClientField() *ClientField {
	return &ClientField{
		OverlayCards:     make(map[*ClientCard]struct{}),
		SelectSumCards:   make(map[*ClientCard]struct{}),
		PlayerDescHints:  [2]map[int]int{make(map[int]int), make(map[int]int)},
		rnd:              rand.New(rand.NewSource(1)),
		SelectedOption:   0,
		DisabledField:    0,
		SelectableField:  0,
		SelectedField:    0,
	}
}

func (cf *ClientField) Clear() {
	cf.Deck = [2][]*ClientCard{}
	cf.Hand = [2][]*ClientCard{}
	cf.MZone = [2][]*ClientCard{}
	cf.SZone = [2][]*ClientCard{}
	cf.Grave = [2][]*ClientCard{}
	cf.Remove = [2][]*ClientCard{}
	cf.Extra = [2][]*ClientCard{}
	cf.OverlayCards = make(map[*ClientCard]struct{})
	cf.SummonableCards = nil
	cf.SPSummonableCards = nil
	cf.MSetableCards = nil
	cf.SSetableCards = nil
	cf.ReposableCards = nil
	cf.ActivatableCards = nil
	cf.AttackableCards = nil
	cf.ContiCards = nil
	cf.ActivatableDescs = nil
	cf.SelectOptions = nil
	cf.SelectOptionsIndex = nil
	cf.Chains = nil
	cf.ExtraPCount = [2]int{}
	cf.SelectedOption = 0
	cf.Attacker = nil
	cf.AttackTarget = nil
	cf.DisabledField = 0
	cf.SelectableField = 0
	cf.SelectedField = 0
	cf.SelectMin = 0
	cf.SelectMax = 0
	cf.MustSelectCount = 0
	cf.SelectableCards = nil
	cf.SelectedCards = nil
	cf.SelectSumCards = make(map[*ClientCard]struct{})
	cf.SelectSumAll = nil
	cf.DeclareOpcodes = nil
	cf.DisplayCards = nil
	cf.SortList = nil
	cf.PlayerDescHints = [2]map[int]int{make(map[int]int), make(map[int]int)}
	cf.GraveAct = [2]bool{}
	cf.RemoveAct = [2]bool{}
	cf.DeckAct = [2]bool{}
	cf.ExtraAct = [2]bool{}
	cf.PZoneAct = [2]bool{}
	cf.ContiAct = false
	cf.ChainForced = false
	cf.LastChain = false
	cf.DeckReversed = false
	cf.SelectContinuous = false
	cf.CantCheckGrave = false
	cf.TagSurrender = false
	cf.TagTeammateSurrender = false
	cf.ShowBP = false
	cf.ShowM2 = false
	cf.ShowEP = false
	cf.ShowShuffle = false
}

func (cf *ClientField) Initial(player, deckc, extrac, sidec int) {
	cf.Deck[player] = make([]*ClientCard, deckc)
	for i := range cf.Deck[player] {
		cf.Deck[player][i] = NewClientCard()
		cf.Deck[player][i].Controler = uint8(player)
		cf.Deck[player][i].Location = 0x01
		cf.Deck[player][i].Sequence = uint8(i)
	}
	cf.Hand[player] = make([]*ClientCard, 0)
	cf.MZone[player] = make([]*ClientCard, 7)
	cf.SZone[player] = make([]*ClientCard, 8)
	cf.Grave[player] = make([]*ClientCard, 0)
	cf.Remove[player] = make([]*ClientCard, 0)
	cf.Extra[player] = make([]*ClientCard, extrac)
	for i := range cf.Extra[player] {
		cf.Extra[player][i] = NewClientCard()
		cf.Extra[player][i].Controler = uint8(player)
		cf.Extra[player][i].Location = 0x40
		cf.Extra[player][i].Sequence = uint8(i)
	}
}

func (cf *ClientField) GetCard(controler int, location uint8, sequence int, subSeq ...int) *ClientCard {
	var list []*ClientCard
	switch location {
	case 0x01: // LOCATION_DECK
		list = cf.Deck[controler]
	case 0x02: // LOCATION_HAND
		list = cf.Hand[controler]
	case 0x04: // LOCATION_MZONE
		list = cf.MZone[controler]
	case 0x08: // LOCATION_SZONE
		list = cf.SZone[controler]
	case 0x10: // LOCATION_GRAVE
		list = cf.Grave[controler]
	case 0x20: // LOCATION_REMOVED
		list = cf.Remove[controler]
	case 0x40: // LOCATION_EXTRA
		list = cf.Extra[controler]
	case 0x80: // LOCATION_OVERLAY
		if sequence < len(cf.MZone[controler]) && cf.MZone[controler][sequence] != nil {
			card := cf.MZone[controler][sequence]
			sub := 0
			if len(subSeq) > 0 {
				sub = subSeq[0]
			}
			if sub < len(card.Overlayed) {
				return card.Overlayed[sub]
			}
		}
		return nil
	}
	if sequence >= 0 && sequence < len(list) {
		return list[sequence]
	}
	return nil
}

// AddCard adds a card to the specified location.
func (cf *ClientField) AddCard(pcard *ClientCard, controler int, location uint8, sequence int) {
	pcard.Controler = uint8(controler)
	pcard.Location = location
	pcard.Sequence = uint8(sequence)
	switch location {
	case 0x01: // DECK
		if sequence != 0 || len(cf.Deck[controler]) == 0 {
			cf.Deck[controler] = append(cf.Deck[controler], pcard)
		} else {
			cf.Deck[controler] = append([]*ClientCard{pcard}, cf.Deck[controler]...)
		}
		cf.resetSequence(cf.Deck[controler], true)
		pcard.IsReversed = false
		pcard.ClearData()
		pcard.ClearTarget()
	case 0x02: // HAND
		cf.Hand[controler] = append(cf.Hand[controler], pcard)
		cf.resetSequence(cf.Hand[controler], false)
	case 0x04: // MZONE
		cf.MZone[controler][sequence] = pcard
	case 0x08: // SZONE
		cf.SZone[controler][sequence] = pcard
	case 0x10: // GRAVE
		cf.Grave[controler] = append(cf.Grave[controler], pcard)
		pcard.Sequence = uint8(len(cf.Grave[controler]) - 1)
	case 0x20: // REMOVED
		cf.Remove[controler] = append(cf.Remove[controler], pcard)
		pcard.Sequence = uint8(len(cf.Remove[controler]) - 1)
	case 0x40: // EXTRA
		if cf.ExtraPCount[controler] == 0 || (pcard.Position&0x05) != 0 {
			cf.Extra[controler] = append(cf.Extra[controler], pcard)
		} else {
			faceupBegin := len(cf.Extra[controler]) - cf.ExtraPCount[controler]
			cf.Extra[controler] = append(cf.Extra[controler][:faceupBegin], append([]*ClientCard{pcard}, cf.Extra[controler][faceupBegin:]...)...)
		}
		cf.resetSequence(cf.Extra[controler], true)
		if pcard.Position&0x05 != 0 {
			cf.ExtraPCount[controler]++
		}
	}
}

// RemoveCard removes a card from the specified location.
func (cf *ClientField) RemoveCard(controler int, location uint8, sequence int) *ClientCard {
	var pcard *ClientCard
	switch location {
	case 0x01: // DECK
		if sequence < len(cf.Deck[controler]) {
			pcard = cf.Deck[controler][sequence]
			for i := sequence; i < len(cf.Deck[controler])-1; i++ {
				cf.Deck[controler][i] = cf.Deck[controler][i+1]
				cf.Deck[controler][i].Sequence--
			}
			cf.Deck[controler] = cf.Deck[controler][:len(cf.Deck[controler])-1]
		}
	case 0x02: // HAND
		if sequence < len(cf.Hand[controler]) {
			pcard = cf.Hand[controler][sequence]
			cf.Hand[controler] = append(cf.Hand[controler][:sequence], cf.Hand[controler][sequence+1:]...)
			cf.resetSequence(cf.Hand[controler], false)
		}
	case 0x04: // MZONE
		if sequence < len(cf.MZone[controler]) {
			pcard = cf.MZone[controler][sequence]
			cf.MZone[controler][sequence] = nil
		}
	case 0x08: // SZONE
		if sequence < len(cf.SZone[controler]) {
			pcard = cf.SZone[controler][sequence]
			cf.SZone[controler][sequence] = nil
		}
	case 0x10: // GRAVE
		if sequence < len(cf.Grave[controler]) {
			pcard = cf.Grave[controler][sequence]
			cf.Grave[controler] = append(cf.Grave[controler][:sequence], cf.Grave[controler][sequence+1:]...)
			cf.resetSequence(cf.Grave[controler], true)
		}
	case 0x20: // REMOVED
		if sequence < len(cf.Remove[controler]) {
			pcard = cf.Remove[controler][sequence]
			cf.Remove[controler] = append(cf.Remove[controler][:sequence], cf.Remove[controler][sequence+1:]...)
			cf.resetSequence(cf.Remove[controler], true)
		}
	case 0x40: // EXTRA
		if sequence < len(cf.Extra[controler]) {
			pcard = cf.Extra[controler][sequence]
			if pcard.Position&0x05 != 0 {
				cf.ExtraPCount[controler]--
			}
			cf.Extra[controler] = append(cf.Extra[controler][:sequence], cf.Extra[controler][sequence+1:]...)
			cf.resetSequence(cf.Extra[controler], true)
		}
	}
	return pcard
}

// UpdateCard updates a single card's info.
func (cf *ClientField) UpdateCard(controler int, location uint8, sequence int, data []byte) {
	pcard := cf.GetCard(controler, location, sequence)
	if pcard != nil && len(data) > 4 {
		pcard.UpdateInfo(data)
	}
}

// UpdateFieldCard updates all cards in a field location.
func (cf *ClientField) UpdateFieldCard(controler int, location uint8, data []byte) {
	var lst []*ClientCard
	switch location {
	case 0x01:
		lst = cf.Deck[controler]
	case 0x02:
		lst = cf.Hand[controler]
	case 0x04:
		lst = cf.MZone[controler]
	case 0x08:
		lst = cf.SZone[controler]
	case 0x10:
		lst = cf.Grave[controler]
	case 0x20:
		lst = cf.Remove[controler]
	case 0x40:
		lst = cf.Extra[controler]
	}
	if lst == nil {
		return
	}
	offset := 0
	for _, card := range lst {
		if card == nil || offset+4 > len(data) {
			continue
		}
		length := int(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
		if length > 4 && offset+length-4 <= len(data) {
			card.UpdateInfo(data[offset : offset+length-4])
		}
		offset += length - 4
	}
}

// swapCards swaps two cards in the field.
func (cf *ClientField) swapCards(c1 int, l1 uint8, s1 int, c2 int, l2 uint8, s2 int) {
	card1 := cf.GetCard(c1, l1, s1)
	card2 := cf.GetCard(c2, l2, s2)
	if card1 == nil || card2 == nil {
		return
	}
	// Swap in their respective lists
	switch l1 {
	case 0x04:
		cf.MZone[c1][s1] = card2
	case 0x08:
		cf.SZone[c1][s1] = card2
	}
	switch l2 {
	case 0x04:
		cf.MZone[c2][s2] = card1
	case 0x08:
		cf.SZone[c2][s2] = card1
	}
}

func (cf *ClientField) ClearSelect() {
	for _, c := range cf.SelectableCards {
		c.IsSelectable = false
		c.IsSelected = false
	}
	cf.SelectableCards = nil
	cf.SelectedCards = nil
	cf.SelectMin = 0
	cf.SelectMax = 0
	cf.SelectCancelable = false
	cf.SelectReady = false
	cf.SelectSumCards = make(map[*ClientCard]struct{})
	cf.SelectSumAll = nil
}

func (cf *ClientField) ClearChainSelect() {
	for _, c := range cf.ActivatableCards {
		c.IsSelectable = false
		c.IsSelected = false
	}
	cf.ActivatableCards = nil
	cf.ActivatableDescs = nil
	cf.ContiCards = nil
	cf.ContiAct = false
	cf.ChainForced = false
}

func (cf *ClientField) ClearCommandFlag() {
	for _, c := range cf.ActivatableCards {
		c.CmdFlag = 0
	}
	for _, c := range cf.SummonableCards {
		c.CmdFlag = 0
	}
	for _, c := range cf.SPSummonableCards {
		c.CmdFlag = 0
	}
	for _, c := range cf.MSetableCards {
		c.CmdFlag = 0
	}
	for _, c := range cf.SSetableCards {
		c.CmdFlag = 0
	}
	for _, c := range cf.ReposableCards {
		c.CmdFlag = 0
	}
	for _, c := range cf.AttackableCards {
		c.CmdFlag = 0
	}
}

func (cf *ClientField) resetSequence(list []*ClientCard, resetHeight bool) {
	for i, card := range list {
		if card != nil {
			card.Sequence = uint8(i)
		}
	}
}
