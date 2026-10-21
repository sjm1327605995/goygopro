package packet

import "fmt"

// --------------------------------------------------
// 统一内部错误码
//
// 注意：这些码是服务端内部的分类码（0x10-0x18），与协议层 network.ERRMSG_*
// （0x1-0x4，客户端据此弹对应文案）不是一回事。只有能映射到 ERRMSG_* 的错误
// 才会经 SendError 上报客户端（见 context.go errmsgCode），其余协议
// 违例类错误与原版 netserver 一样只服务端处理、不回复。
// --------------------------------------------------
const (
	ErrBadRequest         uint8 = 0x10 // 通用非法请求
	ErrPayloadTooShort    uint8 = 0x11 // Payload 长度不足
	ErrBindFailed         uint8 = 0x12 // 数据解析失败
	ErrNotInGame          uint8 = 0x13 // 不在游戏中
	ErrDuelNotStarted     uint8 = 0x14 // 决斗未开始
	ErrDuelAlreadyStarted uint8 = 0x15 // 决斗已开始
	ErrInvalidState       uint8 = 0x16 // 状态非法
	ErrAlreadyInGame      uint8 = 0x17 // 已在游戏中
	ErrJoinFailed         uint8 = 0x18 // 加入房间失败（映射 ERRMSG_JOINERROR）
	ErrPayloadTooLong     uint8 = 0x19 // Payload 长度超限
)

// PacketError 是统一错误类型
type PacketError struct {
	Code    uint8
	Message string
	Cause   error
}

func (perr *PacketError) Error() string {
	if perr.Cause != nil {
		return fmt.Sprintf("packet error [0x%02x] %s: %v", perr.Code, perr.Message, perr.Cause)
	}
	return fmt.Sprintf("packet error [0x%02x] %s", perr.Code, perr.Message)
}

// NewPacketError 创建错误
func NewPacketError(code uint8, msg string, cause ...error) *PacketError {
	perr := &PacketError{Code: code, Message: msg}
	if len(cause) > 0 {
		perr.Cause = cause[0]
	}
	return perr
}

// --------------------------------------------------
// 错误快捷构造函数
// --------------------------------------------------

func ErrPayloadShort(got, need int, pktType uint8) *PacketError {
	return NewPacketError(ErrPayloadTooShort,
		fmt.Sprintf("payload too short: got %d, need %d, pktType=0x%02x", got, need, pktType))
}

// ErrPayloadLong 构造「payload 超过上限」错误（与 ErrPayloadShort 对称，
// 此前超长包误用 ErrPayloadTooShort，日志里无法区分两种违例）。
func ErrPayloadLong(got, max int, pktType uint8) *PacketError {
	return NewPacketError(ErrPayloadTooLong,
		fmt.Sprintf("payload too long: got %d, max %d, pktType=0x%02x", got, max, pktType))
}

func ErrBind(pktType uint8, cause error) *PacketError {
	return NewPacketError(ErrBindFailed,
		fmt.Sprintf("bind failed: pktType=0x%02x", pktType), cause)
}

func ErrNeedGame() *PacketError {
	return NewPacketError(ErrNotInGame, "player not in any game")
}

func ErrNeedDuel() *PacketError {
	return NewPacketError(ErrDuelNotStarted, "duel not started")
}

func ErrNeedLobby() *PacketError {
	return NewPacketError(ErrDuelAlreadyStarted, "duel already started")
}

func ErrInvalidPlayerState(pktType uint8, state uint8) *PacketError {
	return NewPacketError(ErrInvalidState,
		fmt.Sprintf("invalid state 0x%02x for pktType 0x%02x", state, pktType))
}

func ErrAlreadyInGameAction() *PacketError {
	return NewPacketError(ErrAlreadyInGame, "player already in a game")
}

func ErrJoinError() *PacketError {
	return NewPacketError(ErrJoinFailed, "room not found or join error")
}

// ErrWrongRoomPassword 是「密码即房间索引」设计下 GetRoom 失败的密码错误路径：
// 服务器上存在房间但按密码查不到时按密码错误回报（JOINERROR code=1，
// 原版 SysString 1404「密码错误」），与进房后密码不符的提示对齐。
func ErrWrongRoomPassword() *PacketError {
	return NewPacketError(ErrJoinFailed, "room not found: wrong password")
}
