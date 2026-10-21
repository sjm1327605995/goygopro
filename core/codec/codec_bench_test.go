package codec

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 反射基线（legacy 前缀）：重构前热路径的逐字段 reflect.Value.Field/Set
// 写法，仅覆盖 benchmark 用到的 tag 组合（定长标量、bool、定长数组、count
// 列表、if 条件指针），用于对照「无反射计划链」的加速比。
// ---------------------------------------------------------------------------

func legacyKindSize(k reflect.Kind) int {
	switch k {
	case reflect.Bool, reflect.Int8, reflect.Uint8:
		return 1
	case reflect.Int16, reflect.Uint16:
		return 2
	case reflect.Int32, reflect.Uint32:
		return 4
	case reflect.Int64, reflect.Uint64:
		return 8
	}
	return 0
}

// legacyIfMask 解析 if:"Name&mask"，无 if tag 时 ok=false。
func legacyIfMask(sf reflect.StructField) (name string, mask uint64, ok bool) {
	tag, found := sf.Tag.Lookup("if")
	if !found {
		return "", 0, false
	}
	n, m, found := strings.Cut(tag, "&")
	name = strings.TrimSpace(n)
	if !found {
		return name, ^uint64(0), true
	}
	mv, err := strconv.ParseUint(strings.TrimSpace(m), 0, 64)
	if err != nil {
		return name, 0, true
	}
	return name, mv, true
}

func legacyFieldIndex(t reflect.Type, name string) int {
	for i := 0; i < t.NumField(); i++ {
		if strings.EqualFold(t.Field(i).Name, name) {
			return i
		}
	}
	return -1
}

func legacyFixedSize(t reflect.Type) int {
	switch t.Kind() {
	case reflect.Bool,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return legacyKindSize(t.Kind())
	case reflect.Array:
		return t.Len() * legacyFixedSize(t.Elem())
	}
	return 0
}

// legacyCond 求 if 谓词（谓词字段为整型）。
func legacyCond(sv reflect.Value, sf reflect.StructField) bool {
	name, mask, has := legacyIfMask(sf)
	if !has {
		return true
	}
	fi := legacyFieldIndex(sv.Type(), name)
	if fi < 0 {
		return false
	}
	fv := sv.Field(fi)
	var v uint64
	switch fv.Kind() {
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v = uint64(fv.Int())
	default:
		v = fv.Uint()
	}
	return v&mask != 0
}

// legacySizeOf 走值算大小（count 列表用 len；条件字段 nil 跳过）。
func legacySizeOf(fv reflect.Value, sf reflect.StructField, sv reflect.Value) (int, error) {
	if !legacyCond(sv, sf) && fv.Kind() == reflect.Ptr {
		return 0, nil
	}
	switch fv.Kind() {
	case reflect.Bool,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(fv.Type().Size()), nil
	case reflect.Array:
		return legacyFixedSize(fv.Type()), nil
	case reflect.Slice:
		return fv.Len() * legacyFixedSize(fv.Type().Elem()), nil
	case reflect.Ptr:
		if fv.IsNil() {
			return 0, fmt.Errorf("legacy: nil 指针")
		}
		return legacySizeOf(fv.Elem(), sf, sv)
	}
	return 0, fmt.Errorf("legacy: 不支持的类型 %s", fv.Kind())
}

func legacyEncodeValue(buf []byte, off int, fv reflect.Value) int {
	switch fv.Kind() {
	case reflect.Bool:
		if fv.Bool() {
			buf[off] = 1
		}
		return off + 1
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := fv.Int()
		switch fv.Type().Size() {
		case 1:
			buf[off] = byte(int8(v))
		case 2:
			binary.LittleEndian.PutUint16(buf[off:], uint16(int16(v)))
		case 4:
			binary.LittleEndian.PutUint32(buf[off:], uint32(int32(v)))
		default:
			binary.LittleEndian.PutUint64(buf[off:], uint64(v))
		}
		return off + int(fv.Type().Size())
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := fv.Uint()
		switch fv.Type().Size() {
		case 1:
			buf[off] = byte(v)
		case 2:
			binary.LittleEndian.PutUint16(buf[off:], uint16(v))
		case 4:
			binary.LittleEndian.PutUint32(buf[off:], uint32(v))
		default:
			binary.LittleEndian.PutUint64(buf[off:], v)
		}
		return off + int(fv.Type().Size())
	}
	return off
}

// legacyEncodeField 编码一个字段（slice 逐元素；标量数组整块等价展开）。
func legacyEncodeField(buf []byte, off int, fv reflect.Value, sf reflect.StructField, sv reflect.Value) (int, error) {
	if !legacyCond(sv, sf) && fv.Kind() == reflect.Ptr {
		return off, nil
	}
	switch fv.Kind() {
	case reflect.Bool,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return legacyEncodeValue(buf, off, fv), nil
	case reflect.Slice:
		for i := 0; i < fv.Len(); i++ {
			off = legacyEncodeValue(buf, off, fv.Index(i))
		}
		return off, nil
	case reflect.Array:
		for i := 0; i < fv.Len(); i++ {
			off = legacyEncodeValue(buf, off, fv.Index(i))
		}
		return off, nil
	case reflect.Ptr:
		if fv.IsNil() {
			return off, fmt.Errorf("legacy: 无条件指针字段为 nil")
		}
		return legacyEncodeField(buf, off, fv.Elem(), sf, sv)
	}
	return off, fmt.Errorf("legacy: 不支持的类型 %s", fv.Kind())
}

func legacyDecodeValue(b []byte, fv reflect.Value) {
	switch fv.Kind() {
	case reflect.Bool:
		fv.SetBool(b[0] != 0)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var v uint64
		switch len(b) {
		case 1:
			v = uint64(int8(b[0]))
		case 2:
			v = uint64(int64(int16(binary.LittleEndian.Uint16(b))))
		case 4:
			v = uint64(int64(int32(binary.LittleEndian.Uint32(b))))
		default:
			v = binary.LittleEndian.Uint64(b)
		}
		fv.SetInt(int64(v))
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var v uint64
		switch len(b) {
		case 1:
			v = uint64(b[0])
		case 2:
			v = uint64(binary.LittleEndian.Uint16(b))
		case 4:
			v = uint64(binary.LittleEndian.Uint32(b))
		default:
			v = binary.LittleEndian.Uint64(b)
		}
		fv.SetUint(v)
	}
}

// legacyDecodeField 从 data 解码一个字段并返回新偏移。
func legacyDecodeField(data []byte, off int, fv reflect.Value, sf reflect.StructField, sv reflect.Value) (int, error) {
	if !legacyCond(sv, sf) && fv.Kind() == reflect.Ptr {
		fv.SetZero()
		return off, nil
	}
	if fv.Kind() == reflect.Ptr {
		np := reflect.New(fv.Type().Elem())
		fv.Set(np)
		return legacyDecodeField(data, off, np.Elem(), sf, sv)
	}
	w := legacyKindSize(fv.Kind())
	switch fv.Kind() {
	case reflect.Bool,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if off+w > len(data) {
			return off, fmt.Errorf("legacy: 截断")
		}
		legacyDecodeValue(data[off:off+w], fv)
		return off + w, nil
	case reflect.Slice:
		ci := legacyFieldIndex(sv.Type(), sf.Tag.Get("count"))
		if ci < 0 {
			return off, fmt.Errorf("legacy: count 字段缺失")
		}
		cv := sv.Field(ci)
		if cv.Kind() == reflect.Ptr {
			cv = cv.Elem()
		}
		var n int
		switch cv.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n = int(cv.Int())
		default:
			n = int(cv.Uint())
		}
		es := legacyFixedSize(fv.Type().Elem())
		if n < 0 || off+n*es > len(data) {
			return off, fmt.Errorf("legacy: 截断")
		}
		out := reflect.MakeSlice(fv.Type(), n, n)
		for i := 0; i < n; i++ {
			legacyDecodeValue(data[off+i*es:off+(i+1)*es], out.Index(i))
		}
		fv.Set(out)
		return off + n*es, nil
	case reflect.Array:
		es := legacyFixedSize(fv.Type().Elem())
		for i := 0; i < fv.Len(); i++ {
			if off+es > len(data) {
				return off, fmt.Errorf("legacy: 截断")
			}
			legacyDecodeValue(data[off:off+es], fv.Index(i))
			off += es
		}
		return off, nil
	}
	return off, fmt.Errorf("legacy: 不支持的类型 %s", fv.Kind())
}

// legacyMarshal 反射基线编码：先逐字段反射求大小再一次分配，再逐字段反射写。
func legacyMarshal(v any) ([]byte, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("legacy: 需要结构体指针")
	}
	sv := rv.Elem()
	st := sv.Type()
	n := 0
	for i := 0; i < st.NumField(); i++ {
		sz, err := legacySizeOf(sv.Field(i), st.Field(i), sv)
		if err != nil {
			return nil, err
		}
		n += sz
	}
	buf := make([]byte, n)
	off := 0
	for i := 0; i < st.NumField(); i++ {
		var err error
		off, err = legacyEncodeField(buf, off, sv.Field(i), st.Field(i), sv)
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// legacyUnmarshal 反射基线解码：逐字段 reflect.Value.Field/Set 调用。
func legacyUnmarshal(data []byte, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("legacy: 需要结构体指针")
	}
	sv := rv.Elem()
	st := sv.Type()
	off := 0
	for i := 0; i < st.NumField(); i++ {
		var err error
		off, err = legacyDecodeField(data, off, sv.Field(i), st.Field(i), sv)
		if err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 代表性结构体：定长结构 / count 列表 / queryCardWire 式 if 条件字段结构。
// ---------------------------------------------------------------------------

type benchFixed struct {
	A   uint8
	B   uint16
	C   uint32
	D   uint64
	E   int32
	F   int64
	Ok  bool
	Arr [8]byte
}

type benchList struct {
	N     uint8
	Items []uint32 `count:"N"`
}

type benchWire struct {
	Flag    uint32
	Code    *uint32   `if:"Flag&0x01"`
	Pos     *uint32   `if:"Flag&0x02"`
	Atk     *int32    `if:"Flag&0x100"`
	Def     *int32    `if:"Flag&0x200"`
	TC      *uint32   `if:"Flag&0x8000"`
	Targets *[]uint32 `if:"Flag&0x8000" count:"TC"`
	Link    *uint32   `if:"Flag&0x800000"`
	Marker  *uint32   `if:"Flag&0x800000"`
}

func newBenchFixed() *benchFixed {
	return &benchFixed{A: 1, B: 2, C: 3, D: 4, E: -5, F: 6, Ok: true, Arr: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}}
}

func newBenchList() *benchList {
	return &benchList{N: 8, Items: []uint32{1, 2, 3, 4, 5, 6, 7, 8}}
}

func newBenchWire() *benchWire {
	code, pos := uint32(89631139), uint32(1|4<<8|2<<16|1<<24)
	atk, def := int32(3000), int32(2500)
	tc := uint32(2)
	link, marker := uint32(5), uint32(0xbf)
	return &benchWire{
		Flag:    0x800003,
		Code:    &code,
		Pos:     &pos,
		Atk:     &atk,
		Def:     &def,
		TC:      &tc,
		Targets: &[]uint32{pos, pos},
		Link:    &link,
		Marker:  &marker,
	}
}

// ---------------------------------------------------------------------------
// Marshal benchmark：计划链 vs 反射基线
// ---------------------------------------------------------------------------

func BenchmarkMarshalFixed(b *testing.B) {
	v := newBenchFixed()
	b.Run("gen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Marshal(v); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := legacyMarshal(v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMarshalList(b *testing.B) {
	v := newBenchList()
	b.Run("gen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Marshal(v); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := legacyMarshal(v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMarshalWire(b *testing.B) {
	v := newBenchWire()
	b.Run("gen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Marshal(v); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := legacyMarshal(v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Unmarshal benchmark
// ---------------------------------------------------------------------------

func BenchmarkUnmarshalFixed(b *testing.B) {
	data, err := Marshal(newBenchFixed())
	if err != nil {
		b.Fatal(err)
	}
	b.Run("gen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var v benchFixed
			if err := Unmarshal(data, &v); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var v benchFixed
			if err := legacyUnmarshal(data, &v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkUnmarshalList(b *testing.B) {
	data, err := Marshal(newBenchList())
	if err != nil {
		b.Fatal(err)
	}
	b.Run("gen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var v benchList
			if err := Unmarshal(data, &v); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var v benchList
			if err := legacyUnmarshal(data, &v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkUnmarshalWire(b *testing.B) {
	data, err := Marshal(newBenchWire())
	if err != nil {
		b.Fatal(err)
	}
	b.Run("gen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var v benchWire
			if err := Unmarshal(data, &v); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var v benchWire
			if err := legacyUnmarshal(data, &v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// MarshalInto（零分配）vs Marshal；Frozen 重复发送路径
// ---------------------------------------------------------------------------

func BenchmarkMarshalIntoWire(b *testing.B) {
	v := newBenchWire()
	dst := make([]byte, 128)
	b.Run("gen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := MarshalInto(dst, v); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("marshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := Marshal(v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkResendFrozen 模拟扇出：同一负载发给 N 个接收方。
// Frozen 路径只 copy 线字节；对照组每次重新 Marshal。
func BenchmarkResendFrozen(b *testing.B) {
	v := newBenchWire()
	f := MustFreeze(v)
	out := make([]byte, 128)
	b.Run("frozen", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			copy(out, f.Bytes())
		}
	})
	b.Run("remarshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			wire, err := Marshal(v)
			if err != nil {
				b.Fatal(err)
			}
			copy(out, wire)
		}
	})
}
