package protocol

import "math"

const SIZE_NETWORK_BUFFER = 0x20000
const MAX_DATA_SIZE = math.MaxUint16 - 1
const MAINC_MAX = 250
const SIDEC_MAX = MAINC_MAX

type HostPacket struct {
	Identifier uint16
	Version    uint16
	Port       uint16
	padding    uint16
	IPAddr     uint32
	Name       [20]uint16
	Host       HostInfo
}
type HostInfo struct {
	LFList        uint32
	Rule          uint8
	Mode          uint8
	DuelRule      uint8
	NoCheckDeck   uint8
	NoShuffleDeck uint8
	// byte padding[3]
	padding   [3]byte
	StartLp   int32
	StartHand uint8
	DrawCount uint8
	TimeLimit uint16
}

type HostRequest struct {
	Identifier uint16
}

type CTOSDeckData struct {
	CTOSDeckDataBase
	List []uint32 //MAINC_MAX + SIDEC_MAX
}
type CTOSDeckDataBase struct {
	MainC int32
	SideC int32
}
type CTOSHandResult struct {
	Res uint8
}

type CTOSTPResult struct {
	Res uint8
}

type CTOSPlayerInfo struct {
	Name [20]uint16
}
type CTOSCreateGame struct {
	Info HostInfo
	Name [20]uint16
	Pass [20]uint16
}

type CTOSJoinGame struct {
	Version uint16
	Padding [2]byte
	GameID  uint32
	Pass    [20]uint16
}

type CTOSKick struct {
	Pos uint8
}
type STOCErrorMsg struct {
	Msg     uint8
	padding [3]byte
	Code    uint32
}
type STOCHandResult struct {
	Res1 uint8
	Res2 uint8
}

type STOCCreateGame struct {
	GameID uint32
}
type STOCJoinGame struct {
	Info HostInfo
}
type STOCTypeChange struct {
	Type uint8
}
type STOCExitGame struct {
	Pos uint8
}
type STOCTimeLimit struct {
	Player   uint8
	padding  byte
	LeftTime uint16
}

const (
	LEN_CHAT_PLAYER = 1
	LEN_CHAT_MSG    = 256
	SIZE_STOC_CHAT  = (LEN_CHAT_PLAYER + LEN_CHAT_MSG) * 2
)

type STOCHsPlayerEnter struct {
	Name    [20]uint16
	Pos     uint8
	padding byte
}

type STOCHsPlayerChange struct {
	Status uint8
}

type STOCHsWatchChange struct {
	WatchCount uint16
}
