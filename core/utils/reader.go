package utils

import (
	"encoding/binary"
	"fmt"
)

type YGOReader struct {
	buff   []byte
	offset int
	order  binary.ByteOrder
}

func NewYGOReader(buff []byte, order binary.ByteOrder) *YGOReader {
	return &YGOReader{
		buff:  buff,
		order: order,
	}
}
func (y *YGOReader) Read(list ...any) error {
	for i := range list {

		n, err := binary.Decode(y.buff[y.offset:], y.order, list[i])
		if err != nil {
			return err
		}
		y.offset += n
	}
	return nil
}
func (y *YGOReader) At(pos int) byte {

	return y.buff[y.offset+pos]
}
func (y *YGOReader) Write(list ...any) error {
	for i := range list {
		n, err := binary.Encode(y.buff[y.offset:], y.order, list[i])
		if err != nil {
			return err
		}
		y.offset += n
	}
	return nil
}
func (y *YGOReader) Len() int {
	return len(y.buff[y.offset:])
}
func (y *YGOReader) Bytes() []byte {
	return y.buff[y.offset:]
}

func (y *YGOReader) Next(n int) error {
	if y.offset+n > len(y.buff) {
		return fmt.Errorf("out of range")
	}
	return nil
}
func (y *YGOReader) Clone() *YGOReader {
	return &YGOReader{
		buff:   y.buff,
		offset: y.offset,
	}
}
func (y *YGOReader) SubSlices(clone *YGOReader) []byte {
	if clone.offset >= y.offset {
		return nil
	}
	return y.buff[clone.offset:y.offset]
}
