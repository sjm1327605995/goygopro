package protocol

import (
	"encoding/binary"
	"io"
)

// 以下结构体使用 go-restruct/restruct 标签进行二进制序列化/反序列化
// 安装: go get github.com/go-restruct/restruct
// 使用方式: restruct.Unpack(data, binary.LittleEndian, &v)
//           restruct.Pack(binary.LittleEndian, &v)

const (
	SIZE_NETWORK_BUFFER = 0x20000
	MAX_DATA_SIZE       = 0xFFFF - 1
	MAINC_MAX           = 250
	SIDEC_MAX           = MAINC_MAX

	LEN_CHAT_PLAYER = 1
	LEN_CHAT_MSG    = 256
	SIZE_STOC_CHAT  = (LEN_CHAT_PLAYER + LEN_CHAT_MSG) * 2
)

// HostPacket — 主控广播包
// C++: struct HostPacket { uint16_t identifier, version, port, padding; uint32_t ipaddr; uint16_t name[20]; HostInfo host; };
type HostPacket struct {
	Identifier uint16     `struct:"uint16"`
	Version    uint16     `struct:"uint16"`
	Port       uint16     `struct:"uint16"`
	_          uint16     `struct:"uint16"` // padding
	IPAddr     uint32     `struct:"uint32"`
	Name       [20]uint16 `struct:"[20]uint16"`
	Host       HostInfo
}

// HostInfo — 房间配置
// C++: struct HostInfo { uint32_t lflist; uint8_t rule, mode, duel_rule, no_check_deck, no_shuffle_deck; byte padding[3]; int32_t start_lp; uint8_t start_hand, draw_count; uint16_t time_limit; };
type HostInfo struct {
	LFList        uint32  `struct:"uint32"`
	Rule          uint8   `struct:"uint8"`
	Mode          uint8   `struct:"uint8"`
	DuelRule      uint8   `struct:"uint8"`
	NoCheckDeck   uint8   `struct:"uint8"`
	NoShuffleDeck uint8   `struct:"uint8"`
	_             [3]byte `struct:"[3]byte"` // padding
	StartLp       int32   `struct:"int32"`
	StartHand     uint8   `struct:"uint8"`
	DrawCount     uint8   `struct:"uint8"`
	TimeLimit     uint16  `struct:"uint16"`
}

// HostRequest — 查询请求
// C++: struct HostRequest { uint16_t identifier; };
type HostRequest struct {
	Identifier uint16 `struct:"uint16"`
}

// CTOSDeckData — 卡组数据（自定义 Unpacker/Packer，因为 List 长度 = MainC + SideC）
// C++: struct CTOS_DeckData { int32_t mainc, sidec; uint32_t list[MAINC_MAX + SIDEC_MAX]; };
type CTOSDeckData struct {
	MainC int32
	SideC int32
	List  []uint32 // 长度由 MainC + SideC 决定
}

func (c *CTOSDeckData) Unpack(buf []byte, order binary.ByteOrder) ([]byte, error) {
	if len(buf) < 8 {
		return nil, io.ErrShortBuffer
	}
	c.MainC = int32(order.Uint32(buf[0:4]))
	c.SideC = int32(order.Uint32(buf[4:8]))
	total := int(c.MainC + c.SideC)
	if total < 0 || total > MAINC_MAX+SIDEC_MAX {
		return nil, io.ErrShortBuffer
	}
	buf = buf[8:]
	if len(buf) < total*4 {
		return nil, io.ErrShortBuffer
	}
	c.List = make([]uint32, total)
	for i := 0; i < total; i++ {
		c.List[i] = order.Uint32(buf[i*4 : i*4+4])
	}
	return buf[total*4:], nil
}

func (c *CTOSDeckData) Pack(buf []byte, order binary.ByteOrder) ([]byte, error) {
	size := c.SizeOf()
	if len(buf) < size {
		return nil, io.ErrShortBuffer
	}
	order.PutUint32(buf[0:4], uint32(c.MainC))
	order.PutUint32(buf[4:8], uint32(c.SideC))
	for i, v := range c.List {
		order.PutUint32(buf[8+i*4:8+i*4+4], v)
	}
	return buf[size:], nil
}

func (c *CTOSDeckData) SizeOf() int {
	return 8 + len(c.List)*4
}

// CTOSHandResult — 猜拳结果
// C++: struct CTOS_HandResult { uint8_t res; };
type CTOSHandResult struct {
	Res uint8 `struct:"uint8"`
}

// CTOSTPResult — 先后攻选择
// C++: struct CTOS_TPResult { uint8_t res; };
type CTOSTPResult struct {
	Res uint8 `struct:"uint8"`
}

// CTOSPlayerInfo — 玩家信息
// C++: struct CTOS_PlayerInfo { uint16_t name[20]; };
type CTOSPlayerInfo struct {
	Name [20]uint16 `struct:"[20]uint16"`
}

// CTOSCreateGame — 创建房间
// C++: struct CTOS_CreateGame { HostInfo info; uint16_t name[20]; uint16_t pass[20]; };
type CTOSCreateGame struct {
	Info HostInfo
	Name [20]uint16 `struct:"[20]uint16"`
	Pass [20]uint16 `struct:"[20]uint16"`
}

// CTOSJoinGame — 加入房间
// C++: struct CTOS_JoinGame { uint16_t version; uint8_t padding[2]; uint32_t gameid; uint16_t pass[20]; };
type CTOSJoinGame struct {
	Version uint16     `struct:"uint16"`
	_       [2]byte    `struct:"[2]byte"` // padding
	GameID  uint32     `struct:"uint32"`
	Pass    [20]uint16 `struct:"[20]uint16"`
}

// CTOSKick — 踢人
// C++: struct CTOS_Kick { uint8_t pos; };
type CTOSKick struct {
	Pos uint8 `struct:"uint8"`
}

// STOCErrorMsg — 错误消息
// C++: struct STOC_ErrorMsg { uint8_t msg; uint8_t padding[3]; uint32_t code; };
type STOCErrorMsg struct {
	Msg  uint8   `struct:"uint8"`
	_    [3]byte `struct:"[3]byte"` // padding
	Code uint32  `struct:"uint32"`
}

// STOCHandResult — 猜拳结果通知
// C++: struct STOC_HandResult { uint8_t res1, res2; };
type STOCHandResult struct {
	Res1 uint8 `struct:"uint8"`
	Res2 uint8 `struct:"uint8"`
}

// STOCCreateGame — 创建房间响应
// C++: struct STOC_CreateGame { uint32_t gameid; };
type STOCCreateGame struct {
	GameID uint32 `struct:"uint32"`
}

// STOCJoinGame — 加入房间响应
// C++: struct STOC_JoinGame { HostInfo info; };
type STOCJoinGame struct {
	Info HostInfo
}

// STOCTypeChange — 身份变更
// C++: struct STOC_TypeChange { uint8_t type; };
type STOCTypeChange struct {
	Type uint8 `struct:"uint8"`
}

// STOCExitGame — 退出游戏
// C++: struct STOC_ExitGame { uint8_t pos; };
type STOCExitGame struct {
	Pos uint8 `struct:"uint8"`
}

// STOCTimeLimit — 时间限制
// C++: struct STOC_TimeLimit { uint8_t player; uint8_t padding; uint16_t left_time; };
type STOCTimeLimit struct {
	Player   uint8  `struct:"uint8"`
	_        byte   `struct:"uint8"` // padding
	LeftTime uint16 `struct:"uint16"`
}

// STOCChat — 聊天消息
// C++: struct STOC_Chat { uint16_t player; uint16_t msg[256]; };
// player 字段实际只占 1 字节（uint8），但 C++ 中常按 uint16 解析
// 这里保留与原版一致的定义
type STOCChat struct {
	Player uint16      `struct:"uint16"`
	Msg    [256]uint16 `struct:"[256]uint16"`
}

// STOCHsPlayerEnter — 玩家进入大厅
// C++: struct STOC_HS_PlayerEnter { uint16_t name[20]; uint8_t pos; };
type STOCHsPlayerEnter struct {
	Name [20]uint16 `struct:"[20]uint16"`
	Pos  uint8      `struct:"uint8"`
}

// STOCHsPlayerChange — 玩家状态变更
// C++: struct STOC_HS_PlayerChange { uint8_t status; };
type STOCHsPlayerChange struct {
	Status uint8 `struct:"uint8"`
}

// STOCHsWatchChange — 观战人数变更
// C++: struct STOC_HS_WatchChange { uint16_t watch_count; };
type STOCHsWatchChange struct {
	WatchCount uint16 `struct:"uint16"`
}
