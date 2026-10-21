package codec

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// WireLen 测试结构
// ---------------------------------------------------------------------------

type wlBlobMsg struct {
	Kind uint8
	Len  WireLen16
	Blob []byte
	Tail uint32
}

type wlStructInner struct {
	X uint16
	Y int32
}

type wlStructMsg struct {
	Flag uint8
	Len  WireLen8
	Body wlStructInner
}

type wlSliceMsg struct {
	Kind uint8
	Len  WireLen32
	Nums []uint32
}

type wlNestedOuter struct {
	A    uint16
	Len  WireLen16
	Blob []byte
	In   wlBlobMsg
}

// 以下类型原本定义在各测试函数内；codecgen 只扫描包级声明，提升到这里
// （语义不变，生成物见 zz_codec_gen_test.go）。
type (
	wlNoTarget    struct{ A uint8; L WireLen16 }
	wlThenGreedy  struct {
		L    WireLen16
		Blob []byte
		Tail []byte `greedy:""`
	}
	wlBadTarget struct {
		L    WireLen16
		Code uint32
	}
	regFresh struct{ A uint32 }
	regGood  struct{ B uint8 }
	regOk    struct{ C uint8 }
	regLazy  struct {
		L    WireLen16
		Blob []byte
	}
	wlSmall struct {
		Len  WireLen8
		Blob []byte
	}
	withCount struct {
		N     uint8
		Items []uint32 `count:"N"`
	}
	// marshalIntoWire 覆盖 MarshalInto 的条件字段路径（指针非 nil 时谓词位置位）。
	marshalIntoWire struct {
		Flag uint32
		Code *uint32 `if:"Flag&0x01"`
		Atk  *int32  `if:"Flag&0x100"`
	}
	// notCodedMsg 有生成器不支持的字段（string），拿不到生成方法，
	// 用于「未生成类型」错误路径测试。
	notCodedMsg struct{ S string }
)

// TestWireLenBlob：WireLen16 + []byte 空/小/大负载往返 + Len 自动回填。
func TestWireLenBlob(t *testing.T) {
	for _, payload := range [][]byte{{}, {0xab}, make([]byte, 700)} {
		want := wlBlobMsg{Kind: 7, Blob: payload, Tail: 0xdeadbeef}
		data, err := Marshal(&want)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		// 自动回填：Len == len(Blob)（调用方未赋值）
		if int(want.Len) != len(payload) {
			t.Fatalf("Len 未回填: %d，期望 %d", want.Len, len(payload))
		}
		// 线上前缀 = 小端长度
		if int(data[1])|int(data[2])<<8 != len(payload) {
			t.Fatalf("线上长度前缀错误: %v", data[:3])
		}
		var got wlBlobMsg
		if err := Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("往返不一致:\nwant %+v\ngot  %+v", want, got)
		}
	}
}

// TestWireLenBlobManualValueIgnored：调用方手动填了不一致的 Len，以实际线宽为准。
func TestWireLenBlobManualValueIgnored(t *testing.T) {
	m := wlBlobMsg{Kind: 1, Len: 99, Blob: []byte{1, 2, 3}, Tail: 4}
	data, err := Marshal(&m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if m.Len != 3 {
		t.Fatalf("Len = %d，应以实际线宽回填为 3", m.Len)
	}
	var got wlBlobMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Len != 3 || len(got.Blob) != 3 {
		t.Fatalf("解码 Len = %d, blob = %v", got.Len, got.Blob)
	}
}

// TestWireLenNestedAndStream：嵌套结构体内的 WireLen + Decoder 连解多条消息。
func TestWireLenNestedAndStream(t *testing.T) {
	want := wlNestedOuter{
		A:    0x1234,
		Blob: []byte{9, 9, 9},
		In:   wlBlobMsg{Kind: 5, Blob: []byte{1, 2, 3, 4, 5}, Tail: 42},
	}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got wlNestedOuter
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("嵌套往返不一致:\nwant %+v\ngot  %+v", want, got)
	}

	// Decoder 连解两条含 WireLen 的消息
	m1 := wlBlobMsg{Kind: 1, Blob: []byte{0xaa}}
	m2 := wlBlobMsg{Kind: 2, Blob: []byte{0xbb, 0xcc}, Tail: 7}
	enc := NewEncoder()
	if err := enc.Encode(&m1); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(&m2); err != nil {
		t.Fatal(err)
	}
	dec := NewDecoder(enc.Bytes())
	var g1, g2 wlBlobMsg
	if err := dec.Decode(&g1); err != nil {
		t.Fatalf("Decode m1: %v", err)
	}
	if err := dec.Decode(&g2); err != nil {
		t.Fatalf("Decode m2: %v", err)
	}
	if !reflect.DeepEqual(m1, g1) || !reflect.DeepEqual(m2, g2) {
		t.Fatalf("流式解码不一致: %+v %+v", g1, g2)
	}
	if dec.Remaining() != 0 {
		t.Fatalf("剩余 %d 字节", dec.Remaining())
	}
}

// TestWireLenStruct：WireLen8 + 定长结构体往返 + 长度不符报错。
func TestWireLenStruct(t *testing.T) {
	want := wlStructMsg{Flag: 1, Body: wlStructInner{X: 0xbeef, Y: -7}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want.Len != 6 { // X(2) + Y(4)
		t.Fatalf("Len 回填 = %d，期望 6", want.Len)
	}
	var got wlStructMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("往返不一致:\nwant %+v\ngot  %+v", want, got)
	}
	// 手工构造长度不符的数据：Len=5，线宽=6
	bad := []byte{1, 5, 0xef, 0xbe, 0xff, 0xff, 0xff, 0xf9, 0}
	bad = append(bad[:2], append(make([]byte, 6), 0)...) // flag=1 len=5 + 6 字节占位
	var g2 wlStructMsg
	err = Unmarshal(bad, &g2)
	if err == nil || !strings.Contains(err.Error(), "不符") {
		t.Fatalf("长度不符应报错，得到 %v", err)
	}
}

// TestWireLenSlice：WireLen32 + []uint32 往返 + 非整除报错。
func TestWireLenSlice(t *testing.T) {
	want := wlSliceMsg{Kind: 3, Nums: []uint32{10, 20, 30}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want.Len != 12 {
		t.Fatalf("Len 回填 = %d，期望 12（字节数）", want.Len)
	}
	var got wlSliceMsg
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("往返不一致:\nwant %+v\ngot  %+v", want, got)
	}
	// 非整除：Len=10 不是 4 的倍数（剩余字节充足）
	bad := []byte{3, 10, 0, 0, 0}
	bad = append(bad, make([]byte, 10)...)
	var g2 wlSliceMsg
	err = Unmarshal(bad, &g2)
	if err == nil || !strings.Contains(err.Error(), "整数倍") {
		t.Fatalf("非整除应报错，得到 %v", err)
	}
	// 截断：Len=12 但只剩 4 字节
	bad2 := []byte{3, 12, 0, 0, 0, 1, 0, 0, 0}
	var g3 wlSliceMsg
	err = Unmarshal(bad2, &g3)
	if err == nil || !strings.Contains(err.Error(), "need 12 have 4") {
		t.Fatalf("截断应报错，得到 %v", err)
	}
}

// TestWireLenOverflow：线宽超出 WireLen 宽度（300 字节装进 WireLen8）。
func TestWireLenOverflow(t *testing.T) {
	m := wlSmall{Blob: make([]byte, 300)}
	_, err := Marshal(&m)
	if err == nil || !strings.Contains(err.Error(), "超出 WireLen 宽度") {
		t.Fatalf("期望溢出错误，得到 %v", err)
	}
}

// TestWireLenInvalid：布局校验——WireLen 无目标、目标后随 greedy、目标类型非法。
func TestWireLenInvalid(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"无目标字段", &wlNoTarget{}, "必须紧跟目标字段"},
		{"目标后是 greedy", &wlThenGreedy{}, "不得是 greedy"},
		{"目标类型非法", &wlBadTarget{}, "目标字段类型不支持"},
	}
	for _, c := range cases {
		if _, err := Marshal(c.v); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s：期望包含 %q 的错误，得到 %v", c.name, c.want, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Register 测试
// ---------------------------------------------------------------------------

type regInner struct {
	X uint16
}

type regOuter struct {
	Head  uint8
	Inner regInner
	P     *regInner
	N     uint8
	Items []regInner `count:"N"`
}

type regBad struct {
	Items []uint32 // 无 count/greedy tag
}

// TestRegisterGenerated：注册生成类型成功；嵌套/值/指针/列表元素类型由
// codecgen 统一生成，无需逐个注册；重复注册幂等。
func TestRegisterGenerated(t *testing.T) {
	if err := Register(&regOuter{}, &regFresh{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	// 嵌套类型（值字段、指针目标、slice 元素）各自带生成方法，可直接使用
	var inner regInner
	if err := Unmarshal([]byte{0x34, 0x12}, &inner); err != nil {
		t.Fatalf("嵌套类型解码: %v", err)
	}
	if inner.X != 0x1234 {
		t.Fatalf("嵌套类型解码值错误: %+v", inner)
	}
	// 幂等：重复注册同样成功
	if err := Register(&regOuter{}, &regFresh{}); err != nil {
		t.Fatalf("重复 Register: %v", err)
	}
}

// TestRegisterBadTag：坏 tag 注册即报错且带类型名。
func TestRegisterBadTag(t *testing.T) {
	err := Register(&regBad{})
	if err == nil {
		t.Fatal("坏 tag 应注册失败")
	}
	if !strings.Contains(err.Error(), "regBad") {
		t.Fatalf("错误应带类型名: %v", err)
	}
	// 多样本聚合：坏样本报错、好样本仍成功注册
	err = Register(&regBad{}, &regGood{})
	if err == nil || !strings.Contains(err.Error(), "regBad") {
		t.Fatalf("聚合错误应包含坏类型名: %v", err)
	}
	if _, merr := Marshal(&regGood{}); merr != nil {
		t.Fatalf("好样本应可用: %v", merr)
	}
}

// TestRegisterBadSample：非结构体样本报错。
func TestRegisterBadSample(t *testing.T) {
	if err := Register(42); err == nil || !strings.Contains(err.Error(), "int") {
		t.Fatalf("非结构体样本应报错，得到 %v", err)
	}
	if err := Register(nil); err == nil {
		t.Fatal("nil 样本应报错")
	}
	if !errors.Is(Register(), nil) {
		t.Fatal("空注册应返回 nil")
	}
}

// TestMustRegister：panic 版语义。
func TestMustRegister(t *testing.T) {
	MustRegister(&regOk{}) // 不应 panic
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("MustRegister 对坏类型应 panic")
		}
		if s, ok := r.(string); !ok || !strings.Contains(s, "regBad") {
			t.Fatalf("panic 信息应带类型名: %v", r)
		}
	}()
	MustRegister(&regBad{})
}

// TestUngeneratedTypeErrors：未生成（候选都不构成）的类型在首次使用时返回
// 带类型名的错误，Register 同样 fail-fast。
func TestUngeneratedTypeErrors(t *testing.T) {
	var m notCodedMsg
	if _, err := Marshal(&m); err == nil || !strings.Contains(err.Error(), "notCodedMsg") {
		t.Fatalf("未生成类型 Marshal 应报类型名错误，得到 %v", err)
	}
	if err := Register(&m); err == nil || !strings.Contains(err.Error(), "notCodedMsg") {
		t.Fatalf("未生成类型 Register 应报类型名错误，得到 %v", err)
	}
	// 惰性可用性（旧 TestLazyBuildStillWorks 的语义）：未显式注册的类型
	// 首次调用即可用（方法随类型编译进包，无需构建缓存）。
	want := regLazy{Blob: []byte{1, 2, 3}}
	data, err := Marshal(&want)
	if err != nil {
		t.Fatalf("未注册类型 Marshal: %v", err)
	}
	var got regLazy
	if err := Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("往返不一致: %+v vs %+v", got, want)
	}
}

// ---------------------------------------------------------------------------
// MarshalInto / Freeze
// ---------------------------------------------------------------------------

// TestMarshalIntoRoundTrip：MarshalInto 与 Marshal 字节等价，解码后一致。
func TestMarshalIntoRoundTrip(t *testing.T) {
	code, atk := uint32(89631139), int32(3000)
	rtWire := &marshalIntoWire{Flag: 0x101, Code: &code, Atk: &atk}
	cases := []any{newBenchFixed(), newBenchList(), rtWire}
	for _, v := range cases {
		want, err := Marshal(v)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		dst := make([]byte, len(want)+16) // 尾部多余容量不影响
		n, err := MarshalInto(dst, v)
		if err != nil {
			t.Fatalf("MarshalInto: %v", err)
		}
		if n != len(want) {
			t.Fatalf("MarshalInto n = %d，期望 %d", n, len(want))
		}
		if !bytes.Equal(dst[:n], want) {
			t.Fatalf("MarshalInto 与 Marshal 字节不等:\n got %v\nwant %v", dst[:n], want)
		}
		// 解码回原值
		rv := reflect.New(reflect.TypeOf(v).Elem())
		if err := Unmarshal(dst[:n], rv.Interface()); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if !reflect.DeepEqual(rv.Interface(), v) {
			t.Fatalf("往返不一致: %+v vs %+v", rv.Interface(), v)
		}
	}
}

// TestMarshalIntoCapacity：容量不足返回带 need/have 的 *Error。
func TestMarshalIntoCapacity(t *testing.T) {
	v := newBenchList()
	full, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	small := make([]byte, len(full)-1)
	n, err := MarshalInto(small, v)
	if err == nil {
		t.Fatal("容量不足应报错")
	}
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("错误类型 = %T", err)
	}
	if e.Op != "marshal" || e.Need != len(full) || e.Have != len(small) {
		t.Fatalf("Error 定位错误: %+v", e)
	}
	if n != 0 {
		t.Fatalf("失败应返回 n=0，得到 %d", n)
	}
	// 恰好容量成功
	exact := make([]byte, len(full))
	if n, err := MarshalInto(exact, v); err != nil || n != len(full) {
		t.Fatalf("恰好容量应成功: n=%d err=%v", n, err)
	}
	// 非结构体按既有语义报错
	if _, err := MarshalInto(small, 42); err == nil {
		t.Fatal("非结构体应报错")
	}
}

// TestMarshalIntoStructValue：结构体值经装箱副本编码，与指针路径字节一致
// 且调用方值不被回填副作用修改。
func TestMarshalIntoStructValue(t *testing.T) {
	v := withCount{Items: []uint32{1, 2, 3}}
	dst := make([]byte, 16)
	n, err := MarshalInto(dst, v) // 注意：传值
	if err != nil {
		t.Fatalf("MarshalInto(值): %v", err)
	}
	if v.N != 0 {
		t.Fatalf("值语义被破坏：调用方 N 被回填为 %d", v.N)
	}
	want, err := Marshal(&withCount{Items: []uint32{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dst[:n], want) {
		t.Fatalf("值路径与指针路径字节不等:\n got %v\nwant %v", dst[:n], want)
	}
	// 指针路径照常回填
	vp := withCount{Items: []uint32{1, 2, 3}}
	if _, err := MarshalInto(dst, &vp); err != nil {
		t.Fatal(err)
	}
	if vp.N != 3 {
		t.Fatalf("指针路径应回填 N=3，得到 %d", vp.N)
	}
}

// TestFreeze：Freeze/MustFreeze 字节与 Marshal 一致，Len/Bytes 正确。
func TestFreeze(t *testing.T) {
	wire, err := Marshal(newBenchWire())
	if err != nil {
		t.Fatal(err)
	}
	f, err := Freeze(newBenchWire())
	if err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if f.Len() != len(wire) || !bytes.Equal(f.Bytes(), wire) {
		t.Fatalf("Freeze 字节与 Marshal 不等")
	}
	mf := MustFreeze(newBenchWire())
	if !bytes.Equal(mf.Bytes(), wire) {
		t.Fatalf("MustFreeze 字节不等")
	}
	// 值语义：拷贝共享同一线字节（只读约定）
	f2 := f
	if !bytes.Equal(f2.Bytes(), f.Bytes()) {
		t.Fatal("Frozen 拷贝应共享线字节")
	}
	// Freeze 失败路径
	if _, err := Freeze(42); err == nil {
		t.Fatal("Freeze 非结构体应报错")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustFreeze 对坏类型应 panic")
		}
	}()
	MustFreeze(42)
}
