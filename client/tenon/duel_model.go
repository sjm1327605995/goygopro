package tenon

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	locationDeck   = 0x01
	locationHand   = 0x02
	locationMZone  = 0x04
	locationSZone  = 0x08
	locationGrave  = 0x10
	locationBanish = 0x20
	locationExtra  = 0x40
)

type DuelCard struct {
	Code     uint32
	Position uint8
}

type DuelPiles struct {
	Deck, Grave, Banish, Extra int
}

type DuelSide struct {
	MZone  [7]*DuelCard
	SZone  [8]*DuelCard
	Grave  []DuelCard
	Banish []DuelCard
}

type CommandAction struct {
	Code     uint32
	Player   int
	Location int
	Sequence int
	Index    int
	Desc     uint32
	Direct   bool
}

type DuelPrompt struct {
	Mode     string
	Summon   []CommandAction
	SPSummon []CommandAction
	Repos    []CommandAction
	MSet     []CommandAction
	SSet     []CommandAction
	Activate []CommandAction
	Attack   []CommandAction
	ToBP     bool
	ToM2     bool
	ToEP     bool
	Shuffle  bool
}

type PlaceZone struct {
	Player   int
	Seat     int
	Location int
	Sequence int
}

type PlacePrompt struct {
	Player     int
	Count      int
	Cancelable bool
	Disfield   bool
	Zones      []PlaceZone
	Selected   []int
}

type CardChoice struct {
	Code          uint32
	Player        int
	Seat          int
	Location      int
	Sequence      int
	ResponseIndex int
}

type CardSelectionPrompt struct {
	Kind       string
	Min, Max   int
	Cancelable bool
	Choices    []CardChoice
	Selected   []int
}

type DuelModel struct {
	PlayerSlot    int
	Names         [2]string
	LP            [2]int32
	Piles         [2]DuelPiles
	Hands         [2][]uint32
	Board         [2]DuelSide
	Turn          int
	TurnPlayer    int
	Phase         uint16
	Shuffle       [2]int
	Message       string
	Prompt        DuelPrompt
	Placement     *PlacePrompt
	CardSelection *CardSelectionPrompt
	BoardRevision int
	Revision      int
	onChange      func()
}

func NewDuelModel(onChange func()) *DuelModel {
	m := &DuelModel{onChange: onChange}
	m.reset()
	return m
}

func (m *DuelModel) reset() {
	m.PlayerSlot = 0
	m.Names = [2]string{"决斗者 / DUELIST", "挑战者 / CHALLENGER"}
	m.LP = [2]int32{8000, 8000}
	m.Piles = [2]DuelPiles{}
	m.Hands = [2][]uint32{}
	m.Board = [2]DuelSide{}
	m.Turn = 0
	m.TurnPlayer = 0
	m.Phase = 0
	m.Shuffle = [2]int{}
	m.Prompt = DuelPrompt{}
	m.Placement = nil
	m.CardSelection = nil
	m.BoardRevision = 0
	m.Message = ""
}

func (m *DuelModel) changed() {
	m.Revision++
	if m.onChange != nil {
		m.onChange()
	}
}

func (m *DuelModel) consumePrompt() {
	m.Prompt = DuelPrompt{}
	m.changed()
}

func (m *DuelModel) displaySeat(seat int) int {
	return (seat - m.PlayerSlot + 2) % 2
}

func (m *DuelModel) Attach(bus *EventBus) {
	for _, name := range []string{
		"duel:start", "duel:reload_field", "duel:update_data", "duel:update_card",
		"duel:summoning", "duel:spsummoning", "duel:flipsummoning", "duel:set",
		"duel:pos_change", "duel:move", "duel:draw", "duel:new_turn", "duel:new_phase", "duel:shuffle_hand",
		"duel:damage", "duel:recover", "duel:pay_lpcost", "duel:lp_update",
		"duel:ai_name", "duel:show_hint", "duel:waiting", "stoc:player_enter", "duel:select_idlecmd", "duel:select_battlecmd", "duel:select_place", "duel:select_card", "duel:select_unselect",
	} {
		name := name
		bus.On(name, func(event Event) { m.Apply(name, event.Data) })
	}
}

func (m *DuelModel) Apply(name string, data any) {
	boardChanged := false
	switch name {
	case "duel:start":
		boardChanged = true
		m.reset()
		m.PlayerSlot = eventInt(data, "playerType", 0) & 0x0f
		m.LP = [2]int32{int32(eventInt(data, "lp0", 8000)), int32(eventInt(data, "lp1", 8000))}
		m.Piles[0].Deck = eventInt(data, "deck0", 0)
		m.Piles[1].Deck = eventInt(data, "deck1", 0)
		m.Piles[0].Extra = eventInt(data, "extra0", 0)
		m.Piles[1].Extra = eventInt(data, "extra1", 0)
		m.Message = "对局开始"
	case "duel:reload_field":
		boardChanged = true
		m.applyReload(data)
	case "duel:update_data":
		boardChanged = true
		m.applyUpdateData(data)
	case "duel:update_card":
		boardChanged = true
		m.applyUpdateCard(data)
	case "duel:summoning", "duel:spsummoning", "duel:flipsummoning", "duel:set":
		boardChanged = true
		seat := m.displaySeat(eventInt(data, "cc", 0))
		m.putCard(seat, eventInt(data, "cl", 0), eventInt(data, "cs", 0), DuelCard{
			Code: uint32(eventInt(data, "code", 0)), Position: uint8(eventInt(data, "cp", 1)),
		})
	case "duel:pos_change":
		boardChanged = true
		seat := m.displaySeat(eventInt(data, "cc", 0))
		m.putCard(seat, eventInt(data, "cl", 0), eventInt(data, "cs", 0), DuelCard{
			Code: uint32(eventInt(data, "code", 0)), Position: uint8(eventInt(data, "cp", 1)),
		})
	case "duel:move":
		boardChanged = true
		m.applyMove(data)
	case "duel:draw":
		boardChanged = true
		seat := m.displaySeat(eventInt(data, "player", 0))
		codes := eventIntSlice(data, "cards")
		m.Hands[seat] = append(m.Hands[seat], codes...)
		m.Piles[seat].Deck = max(0, m.Piles[seat].Deck-len(codes))
	case "duel:new_turn":
		m.Prompt = DuelPrompt{}
		m.Placement = nil
		m.CardSelection = nil
		m.Turn++
		m.TurnPlayer = m.displaySeat(eventInt(data, "player", 0))
		m.Message = "新的回合"
	case "duel:new_phase":
		m.Prompt = DuelPrompt{}
		m.Placement = nil
		m.CardSelection = nil
		m.Phase = uint16(eventInt(data, "phase", 0))
	case "duel:shuffle_hand":
		seat := m.displaySeat(eventInt(data, "player", 0))
		m.Shuffle[seat]++
	case "duel:damage", "duel:pay_lpcost":
		seat := m.displaySeat(eventInt(data, "player", 0))
		m.LP[seat] = max(0, m.LP[seat]-int32(eventInt(data, "amount", 0)))
	case "duel:recover":
		seat := m.displaySeat(eventInt(data, "player", 0))
		m.LP[seat] += int32(eventInt(data, "amount", 0))
	case "duel:lp_update":
		seat := m.displaySeat(eventInt(data, "player", 0))
		m.LP[seat] = int32(eventInt(data, "lp", 0))
	case "duel:select_card":
		m.beginCardSelection(data, false)
	case "duel:select_unselect":
		m.beginCardSelection(data, true)
	case "duel:select_place":
		m.Prompt = DuelPrompt{}
		m.CardSelection = nil
		player := eventInt(data, "player", 0)
		count := eventInt(data, "count", 0)
		zones := eventSlice(data, "zones")
		placement := &PlacePrompt{
			Player: player, Count: max(1, count), Cancelable: count == 0,
			Disfield: eventBool(data, "disfield"), Zones: make([]PlaceZone, 0, len(zones)),
		}
		for _, zone := range zones {
			enginePlayer := eventInt(zone, "player", player)
			placement.Zones = append(placement.Zones, PlaceZone{
				Player: enginePlayer, Seat: m.displaySeat(enginePlayer),
				Location: eventInt(zone, "loc", 0), Sequence: eventInt(zone, "seq", 0),
			})
		}
		m.Placement = placement
		m.Message = "请选择卡片的位置"
		if placement.Disfield {
			m.Message = "请选择要变成不能使用的区域"
		}
	case "duel:select_idlecmd":
		m.Placement = nil
		m.CardSelection = nil
		m.Prompt = DuelPrompt{
			Mode: "idle", Summon: commandActions(data, "summon"), SPSummon: commandActions(data, "spsummon"),
			Repos: commandActions(data, "repos"), MSet: commandActions(data, "mset"), SSet: commandActions(data, "sset"),
			Activate: commandActions(data, "activate"), ToBP: eventBool(data, "toBP"), ToEP: eventBool(data, "toEP"), Shuffle: eventBool(data, "shuffle"),
		}
	case "duel:select_battlecmd":
		m.Placement = nil
		m.CardSelection = nil
		m.Prompt = DuelPrompt{
			Mode: "battle", Activate: commandActions(data, "activate"), Attack: commandActions(data, "attack"),
			ToM2: eventBool(data, "toM2"), ToEP: eventBool(data, "toEP"),
		}
	case "duel:ai_name":
		m.Names[m.displaySeat(1)] = eventString(data, "name", m.Names[m.displaySeat(1)])
	case "duel:show_hint":
		m.Message = eventString(data, "text", "")
	case "duel:waiting":
		m.Prompt = DuelPrompt{}
		m.Placement = nil
		m.CardSelection = nil
		m.Message = "等待对方行动…"
	case "stoc:player_enter":
		seat := m.displaySeat(eventInt(data, "pos", 0))
		m.Names[seat] = eventString(data, "name", m.Names[seat])
	default:
		return
	}
	if boardChanged {
		m.BoardRevision++
	}
	m.changed()
}

func (m *DuelModel) beginCardSelection(data any, unselect bool) {
	m.Prompt = DuelPrompt{}
	m.Placement = nil
	selection := &CardSelectionPrompt{Kind: "card"}
	if unselect {
		selection.Kind = "unselect"
		selection.Min, selection.Max = 1, 1
		selection.Cancelable = eventBool(data, "cancelable") || eventBool(data, "finishable")
		selection.Choices = m.cardChoices(eventSlice(data, "cards"), 0)
		selection.Choices = append(selection.Choices, m.cardChoices(eventSlice(data, "unselectList"), len(selection.Choices))...)
	} else {
		selection.Min = max(0, eventInt(data, "min", 1))
		selection.Max = max(selection.Min, eventInt(data, "max", 1))
		selection.Cancelable = eventBool(data, "cancelable")
		selection.Choices = m.cardChoices(eventSlice(data, "cards"), 0)
	}
	m.CardSelection = selection
	m.updateCardSelectionMessage()
}

func (m *DuelModel) cardChoices(cards []any, responseOffset int) []CardChoice {
	choices := make([]CardChoice, 0, len(cards))
	for index, card := range cards {
		player := eventInt(card, "c", 0)
		choices = append(choices, CardChoice{
			Code: uint32(eventInt(card, "code", 0)), Player: player, Seat: m.displaySeat(player),
			Location: eventInt(card, "l", 0), Sequence: eventInt(card, "s", 0), ResponseIndex: responseOffset + index,
		})
	}
	return choices
}

func (m *DuelModel) updateCardSelectionMessage() {
	selection := m.CardSelection
	if selection == nil {
		return
	}
	m.Message = "请选择卡片"
	if selection.Max > 1 || selection.Min != selection.Max {
		m.Message += fmt.Sprintf("（已选 %d，需要 %d-%d）", len(selection.Selected), selection.Min, selection.Max)
	}
}

func (m *DuelModel) applyReload(data any) {
	players := eventSlice(data, "players")
	for seat, value := range players {
		if seat > 1 {
			break
		}
		display := m.displaySeat(seat)
		m.LP[display] = int32(eventInt(value, "lp", 8000))
		m.Piles[display] = DuelPiles{
			Deck: eventInt(value, "deck", 0), Grave: eventInt(value, "grave", 0),
			Banish: eventInt(value, "removed", 0), Extra: eventInt(value, "extra", 0),
		}
	}
}

func (m *DuelModel) applyUpdateData(data any) {
	seat := m.displaySeat(eventInt(data, "player", 0))
	location := eventInt(data, "location", 0)
	queries := eventSlice(data, "cards")
	if location == locationHand {
		hand := make([]uint32, len(queries))
		for i, query := range queries {
			hand[i] = uint32(eventInt(query, "code", 0))
		}
		m.Hands[seat] = hand
		return
	}
	if location == locationGrave || location == locationBanish {
		stack := make([]DuelCard, 0, len(queries))
		for _, query := range queries {
			if eventNil(query) {
				continue
			}
			stack = append(stack, duelCardFromQuery(query))
		}
		if location == locationGrave {
			m.Board[seat].Grave = stack
		} else {
			m.Board[seat].Banish = stack
		}
		return
	}
	for sequence, query := range queries {
		if eventNil(query) {
			m.removeCard(seat, location, sequence)
			continue
		}
		m.putCard(seat, location, sequence, duelCardFromQuery(query))
	}
}

func (m *DuelModel) applyUpdateCard(data any) {
	seat := m.displaySeat(eventInt(data, "player", 0))
	m.putCard(seat, eventInt(data, "location", 0), eventInt(data, "sequence", 0), duelCardFromQuery(data))
}

func (m *DuelModel) applyMove(data any) {
	fromSeat := m.displaySeat(eventInt(data, "pc", 0))
	toSeat := m.displaySeat(eventInt(data, "cc", 0))
	fromLocation := eventInt(data, "pl", 0)
	toLocation := eventInt(data, "cl", 0)
	fromSequence := eventInt(data, "ps", 0)
	toSequence := eventInt(data, "cs", 0)
	card := DuelCard{Code: uint32(eventInt(data, "code", 0)), Position: uint8(eventInt(data, "cp", 1))}
	if existing := m.cardAt(fromSeat, fromLocation, fromSequence); existing != nil {
		card = *existing
		card.Position = uint8(eventInt(data, "cp", int(card.Position)))
	}
	m.removeCard(fromSeat, fromLocation, fromSequence)
	m.putCard(toSeat, toLocation, toSequence, card)
	m.adjustPile(fromSeat, fromLocation, -1)
	m.adjustPile(toSeat, toLocation, 1)
}

func (m *DuelModel) adjustPile(seat, location, delta int) {
	var pile *int
	switch location {
	case locationDeck:
		pile = &m.Piles[seat].Deck
	case locationGrave:
		pile = &m.Piles[seat].Grave
	case locationBanish:
		pile = &m.Piles[seat].Banish
	case locationExtra:
		pile = &m.Piles[seat].Extra
	}
	if pile != nil {
		*pile = max(0, *pile+delta)
	}
}

func (m *DuelModel) putCard(seat, location, sequence int, card DuelCard) {
	if sequence < 0 {
		return
	}
	switch location {
	case locationMZone:
		if sequence < len(m.Board[seat].MZone) {
			m.Board[seat].MZone[sequence] = &card
		}
	case locationSZone:
		if sequence < len(m.Board[seat].SZone) {
			m.Board[seat].SZone[sequence] = &card
		}
	case locationHand:
		for len(m.Hands[seat]) <= sequence {
			m.Hands[seat] = append(m.Hands[seat], 0)
		}
		m.Hands[seat][sequence] = card.Code
	case locationGrave:
		m.Board[seat].Grave = append(m.Board[seat].Grave, card)
	case locationBanish:
		m.Board[seat].Banish = append(m.Board[seat].Banish, card)
	}
}

func (m *DuelModel) cardAt(seat, location, sequence int) *DuelCard {
	if sequence < 0 {
		return nil
	}
	switch location {
	case locationMZone:
		if sequence < len(m.Board[seat].MZone) {
			return m.Board[seat].MZone[sequence]
		}
	case locationSZone:
		if sequence < len(m.Board[seat].SZone) {
			return m.Board[seat].SZone[sequence]
		}
	case locationGrave:
		if sequence < len(m.Board[seat].Grave) {
			return &m.Board[seat].Grave[sequence]
		}
	case locationBanish:
		if sequence < len(m.Board[seat].Banish) {
			return &m.Board[seat].Banish[sequence]
		}
	}
	return nil
}

func (m *DuelModel) removeCard(seat, location, sequence int) {
	if location == locationHand && sequence >= 0 && sequence < len(m.Hands[seat]) {
		m.Hands[seat] = append(m.Hands[seat][:sequence], m.Hands[seat][sequence+1:]...)
		return
	}
	if location == locationMZone && sequence >= 0 && sequence < len(m.Board[seat].MZone) {
		m.Board[seat].MZone[sequence] = nil
		return
	}
	if location == locationSZone && sequence >= 0 && sequence < len(m.Board[seat].SZone) {
		m.Board[seat].SZone[sequence] = nil
		return
	}
	if location == locationGrave && sequence >= 0 && sequence < len(m.Board[seat].Grave) {
		m.Board[seat].Grave = append(m.Board[seat].Grave[:sequence], m.Board[seat].Grave[sequence+1:]...)
		return
	}
	if location == locationBanish && sequence >= 0 && sequence < len(m.Board[seat].Banish) {
		m.Board[seat].Banish = append(m.Board[seat].Banish[:sequence], m.Board[seat].Banish[sequence+1:]...)
	}
}

func duelCardFromQuery(query any) DuelCard {
	position := eventField(query, "position")
	return DuelCard{
		Code:     uint32(eventInt(query, "code", 0)),
		Position: uint8(eventInt(position, "p", 0)),
	}
}

func eventNil(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

func eventField(data any, name string) any {
	if data == nil {
		return nil
	}
	if values, ok := data.(map[string]any); ok {
		if value, exists := values[name]; exists {
			return value
		}
		for key, value := range values {
			if strings.EqualFold(key, name) {
				return value
			}
		}
		return nil
	}
	value := reflect.ValueOf(data)
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}
	typeOf := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := typeOf.Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.Name == name || strings.EqualFold(field.Name, name) || jsonName == name {
			return value.Field(i).Interface()
		}
	}
	return nil
}

func eventInt(data any, name string, fallback int) int {
	value := reflect.ValueOf(eventField(data, name))
	if !value.IsValid() {
		return fallback
	}
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return fallback
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(value.Uint())
	case reflect.Float32, reflect.Float64:
		return int(value.Float())
	}
	return fallback
}

func eventString(data any, name, fallback string) string {
	if value, ok := eventField(data, name).(string); ok {
		return value
	}
	return fallback
}

func eventBool(data any, name string) bool {
	value := eventField(data, name)
	if result, ok := value.(bool); ok {
		return result
	}
	return eventInt(data, name, 0) != 0
}

func commandActions(data any, name string) []CommandAction {
	values := eventSlice(data, name)
	result := make([]CommandAction, 0, len(values))
	for _, value := range values {
		result = append(result, CommandAction{
			Code: uint32(eventInt(value, "code", 0)), Player: eventInt(value, "c", 0),
			Location: eventInt(value, "l", 0), Sequence: eventInt(value, "s", 0), Index: eventInt(value, "idx", 0),
			Desc: uint32(eventInt(value, "desc", 0)), Direct: eventBool(value, "diratt"),
		})
	}
	return result
}

func eventSlice(data any, name string) []any {
	value := reflect.ValueOf(eventField(data, name))
	if !value.IsValid() || value.Kind() != reflect.Slice {
		return nil
	}
	result := make([]any, value.Len())
	for i := range result {
		result[i] = value.Index(i).Interface()
	}
	return result
}

func eventIntSlice(data any, name string) []uint32 {
	values := eventSlice(data, name)
	result := make([]uint32, len(values))
	for i, value := range values {
		result[i] = uint32(eventInt(map[string]any{"value": value}, "value", 0))
	}
	return result
}
