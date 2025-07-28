package duel

import (
	"encoding/binary"
	"errors"
	"github.com/panjf2000/gnet/v2"
	"io"
)

var ErrIncompletePacket = errors.New("incomplete packet")

type SimpleCodec struct {
	Player *DuelPlayer
}

func (codec SimpleCodec) Encode(buf []byte) ([]byte, error) {

	return nil, nil
}

func (codec SimpleCodec) Decode(c gnet.Conn) ([]byte, bool, error) {
	currentBufferLen := c.InboundBuffered()
	if currentBufferLen < 2 {
		return nil, true, nil
	}

	packetLenData, err := c.Peek(2)
	if err != nil {
		if errors.Is(err, io.ErrShortBuffer) {
			err = ErrIncompletePacket
		}
		return nil, false, err
	}
	packetLen := int(binary.LittleEndian.Uint16(packetLenData))

	if currentBufferLen < packetLen {
		return nil, false, nil
	}

	data, err := c.Next(packetLen + 2)

	return data[2:], false, nil
}
