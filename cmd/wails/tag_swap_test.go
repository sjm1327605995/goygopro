package main

import (
	"encoding/binary"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

// TestMsgTagSwapEmitted：MSG_TAG_SWAP（TAG 队友换手）应按
// duelclient.cpp:3782-3788 布局解析并 emit duel:tag_swap：player + 四个计数 +
// topcode + hcount 手牌码 + ecount 额外码（额外码去 0x80000000 公开位）。
// 前端 reducer 据此刷新牌堆计数与本队手牌，duel_manager 同步对方手牌张数。
func TestMsgTagSwapEmitted(t *testing.T) {
	var emitted map[string]interface{}
	c := NewWailsDuelClient(func(eventName string, optionalData ...interface{}) {
		if eventName == "duel:tag_swap" {
			emitted = optionalData[0].(map[string]interface{})
		}
	})

	// 消息体：player=1, mcount=34, ecount=2, pcount=1, hcount=4,
	// topcode=89631139, hand=[46986414, 0, 0, 38033121], extra=[84013237, 0x80000000|86066372]
	body := []byte{
		ocgcore.MSG_TAG_SWAP,
		1, 34, 2, 1, 4,
	}
	body = binary.LittleEndian.AppendUint32(body, 89631139)
	for _, code := range []uint32{46986414, 0, 0, 38033121} {
		body = binary.LittleEndian.AppendUint32(body, code)
	}
	for _, code := range []uint32{84013237, 0x80000000 | 86066372} {
		body = binary.LittleEndian.AppendUint32(body, code)
	}

	if err := c.handleGameMessage(body); err != nil {
		t.Fatalf("handleGameMessage: %v", err)
	}
	if emitted == nil {
		t.Fatalf("MSG_TAG_SWAP 应 emit duel:tag_swap")
	}
	if emitted["player"] != uint8(1) || emitted["deckCount"] != uint8(34) ||
		emitted["extraCount"] != uint8(2) || emitted["extraFaceUpCount"] != uint8(1) ||
		emitted["handCount"] != uint8(4) {
		t.Fatalf("计数字段解析错: %+v", emitted)
	}
	if emitted["topCode"] != int32(89631139) {
		t.Fatalf("topCode = %v, want 89631139", emitted["topCode"])
	}
	hand := emitted["hand"].([]int32)
	if len(hand) != 4 || hand[0] != 46986414 || hand[1] != 0 || hand[3] != 38033121 {
		t.Fatalf("hand = %v", hand)
	}
	extra := emitted["extra"].([]int32)
	if len(extra) != 2 || extra[0] != 84013237 || extra[1] != 86066372 {
		t.Fatalf("extra = %v（0x80000000 公开位应剥离）", extra)
	}
}
