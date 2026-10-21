package packet

import (
	"fmt"
	"io"
	"sync"

	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// PacketHandlerFunc 是包处理函数签名，类似 gin.HandlerFunc
type PacketHandlerFunc func(ctx *PacketContext)

// --------------------------------------------------
// 窄接口：core/packet 不认识 duel.DuelPlayer / duel.IDuelMode，
// 只依赖下面三个最小抽象（两者分别由它们满足）。
// --------------------------------------------------

// Peer 是 CTOS 消息的来源玩家（连接）的最小抽象：
// 应答写口、日志名、CTOS 状态机状态、当前所在游戏。
type Peer interface {
	GetID() string
	Write(p []byte) (int, error)
	// LogName 是日志用显示名（原版 utils.WideString(Name[:])）。
	LogName() string
	// PlayerState 是 CTOS 状态机状态（原版 DuelPlayer.State 字段）。
	PlayerState() uint8
	// CurrentGame 返回玩家当前所在游戏；nil 表示不在任何房间。
	// 必须动态读取（而非缓存）：持锁期间玩家可能被踢出，
	// RoomLockMiddleware 靠二次读取发现引用变化。
	CurrentGame() Game
}

// Game 是处理链对玩家所在会话的最小抽象（duel.IDuelMode 满足它）：
// 房间基座（互斥锁 + 决斗状态）与引擎决斗是否已开始。
type Game interface {
	// BaseRoom 返回房间基座；nil 对应旧判空 BaseMode()==nil。
	BaseRoom() Room
	// DuelActive 报告引擎决斗是否已开始（旧 OCGDuel() != nil）。
	DuelActive() bool
}

// Room 是房间基座（duel.DuelMode）的最小抽象：互斥锁 + 决斗状态。
type Room interface {
	sync.Locker
	DuelStarted() bool
}

// PacketContext 是 Packet 请求的上下文，类似 Gin 的 gin.Context
type PacketContext struct {
	// 原始数据
	RawData []byte // 完整数据（含 pktType 前缀）
	PktType uint8  // 消息类型
	Payload []byte // 去掉前缀后的数据

	// 关联对象
	Peer Peer

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
func NewPacketContext(peer Peer, data []byte) *PacketContext {
	if len(data) == 0 {
		return nil
	}
	return &PacketContext{
		RawData: data,
		PktType: data[0],
		Payload: data[1:],
		Peer:    peer,
		index:   -1,
	}
}

// Next 执行下一个中间件/处理器
func (ctx *PacketContext) Next() {
	ctx.index++
	for ctx.index < len(ctx.handlers) && !ctx.aborted {
		ctx.handlers[ctx.index](ctx)
		ctx.index++
	}
}

// Abort 中止当前请求链，后续中间件不再执行
func (ctx *PacketContext) Abort() {
	ctx.aborted = true
}

// --------------------------------------------------
// 数据绑定（由 BindMiddleware 设置）
// --------------------------------------------------

var payloadKey = "payload"

// SetPayload 设置解析后的消息体
func (ctx *PacketContext) SetPayload(payload interface{}) {
	ctx.Set(payloadKey, payload)
}

// MustPayload 获取解析后的消息体，如果不存在则 panic（会被 RecoverMiddleware 捕获）
func (ctx *PacketContext) MustPayload() interface{} {
	payload, ok := ctx.Get(payloadKey)
	if !ok || payload == nil {
		panic(fmt.Sprintf("packet %d: payload not bound", ctx.PktType))
	}
	return payload
}

// --------------------------------------------------
// 辅助属性（快捷访问）
// --------------------------------------------------

// Game 快捷访问 Peer 当前所在游戏（动态读取，见 Peer.CurrentGame）。
func (ctx *PacketContext) Game() Game {
	if ctx.Peer == nil {
		return nil
	}
	return ctx.Peer.CurrentGame()
}

// --------------------------------------------------
// 响应方法
// --------------------------------------------------

// Reply 发送 STOC 消息给当前玩家（包含 2 字节包长前缀）。帧结构用
// codec.Frame（Len 由调用方显式赋值 = 1+len(Payload)，负载 greedy）。
func (ctx *PacketContext) Reply(pktType uint8, data []byte) error {
	if ctx.Peer == nil {
		return fmt.Errorf("player or connection is nil")
	}
	// 包长前缀只有 2 字节（上限 0xFFFF），超限必须拒绝而不是让长度回绕，
	// 否则客户端按错误长度切帧，后续所有包解析全部错位
	if len(data) > 0xFFFE {
		return fmt.Errorf("stoc packet too large: %d bytes (max 0xFFFE)", len(data))
	}
	buf, err := codec.Marshal(&codec.Frame{Len: uint16(1 + len(data)), Proto: pktType, Payload: data})
	if err != nil {
		return err
	}
	n, err := ctx.Peer.Write(buf)
	if err == nil && n != len(buf) {
		return io.ErrShortWrite
	}
	return err
}

// Error 设置错误信息并中止请求链，同时向客户端发送 STOC_ERROR_MSG
func (ctx *PacketContext) Error(code uint8, msg string, cause ...error) {
	ctx.errCode = code
	ctx.errMsg = msg
	if len(cause) > 0 {
		ctx.errCause = cause[0]
	}
	ctx.Abort()
	// 自动发送错误响应给客户端
	_ = ctx.SendError(code, msg)
}

// SendError 向客户端发送 STOC_ERROR_MSG。只有能映射到 ERRMSG_* 的内部错误才
// 发客户端包；其余内部错误仅服务端处理（原版 netserver 对协议违例同样不回复）。
func (ctx *PacketContext) SendError(errCode uint8, msg string) error {
	return ctx.SendErrorWithCode(errCode, msg, 0)
}

// SendErrorWithCode 与 SendError 相同，但可指定 STOC_ErrorMsg.code 字段
// （如 JOINERROR code=1 = 密码错误，对应原版 SysString 1404 的文案分支）。
// 消息体用 codec.ErrorBody（{ msg; padding[3]; code }，padding 显式 skip）。
func (ctx *PacketContext) SendErrorWithCode(errCode uint8, msg string, code uint32) error {
	if ctx.Peer == nil {
		return fmt.Errorf("player or connection is nil")
	}
	stocMsg, ok := errmsgCode(errCode)
	if !ok {
		return nil
	}
	body, err := codec.Marshal(&codec.ErrorBody{Msg: stocMsg, Code: code})
	if err != nil {
		return err
	}
	utils.NetLogf("server", "send STOC_ERROR_MSG msg=%d code=%d (%s) to player=%q",
		stocMsg, code, msg, ctx.Peer.LogName())
	return ctx.Reply(network.STOC_ERROR_MSG, body)
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
func (ctx *PacketContext) AbortWithError(err *PacketError) {
	ctx.Error(err.Code, err.Message, err.Cause)
}

// AbortWithErrorCode 与 AbortWithError 类似，但可指定 STOC_ErrorMsg.code 字段。
func (ctx *PacketContext) AbortWithErrorCode(err *PacketError, code uint32) {
	ctx.errCode = err.Code
	ctx.errMsg = err.Message
	ctx.errCause = err.Cause
	ctx.Abort()
	_ = ctx.SendErrorWithCode(err.Code, err.Message, code)
}

// --------------------------------------------------
// 通用 KV 存储（类似 gin.Context.Keys）
// --------------------------------------------------

func (ctx *PacketContext) Set(key string, value interface{}) {
	if ctx.keys == nil {
		ctx.keys = make(map[string]interface{})
	}
	ctx.keys[key] = value
}

func (ctx *PacketContext) Get(key string) (interface{}, bool) {
	if ctx.keys == nil {
		return nil, false
	}
	v, ok := ctx.keys[key]
	return v, ok
}
