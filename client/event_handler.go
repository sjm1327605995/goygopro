package client

import (
	"encoding/binary"
	"fmt"
	"math/bits"

	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// ClientAnalyze corresponds to C++ DuelClient::ClientAnalyze in duelclient.cpp
// Parses game messages (MSG_*) and updates ClientField / Game state.
func (dc *DuelClient) ClientAnalyze(msg []byte) bool {
	if len(msg) == 0 {
		return true
	}
	pbuf := msg
	msgType := pbuf[0]
	pbuf = pbuf[1:]

	// C++: mainGame->dInfo.curMsg = msgType
	MainGame.DInfo.CurMsg = int16(msgType)

	// C++: mainGame->dField.HideMenu()
	// The tenon-based cmd bar is rebuilt every frame; no explicit hide needed.

	// 收到新消息就撤掉「等待对方操作」的提示（C++ duelclient.cpp 同一位置）。
	// MSG_WAITING 自己要排除掉 —— 它正是来立这个提示的；MSG_CARD_SELECTED 也排除，
	// 因为它是对方选卡过程中的播报，等待其实还没结束。
	if msgType != network.MSG_WAITING && msgType != network.MSG_CARD_SELECTED {
		MainGame.WaitFrame = -1
		MainGame.ShowingText = ""
	}

	switch msgType {
	case network.MSG_RETRY:
		return dc.handleRetry()
	case network.MSG_HINT:
		return dc.handleHint(pbuf)
	case network.MSG_START:
		return dc.handleStart(pbuf)
	case network.MSG_UPDATE_DATA:
		return dc.handleUpdateData(pbuf)
	case network.MSG_UPDATE_CARD:
		return dc.handleUpdateCard(pbuf)
	case network.MSG_SELECT_BATTLECMD:
		return dc.handleSelectBattleCmd(pbuf)
	case network.MSG_SELECT_IDLECMD:
		return dc.handleSelectIdleCmd(pbuf)
	case network.MSG_SELECT_EFFECTYN:
		return dc.handleSelectEffectYN(pbuf)
	case network.MSG_SELECT_YESNO:
		return dc.handleSelectYesNo(pbuf)
	case network.MSG_SELECT_OPTION:
		return dc.handleSelectOption(pbuf)
	case network.MSG_SELECT_CARD:
		return dc.handleSelectCard(pbuf)
	case network.MSG_SELECT_CHAIN:
		return dc.handleSelectChain(pbuf)
	case network.MSG_SELECT_PLACE:
		return dc.handleSelectPlace(pbuf)
	case network.MSG_SELECT_POSITION:
		return dc.handleSelectPosition(pbuf)
	case network.MSG_SELECT_TRIBUTE:
		return dc.handleSelectTribute(pbuf)
	case network.MSG_SELECT_COUNTER:
		return dc.handleSelectCounter(pbuf)
	case network.MSG_SELECT_SUM:
		return dc.handleSelectSum(pbuf)
	case network.MSG_SELECT_DISFIELD:
		return dc.handleSelectDisfield(pbuf)
	case network.MSG_SORT_CARD:
		return dc.handleSortCard(pbuf)
	case network.MSG_SELECT_UNSELECT_CARD:
		return dc.handleSelectUnselectCard(pbuf)
	case network.MSG_CONFIRM_DECKTOP:
		return dc.handleConfirmDecktop(pbuf)
	case network.MSG_CONFIRM_CARDS:
		return dc.handleConfirmCards(pbuf)
	case network.MSG_SHUFFLE_DECK:
		return dc.handleShuffleDeck(pbuf)
	case network.MSG_SHUFFLE_HAND:
		return dc.handleShuffleHand(pbuf)
	case network.MSG_REFRESH_DECK:
		return dc.handleRefreshDeck(pbuf)
	case network.MSG_SWAP_GRAVE_DECK:
		return dc.handleSwapGraveDeck(pbuf)
	case network.MSG_SHUFFLE_SET_CARD:
		return dc.handleShuffleSetCard(pbuf)
	case network.MSG_REVERSE_DECK:
		return dc.handleReverseDeck(pbuf)
	case network.MSG_DECK_TOP:
		return dc.handleDeckTop(pbuf)
	case network.MSG_SHUFFLE_EXTRA:
		return dc.handleShuffleExtra(pbuf)
	case network.MSG_NEW_TURN:
		return dc.handleNewTurn(pbuf)
	case network.MSG_NEW_PHASE:
		return dc.handleNewPhase(pbuf)
	case network.MSG_MOVE:
		return dc.handleMove(pbuf)
	case network.MSG_POS_CHANGE:
		return dc.handlePosChange(pbuf)
	case network.MSG_SET:
		return dc.handleSet(pbuf)
	case network.MSG_SWAP:
		return dc.handleSwap(pbuf)
	case network.MSG_FIELD_DISABLED:
		return dc.handleFieldDisabled(pbuf)
	case network.MSG_SUMMONING, network.MSG_SPSUMMONING, network.MSG_FLIPSUMMONING:
		return dc.handleSummoning(msgType, pbuf)
	case network.MSG_SUMMONED, network.MSG_SPSUMMONED, network.MSG_FLIPSUMMONED:
		return dc.handleSummoned()
	case network.MSG_CHAINING:
		return dc.handleChaining(pbuf)
	case network.MSG_CHAINED:
		return dc.handleChained(pbuf)
	case network.MSG_CHAIN_SOLVING:
		return dc.handleChainSolving(pbuf)
	case network.MSG_CHAIN_SOLVED:
		return dc.handleChainSolved(pbuf)
	case network.MSG_CHAIN_END:
		return dc.handleChainEnd()
	case network.MSG_CHAIN_NEGATED:
		return dc.handleChainNegated(pbuf)
	case network.MSG_CHAIN_DISABLED:
		return dc.handleChainDisabled(pbuf)
	case network.MSG_CARD_SELECTED:
		return dc.handleCardSelected(pbuf)
	case network.MSG_RANDOM_SELECTED:
		return dc.handleRandomSelected(pbuf)
	case network.MSG_BECOME_TARGET:
		return dc.handleBecomeTarget(pbuf)
	case network.MSG_DRAW:
		return dc.handleDraw(pbuf)
	case network.MSG_DAMAGE:
		return dc.handleDamage(pbuf)
	case network.MSG_RECOVER:
		return dc.handleRecover(pbuf)
	case network.MSG_EQUIP:
		return dc.handleEquip(pbuf)
	case network.MSG_LPUPDATE:
		return dc.handleLPUpdate(pbuf)
	case network.MSG_UNEQUIP:
		return dc.handleUnequip(pbuf)
	case network.MSG_CARD_TARGET:
		return dc.handleCardTarget(pbuf)
	case network.MSG_CANCEL_TARGET:
		return dc.handleCancelTarget(pbuf)
	case network.MSG_PAY_LPCOST:
		return dc.handlePayLPCost(pbuf)
	case network.MSG_ADD_COUNTER:
		return dc.handleAddCounter(pbuf)
	case network.MSG_REMOVE_COUNTER:
		return dc.handleRemoveCounter(pbuf)
	case network.MSG_ATTACK:
		return dc.handleAttack(pbuf)
	case network.MSG_BATTLE:
		return dc.handleBattle(pbuf)
	case network.MSG_ATTACK_DISABLED:
		return dc.handleAttackDisabled()
	case network.MSG_DAMAGE_STEP_START:
		return dc.handleDamageStepStart()
	case network.MSG_DAMAGE_STEP_END:
		return dc.handleDamageStepEnd()
	case network.MSG_MISSED_EFFECT:
		return dc.handleMissedEffect(pbuf)
	case network.MSG_TOSS_COIN:
		return dc.handleTossCoin(pbuf)
	case network.MSG_TOSS_DICE:
		return dc.handleTossDice(pbuf)
	case network.MSG_ROCK_PAPER_SCISSORS:
		return dc.handleRockPaperScissors(pbuf)
	case network.MSG_HAND_RES:
		return dc.handleHandRes(pbuf)
	case network.MSG_ANNOUNCE_RACE:
		return dc.handleAnnounceRace(pbuf)
	case network.MSG_ANNOUNCE_ATTRIB:
		return dc.handleAnnounceAttrib(pbuf)
	case network.MSG_ANNOUNCE_CARD:
		return dc.handleAnnounceCard(pbuf)
	case network.MSG_ANNOUNCE_NUMBER:
		return dc.handleAnnounceNumber(pbuf)
	case network.MSG_TAG_SWAP:
		return dc.handleTagSwap(pbuf)
	case network.MSG_RELOAD_FIELD:
		return dc.handleReloadField(pbuf)
	case network.MSG_AI_NAME:
		return dc.handleAIName(pbuf)
	case network.MSG_SHOW_HINT:
		return dc.handleShowHint(pbuf)
	case network.MSG_MATCH_KILL:
		return dc.handleMatchKill(pbuf)
	case network.MSG_CUSTOM_MSG:
		return dc.handleCustomMsg(pbuf)
	case network.MSG_WIN:
		return dc.handleWin(pbuf)
	case network.MSG_WAITING:
		return dc.handleWaiting()
	case network.MSG_CARD_HINT:
		return dc.handleCardHint(pbuf)
	case network.MSG_PLAYER_HINT:
		return dc.handlePlayerHint(pbuf)
	case network.MSG_CONFIRM_EXTRATOP:
		return dc.handleConfirmExtratop(pbuf)
	default:
		fmt.Printf("Unhandled MSG type: %d\n", msgType)
		return true
	}
}

// MSG_START (4)
func (dc *DuelClient) handleStart(pbuf []byte) bool {
	MainGame.DField.Clear()
	MainGame.DInfo.IsInDuel = true

	playertype := pbuf[0]
	pbuf = pbuf[1:]
	MainGame.DInfo.IsFirst = (playertype & 0xf) == 0
	if playertype&0xf0 != 0 {
		MainGame.DInfo.PlayerType = 7
	}
	if MainGame.DInfo.IsTag {
		if MainGame.DInfo.IsFirst {
			MainGame.DInfo.TagPlayer[1] = true
		} else {
			MainGame.DInfo.TagPlayer[0] = true
		}
	}

	MainGame.DInfo.DuelRule = int(pbuf[0])
	pbuf = pbuf[1:]

	lp0 := int32(binary.LittleEndian.Uint32(pbuf))
	pbuf = pbuf[4:]
	lp1 := int32(binary.LittleEndian.Uint32(pbuf))
	pbuf = pbuf[4:]

	MainGame.DInfo.LP[MainGame.LocalPlayer(0)] = int(lp0)
	MainGame.DInfo.LP[MainGame.LocalPlayer(1)] = int(lp1)
	MainGame.DInfo.StrLP[0] = fmt.Sprintf("%d", MainGame.DInfo.LP[0])
	MainGame.DInfo.StrLP[1] = fmt.Sprintf("%d", MainGame.DInfo.LP[1])

	deckc := binary.LittleEndian.Uint16(pbuf)
	pbuf = pbuf[2:]
	extrac := binary.LittleEndian.Uint16(pbuf)
	pbuf = pbuf[2:]
	MainGame.DField.Initial(MainGame.LocalPlayer(0), int(deckc), int(extrac), 0)

	deckc = binary.LittleEndian.Uint16(pbuf)
	pbuf = pbuf[2:]
	extrac = binary.LittleEndian.Uint16(pbuf)
	MainGame.DField.Initial(MainGame.LocalPlayer(1), int(deckc), int(extrac), 0)

	MainGame.DInfo.Turn = 0
	MainGame.DInfo.IsShuffling = false
	dc.selectHint = 0
	dc.selectUnselectHint = 0
	dc.lastSelectHint = 0

	PushScene("duelField")
	return true
}

// MSG_UPDATE_DATA (6)
func (dc *DuelClient) handleUpdateData(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	location := pbuf[1]
	MainGame.DField.UpdateFieldCard(player, location, pbuf[2:])
	return true
}

// MSG_UPDATE_CARD (7)
func (dc *DuelClient) handleUpdateCard(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	loc := pbuf[1]
	seq := pbuf[2]
	MainGame.DField.UpdateCard(player, loc, int(seq), pbuf[3:])
	return true
}

// MSG_NEW_TURN (40)
func (dc *DuelClient) handleNewTurn(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	MainGame.DInfo.Turn++
	if MainGame.DInfo.IsTag && MainGame.DInfo.Turn != 1 {
		if player == 0 {
			MainGame.DInfo.TagPlayer[0] = !MainGame.DInfo.TagPlayer[0]
		} else {
			MainGame.DInfo.TagPlayer[1] = !MainGame.DInfo.TagPlayer[1]
		}
	}
	return true
}

// MSG_NEW_PHASE (41)
func (dc *DuelClient) handleNewPhase(pbuf []byte) bool {
	phase := binary.LittleEndian.Uint16(pbuf)
	MainGame.DInfo.Phase = phase
	MainGame.DField.ClearCommandFlag()
	MainGame.DField.ShowBP = false
	MainGame.DField.ShowEP = false
	MainGame.DField.ShowM2 = false
	MainGame.DField.ShowShuffle = false
	return true
}

// MSG_DRAW (90)
func (dc *DuelClient) handleDraw(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	count := int(pbuf[1])
	_ = count
	pbuf = pbuf[2:]
	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		pcard := MainGame.DField.GetCard(player, 0x01, len(MainGame.DField.Deck[player])-1-i)
		if pcard != nil && (code != 0 || !MainGame.DField.DeckReversed) {
			pcard.SetCode(code & 0x7fffffff)
		}
	}
	// Move cards from deck to hand
	for i := 0; i < count; i++ {
		deckSize := len(MainGame.DField.Deck[player])
		if deckSize == 0 {
			continue
		}
		pcard := MainGame.DField.Deck[player][deckSize-1]
		MainGame.DField.Deck[player] = MainGame.DField.Deck[player][:deckSize-1]
		MainGame.DField.AddCard(pcard, player, 0x02, 0)
	}
	return true
}

// MSG_MOVE (50)
func (dc *DuelClient) handleMove(pbuf []byte) bool {
	code := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	pc := MainGame.LocalPlayer(int(pbuf[0]))
	pbuf = pbuf[1:]
	pl := pbuf[0]
	pbuf = pbuf[1:]
	ps := int(pbuf[0])
	pbuf = pbuf[1:]
	_ = pbuf[0] // pp
	pbuf = pbuf[1:]
	cc := MainGame.LocalPlayer(int(pbuf[0]))
	pbuf = pbuf[1:]
	cl := pbuf[0]
	pbuf = pbuf[1:]
	cs := int(pbuf[0])
	pbuf = pbuf[1:]
	cp := pbuf[0]
	pbuf = pbuf[1:]
	// reason := binary.LittleEndian.Uint32(pbuf)

	if pl == 0 {
		// Card appearing from nowhere
		pcard := NewClientCard()
		pcard.Position = cp
		pcard.SetCode(code)
		MainGame.DField.AddCard(pcard, cc, cl, cs)
	} else if cl == 0 {
		// Card leaving the field
		pcard := MainGame.DField.GetCard(pc, pl, ps)
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			pcard.ClearTarget()
			MainGame.DField.RemoveCard(pc, pl, ps)
		}
	} else {
		// Normal move
		if pl != 0x80 && cl != 0x80 {
			pcard := MainGame.DField.GetCard(pc, pl, ps)
			if pcard != nil {
				if pcard.Code != code && (code != 0 || cl == 0x40) {
					pcard.SetCode(code)
				}
				pcard.CHint = 0
				pcard.ChValue = 0
				if (pl&0x0c) != 0 && cl != pl {
					pcard.Counters = make(map[int]int)
				}
				if cl != pl {
					pcard.ClearTarget()
					if pcard.EquipTarget != nil {
						pcard.EquipTarget.IsShowEquip = false
						delete(pcard.EquipTarget.Equipped, pcard)
						pcard.EquipTarget = nil
					}
				}
				pcard.IsShowEquip = false
				pcard.IsShowTarget = false
				pcard.IsShowChainTarget = false
				MainGame.DField.RemoveCard(pc, pl, ps)
				pcard.Position = cp
				MainGame.DField.AddCard(pcard, cc, cl, cs)
			}
		} else if pl != 0x80 {
			// Move to overlay
			pcard := MainGame.DField.GetCard(pc, pl, ps)
			if pcard != nil {
				if code != 0 && pcard.Code != code {
					pcard.SetCode(code)
				}
				pcard.Counters = make(map[int]int)
				pcard.ClearTarget()
				if pcard.EquipTarget != nil {
					pcard.EquipTarget.IsShowEquip = false
					delete(pcard.EquipTarget.Equipped, pcard)
					pcard.EquipTarget = nil
				}
				pcard.IsShowEquip = false
				pcard.IsShowTarget = false
				pcard.IsShowChainTarget = false
				olcard := MainGame.DField.GetCard(cc, cl&0x7f, cs)
				MainGame.DField.RemoveCard(pc, pl, ps)
				if olcard != nil {
					olcard.Overlayed = append(olcard.Overlayed, pcard)
					MainGame.DField.OverlayCards[pcard] = struct{}{}
					pcard.OverlayTarget = olcard
					pcard.Location = 0x80
					pcard.Sequence = uint8(len(olcard.Overlayed) - 1)
				}
			}
		}
	}
	return true
}

// MSG_DAMAGE (91)
func (dc *DuelClient) handleDamage(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	val := int32(binary.LittleEndian.Uint32(pbuf[1:]))
	final := MainGame.DInfo.LP[player] - int(val)
	if final < 0 {
		final = 0
	}
	MainGame.DInfo.LP[player] = final
	MainGame.DInfo.StrLP[player] = fmt.Sprintf("%d", final)
	return true
}

// MSG_RECOVER (92)
func (dc *DuelClient) handleRecover(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	val := int32(binary.LittleEndian.Uint32(pbuf[1:]))
	MainGame.DInfo.LP[player] += int(val)
	MainGame.DInfo.StrLP[player] = fmt.Sprintf("%d", MainGame.DInfo.LP[player])
	return true
}

// MSG_EQUIP (93)
func (dc *DuelClient) handleEquip(pbuf []byte) bool {
	c1 := MainGame.LocalPlayer(int(pbuf[0]))
	l1 := pbuf[1]
	s1 := int(pbuf[2])
	c2 := MainGame.LocalPlayer(int(pbuf[4]))
	l2 := pbuf[5]
	s2 := int(pbuf[6])
	pcard := MainGame.DField.GetCard(c1, l1, s1)
	ecard := MainGame.DField.GetCard(c2, l2, s2)
	if pcard != nil && ecard != nil {
		pcard.EquipTarget = ecard
		ecard.Equipped[pcard] = struct{}{}
	}
	return true
}

// MSG_UNEQUIP (94)
func (dc *DuelClient) handleUnequip(pbuf []byte) bool {
	c1 := MainGame.LocalPlayer(int(pbuf[0]))
	l1 := pbuf[1]
	s1 := int(pbuf[2])
	pcard := MainGame.DField.GetCard(c1, l1, s1)
	if pcard != nil && pcard.EquipTarget != nil {
		delete(pcard.EquipTarget.Equipped, pcard)
		pcard.EquipTarget = nil
	}
	return true
}

// MSG_POS_CHANGE (53)
func (dc *DuelClient) handlePosChange(pbuf []byte) bool {
	code := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	cc := MainGame.LocalPlayer(int(pbuf[0]))
	cl := pbuf[1]
	cs := int(pbuf[2])
	pp := pbuf[3]
	cp := pbuf[4]
	pcard := MainGame.DField.GetCard(cc, cl, cs)
	if pcard != nil {
		pcard.Position = cp
		if pcard.Code != code && code != 0 {
			pcard.SetCode(code)
		}
		if (pp&0x80) != 0 && (cp&0x80) == 0 && pcard.EquipTarget != nil {
			delete(pcard.EquipTarget.Equipped, pcard)
			pcard.EquipTarget = nil
		}
	}
	return true
}

// MSG_SET (54)
func (dc *DuelClient) handleSet(pbuf []byte) bool {
	code := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	cc := MainGame.LocalPlayer(int(pbuf[0]))
	cl := pbuf[1]
	cs := int(pbuf[2])
	cp := pbuf[3]
	pcard := MainGame.DField.GetCard(cc, cl, cs)
	if pcard != nil {
		pcard.Position = cp
		if pcard.Code != code {
			pcard.SetCode(code)
		}
	}
	return true
}

// MSG_SWAP (55)
func (dc *DuelClient) handleSwap(pbuf []byte) bool {
	code1 := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	c1 := MainGame.LocalPlayer(int(pbuf[0]))
	l1 := pbuf[1]
	s1 := int(pbuf[2])
	pbuf = pbuf[4:]
	code2 := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	c2 := MainGame.LocalPlayer(int(pbuf[0]))
	l2 := pbuf[1]
	s2 := int(pbuf[2])
	pbuf = pbuf[4:]
	ps1 := pbuf[0]
	ps2 := pbuf[1]

	pcard1 := MainGame.DField.GetCard(c1, l1, s1)
	pcard2 := MainGame.DField.GetCard(c2, l2, s2)
	if pcard1 != nil && pcard2 != nil {
		pcard1.Code = code1
		pcard2.Code = code2
		pcard1.Position = ps1
		pcard2.Position = ps2
		// Swap in field
		MainGame.DField.swapCards(c1, l1, s1, c2, l2, s2)
	}
	return true
}

// MSG_CARD_TARGET (83)
func (dc *DuelClient) handleCardTarget(pbuf []byte) bool {
	c1 := MainGame.LocalPlayer(int(pbuf[0]))
	l1 := pbuf[1]
	s1 := int(pbuf[2])
	c2 := MainGame.LocalPlayer(int(pbuf[4]))
	l2 := pbuf[5]
	s2 := int(pbuf[6])
	pcard := MainGame.DField.GetCard(c1, l1, s1)
	tcard := MainGame.DField.GetCard(c2, l2, s2)
	if pcard != nil && tcard != nil {
		pcard.CardTarget[tcard] = struct{}{}
		tcard.OwnerTarget[pcard] = struct{}{}
	}
	return true
}

// MSG_CANCEL_TARGET (84)
func (dc *DuelClient) handleCancelTarget(pbuf []byte) bool {
	c1 := MainGame.LocalPlayer(int(pbuf[0]))
	l1 := pbuf[1]
	s1 := int(pbuf[2])
	c2 := MainGame.LocalPlayer(int(pbuf[4]))
	l2 := pbuf[5]
	s2 := int(pbuf[6])
	pcard := MainGame.DField.GetCard(c1, l1, s1)
	tcard := MainGame.DField.GetCard(c2, l2, s2)
	if pcard != nil && tcard != nil {
		delete(pcard.CardTarget, tcard)
		delete(tcard.OwnerTarget, pcard)
	}
	return true
}

// MSG_PAY_LPCOST (95)
func (dc *DuelClient) handlePayLPCost(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	cost := int32(binary.LittleEndian.Uint32(pbuf[1:]))
	MainGame.DInfo.LP[player] -= int(cost)
	if MainGame.DInfo.LP[player] < 0 {
		MainGame.DInfo.LP[player] = 0
	}
	MainGame.DInfo.StrLP[player] = fmt.Sprintf("%d", MainGame.DInfo.LP[player])
	return true
}

// MSG_ADD_COUNTER (101)
func (dc *DuelClient) handleAddCounter(pbuf []byte) bool {
	_ = pbuf[0] // type
	c := MainGame.LocalPlayer(int(pbuf[1]))
	l := pbuf[2]
	s := int(pbuf[3])
	count := int(pbuf[4])
	pcard := MainGame.DField.GetCard(c, l, s)
	if pcard != nil {
		pcard.Counters[int(pbuf[0])] += count
	}
	return true
}

// MSG_REMOVE_COUNTER (102)
func (dc *DuelClient) handleRemoveCounter(pbuf []byte) bool {
	ctype := pbuf[0]
	c := MainGame.LocalPlayer(int(pbuf[1]))
	l := pbuf[2]
	s := int(pbuf[3])
	count := int(pbuf[4])
	pcard := MainGame.DField.GetCard(c, l, s)
	if pcard != nil {
		pcard.Counters[int(ctype)] -= count
		if pcard.Counters[int(ctype)] <= 0 {
			delete(pcard.Counters, int(ctype))
		}
	}
	return true
}

// MSG_SHUFFLE_DECK (32)
func (dc *DuelClient) handleShuffleDeck(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	for _, card := range MainGame.DField.Deck[player] {
		card.IsReversed = false
	}
	return true
}

// MSG_SHUFFLE_HAND (33)
func (dc *DuelClient) handleShuffleHand(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	count := int(pbuf[1])
	pbuf = pbuf[2:]
	for i := 0; i < count && i < len(MainGame.DField.Hand[player]); i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		MainGame.DField.Hand[player][i].SetCode(code & 0x7fffffff)
	}
	return true
}

// MSG_REVERSE_DECK (37)
func (dc *DuelClient) handleReverseDeck(pbuf []byte) bool {
	MainGame.DField.DeckReversed = !MainGame.DField.DeckReversed
	return true
}

// MSG_HINT (2)
func (dc *DuelClient) handleHint(pbuf []byte) bool {
	hintType := pbuf[0]
	_ = pbuf[1] // player
	data := binary.LittleEndian.Uint32(pbuf[2:])
	switch hintType {
	case 1: // HINT_SELECTMSG
		dc.selectHint = int(data)
		dc.lastSelectHint = int(data)
	case 3: // HINT_OPSELECTED
		fmt.Printf("Opponent selected: %d\n", data)
	case 4: // HINT_EFFECT
		MainGame.ShowingCode = int(data)
	case 5: // HINT_RACE
		fmt.Printf("Hint race: %d\n", data)
	case 6: // HINT_ATTRIB
		fmt.Printf("Hint attribute: %d\n", data)
	case 7: // HINT_CODE
		fmt.Printf("Hint code: %d\n", data)
	case 8: // HINT_NUMBER
		fmt.Printf("Hint number: %d\n", data)
	}
	return true
}

// MSG_RETRY (1)
func (dc *DuelClient) handleRetry() bool {
	fmt.Println("MSG_RETRY received")
	return false
}

func (dc *DuelClient) handleSelectBattleCmd(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	pbuf = pbuf[1:]

	MainGame.DField.ClearCommandFlag()
	MainGame.DField.ActivatableCards = nil
	MainGame.DField.ActivatableDescs = nil
	MainGame.DField.AttackableCards = nil
	MainGame.DField.ShowM2 = false
	MainGame.DField.ShowEP = false

	// Activatable cards
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		desc := int32(binary.LittleEndian.Uint32(pbuf))
		pbuf = pbuf[4:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			flag := 0
			if code&0x80000000 != 0 {
				flag = EDESCOperation
				code &= 0x7fffffff
			}
			MainGame.DField.ActivatableCards = append(MainGame.DField.ActivatableCards, pcard)
			MainGame.DField.ActivatableDescs = append(MainGame.DField.ActivatableDescs, [2]int{int(desc), flag})
			if flag&EDESCOperation != 0 {
				pcard.ChainCode = code
			} else {
				pcard.CmdFlag |= CommandActivate
			}
		}
	}

	// Attackable cards
	count = int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			pcard.CmdFlag |= CommandAttack
			MainGame.DField.AttackableCards = append(MainGame.DField.AttackableCards, pcard)
			if pcard.Code != code {
				pcard.SetCode(code)
			}
		}
	}

	if pbuf[0] != 0 {
		MainGame.DField.ShowM2 = true
	}
	pbuf = pbuf[1:]
	if pbuf[0] != 0 {
		MainGame.DField.ShowEP = true
	}
	pbuf = pbuf[1:]

	return false
}

func (dc *DuelClient) handleSelectIdleCmd(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	pbuf = pbuf[1:]

	MainGame.DField.ClearCommandFlag()
	MainGame.DField.SummonableCards = nil
	MainGame.DField.SPSummonableCards = nil
	MainGame.DField.ReposableCards = nil
	MainGame.DField.MSetableCards = nil
	MainGame.DField.SSetableCards = nil
	MainGame.DField.ActivatableCards = nil
	MainGame.DField.ActivatableDescs = nil
	MainGame.DField.ContiCards = nil
	MainGame.DField.ShowBP = false
	MainGame.DField.ShowEP = false
	MainGame.DField.ShowShuffle = false

	// Summonable
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		_ = binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			pcard.CmdFlag |= CommandSummon
			MainGame.DField.SummonableCards = append(MainGame.DField.SummonableCards, pcard)
		}
	}

	// SPSummonable
	count = int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			pcard.CmdFlag |= CommandSPSummon
			MainGame.DField.SPSummonableCards = append(MainGame.DField.SPSummonableCards, pcard)
			if pcard.Location == 0x01 { // DECK
				pcard.SetCode(code)
				MainGame.DField.DeckAct[con] = true
			} else if pcard.Location == 0x10 { // GRAVE
				MainGame.DField.GraveAct[con] = true
			} else if pcard.Location == 0x20 { // REMOVED
				MainGame.DField.RemoveAct[con] = true
			} else if pcard.Location == 0x40 { // EXTRA
				MainGame.DField.ExtraAct[con] = true
			}
		}
	}

	// Reposable
	count = int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		_ = binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			pcard.CmdFlag |= CommandRepos
			MainGame.DField.ReposableCards = append(MainGame.DField.ReposableCards, pcard)
		}
	}

	// MSetable
	count = int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		_ = binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			pcard.CmdFlag |= CommandMSet
			MainGame.DField.MSetableCards = append(MainGame.DField.MSetableCards, pcard)
		}
	}

	// SSetable
	count = int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		_ = binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			pcard.CmdFlag |= CommandSSet
			MainGame.DField.SSetableCards = append(MainGame.DField.SSetableCards, pcard)
		}
	}

	// Activatable
	count = int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		desc := int32(binary.LittleEndian.Uint32(pbuf))
		pbuf = pbuf[4:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			flag := 0
			if code&0x80000000 != 0 {
				flag = EDESCOperation
				code &= 0x7fffffff
			}
			MainGame.DField.ActivatableCards = append(MainGame.DField.ActivatableCards, pcard)
			MainGame.DField.ActivatableDescs = append(MainGame.DField.ActivatableDescs, [2]int{int(desc), flag})
			if flag&EDESCOperation != 0 {
				pcard.ChainCode = code
				MainGame.DField.ContiCards = append(MainGame.DField.ContiCards, pcard)
				MainGame.DField.ContiAct = true
			} else {
				pcard.CmdFlag |= CommandActivate
				if pcard.Location == 0x10 { // GRAVE
					MainGame.DField.GraveAct[con] = true
				} else if pcard.Location == 0x20 { // REMOVED
					MainGame.DField.RemoveAct[con] = true
				} else if pcard.Location == 0x40 { // EXTRA
					MainGame.DField.ExtraAct[con] = true
				}
			}
		}
	}

	if pbuf[0] != 0 {
		MainGame.DField.ShowBP = true
	}
	pbuf = pbuf[1:]
	if pbuf[0] != 0 {
		MainGame.DField.ShowEP = true
	}
	pbuf = pbuf[1:]
	if pbuf[0] != 0 {
		MainGame.DField.ShowShuffle = true
	}
	pbuf = pbuf[1:]

	return false
}

// MSG_SELECT_YESNO (13) and MSG_SELECT_EFFECTYN (12)
func (dc *DuelClient) handleSelectYesNo(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	desc := int32(binary.LittleEndian.Uint32(pbuf[1:]))
	MainGame.Dialog.ShowYesNo(desc)
	return false
}

func (dc *DuelClient) handleSelectEffectYN(pbuf []byte) bool {
	return dc.handleSelectYesNo(pbuf)
}

// MSG_SELECT_OPTION (14)
func (dc *DuelClient) handleSelectOption(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	count := int(pbuf[1])
	pbuf = pbuf[2:]
	options := make([]int32, count)
	for i := 0; i < count; i++ {
		options[i] = int32(binary.LittleEndian.Uint32(pbuf))
		pbuf = pbuf[4:]
	}
	MainGame.Dialog.ShowOption(options)
	return false
}

// MSG_SELECT_CARD (15)
func (dc *DuelClient) handleSelectCard(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	cancelable := pbuf[1] != 0
	min := int(pbuf[2])
	max := int(pbuf[3])
	count := int(pbuf[4])
	pbuf = pbuf[5:]

	MainGame.DField.SelectableCards = nil
	MainGame.DField.SelectedCards = nil
	MainGame.DField.SelectCancelable = cancelable
	MainGame.DField.SelectMin = min
	MainGame.DField.SelectMax = max
	MainGame.DField.SelectReady = min == 0

	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		s := int(pbuf[0])
		pbuf = pbuf[1:]
		ss := int(pbuf[0])
		pbuf = pbuf[1:]

		var pcard *ClientCard
		if l&0x80 != 0 {
			base := MainGame.DField.GetCard(c, l&0x7f, s)
			if base != nil && ss < len(base.Overlayed) {
				pcard = base.Overlayed[ss]
			}
		} else {
			pcard = MainGame.DField.GetCard(c, l, s)
		}
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			pcard.SelectSeq = uint32(i)
			pcard.IsSelectable = true
			pcard.IsSelected = false
			MainGame.DField.SelectableCards = append(MainGame.DField.SelectableCards, pcard)
		}
	}

	hint := dc.selectHint
	if hint == 0 {
		hint = 560 // default select string
	}
	MainGame.Dialog.ShowCardSelect(MainGame.DField.SelectableCards, min, max, cancelable, fmt.Sprintf("Select %d-%d", min, max))
	return false
}
func (dc *DuelClient) handleSelectChain(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	pbuf = pbuf[1:]
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	specount := int(pbuf[0])
	pbuf = pbuf[1:]
	_ = binary.LittleEndian.Uint32(pbuf) // hint0
	pbuf = pbuf[4:]
	_ = binary.LittleEndian.Uint32(pbuf) // hint1
	pbuf = pbuf[4:]

	selectTrigger := specount == 0x7f
	MainGame.DField.ChainForced = false
	MainGame.DField.ActivatableCards = nil
	MainGame.DField.ActivatableDescs = nil
	MainGame.DField.ContiCards = nil
	MainGame.DField.ContiAct = false

	for i := 0; i < count; i++ {
		flag := int(pbuf[0])
		pbuf = pbuf[1:]
		forced := int(pbuf[0])
		pbuf = pbuf[1:]
		flag |= forced << 8
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		ss := int(pbuf[0])
		pbuf = pbuf[1:]
		desc := int32(binary.LittleEndian.Uint32(pbuf))
		pbuf = pbuf[4:]

		var pcard *ClientCard
		if l&0x80 != 0 {
			base := MainGame.DField.GetCard(c, l&0x7f, seq)
			if base != nil && ss < len(base.Overlayed) {
				pcard = base.Overlayed[ss]
			}
		} else {
			pcard = MainGame.DField.GetCard(c, l, seq)
		}
		if pcard != nil {
			MainGame.DField.ActivatableCards = append(MainGame.DField.ActivatableCards, pcard)
			MainGame.DField.ActivatableDescs = append(MainGame.DField.ActivatableDescs, [2]int{int(desc), flag})
			pcard.IsSelected = false
			if forced != 0 {
				MainGame.DField.ChainForced = true
			}
			if flag&EDESCOperation != 0 {
				pcard.ChainCode = code
				MainGame.DField.ContiCards = append(MainGame.DField.ContiCards, pcard)
				MainGame.DField.ContiAct = true
			} else {
				pcard.IsSelectable = true
			}
		}
	}

	// Auto-response logic (simplified: no ignore_chain / always_chain settings)
	if !selectTrigger && !MainGame.DField.ChainForced {
		if count == 0 || specount == 0 {
			Client.SetResponseI(-1)
			MainGame.DField.ClearChainSelect()
			Client.SendResponse()
			return true
		}
	}

	// Auto-select forced chain
	for i := 0; i < len(MainGame.DField.ActivatableDescs); i++ {
		if MainGame.DField.ActivatableDescs[i][1]>>8 != 0 {
			Client.SetResponseI(int32(i))
			MainGame.DField.ClearChainSelect()
			Client.SendResponse()
			return true
		}
	}

	// Show chain selection dialog
	options := make([]int32, count)
	for i := 0; i < count; i++ {
		options[i] = int32(i)
	}
	MainGame.Dialog.ShowOption(options)
	return false
}
func (dc *DuelClient) handleSelectPlace(pbuf []byte) bool {
	selectingPlayer := int(pbuf[0])
	pbuf = pbuf[1:]
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	field := ^binary.LittleEndian.Uint32(pbuf)

	if selectingPlayer == MainGame.LocalPlayer(1) {
		field = (field >> 16) | (field << 16)
	}

	MainGame.DField.SelectMin = count
	if count > 0 {
		MainGame.DField.SelectMin = count
	} else {
		MainGame.DField.SelectMin = 1
	}
	MainGame.DField.SelectReady = false
	MainGame.DField.SelectCancelable = count == 0
	MainGame.DField.SelectableField = field
	MainGame.DField.SelectedField = 0

	// Auto-position logic (simplified: no auto-position settings)
	// If only one position is available, auto-select it
	if bits.OnesCount32(field) == 1 {
		resp := make([]byte, 3)
		if field&0x7f != 0 {
			resp[0] = uint8(MainGame.LocalPlayer(0))
			resp[1] = 0x04 // MZONE
			resp[2] = byte(bits.TrailingZeros32(field & 0x7f))
		} else if field&0x3f00 != 0 {
			resp[0] = uint8(MainGame.LocalPlayer(0))
			resp[1] = 0x08 // SZONE
			resp[2] = byte(bits.TrailingZeros32((field >> 8) & 0x3f))
		} else if field&0xc000 != 0 {
			resp[0] = uint8(MainGame.LocalPlayer(0))
			resp[1] = 0x08 // SZONE
			resp[2] = byte(bits.TrailingZeros32((field>>14)&0x3)) + 6
		} else if field&0x7f0000 != 0 {
			resp[0] = uint8(MainGame.LocalPlayer(1))
			resp[1] = 0x04 // MZONE
			resp[2] = byte(bits.TrailingZeros32((field >> 16) & 0x7f))
		} else if field&0x3f000000 != 0 {
			resp[0] = uint8(MainGame.LocalPlayer(1))
			resp[1] = 0x08 // SZONE
			resp[2] = byte(bits.TrailingZeros32((field >> 24) & 0x3f))
		} else if field&0xc0000000 != 0 {
			resp[0] = uint8(MainGame.LocalPlayer(1))
			resp[1] = 0x08 // SZONE
			resp[2] = byte(bits.TrailingZeros32((field>>30)&0x3)) + 6
		}
		MainGame.DField.SelectableField = 0
		Client.SetResponseB(resp)
		Client.SendResponse()
		return true
	}

	return false
}
func (dc *DuelClient) handleSelectPosition(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	pbuf = pbuf[1:]
	code := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	positions := pbuf[0]

	if positions == 0x01 || positions == 0x02 || positions == 0x04 || positions == 0x08 {
		Client.SetResponseI(int32(positions))
		Client.SendResponse()
		return true
	}

	MainGame.Dialog.ShowPosition(code, positions)
	return false
}
func (dc *DuelClient) handleSelectTribute(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	pbuf = pbuf[1:]
	cancelable := pbuf[0] != 0
	pbuf = pbuf[1:]
	min := int(pbuf[0])
	pbuf = pbuf[1:]
	max := int(pbuf[0])
	pbuf = pbuf[1:]
	count := int(pbuf[0])
	pbuf = pbuf[1:]

	MainGame.DField.SelectableCards = nil
	MainGame.DField.SelectedCards = nil
	MainGame.DField.SelectCancelable = cancelable
	MainGame.DField.SelectMin = min
	MainGame.DField.SelectMax = max
	MainGame.DField.SelectReady = min == 0
	MainGame.DField.SelectSumAll = nil
	MainGame.DField.SelectSumCards = make(map[*ClientCard]struct{})

	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		t := int(pbuf[0])
		pbuf = pbuf[1:]

		pcard := MainGame.DField.GetCard(c, l, seq)
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			pcard.SelectSeq = uint32(i)
			pcard.OpParam = uint32(t<<16 | 1)
			pcard.IsSelectable = true
			pcard.IsSelected = false
			MainGame.DField.SelectableCards = append(MainGame.DField.SelectableCards, pcard)
			MainGame.DField.SelectSumAll = append(MainGame.DField.SelectSumAll, pcard)
		}
	}

	// 算出哪些卡还能选：祭品数要落在 [min,max]，凑不出来的卡不该可点。
	MainGame.DField.SelectReady = MainGame.DField.CheckSelectTribute()

	hint := dc.selectHint
	if hint == 0 {
		hint = 531 // default tribute string
	}
	MainGame.Dialog.ShowCardSelect(MainGame.DField.SelectableCards, min, max, cancelable,
		fmt.Sprintf("选择祭品 %d-%d", min, max))
	return false
}
func (dc *DuelClient) handleSelectCounter(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	pbuf = pbuf[1:]
	counterType := binary.LittleEndian.Uint16(pbuf)
	pbuf = pbuf[2:]
	counterCount := int(binary.LittleEndian.Uint16(pbuf))
	pbuf = pbuf[2:]
	count := int(pbuf[0])
	pbuf = pbuf[1:]

	MainGame.DField.SelectableCards = nil
	MainGame.DField.SelectCounterType = int(counterType)
	MainGame.DField.SelectCounterCount = counterCount

	for i := 0; i < count; i++ {
		_ = binary.LittleEndian.Uint32(pbuf) // code
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		t := int(binary.LittleEndian.Uint16(pbuf))
		pbuf = pbuf[2:]

		pcard := MainGame.DField.GetCard(c, l, seq)
		if pcard != nil {
			pcard.OpParam = uint32(t<<16) | uint32(t)
			pcard.IsSelectable = true
			MainGame.DField.SelectableCards = append(MainGame.DField.SelectableCards, pcard)
		}
	}

	return false
}
func (dc *DuelClient) handleSelectSum(pbuf []byte) bool {
	selectMode := int(pbuf[0])
	pbuf = pbuf[1:]
	_ = pbuf[0] // selecting_player
	pbuf = pbuf[1:]
	sumVal := int32(binary.LittleEndian.Uint32(pbuf))
	pbuf = pbuf[4:]
	min := int(pbuf[0])
	pbuf = pbuf[1:]
	max := int(pbuf[0])
	pbuf = pbuf[1:]
	mustCount := int(pbuf[0])
	pbuf = pbuf[1:]

	MainGame.DField.SelectableCards = nil
	MainGame.DField.SelectedCards = nil
	MainGame.DField.SelectMin = min
	MainGame.DField.SelectMax = max
	MainGame.DField.SelectReady = false
	MainGame.DField.SelectSumAll = nil
	MainGame.DField.SelectSumCards = make(map[*ClientCard]struct{})
	MainGame.DField.MustSelectCount = mustCount

	// must_select cards
	mustCards := make([]*ClientCard, 0, mustCount)
	for i := 0; i < mustCount; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		opParam := int32(binary.LittleEndian.Uint32(pbuf))
		pbuf = pbuf[4:]

		pcard := MainGame.DField.GetCard(c, l, seq)
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			pcard.OpParam = uint32(opParam)
			pcard.SelectSeq = 0
			pcard.IsSelected = true
			mustCards = append(mustCards, pcard)
		}
	}

	// optional cards
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	optCards := make([]*ClientCard, 0, count)
	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		opParam := int32(binary.LittleEndian.Uint32(pbuf))
		pbuf = pbuf[4:]

		pcard := MainGame.DField.GetCard(c, l, seq)
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			pcard.OpParam = uint32(opParam)
			pcard.SelectSeq = uint32(i)
			pcard.IsSelectable = true
			pcard.IsSelected = false
			optCards = append(optCards, pcard)
		}
	}

	// Merge must + optional for display
	allCards := append(mustCards, optCards...)
	MainGame.DField.SelectedCards = mustCards
	MainGame.DField.SelectableCards = allCards
	MainGame.DField.SelectSumAll = optCards

	// 算出哪些卡还能选。selectMode 0 是「和正好等于目标」，非 0 是「达到即可但不能多余」。
	MainGame.DField.SelectMode = selectMode
	MainGame.DField.SelectSumVal = int(sumVal)
	MainGame.DField.SelectReady = MainGame.DField.CheckSelectSum()

	hint := dc.selectHint
	if hint == 0 {
		hint = 560
	}
	MainGame.Dialog.ShowCardSelect(MainGame.DField.SelectableCards, min+mustCount, max+mustCount, false,
		fmt.Sprintf("选择卡片凑出 %d", sumVal))
	return false
}
func (dc *DuelClient) handleSelectDisfield(pbuf []byte) bool {
	// MSG_SELECT_DISFIELD has the same format as MSG_SELECT_PLACE
	return dc.handleSelectPlace(pbuf)
}
func (dc *DuelClient) handleSortCard(pbuf []byte) bool {
	_ = pbuf[0] // player
	pbuf = pbuf[1:]
	count := int(pbuf[0])
	pbuf = pbuf[1:]

	cards := make([]*ClientCard, 0, count)
	for i := 0; i < count; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]

		pcard := MainGame.DField.GetCard(c, l, seq)
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			cards = append(cards, pcard)
		}
	}

	MainGame.Dialog.ShowSort(cards)
	return false
}
func (dc *DuelClient) handleSelectUnselectCard(pbuf []byte) bool {
	_ = pbuf[0] // selecting_player
	pbuf = pbuf[1:]
	finishable := pbuf[0] != 0
	pbuf = pbuf[1:]
	cancelable := pbuf[0] != 0
	pbuf = pbuf[1:]
	min := int(pbuf[0])
	pbuf = pbuf[1:]
	max := int(pbuf[0])
	pbuf = pbuf[1:]
	count1 := int(pbuf[0])
	pbuf = pbuf[1:]

	MainGame.DField.SelectableCards = nil
	MainGame.DField.SelectedCards = nil
	MainGame.DField.SelectCancelable = finishable || cancelable
	MainGame.DField.SelectMin = min
	MainGame.DField.SelectMax = max
	MainGame.DField.SelectReady = finishable

	for i := 0; i < count1; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		ss := int(pbuf[0])
		pbuf = pbuf[1:]

		var pcard *ClientCard
		if l&0x80 != 0 {
			base := MainGame.DField.GetCard(c, l&0x7f, seq)
			if base != nil && ss < len(base.Overlayed) {
				pcard = base.Overlayed[ss]
			}
		} else {
			pcard = MainGame.DField.GetCard(c, l, seq)
		}
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			pcard.SelectSeq = uint32(i)
			pcard.IsSelectable = true
			pcard.IsSelected = false
			MainGame.DField.SelectableCards = append(MainGame.DField.SelectableCards, pcard)
		}
	}

	count2 := int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count2; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		ss := int(pbuf[0])
		pbuf = pbuf[1:]

		var pcard *ClientCard
		if l&0x80 != 0 {
			base := MainGame.DField.GetCard(c, l&0x7f, seq)
			if base != nil && ss < len(base.Overlayed) {
				pcard = base.Overlayed[ss]
			}
		} else {
			pcard = MainGame.DField.GetCard(c, l, seq)
		}
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			pcard.SelectSeq = uint32(i)
			pcard.IsSelectable = true
			pcard.IsSelected = true
			MainGame.DField.SelectedCards = append(MainGame.DField.SelectedCards, pcard)
			MainGame.DField.SelectableCards = append(MainGame.DField.SelectableCards, pcard)
		}
	}

	hint := dc.selectHint
	if hint == 0 {
		hint = 560
	}
	MainGame.Dialog.ShowCardSelect(MainGame.DField.SelectableCards, min, max, MainGame.DField.SelectCancelable, fmt.Sprintf("Select %d-%d", min, max))
	return false
}
func (dc *DuelClient) handleConfirmDecktop(pbuf []byte) bool {
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	cards := make([]*ClientCard, 0, count)
	for i := 0; i < count; i++ {
		if len(pbuf) < 8 {
			break
		}
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			cards = append(cards, pcard)
		}
	}
	if len(cards) > 0 {
		MainGame.Dialog.ShowCardSelect(cards, 0, len(cards), false, "Confirm Decktop")
	}
	return true
}
func (dc *DuelClient) handleConfirmCards(pbuf []byte) bool {
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	cards := make([]*ClientCard, 0, count)
	for i := 0; i < count; i++ {
		if len(pbuf) < 8 {
			break
		}
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		con := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		loc := pbuf[0]
		pbuf = pbuf[1:]
		seq := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			if code != 0 && pcard.Code != code {
				pcard.SetCode(code)
			}
			cards = append(cards, pcard)
		}
	}
	if len(cards) > 0 {
		MainGame.Dialog.ShowCardSelect(cards, 0, len(cards), false, "Confirm Cards")
	}
	return true
}
func (dc *DuelClient) handleRefreshDeck(pbuf []byte) bool {
	return true
}
func (dc *DuelClient) handleSwapGraveDeck(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	MainGame.DField.Deck[player], MainGame.DField.Grave[player] = MainGame.DField.Grave[player], MainGame.DField.Deck[player]
	for _, c := range MainGame.DField.Deck[player] {
		c.Location = 0x01
	}
	for _, c := range MainGame.DField.Grave[player] {
		c.Location = 0x10
	}
	return true
}
func (dc *DuelClient) handleShuffleSetCard(pbuf []byte) bool {
	if len(pbuf) < 2 {
		return true
	}
	loc := pbuf[0]
	count := int(pbuf[1])
	pbuf = pbuf[2:]
	codes := make([]uint32, count)
	controllers := make([]int, count)
	sequences := make([]int, count)
	for i := 0; i < count; i++ {
		if len(pbuf) < 6 {
			break
		}
		codes[i] = binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		controllers[i] = MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		sequences[i] = int(pbuf[0])
		pbuf = pbuf[1:]
	}
	for i := 0; i < count; i++ {
		pcard := MainGame.DField.GetCard(controllers[i], loc, sequences[i])
		if pcard != nil && codes[i] != 0 && pcard.Code != codes[i] {
			pcard.SetCode(codes[i])
		}
	}
	return true
}
func (dc *DuelClient) handleDeckTop(pbuf []byte) bool {
	if len(pbuf) < 6 {
		return true
	}
	con := MainGame.LocalPlayer(int(pbuf[0]))
	loc := pbuf[1]
	seq := int(pbuf[2])
	code := binary.LittleEndian.Uint32(pbuf[3:])
	pcard := MainGame.DField.GetCard(con, loc, seq)
	if pcard != nil {
		if code != 0 && pcard.Code != code {
			pcard.SetCode(code)
		}
		pcard.IsReversed = code == 0
	}
	return true
}
func (dc *DuelClient) handleShuffleExtra(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	count := int(pbuf[1])
	pbuf = pbuf[2:]
	for i := 0; i < count && i < len(MainGame.DField.Extra[player]); i++ {
		if len(pbuf) < 4 {
			break
		}
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		MainGame.DField.Extra[player][i].SetCode(code & 0x7fffffff)
	}
	return true
}
func (dc *DuelClient) handleFieldDisabled(pbuf []byte) bool {
	if len(pbuf) >= 4 {
		MainGame.DField.DisabledField = binary.LittleEndian.Uint32(pbuf)
	}
	return true
}
func (dc *DuelClient) handleSummoning(msgType uint8, pbuf []byte) bool {
	if len(pbuf) < 8 {
		return true
	}
	code := binary.LittleEndian.Uint32(pbuf)
	con := MainGame.LocalPlayer(int(pbuf[4]))
	loc := pbuf[5]
	seq := int(pbuf[6])
	pos := pbuf[7]
	pcard := MainGame.DField.GetCard(con, loc, seq)
	if pcard != nil {
		if code != 0 && pcard.Code != code {
			pcard.SetCode(code)
		}
		pcard.Position = pos
	}
	return true
}
func (dc *DuelClient) handleSummoned() bool {
	MainGame.DField.ClearCommandFlag()
	return true
}
func (dc *DuelClient) handleChaining(pbuf []byte) bool {
	if len(pbuf) < 13 {
		return true
	}
	code := binary.LittleEndian.Uint32(pbuf)
	con := MainGame.LocalPlayer(int(pbuf[4]))
	loc := pbuf[5]
	seq := int(pbuf[6])
	_ = pbuf[7] // subsequence
	desc := binary.LittleEndian.Uint32(pbuf[8:])
	pcard := MainGame.DField.GetCard(con, loc, seq)
	if pcard != nil {
		if code != 0 && pcard.Code != code {
			pcard.SetCode(code)
		}
		pcard.IsShowChainTarget = true
		ci := ChainInfo{
			Code: int(code),
			Desc: int(desc),
		}
		MainGame.DField.Chains = append(MainGame.DField.Chains, ci)
	}
	return true
}
func (dc *DuelClient) handleChained(pbuf []byte) bool {
	if len(pbuf) >= 1 {
		fmt.Printf("Chain %d started\n", pbuf[0])
	}
	return true
}
func (dc *DuelClient) handleChainSolving(pbuf []byte) bool {
	if len(pbuf) >= 1 {
		fmt.Printf("Chain %d solving\n", pbuf[0])
	}
	return true
}
func (dc *DuelClient) handleChainSolved(pbuf []byte) bool {
	if len(pbuf) >= 1 {
		fmt.Printf("Chain %d solved\n", pbuf[0])
	}
	return true
}
func (dc *DuelClient) handleChainEnd() bool {
	MainGame.DField.Chains = nil
	for c := range MainGame.DField.OverlayCards {
		c.IsShowChainTarget = false
	}
	return true
}
func (dc *DuelClient) handleChainNegated(pbuf []byte) bool {
	if len(pbuf) >= 1 {
		fmt.Printf("Chain %d negated\n", pbuf[0])
	}
	return true
}
func (dc *DuelClient) handleChainDisabled(pbuf []byte) bool {
	if len(pbuf) >= 1 {
		fmt.Printf("Chain %d disabled\n", pbuf[0])
	}
	return true
}
func (dc *DuelClient) handleCardSelected(pbuf []byte) bool {
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		if len(pbuf) < 3 {
			break
		}
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		s := int(pbuf[0])
		pbuf = pbuf[1:]
		_ = MainGame.DField.GetCard(c, l, s)
	}
	return true
}
func (dc *DuelClient) handleRandomSelected(pbuf []byte) bool {
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		if len(pbuf) < 7 {
			break
		}
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		c := MainGame.LocalPlayer(int(pbuf[0]))
		pbuf = pbuf[1:]
		l := pbuf[0]
		pbuf = pbuf[1:]
		s := int(pbuf[0])
		pbuf = pbuf[1:]
		pcard := MainGame.DField.GetCard(c, l, s)
		if pcard != nil && code != 0 && pcard.Code != code {
			pcard.SetCode(code)
		}
	}
	return true
}
func (dc *DuelClient) handleBecomeTarget(pbuf []byte) bool {
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	for i := 0; i < count; i++ {
		if len(pbuf) < 4 {
			break
		}
		con := MainGame.LocalPlayer(int(pbuf[0]))
		loc := pbuf[1]
		seq := int(pbuf[2])
		pcard := MainGame.DField.GetCard(con, loc, seq)
		if pcard != nil {
			pcard.IsShowTarget = true
		}
		pbuf = pbuf[4:]
	}
	return true
}
func (dc *DuelClient) handleLPUpdate(pbuf []byte) bool {
	player := MainGame.LocalPlayer(int(pbuf[0]))
	lp := int32(binary.LittleEndian.Uint32(pbuf[1:]))
	MainGame.DInfo.LP[player] = int(lp)
	MainGame.DInfo.StrLP[player] = fmt.Sprintf("%d", lp)
	return true
}
func (dc *DuelClient) handleAttack(pbuf []byte) bool {
	if len(pbuf) < 9 {
		return true
	}
	code := binary.LittleEndian.Uint32(pbuf)
	_ = code
	con := MainGame.LocalPlayer(int(pbuf[4]))
	loc := pbuf[5]
	seq := int(pbuf[6])
	_ = MainGame.DField.GetCard(con, loc, seq)
	MainGame.IsAttacking = 1
	if len(pbuf) >= 18 {
		tcode := binary.LittleEndian.Uint32(pbuf[9:])
		_ = tcode
		tcon := MainGame.LocalPlayer(int(pbuf[13]))
		tloc := pbuf[14]
		tseq := int(pbuf[15])
		_ = MainGame.DField.GetCard(tcon, tloc, tseq)
	}
	return true
}
func (dc *DuelClient) handleBattle(pbuf []byte) bool {
	MainGame.IsAttacking = 0
	return true
}
func (dc *DuelClient) handleAttackDisabled() bool {
	MainGame.IsAttacking = 0
	return true
}
func (dc *DuelClient) handleDamageStepStart() bool {
	MainGame.IsAttacking = 2
	return true
}
func (dc *DuelClient) handleDamageStepEnd() bool {
	MainGame.IsAttacking = 0
	return true
}
func (dc *DuelClient) handleMissedEffect(pbuf []byte) bool {
	if len(pbuf) >= 4 {
		code := binary.LittleEndian.Uint32(pbuf)
		fmt.Printf("Missed effect: %d\n", code)
	}
	return true
}
func (dc *DuelClient) handleTossCoin(pbuf []byte) bool {
	if len(pbuf) < 2 {
		return true
	}
	player := MainGame.LocalPlayer(int(pbuf[0]))
	count := int(pbuf[1])
	if len(pbuf) < 2+count {
		return true
	}
	results := make([]int, count)
	for i := 0; i < count; i++ {
		results[i] = int(pbuf[2+i])
	}
	fmt.Printf("Player %d tossed coin: %v\n", player, results)
	return true
}
func (dc *DuelClient) handleTossDice(pbuf []byte) bool {
	if len(pbuf) < 2 {
		return true
	}
	player := MainGame.LocalPlayer(int(pbuf[0]))
	count := int(pbuf[1])
	if len(pbuf) < 2+count {
		return true
	}
	results := make([]int, count)
	for i := 0; i < count; i++ {
		results[i] = int(pbuf[2+i])
	}
	fmt.Printf("Player %d tossed dice: %v\n", player, results)
	return true
}
func (dc *DuelClient) handleRockPaperScissors(pbuf []byte) bool {
	_ = pbuf[0]                                  // player
	MainGame.Dialog.ShowOption([]int32{1, 2, 3}) // Rock, Paper, Scissors
	return false
}
func (dc *DuelClient) handleHandRes(pbuf []byte) bool {
	if len(pbuf) >= 2 {
		fmt.Printf("Hand result: %d vs %d\n", pbuf[0], pbuf[1])
	}
	return true
}
func (dc *DuelClient) handleAnnounceRace(pbuf []byte) bool {
	_ = pbuf[0] // player
	pbuf = pbuf[1:]
	_ = pbuf[0] // announce_count
	pbuf = pbuf[1:]
	avail := binary.LittleEndian.Uint32(pbuf)
	MainGame.Dialog.ShowAnnounceRace(avail)
	return false
}
func (dc *DuelClient) handleAnnounceAttrib(pbuf []byte) bool {
	_ = pbuf[0] // player
	pbuf = pbuf[1:]
	_ = pbuf[0] // announce_count
	pbuf = pbuf[1:]
	avail := binary.LittleEndian.Uint32(pbuf)
	MainGame.Dialog.ShowAnnounceAttrib(avail)
	return false
}
func (dc *DuelClient) handleAnnounceCard(pbuf []byte) bool {
	_ = pbuf[0] // player
	pbuf = pbuf[1:]
	count := int(pbuf[0])
	pbuf = pbuf[1:]
	opcodes := make([]uint32, count)
	for i := 0; i < count; i++ {
		opcodes[i] = binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
	}
	MainGame.Dialog.ShowAnnounceCard(opcodes)
	return false
}
func (dc *DuelClient) handleAnnounceNumber(pbuf []byte) bool {
	_ = pbuf[0] // player
	pbuf = pbuf[1:]
	count := int(pbuf[0])
	pbuf = pbuf[1:]

	options := make([]int32, count)
	for i := 0; i < count; i++ {
		options[i] = int32(binary.LittleEndian.Uint32(pbuf))
		pbuf = pbuf[4:]
	}

	MainGame.Dialog.ShowOption(options)
	return false
}
func (dc *DuelClient) handleTagSwap(pbuf []byte) bool {
	if len(pbuf) < 17 {
		return true
	}
	player := MainGame.LocalPlayer(int(pbuf[0]))
	pbuf = pbuf[1:]
	mcount := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	ecount := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	pcount := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]
	hcount := binary.LittleEndian.Uint32(pbuf)
	pbuf = pbuf[4:]

	// Update deck count
	MainGame.DField.Deck[player] = make([]*ClientCard, 0, mcount)
	for i := uint32(0); i < mcount; i++ {
		MainGame.DField.Deck[player] = append(MainGame.DField.Deck[player], NewClientCard())
	}
	// Update extra count
	MainGame.DField.Extra[player] = make([]*ClientCard, 0, ecount)
	for i := uint32(0); i < ecount; i++ {
		MainGame.DField.Extra[player] = append(MainGame.DField.Extra[player], NewClientCard())
	}
	// Read top deck code if hand present
	if hcount > 0 && len(pbuf) >= 4 {
		topcode := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		if len(MainGame.DField.Deck[player]) > 0 {
			MainGame.DField.Deck[player][len(MainGame.DField.Deck[player])-1].SetCode(topcode)
		}
	}
	// Read field monsters
	for i := uint32(0); i < pcount && len(pbuf) >= 5; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		pos := pbuf[0]
		pbuf = pbuf[1:]
		if int(i) < len(MainGame.DField.MZone[player]) {
			pcard := MainGame.DField.MZone[player][i]
			if pcard != nil {
				pcard.SetCode(code)
				pcard.Position = pos
			}
		}
	}
	// Read hand
	MainGame.DField.Hand[player] = make([]*ClientCard, 0, hcount)
	for i := uint32(0); i < hcount && len(pbuf) >= 4; i++ {
		code := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		pcard := NewClientCard()
		pcard.SetCode(code)
		MainGame.DField.Hand[player] = append(MainGame.DField.Hand[player], pcard)
	}
	return true
}
func (dc *DuelClient) handleReloadField(pbuf []byte) bool {
	if len(pbuf) < 1 {
		return true
	}
	MainGame.DInfo.DuelRule = int(pbuf[0])
	pbuf = pbuf[1:]
	for player := 0; player < 2; player++ {
		if len(pbuf) < 12 {
			break
		}
		lp := int32(binary.LittleEndian.Uint32(pbuf))
		pbuf = pbuf[4:]
		deckCount := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		extraCount := binary.LittleEndian.Uint32(pbuf)
		pbuf = pbuf[4:]
		MainGame.DInfo.LP[player] = int(lp)
		MainGame.DInfo.StrLP[player] = fmt.Sprintf("%d", lp)
		MainGame.DField.Deck[player] = make([]*ClientCard, 0, deckCount)
		MainGame.DField.Extra[player] = make([]*ClientCard, 0, extraCount)
		if len(pbuf) < 1 {
			break
		}
		mzoneCount := int(pbuf[0])
		pbuf = pbuf[1:]
		for i := 0; i < mzoneCount && len(pbuf) >= 1; i++ {
			val := pbuf[0]
			pbuf = pbuf[1:]
			if val != 0 {
				pcard := NewClientCard()
				pcard.Controler = uint8(player)
				pcard.Location = 0x04
				pcard.Sequence = uint8(i)
				pcard.Position = val & 0xff
				MainGame.DField.MZone[player] = append(MainGame.DField.MZone[player], pcard)
			}
		}
		if len(pbuf) < 1 {
			break
		}
		szoneCount := int(pbuf[0])
		pbuf = pbuf[1:]
		for i := 0; i < szoneCount && len(pbuf) >= 1; i++ {
			val := pbuf[0]
			pbuf = pbuf[1:]
			if val != 0 {
				pcard := NewClientCard()
				pcard.Controler = uint8(player)
				pcard.Location = 0x08
				pcard.Sequence = uint8(i)
				pcard.Position = val & 0xff
				MainGame.DField.SZone[player] = append(MainGame.DField.SZone[player], pcard)
			}
		}
		if len(pbuf) < 1 {
			break
		}
		graveCount := int(pbuf[0])
		pbuf = pbuf[1:]
		for i := 0; i < graveCount && len(pbuf) >= 1; i++ {
			val := pbuf[0]
			pbuf = pbuf[1:]
			if val != 0 {
				pcard := NewClientCard()
				pcard.Controler = uint8(player)
				pcard.Location = 0x10
				MainGame.DField.Grave[player] = append(MainGame.DField.Grave[player], pcard)
			}
		}
		if len(pbuf) < 1 {
			break
		}
		removeCount := int(pbuf[0])
		pbuf = pbuf[1:]
		for i := 0; i < removeCount && len(pbuf) >= 1; i++ {
			val := pbuf[0]
			pbuf = pbuf[1:]
			if val != 0 {
				pcard := NewClientCard()
				pcard.Controler = uint8(player)
				pcard.Location = 0x20
				MainGame.DField.Remove[player] = append(MainGame.DField.Remove[player], pcard)
			}
		}
	}
	return true
}
func (dc *DuelClient) handleAIName(pbuf []byte) bool {
	name := Utf16ToString(pbuf)
	fmt.Printf("AI Name: %s\n", name)
	return true
}
func (dc *DuelClient) handleShowHint(pbuf []byte) bool {
	if len(pbuf) >= 4 {
		hint := binary.LittleEndian.Uint32(pbuf)
		fmt.Printf("Show hint: %d\n", hint)
	}
	return true
}
func (dc *DuelClient) handleMatchKill(pbuf []byte) bool {
	if len(pbuf) >= 4 {
		mk := binary.LittleEndian.Uint32(pbuf)
		dc.matchKill = int(mk)
	}
	return true
}
func (dc *DuelClient) handleCustomMsg(pbuf []byte) bool {
	msg := string(pbuf)
	fmt.Printf("Custom msg: %s\n", msg)
	return true
}

// MSG_WIN (5) — 决斗结束。
// 对应 C++ duelclient.cpp 的 case MSG_WIN。
func (dc *DuelClient) handleWin(pbuf []byte) bool {
	if len(pbuf) < 2 {
		return true
	}
	player := int(pbuf[0])
	winType := int(pbuf[1])

	MainGame.DInfo.IsFinished = true
	MainGame.DInfo.WinPlayer = player
	MainGame.DInfo.WinType = winType

	// player == 2 表示平局；否则按本地视角判断是自己还是对手赢。
	// C++ 用 showcardcode 1/2/3 驱动一张胜负图，这里只记文本，由 GUI 决定怎么呈现。
	switch {
	case player == 2:
		MainGame.DInfo.VicString = "平局"
	case MainGame.LocalPlayer(player) == 0:
		MainGame.DInfo.VicString = winnerLine(MainGame.DInfo.ClientName, winType, dc.matchKill)
	default:
		MainGame.DInfo.VicString = winnerLine(MainGame.DInfo.HostName, winType, dc.matchKill)
	}
	MainGame.ShowCard = 101
	return true
}

// winnerLine 拼胜利文本。type >= 0x10 的胜利原因与玩家无关（超时、连接断开等），
// C++ 那边也是只显示原因、不带名字。matchKill 非零时是「某张卡直接终结比赛」。
func winnerLine(name string, winType, matchKill int) string {
	switch {
	case matchKill != 0:
		return fmt.Sprintf("由「%d」终结比赛", matchKill)
	case winType < 0x10:
		return fmt.Sprintf("[%s] %s", name, victoryReason(winType))
	default:
		return victoryReason(winType)
	}
}

// victoryReason 取胜负原因的文本（strings.conf 的 !victory 段）。
//
// 此前这里是硬编码的六条，而且**每一条都错位了**：0x0 被当成「LP 归零」，
// 实际是「投降」；0x1 才是基本分变成 0。玩家看到的败因根本对不上。
// 真实数据有 26 条，包括各种卡的特殊胜利，猜是猜不出来的。
func victoryReason(winType int) string {
	return Strings.Victory(winType)
}

// MSG_WAITING (3) — 等待对方操作。
// C++ 是把 stHintMsg 设为系统串 1390 并显示；这里写进 ShowingText，由 GUI 呈现。
func (dc *DuelClient) handleWaiting() bool {
	MainGame.WaitFrame = 0
	// 1390 是 C++ 这里用的系统串（"等待行动中..."）。此前是硬编码的自拟文案，
	// 现在统一走 strings.conf，跟原版一字不差。
	MainGame.ShowingText = Strings.System(1390)
	return true
}

// MSG_CARD_HINT (160) — 给某张卡挂提示。
// 格式: player(1) location(1) sequence(1) 保留(1) chtype(1) value(4)
func (dc *DuelClient) handleCardHint(pbuf []byte) bool {
	if len(pbuf) < 9 {
		return true
	}
	con := MainGame.LocalPlayer(int(pbuf[0]))
	loc := pbuf[1]
	seq := int(pbuf[2])
	// pbuf[3] 是 C++ 里读掉但没用的一字节
	chType := int(pbuf[4])
	value := int(binary.LittleEndian.Uint32(pbuf[5:]))

	pcard := MainGame.DField.GetCard(con, loc, seq)
	if pcard == nil {
		return true
	}

	switch chType {
	case ocgcore.CHINT_DESC_ADD:
		pcard.DescHints[value]++
	case ocgcore.CHINT_DESC_REMOVE:
		pcard.DescHints[value]--
		if pcard.DescHints[value] == 0 {
			delete(pcard.DescHints, value)
		}
	default:
		pcard.CHint = uint8(chType)
		pcard.ChValue = uint32(value)
	}
	MainGame.FieldRev.Bump()
	return true
}

// MSG_PLAYER_HINT (165) — 给某个玩家挂提示。
// 格式: player(1) chtype(1) value(4)
func (dc *DuelClient) handlePlayerHint(pbuf []byte) bool {
	if len(pbuf) < 6 {
		return true
	}
	player := MainGame.LocalPlayer(int(pbuf[0]))
	chType := int(pbuf[1])
	value := int(binary.LittleEndian.Uint32(pbuf[2:]))

	// value == CARD_QUESTION 且是自己时，这条提示的含义是「不能查看墓地」，
	// 而不是往提示表里塞一条（C++ duelclient.cpp 里的特例）。
	if value == CardQuestion && player == 0 {
		switch chType {
		case ocgcore.PHINT_DESC_ADD:
			MainGame.DField.CantCheckGrave = true
		case ocgcore.PHINT_DESC_REMOVE:
			MainGame.DField.CantCheckGrave = false
		}
		MainGame.FieldRev.Bump()
		return true
	}

	hints := MainGame.DField.PlayerDescHints[player]
	switch chType {
	case ocgcore.PHINT_DESC_ADD:
		hints[value]++
	case ocgcore.PHINT_DESC_REMOVE:
		hints[value]--
		if hints[value] == 0 {
			delete(hints, value)
		}
	}
	MainGame.FieldRev.Bump()
	return true
}

// MSG_CONFIRM_EXTRATOP (42) — 确认额外卡组顶端的若干张。
// 格式: player(1) count(1) 之后每张 code(4) + 3 字节位置信息
//
// 取卡的方式照搬 C++：从 extra 末尾往前数，跳过已翻开的 extra_p_count 张 ——
// 额外卡组里表侧的（灵摆）排在末尾，要确认的是它们下面那些背面的。
func (dc *DuelClient) handleConfirmExtratop(pbuf []byte) bool {
	if len(pbuf) < 2 {
		return true
	}
	player := MainGame.LocalPlayer(int(pbuf[0]))
	count := int(pbuf[1])

	MainGame.DField.SelectableCards = nil
	extra := MainGame.DField.Extra[player]
	rest := pbuf[2:]
	for i := 0; i < count; i++ {
		if len(rest) < 7 {
			break
		}
		code := binary.LittleEndian.Uint32(rest)
		rest = rest[7:] // code(4) + 3 字节

		idx := len(extra) - 1 - i - MainGame.DField.ExtraPCount[player]
		if idx < 0 || idx >= len(extra) || extra[idx] == nil {
			continue
		}
		if code != 0 {
			extra[idx].SetCode(code)
		}
		MainGame.DField.SelectableCards = append(MainGame.DField.SelectableCards, extra[idx])
	}
	MainGame.FieldRev.Bump()
	return true
}
