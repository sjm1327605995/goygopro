package main

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/core/utils"
)

// ------------------------------------------------------------------
// Incoming STOC Packet Dispatch
// ------------------------------------------------------------------

// handleSTOCPacket 按 stocBindings 表分发一个 STOC 包：
// 查表 → restruct.Unpack(包结构体) → decorate 组前端 map → emit。
// 表内没有的类型记日志后丢弃（以前静默丢弃，便于发现协议偏差）。
func (c *WailsDuelClient) handleSTOCPacket(proto byte, payload []byte) {
	binding, ok := stocBindings[proto]
	if !ok {
		log.Printf("[WailsDuelClient] unknown STOC packet 0x%02x (%d bytes), dropped", proto, len(payload))
		return
	}

	if binding.handle != nil {
		binding.handle(c, payload)
		return
	}

	if binding.newMsg == nil {
		// 无包体：只发空事件
		c.emit(binding.event, map[string]interface{}{})
		return
	}

	msg := binding.newMsg()
	// 与原 switch 分支一致：Unpack 错误被忽略，仍按零值/部分解析 emit。
	_ = restruct.Unpack(payload, binary.LittleEndian, msg)
	if binding.decorate != nil {
		binding.decorate(c, msg)
		return
	}
	c.emit(binding.event, msg)
}

// handleGameMessage parses YGOPRO engine MSG_* stream into structured 3D client
// events. It returns an error when the stream cannot be fully parsed to byte
// alignment (unknown message layout or truncated body); the caller decides
// whether to log, drop the batch, or surface the failure.
//
// 每种消息的解析与转发行为由 engineBindings 声明（engine_bindings.go）：
// 查表 → pbuf.Unpack(消息结构体) → decorate 派生字段 → emit。
// 表内没有的消息走 skipEngineMessageBody 的残余分支保持对齐。
func (c *WailsDuelClient) handleGameMessage(msgBuffer []byte) error {
	pbuf := utils.NewYGOBuffer(msgBuffer, binary.LittleEndian)

	for pbuf.Len() > 0 {
		var engType uint8
		if err := pbuf.Read(&engType); err != nil {
			return fmt.Errorf("read opcode at offset %d: %w", pbuf.Offset(), err)
		}

		binding, ok := engineBindings[engType]
		if !ok {
			// Messages the UI does not decode still occupy bytes in the stream.
			// Skip their bodies so the parser keeps message alignment; on an
			// unknown layout, stop parsing this batch rather than desync.
			if err := skipEngineMessageBody(pbuf, engType); err != nil {
				return fmt.Errorf("opcode 0x%02x at offset %d: %w", engType, pbuf.Offset(), err)
			}
			continue
		}

		if binding.newMsg == nil {
			// 无消息体：只发空事件
			if binding.event != "" {
				c.emit(binding.event, map[string]interface{}{})
			}
			continue
		}

		msg := binding.newMsg()
		if err := pbuf.Unpack(msg); err != nil {
			return fmt.Errorf("%s at offset %d: %w", binding.name, pbuf.Offset(), err)
		}
		if binding.decorate != nil {
			if err := binding.decorate(c, engType, pbuf, msg); err != nil {
				return fmt.Errorf("%s at offset %d: %w", binding.name, pbuf.Offset(), err)
			}
			continue
		}
		if binding.event != "" {
			c.emit(binding.event, msg)
		}
	}
	return nil
}
