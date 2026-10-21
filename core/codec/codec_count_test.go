package codec

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Count 标记类型测试（Count8/16/32）
// ---------------------------------------------------------------------------

type cntEntry struct {
	Code int32
	C    uint8
	L    uint8
	S    uint8
}

// cntScalarMsg：Count8 + []uint32（定宽标量元素）。
type cntScalarMsg struct {
	Player uint8
	Count  Count8
	Nums   []uint32
	Tail   uint16
}

// cntStructMsg：Count16 + 定长结构体元素。
type cntStructMsg struct {
	Kind  uint8
	Count Count16
	Cards []cntEntry
}

// cntWideMsg：Count32 + 大列表。
type cntWideMsg struct {
	Count Count32
	Nums  []uint64
}

// cntGapMsg：Count 与 slice 之间隔了非 slice 字段（SELECT_CHAIN 式布局）。
type cntGapMsg struct {
	Player   uint8
	Count    Count8
	SpeCount uint8
	Hint0    uint32
	Hint1    uint32
	Chains   []cntEntry
}

// cntMixedMsg：同一结构体里 WireLen blob 与 Count 列表混用。
type cntMixedMsg struct {
	Kind  uint8
	WLen  WireLen16
	Blob  []byte
	Count Count8
	Nums  []uint16
}

// TestCountMarkerRoundTrip：Count8/16/32 标量/结构体元素往返 + 自动回填。
func TestCountMarkerRoundTrip(t *testing.T) {
	t.Run("Count8Scalar", func(t *testing.T) {
		want := cntScalarMsg{Player: 7, Nums: []uint32{0xdeadbeef, 0x12345678, 3}, Tail: 0xabcd}
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if want.Count != 3 {
			t.Fatalf("Count 未回填: %d，期望 3", want.Count)
		}
		if data[1] != 3 {
			t.Fatalf("线上计数前缀错误: %v", data[:2])
		}
		var back cntScalarMsg
		if err := Unmarshal(data, &back); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if back.Count != 3 || len(back.Nums) != 3 || back.Nums[0] != 0xdeadbeef ||
			back.Player != 7 || back.Tail != 0xabcd {
			t.Fatalf("往返不等: %+v", back)
		}
	})

	t.Run("Count16StructElems", func(t *testing.T) {
		want := cntStructMsg{Kind: 1, Cards: []cntEntry{
			{Code: 111, C: 0, L: 4, S: 1},
			{Code: 222, C: 1, L: 4, S: 2},
		}}
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if want.Count != 2 || data[1] != 2 || data[2] != 0 {
			t.Fatalf("回填/前缀错误: Count=%d prefix=%v", want.Count, data[1:3])
		}
		var back cntStructMsg
		if err := Unmarshal(data, &back); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if back.Count != 2 || len(back.Cards) != 2 || back.Cards[1].Code != 222 || back.Cards[1].S != 2 {
			t.Fatalf("往返不等: %+v", back)
		}
	})

	t.Run("Count32Wide", func(t *testing.T) {
		want := cntWideMsg{Nums: make([]uint64, 300)}
		for i := range want.Nums {
			want.Nums[i] = uint64(i) * 7
		}
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if want.Count != 300 {
			t.Fatalf("Count 未回填: %d", want.Count)
		}
		var back cntWideMsg
		if err := Unmarshal(data, &back); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if back.Count != 300 || len(back.Nums) != 300 || back.Nums[299] != 299*7 {
			t.Fatalf("往返不等: count=%d len=%d", back.Count, len(back.Nums))
		}
	})

	t.Run("EmptyList", func(t *testing.T) {
		want := cntScalarMsg{Player: 1, Nums: nil, Tail: 9}
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if want.Count != 0 || len(data) != 1+1+0+2 {
			t.Fatalf("空列表: Count=%d len=%d", want.Count, len(data))
		}
		var back cntScalarMsg
		if err := Unmarshal(data, &back); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if back.Count != 0 || len(back.Nums) != 0 || back.Tail != 9 {
			t.Fatalf("往返不等: %+v", back)
		}
	})
}

// TestCountMarkerManualValueIgnored：调用方手动填了不一致的 Count，以实际
// len(slice) 为准（与 WireLen 同语义）。
func TestCountMarkerManualValueIgnored(t *testing.T) {
	m := cntScalarMsg{Player: 1, Count: 99, Nums: []uint32{5}, Tail: 2}
	data, err := Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if m.Count != 1 || data[1] != 1 {
		t.Fatalf("手动值未被实际值覆盖: Count=%d 线上=%d", m.Count, data[1])
	}
}

// TestCountMarkerOverflow：元素个数超出 Count 宽度（300 个条目装进 Count8）。
func TestCountMarkerOverflow(t *testing.T) {
	m := cntScalarMsg{Nums: make([]uint32, 300)}
	_, err := Marshal(&m)
	if err == nil || !strings.Contains(err.Error(), "表示范围") {
		t.Fatalf("期望溢出报错，得到: %v", err)
	}
}

// TestCountMarkerGapFields：Count 与目标 slice 之间隔了非 slice 字段
// （MSG_SELECT_CHAIN 式：count, spe_count, hint0, hint1, 条目）。
func TestCountMarkerGapFields(t *testing.T) {
	want := cntGapMsg{Player: 0, SpeCount: 1, Hint0: 0x10, Hint1: 0x20,
		Chains: []cntEntry{{Code: 99, C: 0, L: 4, S: 2}}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want.Count != 1 {
		t.Fatalf("Count 未回填: %d", want.Count)
	}
	// 线序：player(1) count(1) spe(1) hint0(4) hint1(4) 条目(7)
	wantWire := []byte{0, 1, 1, 0x10, 0, 0, 0, 0x20, 0, 0, 0, 99, 0, 0, 0, 0, 4, 2}
	if !bytes.Equal(data, wantWire) {
		t.Fatalf("wire:\n got = %v\nwant = %v", data, wantWire)
	}
	var back cntGapMsg
	if err := Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.Count != 1 || back.SpeCount != 1 || back.Hint0 != 0x10 || back.Hint1 != 0x20 ||
		len(back.Chains) != 1 || back.Chains[0].Code != 99 || back.Chains[0].S != 2 {
		t.Fatalf("往返不等: %+v", back)
	}
}

// TestCountMarkerMixedWithWireLen：同一结构体内 WireLen blob 与 Count 列表混用。
func TestCountMarkerMixedWithWireLen(t *testing.T) {
	want := cntMixedMsg{Kind: 4, Blob: []byte{0xaa, 0xbb}, Nums: []uint16{11, 22, 33}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want.WLen != 2 || want.Count != 3 {
		t.Fatalf("回填错误: WLen=%d Count=%d", want.WLen, want.Count)
	}
	// 线序：kind(1) wlen(2LE=2,0) blob(2) count(1) nums(3*2)
	wantWire := []byte{4, 2, 0, 0xaa, 0xbb, 3, 11, 0, 22, 0, 33, 0}
	if !bytes.Equal(data, wantWire) {
		t.Fatalf("wire:\n got = %v\nwant = %v", data, wantWire)
	}
	var back cntMixedMsg
	if err := Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.WLen != 2 || back.Count != 3 || len(back.Blob) != 2 || len(back.Nums) != 3 ||
		back.Nums[2] != 33 {
		t.Fatalf("往返不等: %+v", back)
	}
}

// TestCountMarkerDecoderStream：Decoder 连解多条含 Count 的消息。
func TestCountMarkerDecoderStream(t *testing.T) {
	m1 := cntScalarMsg{Player: 1, Nums: []uint32{10}, Tail: 1}
	m2 := cntScalarMsg{Player: 2, Nums: []uint32{20, 30}, Tail: 2}
	b1, err := Marshal(&m1)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	b2, err := Marshal(&m2)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	dec := NewDecoder(append(b1, b2...))
	var r1, r2 cntScalarMsg
	if err := dec.Decode(&r1); err != nil {
		t.Fatalf("Decode#1: %v", err)
	}
	if err := dec.Decode(&r2); err != nil {
		t.Fatalf("Decode#2: %v", err)
	}
	if dec.Remaining() != 0 {
		t.Fatalf("剩余 %d 字节未消费", dec.Remaining())
	}
	if r1.Count != 1 || r1.Nums[0] != 10 || r2.Count != 2 || r2.Nums[1] != 30 || r2.Tail != 2 {
		t.Fatalf("流式解码不等: %+v %+v", r1, r2)
	}
}

// 以下类型原本定义在 TestCountMarkerInvalid 内；codecgen 只扫描包级声明。
type (
	cntNoTarget    struct {
		Count Count8
		Tail  uint32
	}
	cntGreedyAfter struct {
		Count Count8
		Nums  []uint32
		Rest  []byte `greedy:""`
	}
	cntCondOnCount struct {
		Flag  uint8
		Count Count8 `if:"Flag&0x1"`
		Nums  []uint32
	}
	cntCountAsElem struct {
		Count Count8
		Nums  []uint32
	}
	cntArrWithCount struct {
		Items [2]cntCountAsElem
	}
	cntSliceNoBinding struct {
		Nums []uint32
	}
)

// TestCountMarkerInvalid：布局校验——Count 后无 slice、目标后随 greedy、
// Count 配 if、含 Count 的结构体作数组元素、目标 slice 未绑定。
func TestCountMarkerInvalid(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sample any
	}{
		{"noTarget", &cntNoTarget{}},
		{"greedyAfter", &cntGreedyAfter{}},
		{"condOnCount", &cntCondOnCount{}},
		{"arrWithCount", &cntArrWithCount{}},
		{"sliceNoBinding", &cntSliceNoBinding{}},
	} {
		if err := Register(tc.sample); err == nil {
			t.Errorf("%s: 期望布局校验报错", tc.name)
		}
	}
}
