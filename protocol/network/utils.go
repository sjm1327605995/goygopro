package network

import (
	"encoding/binary"
)

func GetPosition(data []byte, offset int) uint32 {
	info := binary.LittleEndian.Uint32(data[offset:])
	return info >> 24
}
