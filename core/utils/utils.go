package utils

import (
	"encoding/binary"
	"io"
)

// Buffer 封装字节缓冲区并提供读写方法
type Buffer struct {
	data   []byte
	offset int // 共享的读写位置
}

func NewBuffer(data []byte) *Buffer {
	return &Buffer{data: data}
}

func (b *Buffer) Data() []byte {
	return b.data
}

func (b *Buffer) Offset() int {
	return b.offset
}

func (b *Buffer) Seek(offset int) {
	if offset < 0 {
		offset = 0
	} else if offset > len(b.data) {
		offset = len(b.data)
	}
	b.offset = offset
}
func (b *Buffer) Read(p []byte) (n int, err error) {
	if b.offset+len(p) > len(b.data) {
		p = p[:len(b.data)-b.offset]
	}
	if len(p) == 0 {
		return 0, nil
	}
	n = copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}
func (b *Buffer) Write(p []byte) (n int, err error) {
	// 确保缓冲区足够大
	required := b.offset + len(p)
	if required > cap(b.data) {
		// 扩展缓冲区（容量翻倍或至少满足需求）
		newCap := 2*cap(b.data) + len(p)
		if newCap < required {
			newCap = required
		}
		newData := make([]byte, len(b.data), newCap)
		copy(newData, b.data)
		b.data = newData
	}

	// 确保长度足够
	if required > len(b.data) {
		b.data = b.data[:required]
	}

	copy(b.data[b.offset:], p)
	b.offset += len(p) // 移动偏移量
	return len(p), nil
}

// NullTerminate 将任意类型的切片的最后一个元素设置为零值
// 这是对C++模板函数的Go语言实现：
// template<size_t N, typename T>
//
//	static void NullTerminate(T(&str)[N]) {
//	    str[N - 1] = 0;
//	}
func NullTerminate[T any](slice []T, zero T) {
	if len(slice) > 0 {
		slice[len(slice)-1] = zero
	}
}

// CopyWStrRef 泛型字符串复制函数(带指针移动)，支持int32/rune/byte/uint16类型
func CopyWStrRef[T1, T2 ~int32 | ~byte | ~uint16](src []T1, pstr *[]T2, bufsize int) int {
	l := 0
	for l < len(src) && src[l] != 0 && l < bufsize-1 {
		(*pstr)[l] = T2(src[l])
		l++
	}
	*pstr = (*pstr)[l:]
	(*pstr)[0] = 0
	return l
}

// Wcscmp 泛型实现，支持多种字符类型
func Wcscmp[T ~uint16 | ~rune](s1, s2 []T) int {
	for i := 0; ; i++ {
		if i >= len(s1) || s1[i] == 0 {
			if i >= len(s2) || s2[i] == 0 {
				return 0
			}
			return -1
		}
		if i >= len(s2) || s2[i] == 0 {
			return 1
		}
		if s1[i] < s2[i] {
			return -1
		}
		if s1[i] > s2[i] {
			return 1
		}
	}
}
func BatchRead(reader io.Reader, order binary.ByteOrder, data ...any) error {
	for _, v := range data {
		err := binary.Read(reader, order, v)
		if err != nil {
			return err
		}
	}
	return nil

}
func BatchWrite(writer io.Writer, order binary.ByteOrder, data ...any) error {
	for _, v := range data {
		err := binary.Write(writer, order, v)
		if err != nil {
			return err
		}
	}
	return nil
}
func BatchDecode(buff []byte, offset *int, order binary.ByteOrder, data ...any) error {
	for _, v := range data {
		n, err := binary.Decode(buff[*offset:], order, v)
		if err != nil {
			return err
		}
		*offset += n
	}
	return nil

}
func BatchEncode(buff []byte, offset *int, order binary.ByteOrder, data ...any) error {
	for _, v := range data {
		n, err := binary.Encode(buff[*offset:], order, v)
		if err != nil {
			return err
		}
		*offset += n
	}
	return nil

}
