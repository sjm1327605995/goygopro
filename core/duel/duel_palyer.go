package duel

import (
	"github.com/panjf2000/gnet/v2"
)

type DuelPlayer struct {
	ID    string
	Type  uint8
	Name  [20]uint16
	Game  IDuelMode
	Conn  gnet.Conn
	State uint8
}

func (d *DuelPlayer) SetHostPlayer() {
	//TODO implement me
	panic("implement me")
}

func (d *DuelPlayer) GetID() string {
	return d.ID
}
func (d *DuelPlayer) SetID(id string) {
	d.ID = id
}
func (d *DuelPlayer) Disconnect() {
	d.Conn.Close()
}

func (d *DuelPlayer) Write(data []byte) (int, error) {
	return d.Conn.Write(data)
}
func (d *DuelPlayer) Close() error {
	return d.Conn.Close()
}

var (
	CURRENT_RULE uint8 = 5
	MODE_SINGLE  uint8 = 0
	MODE_MATCH   uint8 = 1
	MODE_TAG     uint8 = 2
)

func (d *DuelPlayer) HandleCTOSPacket(data []byte) {
	// 使用全局路由器分发消息
	// router 在包初始化时由 BuildRouter() 创建
	packetRouter.Dispatch(d, data)
}

// packetRouter 是全局路由器实例
var packetRouter = BuildPacketRouter()
