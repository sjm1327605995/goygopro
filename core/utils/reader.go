package utils

import (
	"encoding/binary"
	"fmt"
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
func (y *YGOBuffer) SubSlices(clone *YGOBuffer) []byte {
	return y.SubSlicesOffset(clone, 0)
}
func (y *YGOBuffer) SubSlicesOffset(clone *YGOBuffer, offset int) []byte {
	if y.offset >= clone.offset {
		return nil
	}
	return y.buff[y.offset : clone.offset+offset]
}
