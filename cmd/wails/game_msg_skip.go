package main

import (
	"fmt"

	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// skipEngineMessageBody advances pbuf past the body of an engine message that
// has no entry in engineBindings (engine_bindings.go). Most message layouts now
// live in that table as codec-decoded structs; this residual switch only covers
// messages whose body codec cannot express and that have no UI consumer。
// It mirrors the skip offsets of the C++ ReplayMode::ReplayAnalyze
// (Fluorohydride/ygopro gframe/replay_mode.cpp) so the parser never loses message
// alignment, even for messages the UI ignores.
//
// It returns an error for message types whose layout is unknown; callers should
// stop parsing the batch rather than continue with a misaligned buffer.
func skipEngineMessageBody(pbuf *utils.YGOBuffer, engType uint8) error {
	skip := func(n int) error { return pbuf.Next(n) }

	switch engType {
	case ocgcore.MSG_DAMAGE_STEP_START, ocgcore.MSG_DAMAGE_STEP_END:
		return nil // no body

	case ocgcore.MSG_BE_CHAIN_TARGET:
		return skip(1)

	case ocgcore.MSG_CREATE_RELATION, ocgcore.MSG_RELEASE_RELATION:
		return skip(16)

	// MSG_RETRY / MSG_REVERSE_DECK / MSG_SHUFFLE_DECK / MSG_SWAP_GRAVE_DECK
	// 已迁入 engineBindings（无体事件或 player 单字节消息）。
	// MSG_MATCH_KILL 已迁入 engineBindings（MatchKillMsg，emit duel:match_kill）
	// MSG_TAG_SWAP 已迁入 engineBindings（tagSwapMsg，emit duel:tag_swap）

	default:
		return &unknownEngineMessageError{engType: engType}
	}
}

type unknownEngineMessageError struct {
	engType uint8
}

func (e *unknownEngineMessageError) Error() string {
	return fmt.Sprintf("unknown engine message type: %d", e.engType)
}
