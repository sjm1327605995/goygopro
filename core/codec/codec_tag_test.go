package codec

// bit:"N" / codec.Rest 的专项测试：谓词绑定（声明序向前最近的前导整型
// 字段）、位号越界、缺前导字段、非指针、与 if 冲突；Rest 的 greedy 语义
// 与末尾约束；ErrorBody 去掉 skip tag 后的黄金字节不变；greedy tag 向后
// 兼容。

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// bit:"N"
// ---------------------------------------------------------------------------

type bitWireMsg struct {
	Flag  uint32
	Code  *uint32   `bit:"0"` // QUERY_CODE 位
	Pos   *uint32   `bit:"1"`
	Atk   *int32    `bit:"8"`
	ListN *uint32   `bit:"15"`
	List  *[]uint16 `bit:"15" count:"ListN"`
}

// TestBitTagRoundTrip：bit 谓词全组合往返（含 bit+count 条件列表），
// 语义与 if:"Flag&1<<N" 完全一致。
func TestBitTagRoundTrip(t *testing.T) {
	for flags := uint32(0); flags < 4; flags++ {
		want := bitWireMsg{Flag: flags}
		if flags&0x01 != 0 {
			v := uint32(0xc0ffee)
			want.Code = &v
		}
		if flags&0x02 != 0 {
			v := uint32(0x11)
			want.Pos = &v
		}
		if flags&0x100 != 0 {
			v := int32(-3000)
			want.Atk = &v
		}
		if flags&0x8000 != 0 {
			n := uint32(2)
			want.ListN = &n
			want.List = &[]uint16{11, 22}
		}
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("flags=%#x Marshal: %v", flags, err)
		}
		var got bitWireMsg
		if err := Unmarshal(data, &got); err != nil {
			t.Fatalf("flags=%#x Unmarshal: %v", flags, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("flags=%#x 往返不一致:\nwant %+v\ngot  %+v", flags, want, got)
		}
		if (got.Code != nil) != (flags&0x01 != 0) || (got.List != nil) != (flags&0x8000 != 0) {
			t.Fatalf("flags=%#x 谓词 nil 状态错误: %+v", flags, got)
		}
	}
}

// bitNearestMsg：A 与 Flag 都是前导整型字段，谓词应绑定更近的 Flag。
type bitNearestMsg struct {
	A    uint8
	Flag uint32
	Code *uint32 `bit:"0"`
}

// TestBitTagBindsNearestPrecedingInt：A 的 bit0 置位、Flag 的 bit0 清零时
// Code 必须为 nil（谓词是 Flag 而非 A）。
func TestBitTagBindsNearestPrecedingInt(t *testing.T) {
	data := []byte{1, 0, 0, 0, 0} // A=1, Flag=0
	var m bitNearestMsg
	if err := Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Code != nil {
		t.Fatalf("谓词应绑定 Flag（bit0=0），Code 却非 nil: %+v", m)
	}
}

type (
	bitNoPredMsg struct {
		Code *uint32 `bit:"0"` // 声明序之前没有整型字段
	}
	bitTooBigMsg struct {
		Flag uint8
		Code *uint32 `bit:"8"` // 8 >= 1 字节 × 8
	}
	bitNonPtrMsg struct {
		Flag uint32
		Code uint32 `bit:"0"`
	}
	bitIfConflictMsg struct {
		Flag uint32
		Code *uint32 `bit:"0" if:"Flag&0x1"`
	}
)

// TestBitTagValidation：缺前导整型字段 / 位号越界 / 非指针 / 与 if 同用。
func TestBitTagValidation(t *testing.T) {
	cases := []struct {
		name   string
		sample any
		want   string
	}{
		{"缺前导整型字段", &bitNoPredMsg{}, "谓词字段缺失"},
		{"位号越界", &bitTooBigMsg{}, "超出谓词字段"},
		{"非指针字段", &bitNonPtrMsg{}, "只能用于指针字段"},
		{"与 if 同用", &bitIfConflictMsg{}, "bit 与 if"},
	}
	for _, tc := range cases {
		if err := Register(tc.sample); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s：期望包含 %q 的错误，得到 %v", tc.name, tc.want, err)
		}
		if _, err := Marshal(tc.sample); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s（Marshal）：期望包含 %q 的错误，得到 %v", tc.name, tc.want, err)
		}
	}
}

// ---------------------------------------------------------------------------
// codec.Rest
// ---------------------------------------------------------------------------

type restMsg struct {
	Kind uint16
	Body Rest
}

type restMidMsg struct {
	Body Rest
	Tail uint8
}

// TestRestRoundTrip：Rest 吃光剩余字节，编码原样写回。
func TestRestRoundTrip(t *testing.T) {
	for _, payload := range [][]byte{{}, {0xab}, bytes.Repeat([]byte{7}, 300)} {
		want := restMsg{Kind: 0x1234, Body: payload}
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if !bytes.Equal(data[2:], payload) {
			t.Fatalf("Rest 线字节不符: %v", data[2:])
		}
		var got restMsg
		if err := Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if got.Kind != 0x1234 || !bytes.Equal(got.Body, payload) {
			t.Fatalf("往返不一致: %+v", got)
		}
		if len(payload) == 0 && got.Body == nil {
			t.Fatal("空 Rest 解码应保持非 nil（与 greedy 语义一致）")
		}
	}
}

// TestRestMustBeLast：Rest 只能出现在结构体末尾。
func TestRestMustBeLast(t *testing.T) {
	if err := Register(&restMidMsg{}); err == nil || !strings.Contains(err.Error(), "末尾") {
		t.Fatalf("Rest 不在末尾应报错，得到 %v", err)
	}
}

// greedyLegacyMsg 保留 greedy:"" tag 的向后兼容往返（Rest 是优先形式）。
type greedyLegacyMsg struct {
	A    uint16
	Tail []byte `greedy:""`
}

func TestGreedyTagBackwardCompat(t *testing.T) {
	want := greedyLegacyMsg{A: 1, Tail: []byte{9, 8, 7}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) != 2+3 {
		t.Fatalf("greedy tag 编码长度 = %d，期望 5", len(data))
	}
	var got greedyLegacyMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("greedy tag 往返不一致: %+v vs %+v", want, got)
	}
}

// ---------------------------------------------------------------------------
// ErrorBody 无 skip 的黄金字节
// ---------------------------------------------------------------------------

// TestErrorBodyGoldenWithoutSkip：Pad 改为导出的定长零值数组（去掉
// skip:"3"）后线格式不变：msg + 3 个零填充 + code。
func TestErrorBodyGoldenWithoutSkip(t *testing.T) {
	want := ErrorBody{Msg: 5, Code: 0xdeadbeef}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	wantWire := []byte{5, 0, 0, 0, 0xef, 0xbe, 0xad, 0xde}
	if !bytes.Equal(data, wantWire) {
		t.Fatalf("黄金字节不符:\n got %v\nwant %v", data, wantWire)
	}
	// 复用脏 Pad 的结构体：解码把线上字节读入数组（旧 skip 是跳过保留原值）
	dirty := ErrorBody{Pad: [3]byte{9, 9, 9}}
	if err := Unmarshal(data, &dirty); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dirty != want {
		t.Fatalf("往返不一致: %+v vs %+v", dirty, want)
	}
}
