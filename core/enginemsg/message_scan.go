package enginemsg

import (
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// This file mirrors the message-walk of the C++ client's
// ReplayMode::ReplayAnalyze (Fluorohydride/ygopro gframe/replay_mode.cpp): it walks an
// engine message batch message by message and reports whether the batch
// contains a message the driver must answer with a recorded response.
//
// The response decision is made from the STREAM, not from the engine's
// PROCESSOR_WAITING flag, exactly like the C++ client. Most waits carry the
// flag, but some (e.g. PROCESSOR_ROCK_PAPER_SCISSORS) return only
// buffer_size() while still expecting a response — keying off the flag alone
// desynchronizes the recorded response stream.

// BatchWinOffset is the sentinel returned for a batch containing MSG_WIN: the
// engine's adjust_step never terminates the duel on its own (it re-emits
// MSG_WIN on every subsequent adjust), so the driver must stop — the C++
// client does the same by returning false from SinglePlayAnalyze (win 之外
// 的消息不再处理).
const BatchWinOffset = -2

// BatchResponseOffset walks the batch and returns the offset of the first
// response-needing message, BatchWinOffset if the batch contains MSG_WIN, or
// -1 if the batch needs no response. ok is false
// when the batch could not be walked (unknown message layout); the caller can
// fall back to the engine flag in that case.
//
// 消息布局的唯一事实源是 EngineMsgLayouts（engine_msg_layout.go）。
func BatchResponseOffset(msg []byte) (offset int, ok bool) {
	pos := 0
	for pos < len(msg) {
		body := pos + 1
		end, response, win, ok := WalkEngineMessage(msg, body)
		if !ok {
			return 0, false
		}
		if win {
			return BatchWinOffset, true
		}
		if response {
			return pos, true
		}
		pos = end
	}
	return -1, true
}

func adv(pos *int, msg []byte, count int) bool {
	if count < 0 || *pos+count > len(msg) {
		return false
	}
	*pos += count
	return true
}

// walkCountList walks "count(1) + count*size" starting at *pos; the count byte
// itself is part of the message and is consumed here.
func walkCountList(pos *int, msg []byte, size int) bool {
	if !adv(pos, msg, 1) {
		return false
	}
	count := int(msg[*pos-1])
	return adv(pos, msg, count*size)
}

// walkQueryBlobs walks length-prefixed card-query blobs until the buffer ends
// or a fragment too small to be a query appears. MZONE/SZONE 空槽写 LEN_EMPTY(4)
// 标记（ocgapi.cpp query_field_card）：跳过 4 字节继续，保持后面的 blob 可达。
func walkQueryBlobs(pos *int, msg []byte) bool {
	for *pos < len(msg) {
		if len(msg)-*pos < 4 {
			return false
		}
		blobLen := int(uint32(msg[*pos]) | uint32(msg[*pos+1])<<8 | uint32(msg[*pos+2])<<16 | uint32(msg[*pos+3])<<24)
		if blobLen == ocgcore.LEN_EMPTY {
			// 空槽标记：不是 blob，占 4 字节
			*pos += 4
			continue
		}
		if blobLen < ocgcore.LEN_HEADER {
			// Not a query blob; whatever remains belongs to another message.
			return true
		}
		if !adv(pos, msg, blobLen) {
			return false
		}
	}
	return true
}

// walkOneQueryBlob walks exactly one length-prefixed card-query blob.
func walkOneQueryBlob(pos *int, msg []byte) bool {
	if len(msg)-*pos < 4 {
		return false
	}
	blobLen := int(uint32(msg[*pos]) | uint32(msg[*pos+1])<<8 | uint32(msg[*pos+2])<<16 | uint32(msg[*pos+3])<<24)
	if blobLen < ocgcore.LEN_HEADER {
		return true
	}
	return adv(pos, msg, blobLen)
}

func walkReloadField(pos *int, msg []byte) bool {
	if !adv(pos, msg, 1) {
		return false
	}
	for player := 0; player < 2; player++ {
		if !adv(pos, msg, 4) {
			return false
		}
		for i := 0; i < 7; i++ {
			if !adv(pos, msg, 1) {
				return false
			}
			if msg[*pos-1] != 0 && !adv(pos, msg, 2) {
				return false
			}
		}
		for i := 0; i < 8; i++ {
			if !adv(pos, msg, 1) {
				return false
			}
			if msg[*pos-1] != 0 && !adv(pos, msg, 1) {
				return false
			}
		}
		if !adv(pos, msg, 6) {
			return false
		}
	}
	// chain count(1) + 每条 15 字节（code + info_location + 控制者/区/序号 + 描述，
	// single_mode.cpp:730-731：pbuf += count * 15）
	if !adv(pos, msg, 1) {
		return false
	}
	return adv(pos, msg, int(msg[*pos-1])*15)
}

// SplitBatchMessages 把引擎消息批次按 engineMsgLayouts 的权威布局切分为
// 单条完整消息（type 字节 + body）。ok=false 表示批次中途遇到未知布局，
// 调用方应把批次按原样整体透传（走 BatchResponseOffset 的 !ok 回退分支）
// 或丢弃。供驱动方（如故事决斗的 AI 代答器）在转发前逐条检查消息。
func SplitBatchMessages(msg []byte) (msgs [][]byte, ok bool) {
	var out [][]byte
	pos := 0
	for pos < len(msg) {
		end, _, _, ok := WalkEngineMessage(msg, pos+1)
		if !ok {
			return nil, false
		}
		out = append(out, msg[pos:end])
		pos = end
	}
	return out, true
}
