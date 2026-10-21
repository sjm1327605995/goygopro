package duel

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// TestMsgStartBodyGolden 锁定 MSG_START 负载的 19 字节布局（类型/视角/
// 规则/双 LP/双方卡组与额外数），与原版 single_duel.cpp StartDuel 手写
// 缓冲的写入顺序逐字节一致。
func TestMsgStartBodyGolden(t *testing.T) {
	body := msgStartBody{
		Type:     ocgcore.MSG_START,
		Player:   0x11,
		DuelRule: 5,
		LP:       [2]int32{8000, 8000},
		Side:     [2]msgStartSide{{Deck: 40, Extra: 15}, {Deck: 35, Extra: 10}},
	}
	got, err := codec.Marshal(&body)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := make([]byte, 19)
	want[0] = ocgcore.MSG_START
	want[1] = 0x11
	want[2] = 5
	binary.LittleEndian.PutUint32(want[3:], 8000)
	binary.LittleEndian.PutUint32(want[7:], 8000)
	binary.LittleEndian.PutUint16(want[11:], 40)
	binary.LittleEndian.PutUint16(want[13:], 15)
	binary.LittleEndian.PutUint16(want[15:], 35)
	binary.LittleEndian.PutUint16(want[17:], 10)

	if !bytes.Equal(got, want) {
		t.Fatalf("布局不符:\n got  = %v\n want = %v", got, want)
	}
	if len(got) != 19 {
		t.Fatalf("MSG_START 负载应为 19 字节，got %d", len(got))
	}

	// 往返 + 默认值（视角 0，beginDuel 首次编码的形态）。
	var back msgStartBody
	if err := codec.Unmarshal(got, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back != body {
		t.Fatalf("往返不等: %+v vs %+v", back, body)
	}
}
