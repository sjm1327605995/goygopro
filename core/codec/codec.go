// Package codec 提供面向 YGO 协议的声明式二进制序列化。
//
// 项目里大量引擎消息（STOC / MSG）是"定长头 + flag 驱动的条件字段 +
// 长度前缀列表"的二进制布局，手写偏移（如 decodeQueryBody 的闭包+u32()
// 逐字段钳制）既冗长又容易越界。本库把 struct tag 声明的布局在**构建期**
// 翻译成每个结构体自己的编解码方法（core/codec/internal/codecgen 生成
// zz_codec_gen.go），运行时零反射：Marshal / Unmarshal 经内建接口一次
// 分发到生成代码，生成代码用 encoding/binary 小端原语直接读写缓冲区，
// 语义等同 C 的 *(T*)buf 且完全可移植（js/wasm 可直接构建）。
// 所有截断、类型不符、布局非法都以带定位信息的 error 返回，绝不 panic。
// 字节序固定 LittleEndian（YGO 协议全为 LE，不做参数）。
//
// # struct tag / 标记类型一览（生成器在构建期解释）
//
//	| 机制     | 语法/类型         | 含义                                                                                       |
//	|----------|-------------------|--------------------------------------------------------------------------------------------|
//	| （无）   | —                 | 按字段顺序定长编码。支持整型、bool、定长数组 [N]T、嵌套结构体（值或指针）                   |
//	| count    | count:"字段名"    | 用于 slice：反序列化时长度取前面已解析的整型字段的值；序列化时自动把 len(slice) 回填该字段 |
//	| bit      | bit:"N"           | 条件字段（仅指针）：声明序向前最近的前导整型字段第 N 位置位才占字节，否则指针置 nil     |
//	| WireLen  | WireLen8/16/32    | 标记字段：值 = 紧邻下一个字段的线上字节长度，编码自动回填、解码先读长度再按长度读数据      |
//	| Count    | Count8/16/32      | 标记字段：值 = 之后第一个 slice 字段的元素个数，编码自动回填、解码先读个数再按个数读定宽元素 |
//	| Rest     | codec.Rest        | 消息体尾部的剩余字节载荷（greedy 的具名类型形式，优先使用；字段类型即 Rest，无需 tag）    |
//	| greedy   | greedy:""         | （保留，高级用法）仅 []byte，吃掉全部剩余字节；只能出现在结构体末尾                         |
//	| if       | if:"字段名&掩码"  | （保留，向后兼容）条件字段：谓词 (字段&掩码)!=0 时才占用字节，否则指针置 nil                |
//	| skip     | skip:"N"          | （保留，高级用法）跳过 N 字节（反序列化推进游标；序列化写 N 个零）                          |
//
// count 与 WireLen 的取舍：
//
//	|            | count tag                    | WireLen 标记                                      |
//	|------------|------------------------------|---------------------------------------------------|
//	| 语义       | 元素个数 + 定宽条目列表      | 字节长度 + 变长负载/单个结构体                    |
//	| 长度字段   | 任意前序整型字段（字符串引用） | 与数据紧邻的标记字段，无需引用                  |
//	| 长度单位   | 条目数 × 元素线宽            | 字节数                                            |
//	| 回填       | len(slice) 写入被引用字段    | 目标线宽自动写入 WireLen 字段与线上前缀           |
//	| 典型场景   | SELECT_CARD 卡列表、计数数组 | 长度前缀 blob、内嵌定长消息帧                   |
//
// 规则细节：
//
//   - bool 序列化为 1 字节 0/1；反序列化非 0 即 true。string 不支持（用 []byte）。
//   - count 的长度字段必须是同一结构体中出现在 slice 之前的整型字段（条件
//     布局下也可以是 *int 指针——slice 解码时指针必已分配），元素支持定长类型
//     与嵌套结构体。序列化方向会原地回填 len(slice)，长度溢出长度字段宽度时报错。
//   - Count 标记字段沿声明序向后绑定第一个未绑定且非 greedy 的 slice 字段；
//     中间的非 slice 字段（如 SELECT_CHAIN 的 spe_count/hint）正常编解码、
//     不参与绑定。
//   - bit 用于指针字段（*uint32 / *结构体 / *[]T）：谓词是声明序向前最近的
//     前导整型字段的第 N 位（N 必须小于字段线宽 × 8）；不置位时指针置 nil、
//     不占字节。它是 if:"名&1<<N" 的位置约定写法（谓词字段不写名字、掩码
//     不写表达式），优先使用；if 保留作向后兼容与任意掩码场景。
//   - if 配合指针字段使用（*uint32 / *结构体 / *[]T）；谓词不成立时指针置 nil。
//     掩码支持 0x 十六进制，省略 "&掩码" 表示全 1。序列化方向：指针非 nil 才写，
//     由调用方保证指针非 nil 时谓词成立（库不做校验）。
//   - WireLen 字段与紧邻的下一个字段绑定：目标允许 []byte（blob）、定长结构体
//     （消费长度必须恰等于线宽，不符报 *Error）、定长元素 slice（长度必须是元素
//     线宽的整数倍）。目标字段之后不得是 greedy 字段。编码方向以实际线宽回填
//     （调用方手动填的值被覆盖）；线宽溢出 WireLen 宽度报 *Error。
//   - 无条件指针字段 nil 在序列化时报错；反序列化方向无条件指针总是分配。
//   - greedy 字段吃掉全部剩余字节，因此不能出现在连续消息流的中间消息里。
//   - Decoder 用于一条连接上连续多条消息；Decode 遇错后游标不推进（幂等可重试）。
//   - Unmarshal 允许 data 尾部有多余字节（与 Decoder 语义一致），需要严格校验
//     的调用方自行比较消耗长度。
//
// # 代码生成与注册
//
// 每个含上述 tag/标记类型的结构体需要由 codecgen 生成编解码方法：在包内放
// 指令 `//go:generate codecgen`（测试类型用 `//go:generate codecgen -test`），
// 生成的 zz_codec_gen.go 提供：
//
//	func (m *T) CodecGenPtr()                          // 指针标记（运行时区分值/指针入参）
//	func (m T) CodecSize() (int, error)                // 线宽（count/WireLen 回填后的尺寸）
//	func (m T) CodecMarshalTo(dst []byte) int          // 编码（调用前须先经 CodecSize 校验）
//	func (m *T) CodecUnmarshalFrom(src []byte) (int, error)
//
// 值接收者的 size/marshal 让「传结构体值」天然获得副本语义（count 回填不污染
// 调用方值），无需运行时反射。未生成的类型在首次使用时返回带类型名的 *Error；
// Register/MustRegister 在启动期对生成类型 fail-fast（含 CodecValidate 的
// 生成类型会报出构建期校验错误），重复注册幂等。
//
// 线帧结构（FrameHeader/FrameLen/Frame/ErrorBody）由本包定义并维护，
// 服务端与客户端直接引用，不再各自重复定义。
//
//go:generate codecgen
//go:generate codecgen -test
package codec

import (
	"errors"
	"fmt"
	"strings"
)

// WireLen8/16/32 是标记字段类型：字段值 = 紧邻的下一个字段在线上占用的
// 字节长度（长度前缀线语义）。编码时以目标字段的实际线宽自动回填（调用
// 方无需赋值，手动填了不一致的值也以实际计算为准）；解码时先读长度、再
// 按长度读数据并校验（结构体目标必须恰等于线宽；定宽元素 slice 长度必须
// 是元素线宽的整数倍）。目标字段允许 []byte（blob）、定长结构体、定宽
// 元素的 slice；目标字段之后不得是 greedy 字段。
//
// 与 count tag 的取舍：count 表达「元素个数 + 定宽条目」列表（长度字段
// 是条目数，可位于结构体任意前序位置）；WireLen 表达「字节长度 + 变长
// 负载/单结构」（长度字段与数据紧邻，自动回填、无需字符串引用）。
type WireLen8 uint8

type WireLen16 uint16

type WireLen32 uint32

// Count8/16/32 是标记字段类型：字段值 = 本字段之后第一个 slice 字段的
// 元素个数（条目数线语义，元素宽度由生成器从元素类型推定，个数 × 元素
// 线宽即消费字节）。与 WireLen 的区别：WireLen 记字节长度（面向 blob/
// 变长负载），Count 记元素个数（面向定宽条目列表）。编码时自动把
// len(slice) 回填进 Count 字段（调用方无需赋值，手动填了不一致的值也以
// 实际为准），溢出 Count 宽度报 *Error；解码时先读个数、再按个数读元素。
// 目标 slice 必须是定宽元素（定长结构体或定宽标量），不得配 if/greedy/
// skip；含 Count 字段的结构体不是定长类型，不能作数组或列表元素。
//
// 绑定规则：Count 字段沿声明序向后扫描，绑定其后第一个未绑定且非
// greedy 的 slice 字段；中间的非 slice 字段（如 SELECT_CHAIN 的
// spe_count/hint）正常编解码、不参与绑定。每个 Count 字段必须绑到一个
// slice，每个 slice 至多被一个 Count 字段（或 count tag）绑定。
type Count8 uint8

type Count16 uint16

type Count32 uint32

// Rest 是消息体尾部的剩余字节载荷：解码时吃光全部剩余字节，编码时原样
// 写出。字段类型声明为 Rest 即获得 greedy 语义（greedy:"" tag 的具名类型
// 形式，优先使用）；只能出现在结构体末尾，不能作 count/WireLen 的绑定
// 目标。典型用途是线帧负载（Frame.Payload）。
type Rest []byte

// ---------------------------------------------------------------------------
// 共享线帧结构（core/duel 与 cmd/wails 共用，消除逐包重复定义）
// ---------------------------------------------------------------------------

// FrameHeader 是出站帧的 3 字节定长头（2 字节小端长度 + 1 字节 proto），
// 对应 C++ netserver.h SendPacketToPlayer 写的 (uint16)(1+len) + proto。
// Len 是普通字段而非 WireLen 标记：线语义为 1 + len(Payload)，由调用方
// 显式赋值，codec 不做自动回填。
type FrameHeader struct {
	Len   uint16
	Proto uint8
}

// FrameLen 是入站帧的 2 字节小端长度头（长度不含自身 2 字节、含 proto
// 字节），供拆帧循环先 Peek 长度再读整帧。
type FrameLen struct {
	Len uint16
}

// Frame 是完整线格式帧：2 字节小端长度 + 1 字节 proto + 负载。线格式的
// Len 含 proto 字节本身（= 1+len(Payload)），不满足 codec count 回填语义
// （count 回填的是 len(slice)），故 Len 由调用方显式赋值，负载用 Rest
// 吃光全部剩余字节。
type Frame struct {
	Len     uint16
	Proto   uint8
	Payload Rest
}

// ErrorBody 对应 STOC_ErrorMsg { uint8 msg; uint8 padding[3]; uint32 code }。
// Pad 是导出的定长零值数组：普通编解码对线上填充字节逐字节写零/读入
// （与旧 skip:"3" 的线格式完全相同，无需 tag）。Pad 必须保持零值——它是
// 线上填充字节，赋值会被原样发到线上。
type ErrorBody struct {
	Msg  uint8
	Pad  [3]byte
	Code uint32
}

// ---------------------------------------------------------------------------
// 生成代码的内建接口（codecgen 产物实现）
// ---------------------------------------------------------------------------

// genSizer 是生成代码的线宽接口（指针接收者）：原地回填 count/WireLen
// 后返回线宽。
type genSizer interface {
	CodecSize() (int, error)
}

// genValSizer 是结构体值入参的线宽接口（值接收者）：回填发生在方法副本上。
type genValSizer interface {
	CodecSizeVal() (int, error)
}

// genMarshaler 是生成代码的编码接口（指针接收者）：调用前必须先经
// CodecSize 完成校验与回填，因此编码本身不会再失败，只返回写入字节数。
type genMarshaler interface {
	CodecMarshalTo(dst []byte) int
}

// genValCoder 是结构体值入参的编码接口（值接收者）：在自己的副本上完成
// 回填 + 容量检查 + 编码，调用方的值不受影响。
type genValCoder interface {
	CodecEncodeInto(dst []byte) (int, error)
}

// genUnmarshaler 是生成代码的解码接口：返回消费的字节数；出错时返回
// *Error（Offset 相对本消息起始，外层调用经 WrapErr 累积成绝对偏移）。
type genUnmarshaler interface {
	CodecUnmarshalFrom(src []byte) (int, error)
}

// genPtrMarker 是生成代码的指针标记：只存在于指针方法集，运行时据此
// 区分「结构体指针」与「结构体值」入参（Marshal/Unmarshal 要求指针，
// WireSize/MarshalInto/Freeze 两者皆可）。
type genPtrMarker interface {
	CodecGenPtr()
}

// genValidator 只由布局非法的生成类型实现：返回构建期校验错误，
// 供 Register 启动期 fail-fast（错误信息带类型与字段名）。
type genValidator interface {
	CodecValidate() error
}

// ---------------------------------------------------------------------------
// 生成代码使用的错误构造助手（codecgen 产物调用）
// ---------------------------------------------------------------------------

// TruncErr 构造边界错误（offset/need/have 定位）。
func TruncErr(st, field string, off, need, have int) *Error {
	return &Error{Op: "unmarshal", Struct: st, Field: field, Offset: off, Need: need, Have: have}
}

// FieldErr 构造带补充信息的字段错误。
func FieldErr(op, st, field string, off int, msg string) *Error {
	return &Error{Op: op, Struct: st, Field: field, Offset: off, Msg: msg}
}

// LayoutErr 是生成代码回呈布局校验错误的助手（CodecValidate 与各 stub）。
func LayoutErr(op, st, field, msg string) *Error {
	return &Error{Op: op, Struct: st, Field: field, Msg: msg}
}

// BadLenErr 构造非法计数值错误（负值或超出 1<<24 上限）。
func BadLenErr(st, field string, off int, n int64) *Error {
	return FieldErr("unmarshal", st, field, off, fmt.Sprintf("长度字段值为 %d，非法", n))
}

// CountOverflowErr 构造 count 回填溢出错误（与生成器校验的宽度对应）。
func CountOverflowErr(st, field, countName string, n, width int, signed bool) *Error {
	if signed {
		return FieldErr("marshal", st, field, 0, fmt.Sprintf("len=%d 超出长度字段的表示范围", n))
	}
	return FieldErr("marshal", st, field, 0, fmt.Sprintf("len=%d 超出长度字段 %s 的表示范围", n, countName))
}

// WireLenOverflowErr 构造线宽超出 WireLen 宽度错误。
func WireLenOverflowErr(st, field string, n, width int) *Error {
	return FieldErr("marshal", st, field, 0, fmt.Sprintf("线宽 %d 超出 WireLen 宽度（%d 字节）的表示范围", n, width))
}

// CapacityErr 构造输出缓冲容量不足错误（MarshalInto / 值编码路径）。
func CapacityErr(st string, need, have int) *Error {
	return &Error{Op: "marshal", Struct: st, Offset: 0, Need: need, Have: have}
}

// WireLenMismatchErr 构造 WireLen 与定长结构体线宽不符错误。
func WireLenMismatchErr(st, field string, got, want int) *Error {
	return FieldErr("unmarshal", st, field, 0, fmt.Sprintf("WireLen=%d 与结构体线宽 %d 不符", got, want))
}

// WireLenNotMultipleErr 构造 WireLen 非元素线宽整数倍错误。
func WireLenNotMultipleErr(st, field string, n, elem int) *Error {
	return FieldErr("unmarshal", st, field, 0, fmt.Sprintf("长度 %d 不是元素线宽 %d 的整数倍", n, elem))
}

// WrapErr 给嵌套结构体调用返回的错误叠加外层定位信息：Struct 置为外层
// 结构体名、Field 加 "前缀."、Offset 平移 delta（生成代码在每次嵌套调用
// 处使用，使顶层看到的定位与手写单函数等价）。
func WrapErr(err error, st, prefix string, delta int) error {
	e, ok := err.(*Error)
	if !ok {
		return err
	}
	cp := *e
	cp.Struct = st
	if cp.Field != "" {
		cp.Field = prefix + "." + cp.Field
	} else {
		cp.Field = prefix
	}
	cp.Offset += delta
	return &cp
}

// ---------------------------------------------------------------------------
// Error：带定位信息的编解码失败
// ---------------------------------------------------------------------------

// Error 描述一次编解码失败的具体位置：哪个结构体、哪个字段、消息内偏移、
// 需要/可用字节数。信息形如：
//
//	codec: unmarshal MSG_SELECT_CARD.Cards: offset 12 need 8 have 3
type Error struct {
	Op     string // "marshal" 或 "unmarshal"
	Struct string // 结构体名（运行时入口的顶层结构体名）
	Field  string // 字段名（嵌套时用 "." 连接路径，如 Inner.Code）
	Offset int    // 消息内偏移（Decoder 上为绝对偏移）
	Need   int    // 需要的字节数
	Have   int    // 实际可用的字节数
	Msg    string // 补充信息（类型/tag 非法等）
}

func (err *Error) Error() string {
	head := "codec: " + err.Op
	if err.Struct != "" {
		head += " " + err.Struct
	}
	if err.Field != "" {
		head += "." + err.Field
	}
	if err.Msg != "" {
		return head + ": " + err.Msg
	}
	return fmt.Sprintf("%s: offset %d need %d have %d", head, err.Offset, err.Need, err.Have)
}

// ---------------------------------------------------------------------------
// 公共入口：接口分发到生成代码
// ---------------------------------------------------------------------------

// typeNameOf 取 %T 表示并去掉指针星号（错误信息用，零反射）。
func typeNameOf(v any) string {
	s := fmt.Sprintf("%T", v)
	return strings.TrimPrefix(s, "*")
}

// 需要指针入参的入口共用的取方法逻辑：非生成类型、未生成类型、非指针
// 三种失败分别给出带类型名的错误。
func ptrCodec(v any, op string) (genSizer, genMarshaler, genUnmarshaler, error) {
	if v == nil {
		return nil, nil, nil, &Error{Op: op, Msg: "需要非 nil 的结构体指针"}
	}
	pm, ok := v.(genPtrMarker)
	if !ok {
		if _, isVal := v.(genValSizer); isVal {
			// 生成类型的结构体值：方法集里没有指针标记，说明传了值。
			return nil, nil, nil, &Error{Op: op,
				Msg: fmt.Sprintf("需要非 nil 的结构体指针，得到 %T（结构体值请传指针）", v)}
		}
		return nil, nil, nil, &Error{Op: op,
			Msg: fmt.Sprintf("类型 %T 没有生成的 codec 方法（请对该包执行 codecgen）", v)}
	}
	_ = pm
	s, ok1 := v.(genSizer)
	m, ok2 := v.(genMarshaler)
	u, ok3 := v.(genUnmarshaler)
	if !ok1 || !ok2 || !ok3 {
		return nil, nil, nil, &Error{Op: op,
			Msg: fmt.Sprintf("类型 %T 的 codec 生成物不完整（重新跑 codecgen）", v)}
	}
	return s, m, u, nil
}

// Marshal 把结构体 v（非 nil 指针）序列化为字节，字节序固定 LittleEndian。
// 先经 CodecSize 算出总大小（count/WireLen 字段被 len/线宽原地回填，副作
// 用发生在调用方的结构体上）再一次分配编码。
func Marshal(v any) ([]byte, error) {
	s, m, _, err := ptrCodec(v, "marshal")
	if err != nil {
		return nil, err
	}
	total, err := s.CodecSize()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, total)
	if n := m.CodecMarshalTo(buf); n != total {
		return nil, &Error{Op: "marshal", Struct: typeNameOf(v),
			Msg: fmt.Sprintf("内部错误：编码后偏移 %d 与预算大小 %d 不一致", n, total)}
	}
	return buf, nil
}

// WireSize 返回 v 的线宽（与 Marshal 产生的字节数一致；指针入参时
// count/WireLen 字段被原地回填，与 Marshal 同语义；结构体值入参在副本上
// 回填，调用方的值不受影响）。调用方据此一次分配后配合 MarshalInto 实现
// 单次分配的编码。v 接受结构体指针或结构体值。
func WireSize(v any) (int, error) {
	if v == nil {
		return 0, &Error{Op: "marshal", Msg: "需要非 nil 的结构体（指针）"}
	}
	if s, ok := v.(genSizer); ok {
		return s.CodecSize()
	}
	if sv, ok := v.(genValSizer); ok {
		return sv.CodecSizeVal()
	}
	return 0, &Error{Op: "marshal",
		Msg: fmt.Sprintf("类型 %T 没有生成的 codec 方法（请对该包执行 codecgen）", v)}
}

// MarshalInto 把 v 编码进调用方提供的 dst，零分配（生成代码直接写入
// dst）。指针入参先走 CodecSize（顺带完成 count/WireLen 回填）再编码；
// 结构体值入参走 CodecEncodeInto（副本上回填 + 编码）。dst 容量不足时
// 返回带 need/have 的 *Error；调用方不需要预知大小。需要独立字节切片的
// 场景用 Marshal，同一负载多次发送的场景用 Freeze。
//
// v 接受结构体指针或结构体值（语义同 WireSize）。
func MarshalInto(dst []byte, v any) (int, error) {
	if v == nil {
		return 0, &Error{Op: "marshal", Msg: "需要非 nil 的结构体（指针）"}
	}
	if s, ok := v.(genSizer); ok {
		total, err := s.CodecSize()
		if err != nil {
			return 0, err
		}
		if total > len(dst) {
			return 0, CapacityErr(typeNameOf(v), total, len(dst))
		}
		m := v.(genMarshaler)
		if n := m.CodecMarshalTo(dst); n != total {
			return 0, &Error{Op: "marshal", Struct: typeNameOf(v),
				Msg: fmt.Sprintf("内部错误：编码后偏移 %d 与预算大小 %d 不一致", n, total)}
		}
		return total, nil
	}
	if sv, ok := v.(genValCoder); ok {
		return sv.CodecEncodeInto(dst)
	}
	return 0, &Error{Op: "marshal",
		Msg: fmt.Sprintf("类型 %T 没有生成的 codec 方法（请对该包执行 codecgen）", v)}
}

// Frozen 是序列化一次的不可变负载：Freeze 时编码一次，之后 Bytes/Len
// 直接取字节，零序列化开销。典型用途是同一负载多次发送（扇出/周期包）：
// 构造一次 Freeze，ReSend 循环里只 copy 线字节。
//
// 不可变性由值语义 + 只读约定保证：Frozen 按值传递/存储，Bytes 返回的
// 切片引用内部缓冲，调用方不得修改其内容（文档约定，不强制拷贝）。
type Frozen struct {
	wire []byte
}

// Freeze 序列化 v 为不可变负载；失败返回 *Error。
func Freeze(v any) (Frozen, error) {
	b, err := Marshal(v)
	if err != nil {
		return Frozen{}, err
	}
	return Frozen{wire: b}, nil
}

// MustFreeze 是 Freeze 的 panic 版，供 init/启动期使用。
func MustFreeze(v any) Frozen {
	f, err := Freeze(v)
	if err != nil {
		panic("codec: MustFreeze: " + err.Error())
	}
	return f
}

// Bytes 返回负载线字节（只读约定：不得修改返回的切片）。
func (frozen Frozen) Bytes() []byte { return frozen.wire }

// Len 返回负载线字节数。
func (frozen Frozen) Len() int { return len(frozen.wire) }

// unmarshalFrom 是解码入口的公共尾部：v 必须实现生成解码接口（即非 nil
// 的结构体指针），返回消费字节数。
func unmarshalFrom(v any, data []byte, op string) (int, error) {
	_, _, u, err := ptrCodec(v, op)
	if err != nil {
		return 0, err
	}
	return u.CodecUnmarshalFrom(data)
}

// DecodePrefix 从 data 头部解析一个结构体，返回消费的字节数（供游标推进）。
// 错误语义同 Unmarshal；count/greedy 结构体的消费长度由内容决定，
// 尾部多余字节不计入消费。
func DecodePrefix(data []byte, v any) (int, error) {
	return unmarshalFrom(v, data, "unmarshal")
}

// Unmarshal 把 data 解析进 v（非 nil 结构体指针）。任何截断/类型不符都返回
// 带上下文的 *Error。允许 data 尾部有多余字节。
func Unmarshal(data []byte, v any) error {
	_, err := unmarshalFrom(v, data, "unmarshal")
	return err
}

// Decoder 增量游标，用于一条连接上连续多条消息。
type Decoder struct {
	data []byte
	pos  int
}

func NewDecoder(data []byte) *Decoder { return &Decoder{data: data} }

// Pos 返回当前游标位置。
func (dec *Decoder) Pos() int { return dec.pos }

// Remaining 返回剩余字节数。
func (dec *Decoder) Remaining() int { return len(dec.data) - dec.pos }

// Rest 返回剩余字节（视图，不是拷贝）。
func (dec *Decoder) Rest() []byte { return dec.data[dec.pos:] }

// Decode 从当前游标解析一个结构体；成功才推进游标，失败时游标保持不动
// （幂等可重试），但 v 可能已被部分写入。错误里的 Offset 为流内绝对偏移。
func (dec *Decoder) Decode(v any) error {
	n, err := unmarshalFrom(v, dec.data[dec.pos:], "unmarshal")
	if err != nil {
		if e, ok := err.(*Error); ok {
			cp := *e
			cp.Offset += dec.pos
			err = &cp
		}
		return err
	}
	dec.pos += n
	return nil
}

// Encoder 增量构造：连续 Encode 多个结构体拼成一条字节流。
type Encoder struct {
	buf []byte
}

func NewEncoder() *Encoder { return &Encoder{} }

// Encode 把 v 追加到内部缓冲区。
func (enc *Encoder) Encode(v any) error {
	b, err := Marshal(v)
	if err != nil {
		return err
	}
	enc.buf = append(enc.buf, b...)
	return nil
}

// Bytes 返回已编码的全部字节（视图，不是拷贝）。
func (enc *Encoder) Bytes() []byte { return enc.buf }

// Register 在启动期校验给定结构体的生成代码：样本必须是非 nil 结构体
// 指针且实现了生成接口；布局非法的生成类型（带 CodecValidate）在此报出
// 构建期校验错误，而不是第一条消息到达时才暴露。重复注册同一类型幂等。
// 未生成的类型返回带类型名的错误。返回所有样本的错误（errors.Join 聚合，
// nil 表示全部成功）。
//
// 典型用法（main 或包的 init 里）：
//
//	func init() {
//	    codec.MustRegister(&protocol.SelectCardMsg{}, &protocol.MoveMsg{})
//	}
func Register(samples ...any) error {
	var errs []error
	for _, sample := range samples {
		if sample == nil {
			errs = append(errs, LayoutErr("register", "", "", "nil 样本"))
			continue
		}
		if _, _, u, err := ptrCodec(sample, "register"); err != nil {
			errs = append(errs, err)
			continue
		} else if iv, ok := u.(genValidator); ok {
			if err := iv.CodecValidate(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// MustRegister 是 Register 的 panic 版，供 init() 使用：注册失败说明
// 布局/tag 写错了或忘了生成，属于编程错误，应当启动即崩。
func MustRegister(samples ...any) {
	if err := Register(samples...); err != nil {
		panic("codec: MustRegister: " + err.Error())
	}
}
