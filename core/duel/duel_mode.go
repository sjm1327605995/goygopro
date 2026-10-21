package duel

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"sync"
	"unsafe"

	"github.com/antlabs/timer"
	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/core/data"
	"github.com/sjm1327605995/goygopro/core/packet"
	"github.com/sjm1327605995/goygopro/core/replay"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
)

type IDuelMode interface {
	BaseMode() *DuelMode
	// BaseRoom / DuelActive 满足 packet.Game（core/packet 的中间件经此
	// 持房间锁、查决斗状态）；由 *DuelMode 实现并提升给各模式。
	BaseRoom() packet.Room
	DuelActive() bool
	Chat(player *DuelPlayer, payload []byte)
	JoinGame(player *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool)
	LeaveGame(player *DuelPlayer)
	ToDuelList(player *DuelPlayer)
	ToObserver(player *DuelPlayer)
	PlayerReady(player *DuelPlayer, isReady bool)
	PlayerKick(player *DuelPlayer, pos byte)
	UpdateDeck(player *DuelPlayer, payload []byte)
	StartDuel(player *DuelPlayer)
	HandResult(player *DuelPlayer, res byte)
	TPResult(player *DuelPlayer, tp byte)
	Process()
	Analyze(msgBuffer []byte) int
	Surrender(player *DuelPlayer)
	GetResponse(player *DuelPlayer, payload []byte)
	TimeConfirm(player *DuelPlayer)
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
	RoomID      string   // 在 RoomManager 中的索引ID
	room        duelRoom // 构造时由 newSingleDuel/newTagDuel 二段设置为具体模式自身
	buff        [protocol.SIZE_NETWORK_BUFFER]byte // 单飞编码工作区：发出即拷贝（sendFrame），不得跨调用持有
	buffOffset  int
	Duel        *ocgcore.Duel
	startOffset int64
	lastReplay  *replay.Replay

	// 以下为 SingleDuel / TagDuel 共用的对局状态。
	// SingleDuel 只使用下标 0/1（两名玩家），TagDuel 使用 0..3。
	Observers    map[string]*DuelPlayer
	ready        [4]bool
	pDeck        [4]*data.Deck
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

// 房间共享帧缓冲的线格式常量（对应原版 SendBufferToPlayer 的 3 字节帧头：
// 2 字节小端长度 + 1 字节 proto，netserver.cpp）。
const (
	// frameLenFieldBytes 是帧头长度字段的字节数（小端 uint16，含 proto 字节）。
	frameLenFieldBytes = 2
	// frameProtoOffset 是 proto 字节在帧头内的偏移（紧随长度字段）。
	frameProtoOffset = frameLenFieldBytes
	// frameHeaderSize 是帧头总长 = 负载在共享缓冲中的起始偏移。
	frameHeaderSize = frameLenFieldBytes + 1
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
	// 原版 CreateChatPacket（netserver.cpp:374-386）只校验 NUL 结尾，
	// 仅含终止符的空消息（2 字节）照发。
	if size < SizeOfUint16 {
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

func (mode *DuelMode) BaseMode() *DuelMode {
	return mode
}

// BaseRoom 返回房间基座自身，满足 packet.Room（core/packet 的
// RoomLockMiddleware 经此持锁/二次确认，RequireLobby 经此查决斗状态）。
func (mode *DuelMode) BaseRoom() packet.Room {
	return mode
}

// Lock/Unlock 直通 Mu，满足 packet.Room 的 sync.Locker。
func (mode *DuelMode) Lock()   { mode.Mu.Lock() }
func (mode *DuelMode) Unlock() { mode.Mu.Unlock() }

// DuelStarted 报告引擎决斗是否已创建（Duel 字段非 nil），满足 packet.Room。
func (mode *DuelMode) DuelStarted() bool { return mode.Duel != nil }

// DuelActive 满足 packet.Game（旧 OCGDuel() != nil 判定的抽象）。
func (mode *DuelMode) DuelActive() bool { return mode.Duel != nil }

// CreateChatPacket 创建聊天消息数据包并写入到指定的Writer
// src: 源聊天消息数据
// dst: 目标Writer接口
// dstPlayerType: 目标玩家类型
// 返回值: 写入的字节数（如果发生错误返回0）
func (mode *DuelMode) CreateChatPacket(src []byte, dst io.Writer, dstPlayerType uint16) int {
	// 消息须为合法定长、偶数字节（uint16 语义）且以 NUL 结尾。
	if !CheckMsgSize(len(src)) || len(src) == 0 || len(src)%2 != 0 ||
		binary.LittleEndian.Uint16(src[len(src)-2:]) != 0 {
		return 0
	}

	var hdr [2]byte
	binary.LittleEndian.PutUint16(hdr[:], dstPlayerType)
	if _, err := dst.Write(hdr[:]); err != nil {
		return 0
	}
	written, err := dst.Write(src)
	if err != nil {
		return 0
	}
	return 2 + written // 2 字节玩家类型 + 消息内容
}

// encodeFrame 把 proto + data 编码进房间共享缓冲（C++ SendBufferToPlayer
// 的编码一次、多次复用 ReSend 的设计），返回完整帧；编码失败返回 nil
// （buffOffset 保持原值，与旧行为一致）。
//
// buff 是单飞工作区：返回的切片只在下一次 encodeFrame 之前有效，任何
// 跨调用存活的需求（发送、扇出）都由 sendFrame 拷贝承担——不要持有返回
// 切片越过下一次编码。
//
// 3 字节帧头（2 字节小端长度 + 1 字节 proto）刻意手写而非 codec.Marshal：
// 这是逐包热路径，省去每次 3 字节小分配；线格式与 codec.FrameHeader 一致
// （两侧均有黄金字节测试锁定）。负载按形态分派：
//   - nil：无负载（Len=1 仅 proto 的通知包）；
//   - codec.Frozen：序列化一次的不可变负载，直接 copy 线字节，零序列化；
//   - []byte：已编码负载（引擎消息路径），直接 copy；
//   - 其他：protocol 结构体，走 codec.MarshalInto 零分配编码（protocol
//     的 padding 已改为导出 Pad 字段，codec 对零值数组输出零字节，
//     见 protocol/types.go）。
func (mode *DuelMode) encodeFrame(proto byte, data any) []byte {
	payloadLen := 0
	switch payload := data.(type) {
	case nil:
	case codec.Frozen:
		payloadLen = copy(mode.buff[frameHeaderSize:], payload.Bytes())
	case []byte:
		payloadLen = copy(mode.buff[frameHeaderSize:], payload)
	default:
		var err error
		if payloadLen, err = codec.MarshalInto(mode.buff[frameHeaderSize:], data); err != nil {
			return nil
		}
	}
	mode.buffOffset = payloadLen + frameHeaderSize
	binary.LittleEndian.PutUint16(mode.buff[:frameLenFieldBytes], uint16(payloadLen+1))
	mode.buff[frameProtoOffset] = proto
	return mode.buff[:mode.buffOffset]
}

// sendFrame 把共享缓冲的当前帧发给 player；nil 玩家静默跳过（原版语义），
// 写失败仅记日志（对端可能刚断线，OnClose 会随后清理）。
//
// buff 是单飞工作区：每次发送先 copy 成独立帧再交出——异步出站会持有该
// 切片直到冲刷，扇出（ReSend*）时每位接收者各持一份拷贝；帧内容的跨接收
// 组差异（如 MSG_START 的视角字节改写）发生在两次 sendFrame 之间、锁内
// 完成，拷贝时序天然正确。per-conn 顺序由 gnet 出站队列 FIFO 保证。
func (mode *DuelMode) sendFrame(player *DuelPlayer) {
	if player == nil {
		return
	}
	proto := mode.buff[frameProtoOffset]
	frame := append([]byte(nil), mode.buff[:mode.buffOffset]...)
	if _, err := player.Write(frame); err != nil {
		log.Printf("[duel] send packet 0x%02x to player %d: %v", proto, player.Type, err)
	}
}

// ReSendToObservers 把当前帧发给全部旁观者（ReSend 的观战扇出）。
func (mode *DuelMode) ReSendToObservers() {
	for _, observer := range mode.Observers {
		mode.sendFrame(observer)
	}
}

func (mode *DuelMode) SendPacketToPlayer(player *DuelPlayer, proto byte) {
	if mode.encodeFrame(proto, nil) != nil {
		mode.sendFrame(player)
	}
}

func (mode *DuelMode) SendPacketDataToPlayer(player *DuelPlayer, proto byte, data any) {
	if mode.encodeFrame(proto, data) != nil {
		mode.sendFrame(player)
	}
}

func (mode *DuelMode) DisconnectPlayer(player *DuelPlayer) error {
	return player.Conn.Close()
}

func (mode *DuelMode) ReSendToPlayer(player *DuelPlayer) {
	mode.sendFrame(player)
}

// ReSendToPlayers 把当前帧发给多个玩家（nil 跳过），收束成组 ReSend 的样板。
func (mode *DuelMode) ReSendToPlayers(players ...*DuelPlayer) {
	for _, player := range players {
		mode.sendFrame(player)
	}
}

// 以下为 IDuelMode 中各模式行为一致的方法：实现为对 duel_common.go /
// duel_refresh.go 包级公共函数的一次转发，room 为构造时设置的具体模式自身。
// DuelMode 刻意不实现 JoinGame / UpdateDeck / StartDuel / TPResult / Analyze /
// Surrender / GetResponse / ToDuelList（各模式逻辑不同），因此裸 *DuelMode
// 不满足 IDuelMode——漏 override 会直接在嵌入它的模式类型上暴露为编译错误。

func (mode *DuelMode) Chat(player *DuelPlayer, payload []byte) {
	duelChat(mode.room, player, payload)
}

func (mode *DuelMode) LeaveGame(player *DuelPlayer) {
	leaveGame(mode.room, player)
}

func (mode *DuelMode) ToObserver(player *DuelPlayer) {
	toObserver(mode.room, player)
}

func (mode *DuelMode) PlayerReady(player *DuelPlayer, isReady bool) {
	playerReady(mode.room, player, isReady)
}

func (mode *DuelMode) PlayerKick(player *DuelPlayer, pos byte) {
	playerKick(mode.room, player, pos)
}

func (mode *DuelMode) HandResult(player *DuelPlayer, res byte) {
	handResult(mode.room, player, res)
}

func (mode *DuelMode) Process() {
	processDuel(mode.room)
}

func (mode *DuelMode) TimeConfirm(player *DuelPlayer) {
	timeConfirm(mode.room, player)
}

func (mode *DuelMode) EndDuel() {
	endDuel(mode.room)
}

func (mode *DuelMode) WaitForResponse(player byte) {
	waitForResponse(mode.room, player)
}

func (mode *DuelMode) RefreshMzone(player int, flag uint32, useCache int) {
	refreshZone(mode.room, player, int(ocgcore.LOCATION_MZONE), flag, useCache)
}

func (mode *DuelMode) RefreshSzone(player int, flag uint32, useCache int) {
	refreshZone(mode.room, player, int(ocgcore.LOCATION_SZONE), flag, useCache)
}

func (mode *DuelMode) RefreshHand(player int, flag uint32, useCache int) {
	refreshHand(mode.room, player, flag, useCache)
}

func (mode *DuelMode) RefreshGrave(player int, flag uint32, useCache int) {
	refreshGrave(mode.room, player, flag, useCache)
}

func (mode *DuelMode) RefreshExtra(player int, flag uint32, useCache int) {
	refreshExtra(mode.room, player, flag, useCache)
}

func (mode *DuelMode) RefreshMzoneDef(player int) {
	refreshMzoneDef(mode.room, player)
}

func (mode *DuelMode) RefreshSzoneDef(player int) {
	refreshSzoneDef(mode.room, player)
}

func (mode *DuelMode) RefreshHandDef(player int) {
	refreshHandDef(mode.room, player)
}

func (mode *DuelMode) RefreshGraveDef(player int) {
	refreshGraveDef(mode.room, player)
}

func (mode *DuelMode) RefreshExtraDef(player int) {
	refreshExtraDef(mode.room, player)
}

func (mode *DuelMode) StopServer() {
	if NetServerEngine != nil {
		_ = NetServerEngine.Stop(context.Background())
	}
	StopBroadcast()
}
func (mode *DuelMode) StopListen() {
	AcceptingConnections.Store(false)
	// 原版 netserver.cpp:79-82 的 StopListen 同样连带 StopBroadcast——
	// single_duel.cpp:325 / tag_duel.cpp:289 的 StopBroadcast 注释掉正是
	// 因为 StopListen 已代劳，决斗期间不应答 LAN 发现广播与原版一致。
	// 下一次建房由 EnsureBroadcast 恢复应答（对齐 CTOS_CREATE_GAME
	// 即 StartBroadcast 的语义）。
	StopBroadcast()
}
func (mode *DuelMode) OCGDuel() *ocgcore.Duel {
	return mode.Duel
}
