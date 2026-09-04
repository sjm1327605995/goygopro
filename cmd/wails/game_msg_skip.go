package main

import (
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// skipEngineMessageBody advances pbuf past the body of an engine message that
// handleGameMessage does not decode into a frontend event. It mirrors the skip
// offsets of the C++ ReplayMode::ReplayAnalyze (source/ygopro/gframe/replay_mode.cpp)
// so the parser never loses message alignment, even for messages the UI ignores.
//
// It returns an error for message types whose layout is unknown; callers should
// stop parsing the batch rather than continue with a misaligned buffer.
func skipEngineMessageBody(pbuf *utils.YGOBuffer, engType uint8) error {
	skip := func(n int) error { return pbuf.Next(n) }

	switch engType {
	case ocgcore.MSG_RETRY, ocgcore.MSG_REVERSE_DECK,
		ocgcore.MSG_ATTACK_DISABLED, ocgcore.MSG_DAMAGE_STEP_START, ocgcore.MSG_DAMAGE_STEP_END:
		return nil // no body

	case ocgcore.MSG_HINT:
		return skip(6)

	case ocgcore.MSG_SELECT_UNSELECT_CARD:
		var p uint8
		if err := pbuf.Read(&p); err != nil {
			return err
		}
		if err := skip(4); err != nil {
			return err
		}
		var c1, c2 uint8
		if err := pbuf.Read(&c1); err != nil {
			return err
		}
		if err := skip(int(c1) * 8); err != nil {
			return err
		}
		if err := pbuf.Read(&c2); err != nil {
			return err
		}
		return skip(int(c2) * 8)

	case ocgcore.MSG_SELECT_COUNTER:
		var p uint8
		if err := pbuf.Read(&p); err != nil {
			return err
		}
		if err := skip(4); err != nil {
			return err
		}
		var c uint8
		if err := pbuf.Read(&c); err != nil {
			return err
		}
		return skip(int(c) * 9)

	case ocgcore.MSG_SELECT_SUM:
		if err := skip(1); err != nil {
			return err
		}
		var p uint8
		if err := pbuf.Read(&p); err != nil {
			return err
		}
		if err := skip(6); err != nil {
			return err
		}
		var c1, c2 uint8
		if err := pbuf.Read(&c1); err != nil {
			return err
		}
		if err := skip(int(c1) * 11); err != nil {
			return err
		}
		if err := pbuf.Read(&c2); err != nil {
			return err
		}
		return skip(int(c2) * 11)

	case ocgcore.MSG_SORT_CARD:
		var p, c uint8
		if err := pbuf.Read(&p, &c); err != nil {
			return err
		}
		return skip(int(c) * 7)

	case ocgcore.MSG_CONFIRM_DECKTOP, ocgcore.MSG_CONFIRM_EXTRATOP:
		var p, c uint8
		if err := pbuf.Read(&p, &c); err != nil {
			return err
		}
		return skip(int(c) * 7)

	case ocgcore.MSG_CONFIRM_CARDS:
		var p uint8
		if err := pbuf.Read(&p); err != nil {
			return err
		}
		if err := skip(1); err != nil {
			return err
		}
		var c uint8
		if err := pbuf.Read(&c); err != nil {
			return err
		}
		return skip(int(c) * 7)

	case ocgcore.MSG_SHUFFLE_DECK, ocgcore.MSG_SWAP_GRAVE_DECK:
		return skip(1) // player

	case ocgcore.MSG_SHUFFLE_HAND, ocgcore.MSG_SHUFFLE_EXTRA:
		var p, c uint8
		if err := pbuf.Read(&p, &c); err != nil {
			return err
		}
		return skip(int(c) * 4)

	case ocgcore.MSG_REFRESH_DECK:
		return skip(1)

	case ocgcore.MSG_SHUFFLE_SET_CARD:
		if err := skip(1); err != nil {
			return err
		}
		var c uint8
		if err := pbuf.Read(&c); err != nil {
			return err
		}
		return skip(int(c) * 8)

	case ocgcore.MSG_DECK_TOP:
		return skip(6)

	case ocgcore.MSG_SWAP:
		return skip(16)

	case ocgcore.MSG_FIELD_DISABLED:
		return skip(4)

	case ocgcore.MSG_CHAIN_NEGATED, ocgcore.MSG_CHAIN_DISABLED:
		return skip(1)

	case ocgcore.MSG_CARD_SELECTED, ocgcore.MSG_RANDOM_SELECTED:
		var p, c uint8
		if err := pbuf.Read(&p, &c); err != nil {
			return err
		}
		return skip(int(c) * 4)

	case ocgcore.MSG_BECOME_TARGET:
		var c uint8
		if err := pbuf.Read(&c); err != nil {
			return err
		}
		return skip(int(c) * 4)

	case ocgcore.MSG_EQUIP, ocgcore.MSG_CARD_TARGET, ocgcore.MSG_CANCEL_TARGET:
		return skip(8)

	case ocgcore.MSG_UNEQUIP:
		return skip(4)

	case ocgcore.MSG_PAY_LPCOST:
		return skip(5)

	case ocgcore.MSG_ADD_COUNTER, ocgcore.MSG_REMOVE_COUNTER:
		return skip(7)

	case ocgcore.MSG_MISSED_EFFECT:
		return skip(8)

	case ocgcore.MSG_BE_CHAIN_TARGET:
		return skip(1)

	case ocgcore.MSG_CREATE_RELATION, ocgcore.MSG_RELEASE_RELATION:
		return skip(16)

	case ocgcore.MSG_TOSS_COIN, ocgcore.MSG_TOSS_DICE:
		var p, c uint8
		if err := pbuf.Read(&p, &c); err != nil {
			return err
		}
		return skip(int(c))

	case ocgcore.MSG_ROCK_PAPER_SCISSORS:
		return skip(1)

	case ocgcore.MSG_HAND_RES:
		return skip(1)

	case ocgcore.MSG_ANNOUNCE_RACE, ocgcore.MSG_ANNOUNCE_ATTRIB:
		return skip(6)

	case ocgcore.MSG_ANNOUNCE_CARD, ocgcore.MSG_ANNOUNCE_NUMBER:
		var p, c uint8
		if err := pbuf.Read(&p, &c); err != nil {
			return err
		}
		return skip(int(c) * 4)

	case ocgcore.MSG_CARD_HINT:
		return skip(9)

	case ocgcore.MSG_PLAYER_HINT:
		return skip(6)

	case ocgcore.MSG_MATCH_KILL:
		return skip(4)

	case ocgcore.MSG_TAG_SWAP:
		// player(1) .. mcount at body[2] .. ecount at body[4] ..
		// body = mcount*4 + ecount*4 + 9 (matches C++ replay_mode.cpp).
		mcount := int(pbuf.At(2))
		ecount := int(pbuf.At(4))
		return skip(mcount*4 + ecount*4 + 9)

	case ocgcore.MSG_AI_NAME, ocgcore.MSG_SHOW_HINT:
		var l uint16
		if err := pbuf.Read(&l); err != nil {
			return err
		}
		return skip(int(l) + 1)

	case ocgcore.MSG_RELOAD_FIELD:
		// Debug.ReloadFieldEnd(); rarely seen outside debug builds.
		if err := skip(1); err != nil {
			return err
		}
		for p := 0; p < 2; p++ {
			if err := skip(4); err != nil {
				return err
			}
			for seq := 0; seq < 7; seq++ {
				var val uint8
				if err := pbuf.Read(&val); err != nil {
					return err
				}
				if val != 0 {
					if err := skip(2); err != nil {
						return err
					}
				}
			}
			for seq := 0; seq < 8; seq++ {
				var val uint8
				if err := pbuf.Read(&val); err != nil {
					return err
				}
				if val != 0 {
					if err := skip(1); err != nil {
						return err
					}
				}
			}
			if err := skip(6); err != nil {
				return err
			}
		}
		return skip(1)

	default:
		return &unknownEngineMessageError{engType: engType}
	}
}

type unknownEngineMessageError struct {
	engType uint8
}

// skipQueryBlobList advances pbuf past ocgcore card-query data. Each query is
// a length-prefixed blob whose int32 length includes the 4 length bytes (see
// SingleDuel.RefreshSzone's parse loop); lengths below LEN_HEADER carry no
// payload. maxBlobs caps the number of blobs to skip (0 = until buffer end),
// which lets MSG_UPDATE_CARD consume exactly one query while leaving any
// subsequent engine messages in the batch intact.
func skipQueryBlobList(pbuf *utils.YGOBuffer, maxBlobs int) error {
	for skipped := 0; maxBlobs == 0 || skipped < maxBlobs; skipped++ {
		if pbuf.Len() < 4 {
			return nil
		}
		var clen int32
		if err := pbuf.Read(&clen); err != nil {
			return err
		}
		if clen < ocgcore.LEN_HEADER {
			// Fragment smaller than the fixed header cannot be a real query;
			// stop rather than skip a bogus size.
			return nil
		}
		if err := pbuf.Next(int(clen) - 4); err != nil {
			return err
		}
	}
	return nil
}

func (e *unknownEngineMessageError) Error() string {
	return "unknown engine message type: " + string(rune('0'+e.engType/10)) + string(rune('0'+e.engType%10))
}
