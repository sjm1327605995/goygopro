package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"

	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// ------------------------------------------------------------------
// codec.Frame / ctosFrameHeader（cmd/wails/duel_client.go 客户端帧线格式）
//
// 线格式：2 字节小端 Len + 1 字节 Proto + greedy Payload。Len 含 proto
// 字节本身（= 1+len(Payload)），是 ygopro 系 CTOS/STOC 帧的 C++ 布局
// （bufferio 风格），与 STOC 入站 readLoop 的 codec.FrameLen 对应。
// ------------------------------------------------------------------

// 用 binary.LittleEndian 独立构造期望帧（不复用生产编码路径）。
func buildCtosFrameBytes(lenField uint16, proto byte, payload []byte) []byte {
	buf := binary.LittleEndian.AppendUint16(nil, lenField)
	buf = append(buf, proto)
	buf = append(buf, payload...)
	return buf
}

// TestCtosFrameMarshalGoldenBytes：代表性取值的 Marshal 结果与手工构造
// 的黄金字节逐字节比对；同时锁定 Len = 1+len(Payload) 的线上语义
// （首两字节恰为 1+len 的小端值）与空 Payload 边界。
func TestCtosFrameMarshalGoldenBytes(t *testing.T) {
	cases := []struct {
		name     string
		lenField uint16
		proto    byte
		payload  []byte
	}{
		{"空负载", 1, 0x01, []byte{}},                                 // CTOS_RESPONSE
		{"空负载_nil", 1, 0x22, nil},                                  // CTOS_HS_READY，无数据
		{"四字节负载", 5, 0x03, []byte{0xde, 0xad, 0xbe, 0xef}},         // CTOS_HAND_RESULT
		{"跨 255 边界负载", 257, 0x02, bytes.Repeat([]byte{0x5a}, 256)}, // CTOS_UPDATE_DECK
		{"Len 与负载无关时原样写出", 0xffff, 0x04, []byte{1, 2}},             // Len 不被重算
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := codec.Marshal(&codec.Frame{Len: tc.lenField, Proto: tc.proto, Payload: tc.payload})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			want := buildCtosFrameBytes(tc.lenField, tc.proto, tc.payload)
			if !bytes.Equal(got, want) {
				t.Fatalf("marshal 字节不符\n got=% x\nwant=% x", got, want)
			}
		})
	}

	// Len = 1+len(Payload) 语义：按契约构造的一批帧，首两字节必须恰为
	// 1+len(Payload) 的小端表示。
	for _, n := range []int{0, 1, 2, 4, 127, 255, 256, 1024} {
		payload := bytes.Repeat([]byte{0xab}, n)
		got, err := codec.Marshal(&codec.Frame{Len: uint16(1 + n), Proto: 0x01, Payload: payload})
		if err != nil {
			t.Fatalf("marshal n=%d: %v", n, err)
		}
		wantLen := binary.LittleEndian.AppendUint16(nil, uint16(1+n))
		if !bytes.Equal(got[:2], wantLen) {
			t.Fatalf("n=%d: Len 字段字节 % x，期望 % x", n, got[:2], wantLen)
		}
		if len(got) != 3+n {
			t.Fatalf("n=%d: 总帧长 %d，期望 %d", n, len(got), 3+n)
		}
	}
}

// TestCtosFrameRoundTrip：Marshal→Unmarshal→DeepEqual。
func TestCtosFrameRoundTrip(t *testing.T) {
	in := &codec.Frame{Len: 7, Proto: 0x10, Payload: []byte{1, 2, 3, 4, 5, 6}}
	buf, err := codec.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out codec.Frame
	if err := codec.Unmarshal(buf, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, &out) {
		t.Fatalf("往返不等\n in=%+v\nout=%+v", in, &out)
	}

	// 空负载边界：解码出 0 长 Payload，头部字段原样。
	empty := &codec.Frame{Len: 1, Proto: 0x22, Payload: []byte{}}
	buf, err = codec.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal 空负载: %v", err)
	}
	var outEmpty codec.Frame
	if err := codec.Unmarshal(buf, &outEmpty); err != nil {
		t.Fatalf("unmarshal 空负载: %v", err)
	}
	if outEmpty.Len != 1 || outEmpty.Proto != 0x22 || len(outEmpty.Payload) != 0 {
		t.Fatalf("空负载往返错: %+v", outEmpty)
	}
}

// TestCtosFrameHeaderRoundTrip：2 字节小端头 Marshal→Unmarshal→DeepEqual。
func TestCtosFrameHeaderRoundTrip(t *testing.T) {
	for _, n := range []uint16{0, 1, 2, 0x1234, 0xffff} {
		in := &codec.FrameLen{Len: n}
		buf, err := codec.Marshal(in)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if want := binary.LittleEndian.AppendUint16(nil, n); !bytes.Equal(buf, want) {
			t.Fatalf("Len=%d: 头字节 % x，期望 % x", n, buf, want)
		}
		var out codec.FrameLen
		if err := codec.Unmarshal(buf, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !reflect.DeepEqual(in, &out) {
			t.Fatalf("Len=%d: 往返不等 %+v vs %+v", n, in, out)
		}
	}
}

// TestCtosFrameUnmarshalTruncation：截断输入返回 *codec.Error 且不 panic。
// codec.Frame 在 3 字节头内截断才报错（greedy Payload 吃掉剩余全部字节）；
// codec.FrameLen 在其 2 字节内截断（1 字节输入）报错。
func TestCtosFrameUnmarshalTruncation(t *testing.T) {
	assertCodecErr := func(name string, data []byte, target any) {
		t.Helper()
		err := codec.Unmarshal(data, target)
		var ce *codec.Error
		if !errors.As(err, &ce) {
			t.Fatalf("%s: 期望 *codec.Error，得到 %v", name, err)
		}
	}

	for _, data := range [][]byte{{}, {0x01}, {0x01, 0x00}} {
		assertCodecErr("codec.Frame 截断", data, &codec.Frame{})
	}
	// codec.FrameLen 的 2 字节截断：只给 1 字节。
	for _, data := range [][]byte{{}, {0x01}} {
		assertCodecErr("codec.FrameLen 截断", data, &codec.FrameLen{})
	}

	// greedy 语义锁定：Len 与剩余字节数无关，尾部全部被 Payload 吸收，
	// 3 字节及以上的输入不会报截断。
	var f codec.Frame
	if err := codec.Unmarshal([]byte{0x01, 0x00, 0x02, 0xaa, 0xbb, 0xcc, 0xdd}, &f); err != nil {
		t.Fatalf("greedy 吸收不应报错: %v", err)
	}
	if f.Len != 1 || f.Proto != 0x02 || !bytes.Equal(f.Payload, []byte{0xaa, 0xbb, 0xcc, 0xdd}) {
		t.Fatalf("greedy 语义不符: %+v", f)
	}
}

// ------------------------------------------------------------------
// queryCardWire（cmd/wails/engine_bindings.go）补强测试。
//
// flag 位与 ocgcore/common.go 的 QUERY_* 一一对应（card.cpp get_infos
// 的 if 顺序），补充 engine_bindings_test.go 已有 3 个测试的缺口：
// 逐位独立置位/复位的线级断言、全 flag 大往返、Marshal 黄金字节。
// ------------------------------------------------------------------

func u32ptr(v uint32) *uint32 { return &v }
func i32ptr(v int32) *int32   { return &v }

// packLocRef 独立构造打包 info_location（c | l<<8 | s<<16 | p<<24），
// 与 protocol.UnpackPackedLoc/newLocPosRef 相对照。
func packLocRef(c, l, s, p uint8) uint32 {
	return uint32(c) | uint32(l)<<8 | uint32(s)<<16 | uint32(p)<<24
}

// TestQueryCardWireGoldenBytes：Marshal 的黄金字节。flag=QUERY_CODE 时
// 线上恰好 8 字节（flag + 一个 u32）；计数列表的 count 由 len(slice)
// 回填（调用方填错的 count 被覆盖）；LINK 占两个连续 u32。
func TestQueryCardWireGoldenBytes(t *testing.T) {
	// Flag=QUERY_CODE：恰好 8 字节。
	buf, err := codec.Marshal(&queryCardWire{Flag: ocgcore.QUERY_CODE, Code: u32ptr(89631139)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := binary.LittleEndian.AppendUint32(nil, ocgcore.QUERY_CODE)
	want = binary.LittleEndian.AppendUint32(want, 89631139)
	if !bytes.Equal(buf, want) {
		t.Fatalf("QUERY_CODE 黄金字节不符\n got=% x\nwant=% x", buf, want)
	}
	if len(buf) != 8 {
		t.Fatalf("Flag=0x01 应恰好 8 字节，得到 %d", len(buf))
	}

	// Flag=0：仅 4 字节 flag，无任何字段。
	buf, err = codec.Marshal(&queryCardWire{})
	if err != nil {
		t.Fatalf("marshal 空 flag: %v", err)
	}
	if want := []byte{0, 0, 0, 0}; !bytes.Equal(buf, want) {
		t.Fatalf("Flag=0 黄金字节不符: % x", buf)
	}

	// TARGET 列表：TargetCount 指针填 9（错值），Marshal 必须按
	// len(Targets)=2 回填成 2。
	targets := []uint32{packLocRef(0, 4, 0, 1), packLocRef(1, 4, 1, 2)}
	buf, err = codec.Marshal(&queryCardWire{
		Flag:        ocgcore.QUERY_TARGET_CARD,
		TargetCount: u32ptr(9),
		Targets:     &targets,
	})
	if err != nil {
		t.Fatalf("marshal TARGET: %v", err)
	}
	want = binary.LittleEndian.AppendUint32(nil, ocgcore.QUERY_TARGET_CARD)
	want = binary.LittleEndian.AppendUint32(want, 2)
	for _, loc := range targets {
		want = binary.LittleEndian.AppendUint32(want, loc)
	}
	if !bytes.Equal(buf, want) {
		t.Fatalf("TARGET 黄金字节不符（count 未按 len 回填）\n got=% x\nwant=% x", buf, want)
	}

	// LINK：link + link_marker 两个连续 u32。
	buf, err = codec.Marshal(&queryCardWire{
		Flag:   ocgcore.QUERY_LINK,
		Link:   u32ptr(1),
		Marker: u32ptr(0x101),
	})
	if err != nil {
		t.Fatalf("marshal LINK: %v", err)
	}
	want = binary.LittleEndian.AppendUint32(nil, ocgcore.QUERY_LINK)
	want = binary.LittleEndian.AppendUint32(want, 1)
	want = binary.LittleEndian.AppendUint32(want, 0x101)
	if !bytes.Equal(buf, want) {
		t.Fatalf("LINK 黄金字节不符\n got=% x\nwant=% x", buf, want)
	}
}

// TestQueryCardWireFlagBitIndependence：未在 engine_bindings_test.go 覆盖的
// 每个 flag 位——置位时线上恰为 flag+u32 并往返一致；指针 nil 时字段不占
// 字节；Flag=0 解码字段全 nil。位置类字段（REASON_CARD/EQUIP_CARD）另验证
// newLocPosRef 展开的等价性。
func TestQueryCardWireFlagBitIndependence(t *testing.T) {
	// packed info_location 值（c=0,l=4,s=1,p=2）。
	reasonLoc := packLocRef(0, 4, 1, 2)
	type bitCase struct {
		name     string
		bit      uint32 // ocgcore QUERY_* 位
		build    func() *queryCardWire
		wireVal  uint32 // 字段的线上 u32 值
		dtoCheck func(t *testing.T, d *queryCardDTO)
	}
	cases := []bitCase{
		{"ALIAS", ocgcore.QUERY_ALIAS, func() *queryCardWire {
			return &queryCardWire{Alias: u32ptr(4007)}
		}, 4007, nil},
		{"TYPE", ocgcore.QUERY_TYPE, func() *queryCardWire {
			return &queryCardWire{Type: u32ptr(0x21)}
		}, 0x21, nil},
		{"LEVEL", ocgcore.QUERY_LEVEL, func() *queryCardWire {
			return &queryCardWire{Level: u32ptr(12 | 1<<24 | 4<<16)} // 低 16 位等级，高位灵摆刻度
		}, 12 | 1<<24 | 4<<16, nil},
		{"RANK", ocgcore.QUERY_RANK, func() *queryCardWire {
			return &queryCardWire{Rank: u32ptr(5)}
		}, 5, nil},
		{"ATTRIBUTE", ocgcore.QUERY_ATTRIBUTE, func() *queryCardWire {
			return &queryCardWire{Attribute: u32ptr(0x20)}
		}, 0x20, nil},
		{"RACE", ocgcore.QUERY_RACE, func() *queryCardWire {
			return &queryCardWire{Race: u32ptr(0x8000)}
		}, 0x8000, nil},
		{"BASE_ATTACK", ocgcore.QUERY_BASE_ATTACK, func() *queryCardWire {
			return &queryCardWire{BaseAttack: i32ptr(-500)}
		}, 0xfffffe0c, nil}, // -500 的补码
		{"BASE_DEFENSE", ocgcore.QUERY_BASE_DEFENSE, func() *queryCardWire {
			return &queryCardWire{BaseDefense: i32ptr(0)}
		}, 0, nil},
		{"REASON", ocgcore.QUERY_REASON, func() *queryCardWire {
			return &queryCardWire{Reason: u32ptr(0x400)}
		}, 0x400, nil},
		{"REASON_CARD", ocgcore.QUERY_REASON_CARD, func() *queryCardWire {
			return &queryCardWire{ReasonCard: u32ptr(reasonLoc)}
		}, reasonLoc, func(t *testing.T, d *queryCardDTO) {
			if d.ReasonCard == nil || *d.ReasonCard != (locPosRefDTO{C: 0, L: 4, S: 1, P: 2}) {
				t.Fatalf("ReasonCard newLocPosRef 展开不符: %+v", d.ReasonCard)
			}
		}},
		{"EQUIP_CARD", ocgcore.QUERY_EQUIP_CARD, func() *queryCardWire {
			return &queryCardWire{EquipCard: u32ptr(reasonLoc)}
		}, reasonLoc, func(t *testing.T, d *queryCardDTO) {
			if d.EquipCard == nil || *d.EquipCard != (locPosRefDTO{C: 0, L: 4, S: 1, P: 2}) {
				t.Fatalf("EquipCard newLocPosRef 展开不符: %+v", d.EquipCard)
			}
		}},
		{"OWNER", ocgcore.QUERY_OWNER, func() *queryCardWire {
			return &queryCardWire{Owner: i32ptr(1)}
		}, 1, nil},
		{"STATUS", ocgcore.QUERY_STATUS, func() *queryCardWire {
			return &queryCardWire{Status: u32ptr(0x200)}
		}, 0x200, nil},
		{"LSCALE", ocgcore.QUERY_LSCALE, func() *queryCardWire {
			return &queryCardWire{LScale: u32ptr(8)}
		}, 8, nil},
		{"RSCALE", ocgcore.QUERY_RSCALE, func() *queryCardWire {
			return &queryCardWire{RScale: u32ptr(10)}
		}, 10, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 置位：线上恰为 flag+u32，黄金字节比对 + 往返 DeepEqual。
			w := tc.build()
			w.Flag = tc.bit
			buf, err := codec.Marshal(w)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			want := binary.LittleEndian.AppendUint32(nil, tc.bit)
			want = binary.LittleEndian.AppendUint32(want, tc.wireVal)
			if !bytes.Equal(buf, want) {
				t.Fatalf("置位黄金字节不符\n got=% x\nwant=% x", buf, want)
			}
			var back queryCardWire
			if err := codec.Unmarshal(buf, &back); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(w, &back) {
				t.Fatalf("置位往返不等\n in=%+v\nout=%+v", w, &back)
			}

			// 复位：flag 置位但字段指针为 nil——Marshal 必须跳过该字段，
			// 线上只剩 4 字节 flag。（codec 序列化方向只看指针 nil 与否，
			// 不做谓词校验，见 core/codec 文档。）
			buf, err = codec.Marshal(&queryCardWire{Flag: tc.bit})
			if err != nil {
				t.Fatalf("marshal 复位: %v", err)
			}
			if want := binary.LittleEndian.AppendUint32(nil, tc.bit); !bytes.Equal(buf, want) {
				t.Fatalf("nil 字段不应占字节，得到 % x", buf)
			}

			// 复位解码：全 nil 字段。
			var zero queryCardWire
			if err := codec.Unmarshal(make([]byte, 4), &zero); err != nil {
				t.Fatalf("unmarshal 空 flag: %v", err)
			}
			if zero.Code != nil || zero.Position != nil || zero.Targets != nil ||
				zero.Link != nil || zero.Marker != nil || zero.Counters != nil {
				t.Fatalf("复位解码应全 nil: %+v", zero)
			}

			if tc.dtoCheck != nil {
				tc.dtoCheck(t, back.toDTO())
			}
		})
	}
}

// TestQueryCardWireAllFlagsRoundTrip：全部 QUERY_* 位同时置位的大往返。
// 锁定「字段顺序即 card.cpp get_infos 的 if 顺序」在组合场景下不串位
// （位置类打包值、负数 int32、计数列表、LINK 双子段混在一起）。
func TestQueryCardWireAllFlagsRoundTrip(t *testing.T) {
	const allFlags = ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION | ocgcore.QUERY_ALIAS |
		ocgcore.QUERY_TYPE | ocgcore.QUERY_LEVEL | ocgcore.QUERY_RANK |
		ocgcore.QUERY_ATTRIBUTE | ocgcore.QUERY_RACE | ocgcore.QUERY_ATTACK |
		ocgcore.QUERY_DEFENSE | ocgcore.QUERY_BASE_ATTACK | ocgcore.QUERY_BASE_DEFENSE |
		ocgcore.QUERY_REASON | ocgcore.QUERY_REASON_CARD | ocgcore.QUERY_EQUIP_CARD |
		ocgcore.QUERY_TARGET_CARD | ocgcore.QUERY_OVERLAY_CARD | ocgcore.QUERY_COUNTERS |
		ocgcore.QUERY_OWNER | ocgcore.QUERY_STATUS | ocgcore.QUERY_LSCALE |
		ocgcore.QUERY_RSCALE | ocgcore.QUERY_LINK

	targets := []uint32{packLocRef(0, 4, 0, 1), packLocRef(1, 4, 2, 3)}
	overlays := []uint32{111, 222, 333}
	// COUNTERS 线上每条是 type | count<<16（card.cpp counter 段）。
	counters := []uint32{0xffff | 1<<16, 0x0001 | 0x1234<<16}

	in := &queryCardWire{
		Flag:        allFlags,
		Code:        u32ptr(89631139),
		Position:    u32ptr(packLocRef(1, 0x10, 2, 1)), // get_info_location 含灵摆覆盖形态
		Alias:       u32ptr(89631140),
		Type:        u32ptr(0x21),
		Level:       u32ptr(12 | 1<<24 | 4<<16),
		Rank:        u32ptr(5),
		Attribute:   u32ptr(0x20),
		Race:        u32ptr(0x8000),
		Attack:      i32ptr(-100),
		Defense:     i32ptr(150),
		BaseAttack:  i32ptr(3000),
		BaseDefense: i32ptr(-1),
		Reason:      u32ptr(0x400),
		ReasonCard:  u32ptr(packLocRef(0, 4, 1, 0)),
		EquipCard:   u32ptr(packLocRef(1, 8, 0, 2)),
		TargetCount: u32ptr(uint32(len(targets))),
		Targets:     &targets,
		OvlCount:    u32ptr(uint32(len(overlays))),
		Overlays:    &overlays,
		CtrCount:    u32ptr(uint32(len(counters))),
		Counters:    &counters,
		Owner:       i32ptr(1),
		Status:      u32ptr(0x200),
		LScale:      u32ptr(8),
		RScale:      u32ptr(10),
		Link:        u32ptr(3),
		Marker:      u32ptr(0xbf),
	}

	buf, err := codec.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out queryCardWire
	if err := codec.Unmarshal(buf, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, &out) {
		t.Fatalf("全 flag 往返不等\n in=%+v\nout=%+v", in, &out)
	}

	// DTO 侧等价性：计数器按 type|count<<16 拆开、位置类展开。
	d := out.toDTO()
	if len(d.Counters) != 2 || d.Counters[0].Type != 0xffff || d.Counters[0].Count != 1 ||
		d.Counters[1].Type != 1 || d.Counters[1].Count != 0x1234 {
		t.Fatalf("COUNTERS 拆包错: %+v", d.Counters)
	}
	if d.Position == nil || *d.Position != (locPosRefDTO{C: 1, L: 0x10, S: 2, P: 1}) {
		t.Fatalf("Position 展开错: %+v", d.Position)
	}
	if d.Targets == nil || len(d.Targets) != 2 || *d.Targets[1] != (locPosRefDTO{C: 1, L: 4, S: 2, P: 3}) {
		t.Fatalf("Targets 展开错: %+v", d.Targets)
	}
}

// TestQueryCardWireCountersUnpack：COUNTERS（0x20000）多条目与边界值的
// type|count<<16 拆包。engine_bindings_test.go 已覆盖单条常规值，
// 这里补 0xffff 满宽 type 与多计数。
func TestQueryCardWireCountersUnpack(t *testing.T) {
	// QUERY_COUNTERS（ocgcore/common.go）：count=2，两条 counter。
	body := binary.LittleEndian.AppendUint32(nil, ocgcore.QUERY_COUNTERS)
	body = binary.LittleEndian.AppendUint32(body, 2)
	body = binary.LittleEndian.AppendUint32(body, 0xffff|7<<16) // type=0xffff, count=7
	body = binary.LittleEndian.AppendUint32(body, 1|0xffff<<16) // type=1, count=0xffff

	var w queryCardWire
	if err := codec.Unmarshal(body, &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	d := w.toDTO()
	if d.Counters == nil || len(d.Counters) != 2 {
		t.Fatalf("COUNTERS 解码错: %+v", d.Counters)
	}
	if d.Counters[0].Type != 0xffff || d.Counters[0].Count != 7 ||
		d.Counters[1].Type != 1 || d.Counters[1].Count != 0xffff {
		t.Fatalf("COUNTERS 拆包错: %+v", d.Counters)
	}
}
