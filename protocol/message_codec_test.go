package protocol

// YGOBuffer.Unpack（codec 计划链）与 restruct 直调的一致性护栏：
//   - 同一份黄金 wire 用例表（message_test.go 的 cases），codec 解码结果与
//     restruct 基线 DeepEqual；
//   - 游标推进 == 线长（count 变长结构按实际消费推进，不多不少）；
//   - codec 编码（count 回填语义）回到 wire 与 restruct Pack 全等。

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/core/utils"
)

func TestMessageYGOBufferUnpackMatchesRestruct(t *testing.T) {
	for _, tc := range goldenRoundTripCases {
		t.Run(tc.name, func(t *testing.T) {
			// restruct 基线（既有语义）：带 count 列表的消息走影子结构体，
			// 结果按同名拷贝回 protocol 结构体再比较
			base := tc.factory()
			rt := restructBaseline(tc.name, base)
			if err := restruct.Unpack(tc.wire, binary.LittleEndian, rt); err != nil {
				t.Fatalf("restruct 基线 Unpack: %v", err)
			}
			if rt != base {
				copyBaseline(base, rt)
			}
			// codec 路径：YGOBuffer.Unpack
			msg := tc.factory()
			buf := utils.NewYGOBuffer(tc.wire, binary.LittleEndian)
			if err := buf.Unpack(msg); err != nil {
				t.Fatalf("YGOBuffer.Unpack: %v", err)
			}
			if !reflect.DeepEqual(msg, base) {
				t.Fatalf("codec 解码与 restruct 基线不一致:\n codec   = %+v\n restruct= %+v", msg, base)
			}
			if off := buf.Offset(); off != len(tc.wire) {
				t.Fatalf("游标消费 %d 字节 != 线长 %d（推进不足会错位后续消息）", off, len(tc.wire))
			}
			if buf.Overflowed() {
				t.Fatal("不应置粘性越界标记")
			}
			// 编码侧：codec Marshal（count 回填）与 restruct Pack 都回到 wire
			packed, err := codec.Marshal(msg)
			if err != nil {
				t.Fatalf("codec.Marshal: %v", err)
			}
			if !bytes.Equal(packed, tc.wire) {
				t.Fatalf("codec.Marshal != wire:\n wire= %v\n got = %v", tc.wire, packed)
			}
		})
	}
}

// TestDecodePrefixVariableLength：变长结构（SelectCardMsg）的 DecodePrefix
// 消费恰好等于实际线长；尾部多余字节不算消费。
func TestDecodePrefixVariableLength(t *testing.T) {
	wire := concat([]byte{0, 1, 1, 2, 2},
		le32(12345), []byte{0, 4, 1, 1},
		le32(678), []byte{1, 2, 0, 2})
	var msg SelectCardMsg
	n, err := codec.DecodePrefix(wire, &msg)
	if err != nil {
		t.Fatalf("DecodePrefix: %v", err)
	}
	if n != len(wire) {
		t.Fatalf("消费 %d != 线长 %d", n, len(wire))
	}
	// 尾部垃圾不影响消费长度
	n, err = codec.DecodePrefix(append(wire, 0xAA, 0xBB), &msg)
	if err != nil {
		t.Fatalf("DecodePrefix(带尾部): %v", err)
	}
	if n != len(wire) {
		t.Fatalf("带尾部时消费 %d != 线长 %d", n, len(wire))
	}
}

// BenchmarkYGOBufferUnpack：codec 计划链 vs 旧 restruct 反射解码。
// 负载为 10 字段头 + 2 条目的 SelectCardMsg（变长 count 列表）。
func BenchmarkYGOBufferUnpack(b *testing.B) {
	wire := concat([]byte{0, 1, 1, 2, 2},
		le32(89631139), []byte{0, 4, 1, 1},
		le32(24094653), []byte{1, 4, 0, 2})
	b.Run("codec", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := utils.NewYGOBuffer(wire, binary.LittleEndian)
			var msg SelectCardMsg
			if err := buf.Unpack(&msg); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("restruct", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var msg rtSelectCardMsg
			if err := restruct.Unpack(wire, binary.LittleEndian, &msg); err != nil {
				b.Fatal(err)
			}
			if _, err := restruct.SizeOf(&msg); err != nil {
				b.Fatal(err)
			}
		}
	})
}
