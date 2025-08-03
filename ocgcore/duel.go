package ocgcore

import "C"
import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"sync"
)

// ErrorHandler 错误处理器函数类型
type ErrorHandler func(message string)

// Duel 决斗结构体
type Duel struct {
	duelPtr      uintptr
	buffer       []byte
	errorHandler ErrorHandler
}

var (
	duels    = make(map[uintptr]*Duel)
	duelLock sync.RWMutex
)

// NewDuel 创建新的决斗实例
func NewDuel(seed uint32) *Duel {
	random := rand.New(rand.NewSource(int64(seed)))
	duelPtr := API.CreateDuel(random.Int31())
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
	d.errorHandler = handler
}

// InitPlayers 初始化玩家
func (d *Duel) InitPlayers(startLp, startHand, drawCount int32) {
	API.SetPlayerInfo(d.duelPtr, 0, startLp, startHand, drawCount)
	API.SetPlayerInfo(d.duelPtr, 1, startLp, startHand, drawCount)
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
	if len(resp) > 64 {
		return errors.New("response too long")
	}
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

// QueryCard(int player, int location, int sequence, int flag = 0xFFFFFF & ~(int)Query.ReasonCard, bool useCache = false)
func (d *Duel) QueryFieldCardDef(player uint8, location uint8) int32 {
	flag := 0xFFFFFF &^ uint32(QueryReasonCard) // Go中的按位取反运算符是^
	var buff = make([]byte, SIZE_QUERY_BUFFER)
	return d.QueryFieldCard(player, location, flag, buff, false)
}

// QueryCard 查询卡片
func (d *Duel) QueryCard(player uint8, location uint8, sequence uint8, flag int32, buff []byte, useCache bool) int32 {
	return API.QueryCard(d.duelPtr, player, location, sequence, flag, buff, btoi(useCache))

}
func (d *Duel) QueryCardDef(player uint8, location uint8, sequence uint8, buff []byte) int32 {
	flag := 0xFFFFFF &^ int(QueryReasonCard) // Go中的按位取反运算符是^
	return d.QueryCard(player, location, sequence, int32(flag), buff, false)
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

// OnMessage 处理消息
func (d *Duel) OnMessage(size uint32) {
	arr := make([]byte, 256)
	API.GetLogMessage(d.duelPtr, arr)
	message := string(bytes.TrimRight(arr, "\x00"))
	fmt.Println(message)
	if d.errorHandler != nil {
		d.errorHandler(message)
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
