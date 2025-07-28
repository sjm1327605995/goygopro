package utils

import (
	"bytes"
	"io"
)

type Reader struct {
	buff   *bytes.Reader
	offset int
}

func NewReader(arr []byte) *Reader {
	return &Reader{buff: bytes.NewReader(arr), offset: 0}
}
func (r *Reader) Read(p []byte) (n int, err error) {
	n, err = r.buff.Read(p)
	r.offset += n
	return
}
func (r *Reader) Len() int {
	return r.buff.Len()
}
func (r *Reader) Discord(n int) error {
	_, err := r.buff.Seek(int64(n), io.SeekCurrent)
	r.offset += n
	return err
}
func (r *Reader) Offset() int {
	return r.offset
}
