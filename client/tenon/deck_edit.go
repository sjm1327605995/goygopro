package tenon

import (
	appservice "github.com/sjm1327605995/goygopro/client/app"
)

type deckTarget uint8

const (
	deckTargetAny deckTarget = iota
	deckTargetMain
	deckTargetExtra
	deckTargetSide
)

const extraDeckTypes = uint32(0x40 | 0x2000 | 0x800000 | 0x4000000)

func cloneDeck(deck appservice.DeckData) appservice.DeckData {
	deck.Main = append([]uint32(nil), deck.Main...)
	deck.Extra = append([]uint32(nil), deck.Extra...)
	deck.Side = append([]uint32(nil), deck.Side...)
	return deck
}

func addDeckCard(deck appservice.DeckData, code, cardType uint32, target deckTarget) (appservice.DeckData, string) {
	if code == 0 {
		return deck, "请先选择卡片"
	}
	extra := cardType&extraDeckTypes != 0
	if target == deckTargetMain && extra {
		target = deckTargetExtra
	}
	if target == deckTargetExtra && !extra {
		return deck, "这张卡不属于额外卡组"
	}
	count := 0
	for _, zone := range [][]uint32{deck.Main, deck.Extra, deck.Side} {
		for _, existing := range zone {
			if existing == code {
				count++
			}
		}
	}
	if count >= 3 {
		return deck, "同名卡最多加入 3 张"
	}
	deck = cloneDeck(deck)
	switch target {
	case deckTargetExtra:
		if len(deck.Extra) >= 15 {
			return deck, "额外卡组最多 15 张"
		}
		deck.Extra = append(deck.Extra, code)
		return deck, "已加入额外卡组"
	case deckTargetSide:
		if len(deck.Side) >= 15 {
			return deck, "副卡组最多 15 张"
		}
		deck.Side = append(deck.Side, code)
		return deck, "已加入副卡组"
	default:
		if len(deck.Main) >= 60 {
			return deck, "主卡组最多 60 张"
		}
		deck.Main = append(deck.Main, code)
		return deck, "已加入主卡组"
	}
}

func removeDeckCard(deck appservice.DeckData, code uint32, target deckTarget) (appservice.DeckData, string) {
	deck = cloneDeck(deck)
	remove := func(cards []uint32) ([]uint32, bool) {
		for index, existing := range cards {
			if existing == code {
				return append(cards[:index], cards[index+1:]...), true
			}
		}
		return cards, false
	}
	var ok bool
	switch target {
	case deckTargetMain:
		deck.Main, ok = remove(deck.Main)
	case deckTargetExtra:
		deck.Extra, ok = remove(deck.Extra)
	case deckTargetSide:
		deck.Side, ok = remove(deck.Side)
	default:
		if deck.Main, ok = remove(deck.Main); !ok {
			if deck.Extra, ok = remove(deck.Extra); !ok {
				deck.Side, ok = remove(deck.Side)
			}
		}
	}
	if !ok {
		return deck, "卡组中没有这张卡"
	}
	return deck, "已移除 1 张卡片"
}
