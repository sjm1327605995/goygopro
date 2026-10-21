package codec

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 测试用的消息结构
// ---------------------------------------------------------------------------

type simpleMsg struct {
	A    uint8
	B    uint16
	C    uint32
	D    uint64
	E    int8
	F    int16
	G    int32
	H    int64
	Ok   bool
	Raw  [4]byte
	Flag int16
}

type innerMsg struct {
	X uint16
	Y int32
}

type nestedMsg struct {
	Head  uint8
	Inner innerMsg
	P     *innerMsg
	Tail  uint32
}

type listMsg struct {
	Kind  uint8
	N     uint8
	Items []uint32 `count:"N"`
}

type structListMsg struct {
	N     uint16
	Cards []innerMsg `count:"N"`
}

type condMsg struct {
	Flags uint32
	Code  *uint32    `if:"Flags&0x01"`
	Pos   *innerMsg  `if:"Flags&0x02"`
	ListN *uint32    `if:"Flags&0x04"` // 条件计数字段：*int 布局，长度随 flag 位出现
	List  *[]uint16 `if:"Flags&0x04" count:"ListN"`
	Spare *uint8    `if:"Flags&0x08"`
}

type greedyMsg struct {
	A    uint16
	Tail Rest
}

type skipMsg struct {
	A   uint32
	Pad [2]byte
	B   uint8
}

type truncMsg struct {
	A     uint32
	N     uint8
	Items []uint32 `count:"N"`
}

// negLenMsg 校验负的 count 字段值（手工构造字节）。
type negLenMsg struct {
	N     int32
	Items []uint8 `count:"N"`
}

// ---------------------------------------------------------------------------
// 基础往返
// ---------------------------------------------------------------------------

func TestRoundTripFixed(t *testing.T) {
	want := simpleMsg{
		A: 0x12, B: 0x3456, C: 0x789abcde, D: 0x0123456789abcdef,
		E: -5, F: -1234, G: 123456, H: -1234567890123,
		Ok: true, Raw: [4]byte{1, 2, 3, 4}, Flag: -2,
	}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) != 1+2+4+8+1+2+4+8+1+4+2 {
		t.Fatalf("编码长度 = %d，期望 37", len(data))
	}
	var got simpleMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("往返不一致:\nwant %+v\ngot  %+v", want, got)
	}
	// bool 真值编码为 1（前 8 个定宽字段占 30 字节，Ok 在 offset 30）
	if data[30] != 1 {
		t.Fatalf("bool 真值字节 = %d，期望 1", data[30])
	}
}

func TestBoolNonZeroIsTrue(t *testing.T) {
	var got simpleMsg
	data := make([]byte, 37)
	data[30] = 7 // bool 字段：非 0 即 true
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !got.Ok {
		t.Fatal("字节 7 应解码为 true")
	}
}

func TestRoundTripNested(t *testing.T) {
	inner := innerMsg{X: 0xbeef, Y: -99}
	want := nestedMsg{Head: 9, Inner: innerMsg{X: 1, Y: 2}, P: &inner, Tail: 0xffffffff}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got nestedMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("往返不一致:\nwant %+v\ngot  %+v", want, got)
	}
	if got.P == nil || *got.P != inner {
		t.Fatalf("嵌套指针字段丢失: %+v", got.P)
	}
}

// ---------------------------------------------------------------------------
// count 列表双向
// ---------------------------------------------------------------------------

func TestCountListRoundTrip(t *testing.T) {
	want := listMsg{Kind: 3, N: 0 /* 序列化自动回填 */, Items: []uint32{10, 20, 30, 40}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// 长度字段被回填为 len(Items)
	if want.N != 4 {
		t.Fatalf("count 未回填: N=%d，期望 4", want.N)
	}
	// 布局：Kind(1) + N(1) + 4*4
	if len(data) != 1+1+16 {
		t.Fatalf("编码长度 = %d，期望 18", len(data))
	}
	if data[1] != 4 {
		t.Fatalf("长度字节 = %d，期望 4", data[1])
	}
	var got listMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("往返不一致:\nwant %+v\ngot  %+v", want, got)
	}
}

func TestStructListRoundTrip(t *testing.T) {
	want := structListMsg{
		N:     0,
		Cards: []innerMsg{{X: 1, Y: -1}, {X: 2, Y: -2}, {X: 3, Y: -3}},
	}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want.N != 3 {
		t.Fatalf("count 未回填: N=%d", want.N)
	}
	var got structListMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("往返不一致:\nwant %+v\ngot  %+v", want, got)
	}
}

func TestEmptyList(t *testing.T) {
	want := listMsg{Kind: 1, Items: []uint32{}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) != 2 || data[1] != 0 {
		t.Fatalf("空列表编码错误: %v", data)
	}
	var got listMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("期望空列表，得到 %v", got.Items)
	}
}

func TestCountOverflow(t *testing.T) {
	m := listMsg{Items: make([]uint32, 300)} // 超出 uint8 表示范围
	if _, err := Marshal(&m); err == nil || !strings.Contains(err.Error(), "表示范围") {
		t.Fatalf("期望长度溢出错误，得到 %v", err)
	}
}

// ---------------------------------------------------------------------------
// if 条件字段：遍历 4 个 bit 的全组合
// ---------------------------------------------------------------------------

func TestCondAllBitCombinations(t *testing.T) {
	for flags := uint32(0); flags < 16; flags++ {
		want := condMsg{Flags: flags}
		if flags&0x01 != 0 {
			v := uint32(0xc0ffee)
			want.Code = &v
		}
		if flags&0x02 != 0 {
			want.Pos = &innerMsg{X: 7, Y: -7}
		}
		if flags&0x04 != 0 {
			ln := uint32(2)
			want.ListN = &ln
			want.List = &[]uint16{11, 22}
		}
		if flags&0x08 != 0 {
			v := uint8(0xab)
			want.Spare = &v
		}
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("flags=%#x Marshal: %v", flags, err)
		}
		var got condMsg
		if err := Unmarshal(data, &got); err != nil {
			t.Fatalf("flags=%#x Unmarshal: %v", flags, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("flags=%#x 往返不一致:\nwant %+v\ngot  %+v", flags, want, got)
		}
		// 显式断言 nil/非 nil 与谓词一致
		if (got.Code != nil) != (flags&0x01 != 0) {
			t.Fatalf("flags=%#x Code nil 状态错误", flags)
		}
		if (got.Pos != nil) != (flags&0x02 != 0) {
			t.Fatalf("flags=%#x Pos nil 状态错误", flags)
		}
		if (got.List != nil) != (flags&0x04 != 0) {
			t.Fatalf("flags=%#x List nil 状态错误", flags)
		}
		if (got.Spare != nil) != (flags&0x08 != 0) {
			t.Fatalf("flags=%#x Spare nil 状态错误", flags)
		}
	}
}

func TestCondNilSkipsBytes(t *testing.T) {
	// flags=0 时所有条件字段缺席，消息应只剩 Flags 自身
	m := condMsg{Flags: 0}
	data, err := Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) != 4 {
		t.Fatalf("flags=0 编码长度 = %d，期望 4（仅 Flags）", len(data))
	}
	var got condMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Code != nil || got.Pos != nil || got.ListN != nil || got.List != nil || got.Spare != nil {
		t.Fatalf("谓词不成立时指针应全部为 nil: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// greedy / skip
// ---------------------------------------------------------------------------

func TestGreedy(t *testing.T) {
	want := greedyMsg{A: 0x1234, Tail: []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) != 2+10 {
		t.Fatalf("编码长度 = %d，期望 12", len(data))
	}
	var got greedyMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("往返不一致:\nwant %+v\ngot  %+v", want, got)
	}
}

func TestSkip(t *testing.T) {
	want := skipMsg{A: 0xdeadbeef, B: 0x5a}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) != 4+2+1 {
		t.Fatalf("编码长度 = %d，期望 7", len(data))
	}
	if !bytes.Equal(data[4:6], []byte{0, 0}) {
		t.Fatalf("skip 区域应写零，得到 %v", data[4:6])
	}
	var got skipMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != want {
		t.Fatalf("往返不一致: want %+v got %+v", want, got)
	}
}

// ---------------------------------------------------------------------------
// Decoder 连续多条消息 + 游标语义
// ---------------------------------------------------------------------------

func TestDecoderMultipleMessages(t *testing.T) {
	// 注意：greedy 字段吃掉全部剩余字节，因此 greedyMsg 只能放在流末尾
	m1 := listMsg{Kind: 1, Items: []uint32{1, 2}}
	m2 := simpleMsg{A: 1, B: 2, C: 3}
	m3 := greedyMsg{A: 9, Tail: []byte{1, 2, 3}}
	enc := NewEncoder()
	for i, m := range []any{&m1, &m2, &m3} {
		if err := enc.Encode(m); err != nil {
			t.Fatalf("Encode #%d: %v", i, err)
		}
	}
	total := len(enc.Bytes())
	dec := NewDecoder(enc.Bytes())
	var g1 listMsg
	if err := dec.Decode(&g1); err != nil {
		t.Fatalf("Decode m1: %v", err)
	}
	if !reflect.DeepEqual(m1, g1) {
		t.Fatalf("m1 不一致: want %+v got %+v", m1, g1)
	}
	if dec.Pos() != 1+1+8 || dec.Remaining() != total-10 {
		t.Fatalf("游标 = %d，期望 10", dec.Pos())
	}
	var g2 simpleMsg
	if err := dec.Decode(&g2); err != nil {
		t.Fatalf("Decode m2: %v", err)
	}
	if !reflect.DeepEqual(m2, g2) {
		t.Fatalf("m2 不一致: want %+v got %+v", m2, g2)
	}
	if len(dec.Rest()) != 5 {
		t.Fatalf("Rest 长度 = %d，期望 5", len(dec.Rest()))
	}
	var g3 greedyMsg
	if err := dec.Decode(&g3); err != nil {
		t.Fatalf("Decode m3: %v", err)
	}
	if !reflect.DeepEqual(m3, g3) {
		t.Fatalf("m3 不一致: want %+v got %+v", m3, g3)
	}
	if dec.Remaining() != 0 {
		t.Fatalf("全部解码后剩余 = %d，期望 0", dec.Remaining())
	}
}

func TestDecoderErrorKeepsCursor(t *testing.T) {
	// 第一条完整，第二条截断：Decode 失败后游标不推进，可重试/跳过
	full := listMsg{Kind: 1, Items: []uint32{1, 2, 3, 4}}
	fullData, err := Marshal(&full)
	if err != nil {
		t.Fatal(err)
	}
	trunc := truncMsg{A: 1, N: 3, Items: []uint32{1, 2, 3}}
	truncData, err := Marshal(&trunc)
	if err != nil {
		t.Fatal(err)
	}
	stream := append(append([]byte{}, fullData...), truncData[:len(truncData)-7]...) // 截掉尾部
	dec := NewDecoder(stream)
	var g listMsg
	if err := dec.Decode(&g); err != nil {
		t.Fatalf("Decode full: %v", err)
	}
	posBefore := dec.Pos()
	var gt truncMsg
	err = dec.Decode(&gt)
	if err == nil {
		t.Fatal("期望截断错误")
	}
	if dec.Pos() != posBefore {
		t.Fatalf("失败后游标被推进: %d -> %d", posBefore, dec.Pos())
	}
	// 错误信息定位到字段与偏移（Decoder 上 Offset 为流内绝对偏移：
	// 第一条消息 18 字节 + 第二条消息内 Items 起始偏移 5）
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("错误类型 = %T，期望 *Error", err)
	}
	if e.Op != "unmarshal" || e.Struct != "truncMsg" || e.Field != "Items" || e.Offset != 23 || e.Need != 12 || e.Have != 5 {
		t.Fatalf("Error 字段错误: %+v", e)
	}
	want := "codec: unmarshal truncMsg.Items: offset 23 need 12 have 5"
	if err.Error() != want {
		t.Fatalf("错误信息 = %q，期望 %q", err.Error(), want)
	}
}

// ---------------------------------------------------------------------------
// 错误信息断言
// ---------------------------------------------------------------------------

func TestTruncationErrorMessage(t *testing.T) {
	m := truncMsg{A: 0x11223344, N: 3, Items: []uint32{1, 2, 3}}
	data, err := Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}
	cut := data[:5+5] // Items 从 offset 5 开始，需要 12 字节，只剩 5
	var got truncMsg
	err = Unmarshal(cut, &got)
	want := "codec: unmarshal truncMsg.Items: offset 5 need 12 have 5"
	if err == nil || err.Error() != want {
		t.Fatalf("错误信息 = %v，期望 %q", err, want)
	}
}

func TestTruncationInScalarField(t *testing.T) {
	data := []byte{1} // A(uint8) 读完后 B(uint16) 截断
	var got simpleMsg
	err := Unmarshal(data, &got)
	if err == nil || err.Error() != "codec: unmarshal simpleMsg.B: offset 1 need 2 have 0" {
		t.Fatalf("错误信息 = %v", err)
	}
}

func TestInvalidArgErrors(t *testing.T) {
	var m simpleMsg
	if err := Unmarshal(nil, m); err == nil || !strings.Contains(err.Error(), "结构体指针") {
		t.Fatalf("非指针参数应报错，得到 %v", err)
	}
	var p *simpleMsg
	if err := Unmarshal(nil, p); err == nil {
		t.Fatal("nil 指针应报错")
	}
	if _, err := Marshal(m); err == nil || !strings.Contains(err.Error(), "结构体指针") {
		t.Fatalf("Marshal 非指针应报错，得到 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 非法 tag / 未知类型
// ---------------------------------------------------------------------------

type badTypeMsg struct {
	S string
}

type badSliceMsg struct {
	Items []uint32 // 没有 count/greedy
}

type badCountRefMsg struct {
	Items []uint32 `count:"N"` // N 不存在
	N     uint8
}

type badGreedyPosMsg struct {
	Tail []byte `greedy:""`
	A    uint8
}

type badSkipMsg struct {
	A uint8 `skip:"abc"`
}

type badCondNonPtrMsg struct {
	Flags uint32
	Code  uint32 `if:"Flags&0x1"`
}

type badCondForwardMsg struct {
	Code  *uint32 `if:"Flags&0x1"`
	Flags uint32
}

func TestInvalidTypeAndTags(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"string 字段", &badTypeMsg{S: "x"}, "string 不支持"},
		{"slice 无 count", &badSliceMsg{}, "count 或 greedy"},
		{"count 目标不存在", &badCountRefMsg{}, "count 目标字段"},
		{"greedy 不在末尾", &badGreedyPosMsg{}, "末尾"},
		{"skip 非法", &badSkipMsg{}, "skip tag 非法"},
		{"if 用于非指针", &badCondNonPtrMsg{}, "指针字段"},
		{"if 谓词字段在后", &badCondForwardMsg{}, "之前"},
	}
	for _, c := range cases {
		if _, err := Marshal(c.v); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s：期望包含 %q 的错误，得到 %v", c.name, c.want, err)
		}
		if err := Unmarshal(nil, c.v); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s（Unmarshal）：期望包含 %q 的错误，得到 %v", c.name, c.want, err)
		}
	}
}

func TestNilUncondPtrErrors(t *testing.T) {
	m := nestedMsg{Head: 1, Inner: innerMsg{}, P: nil, Tail: 2}
	_, err := Marshal(&m)
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("无条件 nil 指针应报错，得到 %v", err)
	}
}

func TestNegativeLengthField(t *testing.T) {
	// N = -1（手工构造字节，Marshal 不会产出）
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, 0xffffffff)
	var m negLenMsg
	err := Unmarshal(data, &m)
	if err == nil || !strings.Contains(err.Error(), "非法") {
		t.Fatalf("负长度应报错，得到 %v", err)
	}
}

// ---------------------------------------------------------------------------
// 随机值 fuzz 往返
// ---------------------------------------------------------------------------

type fuzzInner struct {
	A uint16
	B int8
}

type fuzzMsg struct {
	Flags uint32
	Code  *uint32   `if:"Flags&0x01"`
	Pos   *fuzzInner `if:"Flags&0x02"`
	Count uint8
	Items []uint32 `count:"Count"`
	Pad   [3]byte  `skip:"3"`
	Inner fuzzInner
	Tail  Rest
}

func randomFuzzMsg(r *rand.Rand) fuzzMsg {
	m := fuzzMsg{
		Flags: r.Uint32() & 0x03, // 只用 bit0/bit1，保证指针状态与谓词一致
		Count: uint8(r.Intn(21)),
		Inner: fuzzInner{A: uint16(r.Uint32()), B: int8(r.Uint32())},
		Tail:  make([]byte, r.Intn(17)),
	}
	if m.Flags&0x01 != 0 {
		v := r.Uint32()
		m.Code = &v
	}
	if m.Flags&0x02 != 0 {
		m.Pos = &fuzzInner{A: uint16(r.Uint32()), B: int8(r.Uint32())}
	}
	m.Items = make([]uint32, m.Count)
	for i := range m.Items {
		m.Items[i] = r.Uint32()
	}
	r.Read(m.Tail)
	return m
}

// TestFuzzRoundTrip 随机生成 1000 个满足约束的结构体，Marshal → Unmarshal
// 后与（回填过 Count 的）原值 DeepEqual。
func TestFuzzRoundTrip(t *testing.T) {
	r := rand.New(rand.NewSource(20261019))
	for i := 0; i < 1000; i++ {
		want := randomFuzzMsg(r)
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("第 %d 轮 Marshal: %v", i, err)
		}
		var got fuzzMsg
		if err := Unmarshal(data, &got); err != nil {
			t.Fatalf("第 %d 轮 Unmarshal: %v", i, err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("第 %d 轮往返不一致:\nwant %+v\ngot  %+v\nbytes %x", i, want, got, data)
		}
	}
}

// TestFuzzStream 把 fuzz 消息首尾相接成一条流，用 Decoder 逐条解回。
// 流场景不能使用 greedy 字段（它会吃掉后面消息的字节），这里用无 greedy 的变体。
type streamMsg struct {
	Flags uint32
	Code  *uint32   `if:"Flags&0x01"`
	Pos   *fuzzInner `if:"Flags&0x02"`
	Count uint8
	Items []uint32 `count:"Count"`
	Inner fuzzInner
}

func randomStreamMsg(r *rand.Rand) streamMsg {
	m := streamMsg{
		Flags: r.Uint32() & 0x03,
		Count: uint8(r.Intn(13)),
		Inner: fuzzInner{A: uint16(r.Uint32()), B: int8(r.Uint32())},
	}
	if m.Flags&0x01 != 0 {
		v := r.Uint32()
		m.Code = &v
	}
	if m.Flags&0x02 != 0 {
		m.Pos = &fuzzInner{A: uint16(r.Uint32()), B: int8(r.Uint32())}
	}
	m.Items = make([]uint32, m.Count)
	for i := range m.Items {
		m.Items[i] = r.Uint32()
	}
	return m
}

func TestFuzzStream(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	const n = 200
	msgs := make([]streamMsg, n)
	enc := NewEncoder()
	for i := range msgs {
		msgs[i] = randomStreamMsg(r)
		if err := enc.Encode(&msgs[i]); err != nil {
			t.Fatalf("Encode #%d: %v", i, err)
		}
	}
	dec := NewDecoder(enc.Bytes())
	for i := range msgs {
		var got streamMsg
		if err := dec.Decode(&got); err != nil {
			t.Fatalf("Decode #%d: %v", i, err)
		}
		if !reflect.DeepEqual(msgs[i], got) {
			t.Fatalf("流内第 %d 条不一致:\nwant %+v\ngot  %+v", i, msgs[i], got)
		}
	}
	if dec.Remaining() != 0 {
		t.Fatalf("解码后剩余 %d 字节", dec.Remaining())
	}
}
