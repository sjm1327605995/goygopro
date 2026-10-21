package utils

import (
	"encoding/binary"
	"fmt"

	"github.com/sjm1327605995/goygopro/core/codec"
)

type YGOBuffer struct {
	buff   []byte
	offset int
	order  binary.ByteOrder
	// overflow 是粘性越界标记：任一次 Read/Next/Unpack/ReadNext 越界、
	// 或 At 越界访问时置位，此后 Overflowed() 返回 true（Clone 会继承）。
	// 解析引擎消息时由 runAnalyze 在 handler 返回后检查，把"字段读取失败被
	// 吞掉后继续解析"（缓冲区错位、发出损坏数据）变为"记日志并终止本局"。
	overflow bool
}

func NewYGOBuffer(buff []byte, order binary.ByteOrder) *YGOBuffer {
	return &YGOBuffer{
		buff:  buff,
		order: order,
	}
}
func (buf *YGOBuffer) Offset() int {
	return buf.offset
}
func (buf *YGOBuffer) Read(list ...any) error {
	for i := range list {

		n, err := binary.Decode(buf.buff[buf.offset:], buf.order, list[i])
		if err != nil {
			buf.overflow = true
			return err
		}
		buf.offset += n
	}
	return nil
}

// Overflowed 报告缓冲区自构造/克隆以来是否发生过越界访问。
func (buf *YGOBuffer) Overflowed() bool {
	return buf.overflow
}

// At 返回当前 offset 后第 pos 个字节；越界时置粘性越界标记并返回 0
// （不 panic），由调用方在解析结束后统一检查 Overflowed。
func (buf *YGOBuffer) At(pos int) byte {
	idx := buf.offset + pos
	if idx < 0 || idx >= len(buf.buff) {
		buf.overflow = true
		return 0
	}
	return buf.buff[idx]
}
func (buf *YGOBuffer) Write(list ...any) error {
	for i := range list {
		n, err := binary.Encode(buf.buff[buf.offset:], buf.order, list[i])
		if err != nil {
			return err
		}
		buf.offset += n
	}
	return nil
}
func (buf *YGOBuffer) Len() int {
	return len(buf.buff[buf.offset:])
}

func (buf *YGOBuffer) Bytes() []byte {
	return buf.buff[buf.offset:]
}

func (buf *YGOBuffer) Next(n int) error {
	if buf.offset+n > len(buf.buff) {
		buf.overflow = true
		return fmt.Errorf("out of range")
	}
	buf.offset += n
	return nil
}
func (buf *YGOBuffer) Clone() *YGOBuffer {
	return &YGOBuffer{
		buff:     buf.buff,
		order:    buf.order,
		offset:   buf.offset,
		overflow: buf.overflow,
	}
}

// Unpack 用 codec 计划链（零反射）从当前 offset 解析二进制到 struct 并
// 自动推进 offset；count/greedy 结构体按实际消费推进。解析失败置粘性
// 越界标记。字节序固定 LittleEndian——codec 的既定语义，现有调用点
// 全部如此（NewYGOBuffer 的 order 参数仅 Read/Write 游标 API 使用）。
func (buf *YGOBuffer) Unpack(v interface{}) error {
	n, err := codec.DecodePrefix(buf.buff[buf.offset:], v)
	if err != nil {
		buf.overflow = true
		return err
	}
	buf.offset += n
	return nil
}

// SubSlices 返回从当前 buffer 位置到 clone buffer 位置的切片
func (buf *YGOBuffer) SubSlices(clone *YGOBuffer) []byte {
	if buf.offset >= clone.offset {
		return nil
	}
	return buf.buff[buf.offset:clone.offset]
}
func (buf *YGOBuffer) ReadNext(n int) []byte {
	if buf.offset+n > len(buf.buff) {
		buf.overflow = true
		return nil
	}
	res := buf.buff[buf.offset : buf.offset+n]
	buf.offset += n
	return res
}
