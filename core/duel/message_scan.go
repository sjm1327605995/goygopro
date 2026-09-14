package duel

import (
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// This file mirrors the message-walk of the C++ client's
// ReplayMode::ReplayAnalyze (source/ygopro/gframe/replay_mode.cpp): it walks an
// engine message batch message by message and reports whether the batch
// contains a message the driver must answer with a recorded response.
//
// The response decision is made from the STREAM, not from the engine's
// PROCESSOR_WAITING flag, exactly like the C++ client. Most waits carry the
// flag, but some (e.g. PROCESSOR_ROCK_PAPER_SCISSORS) return only
// buffer_size() while still expecting a response — keying off the flag alone
// desynchronizes the recorded response stream.

// batchWinOffset is the sentinel returned for a batch containing MSG_WIN: the
// engine's adjust_step never terminates the duel on its own (it re-emits
// MSG_WIN on every subsequent adjust), so the driver must stop — the C++
// client does the same by returning false from SinglePlayAnalyze (win 之外
// 的消息不再处理).
const batchWinOffset = -2

// batchResponseOffset walks the batch and returns the offset of the first
// response-needing message, batchWinOffset if the batch contains MSG_WIN, or
// -1 if the batch needs no response. ok is false
// when the batch could not be walked (unknown message layout); the caller can
// fall back to the engine flag in that case.
func batchResponseOffset(msg []byte) (offset int, ok bool) {
	p := 0
	for p < len(msg) {
		op := msg[p]
		body := p + 1

		switch op {
		// ----- messages that consume one recorded response -----
		case ocgcore.MSG_SELECT_BATTLECMD:
			// player, count+count*11, count+count*8, +2
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 11) ||
				!walkCountList(&body, msg, 8) || !adv(&body, msg, 2) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_IDLECMD:
			// player, 5x(count+count*7), count+count*11, +3
			if !adv(&body, msg, 1) {
				return 0, false
			}
			for i := 0; i < 5; i++ {
				if !walkCountList(&body, msg, 7) {
					return 0, false
				}
			}
			if !walkCountList(&body, msg, 11) || !adv(&body, msg, 3) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_EFFECTYN:
			if !adv(&body, msg, 1+12) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_YESNO:
			if !adv(&body, msg, 1+4) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_OPTION:
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 4) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_CARD, ocgcore.MSG_SELECT_TRIBUTE:
			// player, cancelable, min, max, count+count*8
			if !adv(&body, msg, 1+3) || !walkCountList(&body, msg, 8) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_CHAIN:
			// player, count, spec, forced, hint0, hint1, count*14
			if !adv(&body, msg, 1+1) {
				return 0, false
			}
			count := int(msg[body-1])
			if !adv(&body, msg, 9+count*14) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_PLACE, ocgcore.MSG_SELECT_DISFIELD, ocgcore.MSG_SELECT_POSITION:
			if !adv(&body, msg, 1+5) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_COUNTER:
			if !adv(&body, msg, 1+4) || !walkCountList(&body, msg, 9) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_SUM:
			// skip1, player, skip6, count+count*11, count+count*11
			if !adv(&body, msg, 1) || !adv(&body, msg, 1) || !adv(&body, msg, 6) ||
				!walkCountList(&body, msg, 11) || !walkCountList(&body, msg, 11) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SORT_CARD:
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 7) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_SELECT_UNSELECT_CARD:
			if !adv(&body, msg, 1+4) || !walkCountList(&body, msg, 8) || !walkCountList(&body, msg, 8) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_ROCK_PAPER_SCISSORS:
			if !adv(&body, msg, 1) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_ANNOUNCE_RACE, ocgcore.MSG_ANNOUNCE_ATTRIB:
			if !adv(&body, msg, 1+5) {
				return 0, false
			}
			return p, true

		case ocgcore.MSG_ANNOUNCE_CARD, ocgcore.MSG_ANNOUNCE_NUMBER:
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 4) {
				return 0, false
			}
			return p, true

		// ----- informational messages, walked past like the C++ client -----
		case ocgcore.MSG_RETRY, ocgcore.MSG_REVERSE_DECK,
			ocgcore.MSG_ATTACK_DISABLED, ocgcore.MSG_DAMAGE_STEP_START, ocgcore.MSG_DAMAGE_STEP_END,
			ocgcore.MSG_SUMMONED, ocgcore.MSG_SPSUMMONED, ocgcore.MSG_FLIPSUMMONED,
			ocgcore.MSG_CHAIN_END, ocgcore.MSG_WAITING:
			// no body

		case ocgcore.MSG_START:
			if !adv(&body, msg, 15) {
				return 0, false
			}

		case ocgcore.MSG_WIN:
			// Terminal message; the duel is decided (adjust_step 的 LP 判定).
			return batchWinOffset, true

		case ocgcore.MSG_HINT:
			if !adv(&body, msg, 6) {
				return 0, false
			}

		case ocgcore.MSG_UPDATE_DATA:
			if !adv(&body, msg, 2) || !walkQueryBlobs(&body, msg) {
				return 0, false
			}

		case ocgcore.MSG_UPDATE_CARD:
			if !adv(&body, msg, 3) || !walkOneQueryBlob(&body, msg) {
				return 0, false
			}

		case ocgcore.MSG_CONFIRM_DECKTOP, ocgcore.MSG_CONFIRM_EXTRATOP:
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 7) {
				return 0, false
			}

		case ocgcore.MSG_CONFIRM_CARDS:
			if !adv(&body, msg, 1+1) || !walkCountList(&body, msg, 7) {
				return 0, false
			}

		case ocgcore.MSG_SHUFFLE_DECK, ocgcore.MSG_REFRESH_DECK, ocgcore.MSG_SWAP_GRAVE_DECK:
			if !adv(&body, msg, 1) {
				return 0, false
			}

		case ocgcore.MSG_SHUFFLE_HAND, ocgcore.MSG_SHUFFLE_EXTRA:
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 4) {
				return 0, false
			}

		case ocgcore.MSG_SHUFFLE_SET_CARD:
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 8) {
				return 0, false
			}

		case ocgcore.MSG_DECK_TOP:
			if !adv(&body, msg, 6) {
				return 0, false
			}

		case ocgcore.MSG_NEW_TURN:
			if !adv(&body, msg, 1) {
				return 0, false
			}

		case ocgcore.MSG_NEW_PHASE:
			if !adv(&body, msg, 2) {
				return 0, false
			}

		case ocgcore.MSG_MOVE:
			if !adv(&body, msg, 16) {
				return 0, false
			}

		case ocgcore.MSG_POS_CHANGE:
			if !adv(&body, msg, 9) {
				return 0, false
			}

		case ocgcore.MSG_SET:
			if !adv(&body, msg, 8) {
				return 0, false
			}

		case ocgcore.MSG_SWAP:
			if !adv(&body, msg, 16) {
				return 0, false
			}

		case ocgcore.MSG_FIELD_DISABLED:
			if !adv(&body, msg, 4) {
				return 0, false
			}

		case ocgcore.MSG_SUMMONING, ocgcore.MSG_SPSUMMONING, ocgcore.MSG_FLIPSUMMONING:
			if !adv(&body, msg, 8) {
				return 0, false
			}

		case ocgcore.MSG_CHAINING:
			if !adv(&body, msg, 16) {
				return 0, false
			}

		case ocgcore.MSG_CHAINED, ocgcore.MSG_CHAIN_SOLVING, ocgcore.MSG_CHAIN_SOLVED,
			ocgcore.MSG_CHAIN_NEGATED, ocgcore.MSG_CHAIN_DISABLED:
			if !adv(&body, msg, 1) {
				return 0, false
			}

		case ocgcore.MSG_CARD_SELECTED, ocgcore.MSG_RANDOM_SELECTED:
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 4) {
				return 0, false
			}

		case ocgcore.MSG_BECOME_TARGET:
			if !walkCountList(&body, msg, 4) {
				return 0, false
			}

		case ocgcore.MSG_DRAW:
			if !adv(&body, msg, 1) || !walkCountList(&body, msg, 4) {
				return 0, false
			}

		case ocgcore.MSG_DAMAGE, ocgcore.MSG_RECOVER, ocgcore.MSG_LPUPDATE, ocgcore.MSG_PAY_LPCOST:
			if !adv(&body, msg, 5) {
				return 0, false
			}

		case ocgcore.MSG_EQUIP, ocgcore.MSG_CARD_TARGET, ocgcore.MSG_CANCEL_TARGET:
			if !adv(&body, msg, 8) {
				return 0, false
			}

		case ocgcore.MSG_UNEQUIP:
			if !adv(&body, msg, 4) {
				return 0, false
			}

		case ocgcore.MSG_ADD_COUNTER, ocgcore.MSG_REMOVE_COUNTER:
			if !adv(&body, msg, 7) {
				return 0, false
			}

		case ocgcore.MSG_ATTACK:
			if !adv(&body, msg, 8) {
				return 0, false
			}

		case ocgcore.MSG_BATTLE:
			if !adv(&body, msg, 26) {
				return 0, false
			}

		case ocgcore.MSG_MISSED_EFFECT:
			if !adv(&body, msg, 8) {
				return 0, false
			}

		case ocgcore.MSG_BE_CHAIN_TARGET:
			if !adv(&body, msg, 1) {
				return 0, false
			}

		case ocgcore.MSG_CREATE_RELATION, ocgcore.MSG_RELEASE_RELATION:
			if !adv(&body, msg, 16) {
				return 0, false
			}

		case ocgcore.MSG_TOSS_COIN, ocgcore.MSG_TOSS_DICE:
			// player, count, count results
			if !adv(&body, msg, 2) {
				return 0, false
			}
			if !adv(&body, msg, int(msg[body-1])) {
				return 0, false
			}

		case ocgcore.MSG_HAND_RES:
			if !adv(&body, msg, 1) {
				return 0, false
			}

		case ocgcore.MSG_CARD_HINT:
			if !adv(&body, msg, 9) {
				return 0, false
			}

		case ocgcore.MSG_PLAYER_HINT:
			if !adv(&body, msg, 6) {
				return 0, false
			}

		case ocgcore.MSG_TAG_SWAP:
			// body[2]=main count, body[4]=extra count
			if body+5 > len(msg) {
				return 0, false
			}
			size := 9 + int(msg[body+2])*4 + int(msg[body+4])*4
			if !adv(&body, msg, size) {
				return 0, false
			}

		case ocgcore.MSG_RELOAD_FIELD:
			if !walkReloadField(&body, msg) {
				return 0, false
			}

		case ocgcore.MSG_AI_NAME, ocgcore.MSG_SHOW_HINT:
			if body+2 > len(msg) {
				return 0, false
			}
			l := int(msg[body]) | int(msg[body+1])<<8
			if !adv(&body, msg, 2+l+1) {
				return 0, false
			}

		case ocgcore.MSG_MATCH_KILL:
			if !adv(&body, msg, 4) {
				return 0, false
			}

		default:
			// Unknown layout (MSG_REQUEST_DECK, MSG_CUSTOM_MSG, ...): the caller
			// falls back to the engine flag rather than guessing.
			return 0, false
		}
		p = body
	}
	return -1, true
}

func adv(p *int, msg []byte, n int) bool {
	if n < 0 || *p+n > len(msg) {
		return false
	}
	*p += n
	return true
}

// walkCountList walks "count(1) + count*size" starting at *p; the count byte
// itself is part of the message and is consumed here.
func walkCountList(p *int, msg []byte, size int) bool {
	if !adv(p, msg, 1) {
		return false
	}
	count := int(msg[*p-1])
	return adv(p, msg, count*size)
}

// walkQueryBlobs walks length-prefixed card-query blobs until the buffer ends
// or a fragment too small to be a query appears.
func walkQueryBlobs(p *int, msg []byte) bool {
	for *p < len(msg) {
		if len(msg)-*p < 4 {
			return false
		}
		clen := int(uint32(msg[*p]) | uint32(msg[*p+1])<<8 | uint32(msg[*p+2])<<16 | uint32(msg[*p+3])<<24)
		if clen < ocgcore.LEN_HEADER {
			// Not a query blob; whatever remains belongs to another message.
			return true
		}
		if !adv(p, msg, clen) {
			return false
		}
	}
	return true
}

// walkOneQueryBlob walks exactly one length-prefixed card-query blob.
func walkOneQueryBlob(p *int, msg []byte) bool {
	if len(msg)-*p < 4 {
		return false
	}
	clen := int(uint32(msg[*p]) | uint32(msg[*p+1])<<8 | uint32(msg[*p+2])<<16 | uint32(msg[*p+3])<<24)
	if clen < ocgcore.LEN_HEADER {
		return true
	}
	return adv(p, msg, clen)
}

func walkReloadField(p *int, msg []byte) bool {
	if !adv(p, msg, 1) {
		return false
	}
	for pl := 0; pl < 2; pl++ {
		if !adv(p, msg, 4) {
			return false
		}
		for i := 0; i < 7; i++ {
			if !adv(p, msg, 1) {
				return false
			}
			if msg[*p-1] != 0 && !adv(p, msg, 2) {
				return false
			}
		}
		for i := 0; i < 8; i++ {
			if !adv(p, msg, 1) {
				return false
			}
			if msg[*p-1] != 0 && !adv(p, msg, 1) {
				return false
			}
		}
		if !adv(p, msg, 6) {
			return false
		}
	}
	// chain count(1) + 每条 15 字节（code + info_location + 控制者/区/序号 + 描述，
	// single_mode.cpp:730-731：pbuf += count * 15）
	if !adv(p, msg, 1) {
		return false
	}
	return adv(p, msg, int(msg[*p-1])*15)
}
