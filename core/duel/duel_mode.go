package duel

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"sync"
	"unsafe"

	"github.com/antlabs/timer"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
)

type IDuelMode interface {
	BaseMode() *DuelMode
	Chat(dp *DuelPlayer, pData []byte)
	JoinGame(dp *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool)
	LeaveGame(dp *DuelPlayer)
	ToDuelList(dp *DuelPlayer)
	ToObserver(dp *DuelPlayer)
	PlayerReady(dp *DuelPlayer, isReady bool)
	PlayerKick(dp *DuelPlayer, pos byte)
	UpdateDeck(dp *DuelPlayer, pData []byte)
	StartDuel(dp *DuelPlayer)
	HandResult(dp *DuelPlayer, res byte)
	TPResult(dp *DuelPlayer, tp byte)
	Process()
	Analyze(msgBuffer []byte) int
	Surrender(dp *DuelPlayer)
	GetResponse(dp *DuelPlayer, msgBuffer []byte)
	TimeConfirm(dp *DuelPlayer)
	EndDuel()
	OCGDuel() *ocgcore.Duel
}

type DuelMode struct {
	Mu          sync.Mutex
	HostPlayer  *DuelPlayer
	HostInfo    protocol.HostInfo
	DuelStage   int
	ETimer      timer.TimeNoder
	Name        [20]uint16
	Pass        [20]uint16
	RoomID      string // 在 RoomManager 中的索引ID
	room        duelRoom // 构造时由 newSingleDuel/newTagDuel 二段设置为具体模式自身
	buff        [protocol.SIZE_NETWORK_BUFFER]byte
	buffOffset  int
	Duel        *ocgcore.Duel
	startOffset int64
	lastReplay  *Replay

	// 以下为 SingleDuel / TagDuel 共用的对局状态。
	// SingleDuel 只使用下标 0/1（两名玩家），TagDuel 使用 0..3。
	Observers    map[string]*DuelPlayer
	ready        [4]bool
	pDeck        [4]*Deck
	DeckError    [4]uint32
	handResult   [2]uint8
	lastResponse uint8
	timeLimit    [2]int16
	timeElapsed  int16
	// refresh 是两模式仅在查询 flag/缓存/边界常量上不同的刷新参数表，
	// 由 newSingleDuel/newTagDuel 构造时填写（见 refreshFlags）。
	refresh refreshFlags
}

const (
	// 聊天相关常量
	LenChatPlayer = 1
	LenChatMsg    = 256

	SizeOfUint16 = 2 // Go中uint16固定为2字节，但为了保持与C++代码的一致性，
	// 我们仍然使用SizeOfUint16来表示uint16的大小

	// SizeSTOCChat 计算STOC_CHAT包的大小
	SizeSTOCChat = (LenChatPlayer + LenChatMsg) * SizeOfUint16
)

// 如果需要确保SizeOfUint16与实际系统上的uint16大小一致，可以使用以下函数
func init() {
	// 验证SizeOfUint16常量是否与系统上的uint16大小一致
	actualSize := int(unsafe.Sizeof(uint16(0)))
	if SizeOfUint16 != actualSize {
		panic("SizeOfUint16常量与系统上的uint16大小不一致")
	}
}

// CheckMsgSize 检查消息大小是否合法
// 参数:
//
//	size: 消息大小（字节数）
//
// 返回值:
//
//	bool: 消息大小是否合法
func CheckMsgSize(size int) bool {
	// 空字符串不允许（至少需要一个字符和一个null终止符）
	if size < 2*SizeOfUint16 {
		return false
	}

	// 消息不能超过最大长度
	if size > LenChatMsg*SizeOfUint16 {
		return false
	}

	// 消息大小必须是uint16大小的整数倍
	if size%SizeOfUint16 != 0 {
		return false
	}

	return true
}

func (d *DuelMode) BaseMode() *DuelMode {
	return d
}

// CreateChatPacket 创建聊天消息数据包并写入到指定的Writer
// src: 源聊天消息数据
// dst: 目标Writer接口
// dstPlayerType: 目标玩家类型
// 返回值: 写入的字节数（如果发生错误返回0）
func (d *DuelMode) CreateChatPacket(src []byte, dst io.Writer, dstPlayerType uint16) int {
	// 检查消息大小
	if !CheckMsgSize(len(src)) {
		return 0
	}

	// 检查消息是否为偶数字节（因为使用uint16）
	if len(src)%2 != 0 {
		return 0
	}

	// 将字节切片转换为uint16切片进行验证
	srcLen := len(src) / 2
	if srcLen == 0 || binary.LittleEndian.Uint16(src[srcLen*2-2:]) != 0 {
		return 0 // 检查消息是否以0结尾
	}

	// 写入玩家类型
	err := binary.Write(dst, binary.LittleEndian, dstPlayerType)
	if err != nil {
		return 0
	}

	// 写入消息内容
	n, err := dst.Write(src)
	if err != nil {
		return 0
	}

	// 返回总写入字节数
	return 2 + n // 2字节的玩家类型 + 消息长度
}
func (d *DuelMode) SendPacketToPlayer(dp *DuelPlayer, proto byte) {
	d.buffOffset = 3
	binary.LittleEndian.PutUint16(d.buff[:], 1)
	d.buff[2] = proto
	if dp != nil {
		if _, err := dp.Write(d.buff[:d.buffOffset]); err != nil {
			log.Printf("[duel] send packet 0x%02x to player %d: %v", proto, dp.Type, err)
		}
	}
}
func (d *DuelMode) SendPacketDataToPlayer(dp *DuelPlayer, proto byte, data any) {
	n, err := binary.Encode(d.buff[3:], binary.LittleEndian, data)
	if err != nil {
		return
	}
	d.buffOffset = n + 3
	binary.LittleEndian.PutUint16(d.buff[:], uint16(n+1))
	d.buff[2] = proto
	if dp != nil {
		if _, err := dp.Write(d.buff[:d.buffOffset]); err != nil {
			log.Printf("[duel] send packet 0x%02x to player %d: %v", proto, dp.Type, err)
		}
	}
}
func (d *DuelMode) DisconnectPlayer(dp *DuelPlayer) error {
	return dp.Conn.Close()
}
func (d *DuelMode) ReSendToPlayer(dp *DuelPlayer) {
	if dp != nil {
		if _, err := dp.Write(d.buff[:d.buffOffset]); err != nil {
			log.Printf("[duel] resend packet 0x%02x to player %d: %v", d.buff[2], dp.Type, err)
		}
	}
}

// 以下为 IDuelMode 中各模式行为一致的方法：实现为对 duel_common.go /
// duel_refresh.go 包级公共函数的一次转发，room 为构造时设置的具体模式自身。
// DuelMode 刻意不实现 JoinGame / UpdateDeck / StartDuel / TPResult / Analyze /
// Surrender / GetResponse / ToDuelList（各模式逻辑不同），因此裸 *DuelMode
// 不满足 IDuelMode——漏 override 会直接在嵌入它的模式类型上暴露为编译错误。

func (d *DuelMode) Chat(dp *DuelPlayer, pData []byte) {
	duelChat(d.room, dp, pData)
}

func (d *DuelMode) LeaveGame(dp *DuelPlayer) {
	leaveGame(d.room, dp)
}

func (d *DuelMode) ToObserver(dp *DuelPlayer) {
	toObserver(d.room, dp)
}

func (d *DuelMode) PlayerReady(dp *DuelPlayer, isReady bool) {
	playerReady(d.room, dp, isReady)
}

func (d *DuelMode) PlayerKick(dp *DuelPlayer, pos byte) {
	playerKick(d.room, dp, pos)
}

func (d *DuelMode) HandResult(dp *DuelPlayer, res byte) {
	handResult(d.room, dp, res)
}

func (d *DuelMode) Process() {
	processDuel(d.room)
}

func (d *DuelMode) TimeConfirm(dp *DuelPlayer) {
	timeConfirm(d.room, dp)
}

func (d *DuelMode) EndDuel() {
	endDuel(d.room)
}

func (d *DuelMode) WaitForResponse(player byte) {
	waitForResponse(d.room, player)
}

func (d *DuelMode) RefreshMzone(player int, flag uint32, useCache int) {
	refreshZone(d.room, player, int(ocgcore.LOCATION_MZONE), flag, useCache)
}

func (d *DuelMode) RefreshSzone(player int, flag uint32, useCache int) {
	refreshZone(d.room, player, int(ocgcore.LOCATION_SZONE), flag, useCache)
}

func (d *DuelMode) RefreshHand(player int, flag uint32, useCache int) {
	refreshHand(d.room, player, flag, useCache)
}

func (d *DuelMode) RefreshGrave(player int, flag uint32, useCache int) {
	refreshGrave(d.room, player, flag, useCache)
}

func (d *DuelMode) RefreshExtra(player int, flag uint32, useCache int) {
	refreshExtra(d.room, player, flag, useCache)
}

func (d *DuelMode) RefreshMzoneDef(player int) {
	refreshMzoneDef(d.room, player)
}

func (d *DuelMode) RefreshSzoneDef(player int) {
	refreshSzoneDef(d.room, player)
}

func (d *DuelMode) RefreshHandDef(player int) {
	refreshHandDef(d.room, player)
}

func (d *DuelMode) RefreshGraveDef(player int) {
	refreshGraveDef(d.room, player)
}

func (d *DuelMode) RefreshExtraDef(player int) {
	refreshExtraDef(d.room, player)
}

func (d *DuelMode) StopServer() {
	if NetServerEngine != nil {
		_ = NetServerEngine.Stop(context.Background())
	}
	if BroadcastInstance != nil {
		BroadcastInstance.Stop()
	}
}
func (d *DuelMode) StopListen() {
	AcceptingConnections.Store(false)
	if BroadcastInstance != nil {
		BroadcastInstance.Stop()
	}
}
func (d *DuelMode) OCGDuel() *ocgcore.Duel {
	return d.Duel

}
