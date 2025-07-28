package network

import (
	"encoding/binary"
	"io"
)

func GetPosition(data []byte, offset int) uint32 {
	return binary.LittleEndian.Uint32(data[offset:])

}

type BufferIO struct {
	buf []byte
	off int
}

func NewBufferIO(data []byte) *BufferIO {
	return &BufferIO{buf: data}
}
func (b *BufferIO) Read(p []byte) (n int, err error) {
	if b.empty() {
		// Buffer is empty, reset to recover space.

		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	n = copy(p, b.buf[b.off:])
	b.off += n
	return n, nil
}
func (b *BufferIO) Write(p []byte) (n int, err error) {
	currentLen := len(b.buf[b.off:])
	writeLen := len(p)
	if currentLen >= writeLen {
		copy(b.buf[b.off:], p)
	} else {
		splitIndex := writeLen - currentLen
		copy(b.buf[b.off:], p[:splitIndex])
		b.buf = append(b.buf, p[splitIndex:]...)
	}
	b.off += writeLen
	return writeLen, nil
}
func (b *BufferIO) empty() bool {
	return len(b.buf) == 0
}
