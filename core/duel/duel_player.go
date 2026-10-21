package duel

import (
	"errors"
	"fmt"
	"log"

	"github.com/panjf2000/gnet/v2"
	"github.com/sjm1327605995/goygopro/core/packet"
	"github.com/sjm1327605995/goygopro/core/utils"
)

// maxOutboundBuffered 是单连接出站缓冲的背压阈值（1MB）：AsyncWrite 把帧
// 入队到 gnet 的 per-conn 出站缓冲后立即返回，慢客户端不读时缓冲只增不
// 减；超过阈值说明该客户端已掉队约 1MB 的待发数据，继续排队只会拖垮服
// 务端内存，直接断开走既有 leaveGameOnce 清理，对端玩家不受影响。
const maxOutboundBuffered = 1 << 20

type DuelPlayer struct {
	ID    string
	Type  uint8
	Name  [20]uint16
	Game  IDuelMode
	Conn  gnet.Conn
	State uint8
}

func (player *DuelPlayer) GetID() string {
	return player.ID
}
func (player *DuelPlayer) SetID(id string) {
	player.ID = id
}

// Write 把已编码帧异步发往客户端。帧的所有权归调用方：本方法经
// Conn.AsyncWrite 把字节拷贝进 gnet 的 per-conn 出站队列后立即返回
// （并发安全、per-conn FIFO 保序），不在调用方 goroutine（房间锁内）上
// 写 socket——慢客户端不再拖死全房间。真正的写错误异步暴露（gnet 回调/
// OnClose 善后），与旧同步路径"错误仅记日志、断线走 leaveGameOnce"的
// 语义一致。
//
// 出站前做背压检查：per-conn 出站缓冲超过 maxOutboundBuffered 的慢
// 客户端直接 Close（并记日志），由既有断线清理移出房间。
func (player *DuelPlayer) Write(data []byte) (int, error) {
	if player.Conn == nil {
		return 0, errors.New("player connection is nil")
	}
	if pending := player.Conn.OutboundBuffered(); pending > maxOutboundBuffered {
		log.Printf("[duel] player %q outbound buffer %d bytes exceeds limit %d, closing slow client",
			player.LogName(), pending, maxOutboundBuffered)
		_ = player.Conn.Close()
		return 0, fmt.Errorf("player %q outbound buffer %d exceeds limit %d", player.LogName(), pending, maxOutboundBuffered)
	}
	if err := player.Conn.AsyncWrite(data, nil); err != nil {
		return 0, err
	}
	return len(data), nil
}
func (player *DuelPlayer) Close() error {
	return player.Conn.Close()
}

// LogName 是日志用显示名，满足 packet.Peer（原版 utils.WideString(Name[:])）。
func (player *DuelPlayer) LogName() string {
	return utils.WideString(player.Name[:])
}

// PlayerState 暴露 CTOS 状态机状态，满足 packet.Peer（StateMiddleware 用）。
func (player *DuelPlayer) PlayerState() uint8 {
	return player.State
}

// CurrentGame 动态返回玩家所在游戏，满足 packet.Peer（PacketContext.Game 用）。
func (player *DuelPlayer) CurrentGame() packet.Game {
	return player.Game
}

var (
	CURRENT_RULE uint8 = 5
	MODE_SINGLE  uint8 = 0
	MODE_MATCH   uint8 = 1
	MODE_TAG     uint8 = 2
)

// PRO_VERSION 是服务器支持的客户端协议版本（原版 COMMON 的 PRO_VERSION）。
const PRO_VERSION = 0x1361

func (player *DuelPlayer) HandleCTOSPacket(data []byte) {
	// 使用全局路由器分发消息
	// router 在包初始化时由 BuildPacketRouter() 创建
	packetRouter.Dispatch(player, data)
}

// leaveGameOnce 把玩家移出所在房间（若在），并保证只执行一次。
// LeaveGame 不幂等（host 路径会重复 EndDuel/RemoveRoom），主动离开
// （CTOS_LEAVE_GAME）与 TCP 断线（OnClose）两条路径都必须经由这里，
// 通过清空 Game 引用来防止二次触发。
//
// 房间锁在这里统一获取，而不是挂在 CTOS_LEAVE_GAME 路由的
// RoomLockMiddleware 上：OnClose（server_engine.go）是 gnet 事件循环
// 回调、不走路由，middleware 罩不到它；multicore 模式下它可能与持锁的
// CTOS 处理并发，共享的 DuelMode.buff/玩家表会被竞态写坏。两条路径都
// 收敛到本函数锁内执行后，LeaveGame 的所有入口持有同一把锁
// （CTOS 路由因此不再挂 RoomLockMiddleware，避免重复加锁死锁）。
//
// player.Game 的清空放在锁内并二次确认：捕获 game 到真正持锁之间，玩家可能
// 已被 playerKick 在同一把锁内清引用并移出（playerKick 先于 LeaveGame
// 置 players[pos].Game = nil，与本函数同语义）。锁内再比对一次，重复
// 入口（含被踢者随后断线触发的 OnClose）在此直接返回，不会二次执行
// LeaveGame（二次会重复广播 PLAYERCHANGE_LEAVE 等）。Game 非 nil 期间
// RequireNotInGame 拒绝加入，「离开」对房间锁是原子的，不会留下
// 「引用已清但座位未摘」的中间态。
func (player *DuelPlayer) leaveGameOnce() {
	game := player.Game
	if game == nil {
		return
	}
	base := game.BaseMode()
	base.Mu.Lock()
	defer base.Mu.Unlock()
	if player.Game != game {
		// 等锁期间已被踢出/已离开（kick 路径在同锁内处理过）
		return
	}
	player.Game = nil
	game.LeaveGame(player)
}

// packetRouter 是全局路由器实例
var packetRouter = BuildPacketRouter()

// SetPacketRouter 替换全局 CTOS 路由器，供嵌入方/测试注入自定义路由表。
// 只需在启动服务前调用一次（分发路径每次读取包级变量，但不做同步，
// 运行期替换与并发分发不是受支持的用法）。
func SetPacketRouter(r *packet.PacketRouter) {
	packetRouter = r
}
