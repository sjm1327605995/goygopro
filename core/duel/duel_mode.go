package duel

import (
	"encoding/binary"
	"github.com/antlabs/timer"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"io"
	"unsafe"
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
	HostPlayer  *DuelPlayer
	HostInfo    protocol.HostInfo
	DuelStage   int
	ETimer      timer.TimeNoder
	Name        [20]uint16
	Pass        [20]uint16
	buff        [protocol.SIZE_NETWORK_BUFFER]byte
	buffOffset  int
	Duel        *ocgcore.Duel
	startOffset int64
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

// 可选：提供一个更符合Go风格的版本
// 在Go中，我们通常不需要关心类型的大小，因为它们是固定的
// 这个版本更简洁，但功能相同

// CheckMessageSize 检查UTF-16编码消息大小是否合法（Go风格版本）
func CheckMessageSize(sizeInBytes int) bool {
	// 每个UTF-16字符占2字节
	chars := sizeInBytes / 2

	// 至少需要一个字符和一个null终止符
	if chars < 2 {
		return false
	}

	// 不能超过最大长度
	if chars > LenChatMsg {
		return false
	}

	// 必须是偶数字节（UTF-16字符的整数倍）
	if sizeInBytes%2 != 0 {
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
		_, _ = dp.Write(d.buff[:d.buffOffset])
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
		_, _ = dp.Write(d.buff[:d.buffOffset])
	}
}
func (d *DuelMode) DisconnetPlayer(dp *DuelPlayer) error {
	return dp.Conn.Close()
}
func (d *DuelMode) ReSendToPlayer(dp *DuelPlayer) {
	if dp != nil {
		_, _ = dp.Write(d.buff[:d.buffOffset])
	}
}
func (d *DuelMode) Chat(dp *DuelPlayer, pData []byte) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) JoinGame(dp *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) LeaveGame(dp *DuelPlayer) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) ToDuelList(dp *DuelPlayer) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) ToObserver(dp *DuelPlayer) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) PlayerReady(dp *DuelPlayer, isReady bool) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) PlayerKick(dp *DuelPlayer, pos byte) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) UpdateDeck(dp *DuelPlayer, pData []byte) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) StartDuel(dp *DuelPlayer) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) HandResult(dp *DuelPlayer, res byte) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) TPResult(dp *DuelPlayer, tp byte) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) Process() {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) Analyze(msgBuffer []byte) int {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) Surrender(dp *DuelPlayer) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) GetResponse(dp *DuelPlayer, msgBuffer []byte) {
	panic("implement me")
}

func (d DuelMode) TimeConfirm(dp *DuelPlayer) {
	//TODO implement me
	panic("implement me")
}

func (d DuelMode) EndDuel() {
	//TODO implement me
	panic("implement me")
}
func (d DuelMode) StopServer() {

}
func (d DuelMode) StopListen() {

}
func (d DuelMode) OCGDuel() *ocgcore.Duel {
	return d.Duel

}
