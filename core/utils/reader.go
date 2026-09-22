package utils

import (
	"encoding/binary"
	"fmt"

	"github.com/go-restruct/restruct"
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
func (y *YGOBuffer) Offset() int {
	return y.offset
}
func (y *YGOBuffer) Read(list ...any) error {
	for i := range list {

		n, err := binary.Decode(y.buff[y.offset:], y.order, list[i])
		if err != nil {
			y.overflow = true
			return err
		}
		y.offset += n
	}
	return nil
}
// Overflowed 报告缓冲区自构造/克隆以来是否发生过越界访问。
func (y *YGOBuffer) Overflowed() bool {
	return y.overflow
}

// At 返回当前 offset 后第 pos 个字节；越界时置粘性越界标记并返回 0
//（不 panic），由调用方在解析结束后统一检查 Overflowed。
func (y *YGOBuffer) At(pos int) byte {
	idx := y.offset + pos
	if idx < 0 || idx >= len(y.buff) {
		y.overflow = true
		return 0
	}
	return y.buff[idx]
}
func (y *YGOBuffer) Write(list ...any) error {
	for i := range list {
		n, err := binary.Encode(y.buff[y.offset:], y.order, list[i])
		if err != nil {
			return err
		}
		y.offset += n
	}
	return nil
}
func (y *YGOBuffer) Len() int {
	return len(y.buff[y.offset:])
}

func (y *YGOBuffer) Bytes() []byte {
	return y.buff[y.offset:]
}

func (y *YGOBuffer) Next(n int) error {
	if y.offset+n > len(y.buff) {
		y.overflow = true
		return fmt.Errorf("out of range")
	}
	y.offset += n
	return nil
}
func (y *YGOBuffer) Clone() *YGOBuffer {
	return &YGOBuffer{
		buff:     y.buff,
		order:    y.order,
		offset:   y.offset,
		overflow: y.overflow,
	}
}

// Unpack 用 restruct 从当前 offset 解析二进制到 struct，并自动推进 offset
func (y *YGOBuffer) Unpack(v interface{}) error {
	err := restruct.Unpack(y.buff[y.offset:], binary.LittleEndian, v)
	if err != nil {
		y.overflow = true
		return err
	}
	size, err := restruct.SizeOf(v)
	if err != nil {
		return err
	}
	y.offset += size
	return nil
}

// Pack 用 restruct 将 struct 打包为字节数组
func PackGameMsg(v interface{}) []byte {
	data, _ := restruct.Pack(binary.LittleEndian, v)
	return data
}

// SubSlices 返回从当前 buffer 位置到 clone buffer 位置的切片
func (y *YGOBuffer) SubSlices(clone *YGOBuffer) []byte {
	if y.offset >= clone.offset {
		return nil
	}
	return y.buff[y.offset:clone.offset]
}
func (y *YGOBuffer) ReadNext(n int) []byte {
	if y.offset+n > len(y.buff) {
		y.overflow = true
		return nil
	}
	res := y.buff[y.offset : y.offset+n]
	y.offset += n
	return res
}
