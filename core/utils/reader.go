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
			return err
		}
		y.offset += n
	}
	return nil
}
func (y *YGOBuffer) At(pos int) byte {

	return y.buff[y.offset+pos]
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
		return fmt.Errorf("out of range")
	}
	y.offset += n
	return nil
}
func (y *YGOBuffer) Clone() *YGOBuffer {
	return &YGOBuffer{
		buff:   y.buff,
		order:  y.order,
		offset: y.offset,
	}
}
// Unpack 用 restruct 从当前 offset 解析二进制到 struct，并自动推进 offset
func (y *YGOBuffer) Unpack(v interface{}) error {
	err := restruct.Unpack(y.buff[y.offset:], binary.LittleEndian, v)
	if err != nil {
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
	return y.SubSlicesOffset(clone, 0)
}
func (y *YGOBuffer) SubSlicesOffset(clone *YGOBuffer, offset int) []byte {
	if y.offset >= clone.offset {
		return nil
	}
	return y.buff[y.offset : clone.offset+offset]
}
func (y *YGOBuffer) ReadNext(n int) []byte {
	if y.offset+n > len(y.buff) {
		return nil
	}
	res := y.buff[y.offset : y.offset+n]
	y.offset += n
	return res
}
