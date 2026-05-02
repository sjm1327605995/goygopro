package duel

import "fmt"

// --------------------------------------------------
// 统一错误码（与 network.go 中的 ERRMSG_* 对应）
// --------------------------------------------------
const (
	ErrBadRequest      uint8 = 0x10 // 通用非法请求
	ErrPayloadTooShort uint8 = 0x11 // Payload 长度不足
	ErrBindFailed      uint8 = 0x12 // 数据解析失败
	ErrNotInGame       uint8 = 0x13 // 不在游戏中
	ErrDuelNotStarted  uint8 = 0x14 // 决斗未开始
	ErrDuelAlreadyStarted uint8 = 0x15 // 决斗已开始
	ErrInvalidState    uint8 = 0x16 // 状态非法
	ErrAlreadyInGame   uint8 = 0x17 // 已在游戏中
	ErrObserverNotAllowed uint8 = 0x18 // 观战者不允许
)

// PacketError 是统一错误类型
type PacketError struct {
	Code    uint8
	Message string
	Cause   error
}

func (e *PacketError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("packet error [0x%02x] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("packet error [0x%02x] %s", e.Code, e.Message)
}

// NewPacketError 创建错误
func NewPacketError(code uint8, msg string, cause ...error) *PacketError {
	e := &PacketError{Code: code, Message: msg}
	if len(cause) > 0 {
		e.Cause = cause[0]
	}
	return e
}

// --------------------------------------------------
// 错误快捷构造函数
// --------------------------------------------------

func ErrPayloadShort(got, need int, pktType uint8) *PacketError {
	return NewPacketError(ErrPayloadTooShort,
		fmt.Sprintf("payload too short: got %d, need %d, pktType=0x%02x", got, need, pktType))
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
