package duel

import (
	"encoding/binary"
	"fmt"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// PacketHandlerFunc 是包处理函数签名，类似 gin.HandlerFunc
type PacketHandlerFunc func(c *PacketContext)

// PacketContext 是 Packet 请求的上下文，类似 Gin 的 gin.Context
type PacketContext struct {
	// 原始数据
	RawData []byte // 完整数据（含 pktType 前缀）
	PktType uint8  // 消息类型
	Payload []byte // 去掉前缀后的数据

	// 关联对象
	Player *DuelPlayer

	// 内部状态
	handlers []PacketHandlerFunc // 当前路由的处理链
	index    int                 // 当前执行到的中间件索引
	aborted  bool                // 是否已中止

	// 错误信息
	errCode  uint8
	errMsg   string
	errCause error

	// 通用 KV 存储（类似 gin.Context.Keys）
	keys map[string]interface{}
}

// NewPacketContext 创建一个新的 PacketContext
func NewPacketContext(player *DuelPlayer, data []byte) *PacketContext {
	if len(data) == 0 {
		return nil
	}
	return &PacketContext{
		RawData: data,
		PktType: data[0],
		Payload: data[1:],
		Player:  player,
		index:   -1,
	}
}

// Next 执行下一个中间件/处理器
func (c *PacketContext) Next() {
	c.index++
	for c.index < len(c.handlers) && !c.aborted {
		c.handlers[c.index](c)
		c.index++
	}
}

// Abort 中止当前请求链，后续中间件不再执行
func (c *PacketContext) Abort() {
	c.aborted = true
}

// --------------------------------------------------
// 数据绑定（由 BindMiddleware 设置）
// --------------------------------------------------

var payloadKey = "payload"

// SetPayload 设置解析后的消息体
func (c *PacketContext) SetPayload(v interface{}) {
	c.Set(payloadKey, v)
}

// MustPayload 获取解析后的消息体，如果不存在则 panic（会被 RecoverMiddleware 捕获）
func (c *PacketContext) MustPayload() interface{} {
	v, ok := c.Get(payloadKey)
	if !ok || v == nil {
		panic(fmt.Sprintf("packet %d: payload not bound", c.PktType))
	}
	return v
}

// --------------------------------------------------
// 辅助属性（快捷访问）
// --------------------------------------------------

// Game 快捷访问 Player.Game
func (c *PacketContext) Game() IDuelMode {
	if c.Player == nil || c.Player.Game == nil {
		return nil
	}
	return c.Player.Game
}

// BaseMode 快捷访问 Player.Game.BaseMode()
func (c *PacketContext) BaseMode() *DuelMode {
	g := c.Game()
	if g == nil {
		return nil
	}
	return g.BaseMode()
}

// --------------------------------------------------
// 响应方法
// --------------------------------------------------

// Reply 发送 STOC 消息给当前玩家（包含 2 字节包长前缀）
func (c *PacketContext) Reply(pktType uint8, data []byte) error {
	if c.Player == nil || c.Player.Conn == nil {
		return fmt.Errorf("player or connection is nil")
	}
	packetLen := uint16(1 + len(data))
	buf := make([]byte, 2+int(packetLen))
	binary.LittleEndian.PutUint16(buf[0:2], packetLen)
	buf[2] = pktType
	if len(data) > 0 {
		copy(buf[3:], data)
	}
	_, err := c.Player.Conn.Write(buf)
	return err
}

// Error 设置错误信息并中止请求链，同时向客户端发送 STOC_ERROR_MSG
func (c *PacketContext) Error(code uint8, msg string, cause ...error) {
	c.errCode = code
	c.errMsg = msg
	if len(cause) > 0 {
		c.errCause = cause[0]
	}
	c.Abort()
	// 自动发送错误响应给客户端
	_ = c.SendError(code, msg)
}

// SendError 向客户端发送 STOC_ERROR_MSG。只有能映射到 ERRMSG_* 的内部错误才
// 发客户端包；其余内部错误仅服务端处理（原版 netserver 对协议违例同样不回复）。
func (c *PacketContext) SendError(errCode uint8, msg string) error {
	if c.Player == nil || c.Player.Conn == nil {
		return fmt.Errorf("player or connection is nil")
	}
	stocMsg, ok := errmsgCode(errCode)
	if !ok {
		return nil
	}
	// 构造 STOC_ErrorMsg：uint8 msg(ERRMSG_*) + [3]byte padding + uint32 code
	buf := make([]byte, 8)
	buf[0] = stocMsg
	// padding 3 bytes 保持 0
	binary.LittleEndian.PutUint32(buf[4:], 0)
	return c.Reply(network.STOC_ERROR_MSG, buf)
}

// errmsgCode 把内部 PacketError 码映射为 STOC_ERROR_MSG 的 ERRMSG_* 值。
// Msg 必须是 network.ERRMSG_*（duelclient.cpp:261-368 只识别这四种，其余
// 被静默丢弃）。返回 ok=false 表示该错误没有客户端可见的协议错误码。
func errmsgCode(errCode uint8) (msg uint8, ok bool) {
	switch errCode {
	case ErrJoinFailed:
		return network.ERRMSG_JOINERROR, true
	default:
		return 0, false
	}
}

// AbortWithError 中止并发送统一错误响应
func (c *PacketContext) AbortWithError(err *PacketError) {
	c.Error(err.Code, err.Message, err.Cause)
}

// --------------------------------------------------
// 通用 KV 存储（类似 gin.Context.Keys）
// --------------------------------------------------

func (c *PacketContext) Set(key string, value interface{}) {
	if c.keys == nil {
		c.keys = make(map[string]interface{})
	}
	c.keys[key] = value
}

func (c *PacketContext) Get(key string) (interface{}, bool) {
	if c.keys == nil {
		return nil, false
	}
	v, ok := c.keys[key]
	return v, ok
}
