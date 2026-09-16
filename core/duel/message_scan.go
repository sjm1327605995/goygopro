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
//
// 消息布局的唯一事实源是 engineMsgLayouts（engine_msg_layout.go）。
func batchResponseOffset(msg []byte) (offset int, ok bool) {
	p := 0
	for p < len(msg) {
		body := p + 1
		end, response, win, ok := walkEngineMessage(msg, body)
		if !ok {
			return 0, false
		}
		if win {
			return batchWinOffset, true
		}
		if response {
			return p, true
		}
		p = end
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
// or a fragment too small to be a query appears. MZONE/SZONE 空槽写 LEN_EMPTY(4)
// 标记（ocgapi.cpp query_field_card）：跳过 4 字节继续，保持后面的 blob 可达。
func walkQueryBlobs(p *int, msg []byte) bool {
	for *p < len(msg) {
		if len(msg)-*p < 4 {
			return false
		}
		clen := int(uint32(msg[*p]) | uint32(msg[*p+1])<<8 | uint32(msg[*p+2])<<16 | uint32(msg[*p+3])<<24)
		if clen == ocgcore.LEN_EMPTY {
			// 空槽标记：不是 blob，占 4 字节
			*p += 4
			continue
		}
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
