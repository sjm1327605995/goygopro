package packet

import (
	"fmt"
	"log"
	"reflect"

	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// --------------------------------------------------
// PacketRouter 路由注册中心
// --------------------------------------------------

// PacketRouter 是包级别的路由器，类似 gin.Engine
type PacketRouter struct {
	handlers   map[uint8][]PacketHandlerFunc // 消息类型 → 处理链
	middleware []PacketHandlerFunc           // 全局中间件
}

// PacketRouterGroup 路由组，共享一组组级别中间件
type PacketRouterGroup struct {
	router     *PacketRouter
	middleware []PacketHandlerFunc // 组级别中间件
}

// NewPacketRouter 创建一个新的路由器
func NewPacketRouter() *PacketRouter {
	return &PacketRouter{
		handlers: make(map[uint8][]PacketHandlerFunc),
	}
}

// Use 注册全局中间件。注意：只影响之后注册的链，必须在所有 Register 之前调用。
func (router *PacketRouter) Use(middleware ...PacketHandlerFunc) {
	router.middleware = append(router.middleware, middleware...)
}

// Group 创建路由组，组内注册的路由共享组级别中间件
func (router *PacketRouter) Group(middleware ...PacketHandlerFunc) *PacketRouterGroup {
	return &PacketRouterGroup{
		router:     router,
		middleware: append([]PacketHandlerFunc{}, middleware...),
	}
}

// Register 注册单个消息类型的处理链
func (router *PacketRouter) Register(pktType uint8, handlers ...PacketHandlerFunc) {
	if _, dup := router.handlers[pktType]; dup {
		panic(fmt.Sprintf("packet: duplicate registration for pktType 0x%02x", pktType))
	}
	// 合并：全局中间件 + 路由专属中间件
	chain := make([]PacketHandlerFunc, 0, len(router.middleware)+len(handlers))
	chain = append(chain, router.middleware...)
	chain = append(chain, handlers...)
	router.handlers[pktType] = chain
}

// Handle 是 Register 的别名
func (router *PacketRouter) Handle(pktType uint8, handlers ...PacketHandlerFunc) {
	router.Register(pktType, handlers...)
}

// Dispatch 分发消息，由 DuelPlayer.HandleCTOSPacket 调用
func (router *PacketRouter) Dispatch(peer Peer, data []byte) {
	if len(data) == 0 {
		return
	}
	pktType := data[0]
	utils.NetLogf("server", "recv CTOS %s (0x%02x) len=%d player=%q state=0x%02x",
		network.CTOSName(pktType), pktType, len(data), peer.LogName(), peer.PlayerState())
	handlers, ok := router.handlers[pktType]
	if !ok {
		// 未注册的消息类型，静默丢弃或记录日志
		log.Printf("[packet] unhandled type: 0x%02x from player %s", pktType, peer.GetID())
		return
	}

	ctx := NewPacketContext(peer, data)
	ctx.handlers = handlers
	ctx.Next()
}

// --------------------------------------------------
// PacketRouterGroup 方法
// --------------------------------------------------

// Use 给路由组添加中间件
func (group *PacketRouterGroup) Use(middleware ...PacketHandlerFunc) {
	group.middleware = append(group.middleware, middleware...)
}

// Register 在路由组内注册消息
func (group *PacketRouterGroup) Register(pktType uint8, handlers ...PacketHandlerFunc) {
	if _, dup := group.router.handlers[pktType]; dup {
		panic(fmt.Sprintf("packet: duplicate registration for pktType 0x%02x", pktType))
	}
	// 合并：全局中间件 + 组中间件 + 路由专属中间件
	chain := make([]PacketHandlerFunc, 0, len(group.router.middleware)+len(group.middleware)+len(handlers))
	chain = append(chain, group.router.middleware...)
	chain = append(chain, group.middleware...)
	chain = append(chain, handlers...)
	group.router.handlers[pktType] = chain
}

// Handle 是 Register 的别名
func (group *PacketRouterGroup) Handle(pktType uint8, handlers ...PacketHandlerFunc) {
	group.Register(pktType, handlers...)
}

// --------------------------------------------------
// 内置中间件
// --------------------------------------------------

// StateMiddleware 检查玩家状态是否合法
func StateMiddleware(ctx *PacketContext) {
	player := ctx.Peer
	pktType := ctx.PktType
	if pktType != network.CTOS_SURRENDER && pktType != network.CTOS_CHAT {
		if player.PlayerState() == 0xff || (player.PlayerState() != 0 && player.PlayerState() != pktType) {
			ctx.AbortWithError(ErrInvalidPlayerState(pktType, player.PlayerState()))
			return
		}
	}
	ctx.Next()
}

// RecoverMiddleware 捕获 panic，防止单个请求导致服务崩溃
func RecoverMiddleware(ctx *PacketContext) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("[packet] panic recovered: %v | player=%s pktType=0x%02x", recovered, ctx.Peer.GetID(), ctx.PktType)
			ctx.SendError(ErrBadRequest, fmt.Sprintf("internal error: %v", recovered))
			ctx.Abort()
		}
	}()
	ctx.Next()
}

// RequireGame 检查玩家必须已经加入某个游戏
func RequireGame(ctx *PacketContext) {
	if ctx.Game() == nil {
		ctx.AbortWithError(ErrNeedGame())
		return
	}
	ctx.Next()
}

// RequireDuel 检查决斗已经开始（引擎决斗不为 nil）
func RequireDuel(ctx *PacketContext) {
	g := ctx.Game()
	if g == nil || !g.DuelActive() {
		ctx.AbortWithError(ErrNeedDuel())
		return
	}
	ctx.Next()
}

// RequireLobby 检查当前在大厅阶段（决斗未开始）
func RequireLobby(ctx *PacketContext) {
	g := ctx.Game()
	if g == nil || g.BaseRoom() == nil || g.BaseRoom().DuelStarted() {
		ctx.AbortWithError(ErrNeedLobby())
		return
	}
	ctx.Next()
}

// ValidateLength(n) 返回一个中间件，检查 Payload 长度至少为 n
func ValidateLength(minLen int) PacketHandlerFunc {
	return func(ctx *PacketContext) {
		if len(ctx.Payload) < minLen {
			ctx.AbortWithError(ErrPayloadShort(len(ctx.Payload), minLen, ctx.PktType))
			return
		}
		ctx.Next()
	}
}

// Bind 返回一个中间件，自动将 Payload 解析为指定类型的结构体
// 用法: Bind(protocol.CTOSPlayerInfo{}) 或 Bind(&protocol.CTOSPlayerInfo{})
func Bind(sample interface{}) PacketHandlerFunc {
	typ := reflect.TypeOf(sample)
	// 如果传入的是指针，取 elem
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return func(ctx *PacketContext) {
		// 创建新实例
		val := reflect.New(typ).Interface()
		// codec.Unmarshal 与 restruct.Unpack 同语义：按布局消费前缀、
		// 容忍 Payload 尾部多余字节（部分 CTOS 包带变长尾）。
		if err := codec.Unmarshal(ctx.Payload, val); err != nil {
			ctx.AbortWithError(ErrBind(ctx.PktType, err))
			return
		}
		ctx.SetPayload(val)
		ctx.Next()
	}
}

// RoomLockMiddleware 保证进入该中间件后的操作在房间互斥锁保护下执行。
// 注意：RequireGame 与本中间件之间的窗口内，玩家可能被踢/离开
// （leaveGameOnce 把 Player.Game 置 nil），因此加锁后必须二次确认
// 玩家仍在捕获时那个房间，否则中止链路，避免后续 handler 解引用 nil。
func RoomLockMiddleware(ctx *PacketContext) {
	g := ctx.Game()
	if g == nil || g.BaseRoom() == nil {
		ctx.AbortWithError(ErrNeedGame())
		return
	}
	room := g.BaseRoom()
	room.Lock()
	defer room.Unlock()
	if ctx.Game() != g {
		// 持锁期间玩家已被移出该房间（如被房主踢出）
		ctx.AbortWithError(ErrNeedGame())
		return
	}
	ctx.Next()
}
