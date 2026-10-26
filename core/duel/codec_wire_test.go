package duel

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"

	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/core/replay"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 本文件锁定 core/duel 线结构体的字节布局与 codec tag 语义：
//   - 黄金字节：Marshal 结果与测试内手工构造的期望 []byte 逐字节比较
//     （期望值全部用 binary.LittleEndian 独立构造，不复用生产代码路径）；
//   - 往返：Marshal→Unmarshal→DeepEqual；
//   - 健壮性：每个前缀长度截断都必须返回 *codec.Error 且不 panic。
//
// C++ 语义对照：
//   - STOC/CTOS 帧长度前缀：Fluorohydride/ygopro/gframe/netserver.h 的
//     SendPacketToPlayer/SendBufferToPlayer 写 (uint16)(1 + len) + proto，
//     即线长含 proto 字节本身、不含 2 字节长度前缀自身；
//   - STOC_ErrorMsg：{ uint8 msg; uint8 padding[3]; uint32 code }；
//   - ReplayHeader/ExtendedReplayHeader/DuelParameters 字段序见
//     Fluorohydride/ygopro/gframe/replay.h，其线宽测试随结构体一起移到
//     core/replay/replay_wire_test.go。

// ---------------------------------------------------------------------------
// stocFrameHeader（STOC 3 字节定长头）
// ---------------------------------------------------------------------------

func TestWireStocFrameHeaderGolden(t *testing.T) {
	fh := codec.FrameHeader{Len: 0x1234, Proto: 0xAB}

	got, err := codec.Marshal(&fh)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := make([]byte, 3)
	binary.LittleEndian.PutUint16(want[0:2], 0x1234)
	want[2] = 0xAB

	if !bytes.Equal(got, want) {
		t.Fatalf("黄金字节不符:\n got %v\nwant %v", got, want)
	}
}

func TestWireStocFrameHeaderRoundTrip(t *testing.T) {
	// Len=1 是原版 SendPacketToPlayer 的"仅 proto 无负载"帧头取值
	fh := codec.FrameHeader{Len: 1, Proto: 0x02}
	data, err := codec.Marshal(&fh)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back codec.FrameHeader
	if err := codec.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(fh, back) {
		t.Fatalf("往返不等: %+v vs %+v", fh, back)
	}
}

// ---------------------------------------------------------------------------
// stocFrame（Len + Proto + greedy Payload）
// ---------------------------------------------------------------------------

func TestFrameStocFrameGolden(t *testing.T) {
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	// Len 语义锁定：线长 = 1(proto) + len(Payload)，与 netserver.h
	// Write<uint16_t>(p, (uint16_t)(1 + len)) 一致
	f := codec.Frame{Len: uint16(1 + len(payload)), Proto: 0x21, Payload: payload}

	got, err := codec.Marshal(&f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := make([]byte, 0, 3+len(payload))
	hdr := make([]byte, 3)
	binary.LittleEndian.PutUint16(hdr[0:2], uint16(1+len(payload)))
	hdr[2] = 0x21
	want = append(want, hdr...)
	want = append(want, payload...)

	if !bytes.Equal(got, want) {
		t.Fatalf("黄金字节不符:\n got %v\nwant %v", got, want)
	}
}

func TestFrameStocFrameEmptyPayload(t *testing.T) {
	// 空负载：Len=1，帧只剩 3 字节头（对应原版仅 proto 的通知包）
	f := codec.Frame{Len: 1, Proto: 0x05, Payload: nil}
	got, err := codec.Marshal(&f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := []byte{0x01, 0x00, 0x05}
	if !bytes.Equal(got, want) {
		t.Fatalf("空负载黄金字节不符: got %v want %v", got, want)
	}

	var back codec.Frame
	if err := codec.Unmarshal(got, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.Len != 1 || back.Proto != 0x05 || len(back.Payload) != 0 {
		t.Fatalf("空负载往返不符: %+v", back)
	}
}

func TestFrameStocFrameRoundTrip(t *testing.T) {
	f := codec.Frame{Len: 6, Proto: 0x9C, Payload: []byte{1, 2, 3, 4, 5}}
	data, err := codec.Marshal(&f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back codec.Frame
	if err := codec.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(f, back) {
		t.Fatalf("往返不等: %+v vs %+v", f, back)
	}
}

func TestFrameStocFrameGreedyEatsAll(t *testing.T) {
	// greedy tag 语义：Unmarshal 把剩余字节全部吃进 Payload，
	// 帧之后不能再跟其它消息；Len 字段只是数据不参与校验
	data := []byte{0x04, 0x00, 0x21, 0xAA, 0xBB, 0xCC}
	var f codec.Frame
	if err := codec.Unmarshal(data, &f); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !bytes.Equal(f.Payload, []byte{0xAA, 0xBB, 0xCC}) {
		t.Fatalf("greedy 未吃光剩余字节: %v", f.Payload)
	}
}

func TestFrameStocFrameLenNotAutofilled(t *testing.T) {
	// 这些线结构体的 Len 是普通 uint16 字段（不是 WireLen 标记），
	// codec 不做自动回填：调用方写什么就发什么。语义由 PacketContext.Reply
	// 负责维护（Len = 1+len(Payload)，>0xFFFE 拒绝），这里锁定"原样输出"。
	f := codec.Frame{Len: 999, Proto: 0x01, Payload: []byte{0x11}}
	got, err := codec.Marshal(&f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if binary.LittleEndian.Uint16(got[0:2]) != 999 {
		t.Fatalf("Len 应原样输出 999，得到 %d", binary.LittleEndian.Uint16(got[0:2]))
	}
}

// ---------------------------------------------------------------------------
// stocErrorMsg（STOC_ErrorMsg）
// ---------------------------------------------------------------------------

func TestFrameStocErrorMsgGolden(t *testing.T) {
	m := codec.ErrorBody{Msg: 0x02, Code: 0x0ABCDEF1}

	got, err := codec.Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// C++ STOC_ErrorMsg: { uint8 msg; uint8 padding[3]; uint32 code }
	want := []byte{0x02, 0x00, 0x00, 0x00, 0xF1, 0xDE, 0xBC, 0x0A}
	if !bytes.Equal(got, want) {
		t.Fatalf("黄金字节不符:\n got %v\nwant %v", got, want)
	}
}

func TestFrameStocErrorMsgPadWritesZero(t *testing.T) {
	// Pad 是导出的定长零值数组（线上填充字节）：marshal 原样写字节值，
	// 零值 Pad 即对应 C++ 结构体的 zero-initialized padding。
	m := codec.ErrorBody{Msg: 0x01, Code: 7}
	got, err := codec.Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Equal(got[1:4], []byte{0, 0, 0}) {
		t.Fatalf("Pad 区域应为零，得到 %v", got[1:4])
	}
}

func TestFrameStocErrorMsgRoundTrip(t *testing.T) {
	m := codec.ErrorBody{Msg: 0x03, Code: 0xFFFFFFFF}
	data, err := codec.Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back codec.ErrorBody
	if err := codec.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.Msg != m.Msg || back.Code != m.Code {
		t.Fatalf("往返不等: %+v vs %+v", back, m)
	}
}

// ---------------------------------------------------------------------------
// 健壮性：所有线结构体在每个前缀长度截断下都必须返回 *codec.Error 且不 panic
// ---------------------------------------------------------------------------

func TestWireTruncation(t *testing.T) {
	payload := []byte{0x01, 02, 03}
	stocFrameGolden, _ := codec.Marshal(&codec.Frame{Len: 4, Proto: 0x21, Payload: payload})
	stocErrGolden, _ := codec.Marshal(&codec.ErrorBody{Msg: 1, Code: 0xAABBCCDD})
	ctosGolden, _ := codec.Marshal(&codec.FrameLen{Len: 0x0102})
	stocHdrGolden, _ := codec.Marshal(&codec.FrameHeader{Len: 0x0102, Proto: 3})

	cases := []struct {
		name string
		data []byte
		// minErrLen：截断长度 < minErrLen 时必须报错；codec.Frame 的 greedy
		// 负载允许为空，帧头完整的 3 字节前缀即可成功解码
		minErrLen int
		newV      func() any
	}{
		{"codec.FrameHeader", stocHdrGolden, 3, func() any { return &codec.FrameHeader{} }},
		{"codec.Frame", stocFrameGolden, 3, func() any { return &codec.Frame{} }},
		{"codec.ErrorBody", stocErrGolden, 8, func() any { return &codec.ErrorBody{} }},
		{"codec.FrameLen", ctosGolden, 2, func() any { return &codec.FrameLen{} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 逐前缀长度截断：minErrLen 以下必须报错
			for n := 0; n < tc.minErrLen; n++ {
				truncated := tc.data[:n]
				var codecErr *codec.Error
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("截断到 %d 字节时 panic: %v", n, r)
						}
					}()
					err := codec.Unmarshal(truncated, tc.newV())
					if err == nil {
						t.Fatalf("截断到 %d 字节应返回 *codec.Error，得到 nil", n)
					}
					if !errors.As(err, &codecErr) {
						t.Fatalf("截断到 %d 字节应返回 *codec.Error，得到 %T: %v", n, err, err)
					}
				}()
			}
			// 完整输入必须成功（基线，防止 golden 本身不可用）
			if err := codec.Unmarshal(tc.data, tc.newV()); err != nil {
				t.Fatalf("完整输入应可解码: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// encodeFrame：Frozen 快路径与 MarshalInto 结构体路径
// ---------------------------------------------------------------------------

// TestEncodeFrameFrozenFastPath：Frozen 负载直接 copy 线字节，零序列化。
func TestEncodeFrameFrozenFastPath(t *testing.T) {
	d := &DuelMode{}
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	frozen, err := codec.Freeze(&codec.Frame{Len: 5, Proto: 0x77, Payload: payload})
	if err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	frame := d.encodeFrame(0x21, frozen)
	if frame == nil {
		t.Fatal("encodeFrame 应成功")
	}
	// 帧 = 3 字节头（Len=1+线宽, proto）+ Frozen 线字节
	wantLen := uint16(1 + frozen.Len())
	if binary.LittleEndian.Uint16(frame[0:2]) != wantLen || frame[2] != 0x21 {
		t.Fatalf("帧头不符: len=%d proto=%#x", binary.LittleEndian.Uint16(frame[0:2]), frame[2])
	}
	if !bytes.Equal(frame[3:], frozen.Bytes()) {
		t.Fatalf("Frozen 负载不符:\n got %v\nwant %v", frame[3:], frozen.Bytes())
	}
}

// TestEncodeFrameStructPayload：protocol 结构体走 codec.MarshalInto，
// 编码结果与旧 binary.Encode 布局一致（padding 恒零）。
func TestEncodeFrameStructPayload(t *testing.T) {
	d := &DuelMode{}
	frame := d.encodeFrame(network.STOC_ERROR_MSG, &protocol.STOCErrorMsg{
		Msg:  network.ERRMSG_JOINERROR,
		Code: 1,
	})
	if frame == nil {
		t.Fatal("encodeFrame 应成功")
	}
	// { uint8 msg; uint8 padding[3]（零）; uint32 code } + 帧头
	want := []byte{0x09, 0x00, 0x02, // Len=9 proto=STOC_ERROR_MSG(0x02)
		network.ERRMSG_JOINERROR, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00}
	if !bytes.Equal(frame, want) {
		t.Fatalf("结构体负载帧不符:\n got %v\nwant %v", frame, want)
	}
	// nil 负载：Len=1 仅 proto
	frame = d.encodeFrame(0x02, nil)
	if !bytes.Equal(frame, []byte{0x01, 0x00, 0x02}) {
		t.Fatalf("nil 负载帧不符: %v", frame)
	}
}

// ---------------------------------------------------------------------------
// packEngineMsg：引擎消息组包（codec 计划链）黄金字节
// ---------------------------------------------------------------------------

// TestPackEngineMsgGolden：packEngineMsg 输出与手工构造的线字节逐字节比对
//（期望值用 binary.LittleEndian 独立构造，不复用 codec/restruct 路径）。
func TestPackEngineMsgGolden(t *testing.T) {
	// SelectCardMsg：player/cancelable/min/max/count + 2×条目(8B)
	scm := protocol.SelectCardMsg{
		Player: 1, Cancelable: 1, Min: 1, Max: 2,
		Cards: []protocol.SelectCardEntry{
			{Code: 89631139, Controller: 0, Location: 4, Sequence: 2, Position: 1},
			{Code: -1, Controller: 1, Location: 0x10, Sequence: 0, Position: 0x8},
		},
	}
	got := packEngineMsg(0x20, &scm)
	want := []byte{0x20, 1, 1, 1, 2, 2} // 类型 + 头 5 字节 + count=2
	for _, e := range scm.Cards {
		var b [8]byte
		binary.LittleEndian.PutUint32(b[0:4], uint32(e.Code))
		b[4], b[5], b[6], b[7] = e.Controller, e.Location, e.Sequence, e.Position
		want = append(want, b[:]...)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("SelectCardMsg 黄金字节不符:\n got %v\nwant %v", got, want)
	}

	// ShuffleHandMsg：player/count + codes(int32 LE)
	shm := protocol.ShuffleHandMsg{
		Player: 0, Cards: []int32{46986414, 0, -5},
	}
	got = packEngineMsg(0x16, &shm)
	want = []byte{0x16, 0, 3,
		0xAE, 0xF4, 0xCC, 0x02, // 46986414 = 0x02CCF4AE
		0, 0, 0, 0,
		0xFB, 0xFF, 0xFF, 0xFF, // -5
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ShuffleHandMsg 黄金字节不符:\n got %v\nwant %v", got, want)
	}

	// DrawMsg：player/count + cards
		faceUp := uint32(0x80000000) // 引擎对表侧抽卡的公开标记位
	dm := protocol.DrawMsg{Player: 1, Cards: []int32{int32(faceUp | 70781052)}}
	got = packEngineMsg(0x52, &dm)
	want = []byte{0x52, 1, 1, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(want[3:7], uint32(dm.Cards[0]))
	if !bytes.Equal(got, want) {
		t.Fatalf("DrawMsg 黄金字节不符:\n got %v\nwant %v", got, want)
	}
}

// TestDuelParametersRecordGolden：录制参数块与 4×int32 LE 逐字节一致
//（读端 replay.ReadInfo 按 DuelParameters 解析，写读同源）。
func TestDuelParametersRecordGolden(t *testing.T) {
	p := replay.DuelParameters{StartLP: 8000, StartHand: 5, DrawCount: 1, DuelFlag: 0x40000}
	got := codec.MustFreeze(&p).Bytes()
	want := make([]byte, 16)
	binary.LittleEndian.PutUint32(want[0:4], 8000)
	binary.LittleEndian.PutUint32(want[4:8], 5)
	binary.LittleEndian.PutUint32(want[8:12], 1)
	binary.LittleEndian.PutUint32(want[12:16], 0x40000)
	if !bytes.Equal(got, want) {
		t.Fatalf("DuelParameters 录制块不符:\n got %v\nwant %v", got, want)
	}
	// 往返：ReadInfo 的解析路径
	var back replay.DuelParameters
	if err := codec.Unmarshal(got, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back != p {
		t.Fatalf("往返不等: %+v vs %+v", back, p)
	}
}

// BenchmarkPackEngineMsg：codec 计划链 vs 旧 restruct 反射打包。
func BenchmarkPackEngineMsg(b *testing.B) {
	scm := protocol.SelectCardMsg{
		Player: 1, Cancelable: 1, Min: 1, Max: 2,
		Cards: []protocol.SelectCardEntry{
			{Code: 89631139, Controller: 0, Location: 4, Sequence: 2, Position: 1},
			{Code: 24094653, Controller: 1, Location: 0x10, Sequence: 0, Position: 0x8},
		},
	}
	b.Run("codec", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = packEngineMsg(0x20, &scm)
		}
	})
	b.Run("restruct", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = append([]byte{0x20}, scm.Pack()...)
		}
	})
}
