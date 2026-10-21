package packet

import (
	"errors"
	"io"

	"github.com/panjf2000/gnet/v2"
	"github.com/sjm1327605995/goygopro/core/codec"
)

var ErrIncompletePacket = errors.New("incomplete packet")

// SimpleCodec 是纯帧解码器（原版 netserver buff 循环的 gnet 版）：按 2 字节
// 小端长度前缀切帧。它不持有玩家引用——连接上下文直接存 *duel.DuelPlayer
// （见 core/duel/server_engine.go），解码与玩家解耦后本类型对任何 gnet
// 连接可复用。
type SimpleCodec struct{}

func (dec SimpleCodec) Decode(c gnet.Conn) ([]byte, bool, error) {
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
	// Peek(2) 已保证恰好 2 字节，Unmarshal 不会失败
	var frameLen codec.FrameLen
	if err := codec.Unmarshal(packetLenData, &frameLen); err != nil {
		return nil, false, err
	}
	packetLen := int(frameLen.Len)

	if currentBufferLen < packetLen+2 {
		return nil, true, nil
	}

	data, err := c.Next(packetLen + 2)
	if err != nil {
		return nil, false, err
	}
	if len(data) <= 2 {
		// 零长度包：只有长度头、没有 proto 字节。已消耗的字节不退回，
		// 由 OnTraffic 跳过并继续解析同一缓冲区的后续包（与 C++ netserver
		// buff 循环对空包的跳过语义一致）。
		return nil, false, nil
	}
	return data[2:], false, nil
}
