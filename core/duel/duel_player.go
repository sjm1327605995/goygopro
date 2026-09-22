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

func (d *DuelPlayer) GetID() string {
	return d.ID
}
func (d *DuelPlayer) SetID(id string) {
	d.ID = id
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

// PRO_VERSION 是服务器支持的客户端协议版本（原版 COMMON 的 PRO_VERSION）。
const PRO_VERSION = 0x1361

func (d *DuelPlayer) HandleCTOSPacket(data []byte) {
	// 使用全局路由器分发消息
	// router 在包初始化时由 BuildRouter() 创建
	packetRouter.Dispatch(d, data)
}

// leaveGameOnce 把玩家移出所在房间（若在），并保证只执行一次。
// LeaveGame 不幂等（host 路径会重复 EndDuel/RemoveRoom），主动离开
// （CTOS_LEAVE_GAME）与 TCP 断线（OnClose）两条路径都必须经由这里，
// 通过清空 Game 引用来防止二次触发。
func (d *DuelPlayer) leaveGameOnce() {
	game := d.Game
	if game == nil {
		return
	}
	d.Game = nil
	game.LeaveGame(d)
}

// packetRouter 是全局路由器实例
var packetRouter = BuildPacketRouter()

// SetPacketRouter 替换全局 CTOS 路由器，供嵌入方/测试注入自定义路由表。
// 只需在启动服务前调用一次（分发路径每次读取包级变量，但不做同步，
// 运行期替换与并发分发不是受支持的用法）。
func SetPacketRouter(r *PacketRouter) {
	packetRouter = r
}
