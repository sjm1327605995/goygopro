package ocgcore

import (
	"bytes"
	"fmt"
	"sync"
)

// ErrorHandler 错误处理器函数类型
type ErrorHandler func(message string)

// Duel 决斗结构体
type Duel struct {
	duelPtr      uintptr
	buffer       []byte
	errorMu      sync.Mutex
	errorHandler ErrorHandler
}

var (
	duels    = make(map[uintptr]*Duel)
	duelLock sync.RWMutex
)

// NewDuel 创建新的决斗实例
//
// The C++ client derives the engine seed from the replay header through a real
// MT19937 step (std::mt19937 rnd(rh.seed); create_duel(rnd())). math/rand is a
// different generator, so the derived seed — and therefore every shuffle —
// would differ from the recorded duel. Use a local MT19937 instead.
func NewDuel(seed uint32) *Duel {
	rnd := newMT19937(seed)
	duelPtr := API.CreateDuel(int32(rnd.next()))
	return newDuel(duelPtr)
}

// mt19937 is the standard MT19937 generator, matching std::mt19937 bit-for-bit.
type mt19937 struct {
	state [624]uint32
	index int
}

func newMT19937(seed uint32) *mt19937 {
	m := &mt19937{index: 624}
	m.state[0] = seed
	for i := 1; i < 624; i++ {
		prev := m.state[i-1]
		m.state[i] = 1812433253*(prev^(prev>>30)) + uint32(i)
	}
	return m
}

func (m *mt19937) twist() {
	for i := 0; i < 624; i++ {
		y := (m.state[i] & 0x80000000) | (m.state[(i+1)%624] & 0x7fffffff)
		n := m.state[(i+397)%624] ^ (y >> 1)
		if y&1 != 0 {
			n ^= 0x9908b0df
		}
		m.state[i] = n
	}
	m.index = 0
}

func (m *mt19937) next() uint32 {
	if m.index >= 624 {
		m.twist()
	}
	y := m.state[m.index]
	m.index++
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18
	return y
}

// NewDuelV2 使用种子序列创建新的决斗实例（v2 API）
func NewDuelV2(seedSequence [8]uint32) *Duel {
	duelPtr := API.CreateDuelV2(&seedSequence)
	return newDuel(duelPtr)
}

func newDuel(duelPtr uintptr) *Duel {
	if duelPtr == 0 {
		return nil
	}
	d := &Duel{
		duelPtr: duelPtr,
		buffer:  make([]byte, 4096),
	}

	duelLock.Lock()
	duels[duelPtr] = d
	duelLock.Unlock()

	return d
}

// SetErrorHandler 设置错误处理器
func (d *Duel) SetErrorHandler(handler ErrorHandler) {
	d.errorMu.Lock()
	d.errorHandler = handler
	d.errorMu.Unlock()
}

// InitPlayers 初始化玩家
func (d *Duel) InitPlayers(startLp, startHand, drawCount int32) {
	API.SetPlayerInfo(d.duelPtr, 0, startLp, startHand, drawCount)
	API.SetPlayerInfo(d.duelPtr, 1, startLp, startHand, drawCount)
}

// InitPlayer 初始化单个玩家（故事决斗等双方基本分不同的场景；
// InitPlayers 是其双座同值的便捷封装）。
func (d *Duel) InitPlayer(player uint8, startLp, startHand, drawCount int32) {
	API.SetPlayerInfo(d.duelPtr, int32(player), startLp, startHand, drawCount)
}

// AddCard 添加卡片
func (d *Duel) AddCard(cardId uint32, owner int, location uint8) {
	API.NewCard(d.duelPtr, cardId, uint8(owner), uint8(owner),
		location, 0, uint8(CardPositionFaceDownDefence))
}

// AddTagCard 添加标签卡片
func (d *Duel) AddTagCard(cardId uint32, owner uint8, location uint8) {
	API.NewTagCard(d.duelPtr, cardId, owner, location)
}

// Start 开始决斗
func (d *Duel) Start(options int32) {
	API.StartDuel(d.duelPtr, options)
}

// SetResponse 设置响应（整数）
func (d *Duel) SetResponse(resp int32) {
	API.SetResponseI(d.duelPtr, resp)
}

// SetResponseBytes 设置响应（字节数组）
func (d *Duel) SetResponseBytes(resp []byte) error {

	API.SetResponseB(d.duelPtr, resp)
	return nil
}

// QueryFieldCount 查询场地卡片数量
func (d *Duel) QueryFieldCount(player uint8, location uint8) int {
	return int(API.QueryFieldCount(d.duelPtr, player, location))
}

// QueryFieldCard 查询场地卡片
// QueryFieldCard(int player, CardLocation location, int flag = 0xFFFFFF & ~(int)Query.ReasonCard, bool useCache = false)
func (d *Duel) QueryFieldCard(player uint8, location uint8, flag uint32, buff []byte, useCache bool) int32 {
	return API.QueryFieldCard(d.duelPtr, player, location, flag, buff, btoi(useCache))

}

const SIZE_QUERY_BUFFER = 0x4000

// QueryCard 查询卡片
func (d *Duel) QueryCard(player uint8, location uint8, sequence uint8, flag int32, buff []byte, useCache bool) int32 {
	return API.QueryCard(d.duelPtr, player, location, sequence, flag, buff, btoi(useCache))

}

// QueryFieldInfo 查询场地信息
func (d *Duel) QueryFieldInfo() []byte {
	API.QueryFieldInfo(d.duelPtr, d.buffer)
	result := make([]byte, 256)
	copy(result, d.buffer[:256])
	return result
}

// End 结束决斗
func (d *Duel) End() {
	API.EndDuel(d.duelPtr)
	d.Dispose()
}

// GetNativePtr 获取原生指针
func (d *Duel) GetNativePtr() uintptr {
	return d.duelPtr
}

// Dispose 释放资源
func (d *Duel) Dispose() {
	d.buffer = nil
	duelLock.Lock()
	delete(duels, d.duelPtr)
	duelLock.Unlock()
}

// OnMessage 处理消息（由 C 回调 defaultOnMessageHandler 经 duelLock 查表后
// 调用，可能与 SetErrorHandler 并发，故 errorHandler 读写都加 errorMu；
// handler 在锁外调用，避免其在回调里再次触碰 Duel 造成死锁）。
func (d *Duel) OnMessage(size uint32) {
	arr := make([]byte, 256)
	API.GetLogMessage(d.duelPtr, arr)
	message := string(bytes.TrimRight(arr, "\x00"))
	fmt.Println(message)
	d.errorMu.Lock()
	handler := d.errorHandler
	d.errorMu.Unlock()
	if handler != nil {
		handler(message)
	}
}

func (d *Duel) Process() uint32 {
	return API.Process(d.duelPtr)
}

func (d *Duel) GetMessage(buff []byte) int32 {
	return API.GetMessage(d.duelPtr, buff)
}
func btoi(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
