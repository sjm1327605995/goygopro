package duel

import (
	"encoding/binary"
	"fmt"
	"log"
	"reflect"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// --------------------------------------------------
// PacketRouter 路由注册中心
// --------------------------------------------------

// PacketRouter 是包级别的路由器，类似 gin.Engine
type PacketRouter struct {
	groups     []*PacketRouterGroup       // 所有路由组
	handlers   map[uint8][]PacketHandlerFunc // 消息类型 → 处理链
	middleware []PacketHandlerFunc          // 全局中间件
}

// PacketRouterGroup 路由组，支持给一组路由加前缀中间件
type PacketRouterGroup struct {
	router     *PacketRouter
	basePath   string                       // 保留字段，未来扩展
	middleware []PacketHandlerFunc          // 组级别中间件
}

// NewPacketRouter 创建一个新的路由器
func NewPacketRouter() *PacketRouter {
	return &PacketRouter{
		handlers: make(map[uint8][]PacketHandlerFunc),
	}
}

// Use 注册全局中间件
func (r *PacketRouter) Use(middleware ...PacketHandlerFunc) {
	r.middleware = append(r.middleware, middleware...)
}

// Group 创建路由组
func (r *PacketRouter) Group(path string, middleware ...PacketHandlerFunc) *PacketRouterGroup {
	return &PacketRouterGroup{
		router:     r,
		basePath:   path,
		middleware: append([]PacketHandlerFunc{}, middleware...),
	}
}

// Register 注册单个消息类型的处理链
func (r *PacketRouter) Register(pktType uint8, handlers ...PacketHandlerFunc) {
	// 合并：全局中间件 + 路由专属中间件
	chain := make([]PacketHandlerFunc, 0, len(r.middleware)+len(handlers))
	chain = append(chain, r.middleware...)
	chain = append(chain, handlers...)
	r.handlers[pktType] = chain
}

// Handle 是 Register 的别名
func (r *PacketRouter) Handle(pktType uint8, handlers ...PacketHandlerFunc) {
	r.Register(pktType, handlers...)
}

// Dispatch 分发消息，由 DuelPlayer.HandleCTOSPacket 调用
func (r *PacketRouter) Dispatch(player *DuelPlayer, data []byte) {
	if len(data) == 0 {
		return
	}
	pktType := data[0]
	handlers, ok := r.handlers[pktType]
	if !ok {
		// 未注册的消息类型，静默丢弃或记录日志
		log.Printf("[packet] unhandled type: 0x%02x from player %s", pktType, player.GetID())
		return
	}

	ctx := NewPacketContext(player, data)
	if ctx == nil {
		return
	}
	ctx.handlers = handlers
	ctx.Next()
}

// --------------------------------------------------
// PacketRouterGroup 方法
// --------------------------------------------------

// Use 给路由组添加中间件
func (g *PacketRouterGroup) Use(middleware ...PacketHandlerFunc) {
	g.middleware = append(g.middleware, middleware...)
}

// Register 在路由组内注册消息
func (g *PacketRouterGroup) Register(pktType uint8, handlers ...PacketHandlerFunc) {
	// 合并：全局中间件 + 组中间件 + 路由专属中间件
	chain := make([]PacketHandlerFunc, 0, len(g.router.middleware)+len(g.middleware)+len(handlers))
	chain = append(chain, g.router.middleware...)
	chain = append(chain, g.middleware...)
	chain = append(chain, handlers...)
	g.router.handlers[pktType] = chain
}

// Handle 是 Register 的别名
func (g *PacketRouterGroup) Handle(pktType uint8, handlers ...PacketHandlerFunc) {
	g.Register(pktType, handlers...)
}

// --------------------------------------------------
// 内置中间件
// --------------------------------------------------

// StateMiddleware 检查玩家状态是否合法
func StateMiddleware(c *PacketContext) {
	player := c.Player
	pktType := c.PktType
	if pktType != network.CTOS_SURRENDER && pktType != network.CTOS_CHAT {
		if player.State == 0xff || (player.State != 0 && player.State != pktType) {
			c.AbortWithError(ErrInvalidPlayerState(pktType, player.State))
			return
		}
	}
	c.Next()
}

// RecoverMiddleware 捕获 panic，防止单个请求导致服务崩溃
func RecoverMiddleware(c *PacketContext) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[packet] panic recovered: %v | player=%s pktType=0x%02x", r, c.Player.GetID(), c.PktType)
			c.SendError(ErrBadRequest, fmt.Sprintf("internal error: %v", r))
			c.Abort()
		}
	}()
	c.Next()
}

// RequireGame 检查玩家必须已经加入某个游戏
func RequireGame(c *PacketContext) {
	if c.Game() == nil {
		c.AbortWithError(ErrNeedGame())
		return
	}
	c.Next()
}

// RequireDuel 检查决斗已经开始（OCGDuel 不为 nil）
func RequireDuel(c *PacketContext) {
	g := c.Game()
	if g == nil || g.OCGDuel() == nil {
		c.AbortWithError(ErrNeedDuel())
		return
	}
	c.Next()
}

// RequireLobby 检查当前在大厅阶段（决斗未开始）
func RequireLobby(c *PacketContext) {
	g := c.Game()
	if g == nil || g.BaseMode() == nil || g.BaseMode().Duel != nil {
		c.AbortWithError(ErrNeedLobby())
		return
	}
	c.Next()
}

// ValidateLength(n) 返回一个中间件，检查 Payload 长度至少为 n
func ValidateLength(minLen int) PacketHandlerFunc {
	return func(c *PacketContext) {
		if len(c.Payload) < minLen {
			c.AbortWithError(ErrPayloadShort(len(c.Payload), minLen, c.PktType))
			return
		}
		c.Next()
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

	return func(c *PacketContext) {
		// 创建新实例
		val := reflect.New(typ).Interface()
		if err := restruct.Unpack(c.Payload, binary.LittleEndian, val); err != nil {
			c.AbortWithError(ErrBind(c.PktType, err))
			return
		}
		c.SetPayload(val)
		c.Next()
	}
}

// BindWithSize 是 Bind 的增强版，先检查长度再解析
func BindWithSize(sample interface{}, minLen int) PacketHandlerFunc {
	bind := Bind(sample)
	return func(c *PacketContext) {
		if len(c.Payload) < minLen {
			log.Printf("[packet] bind length check failed: need %d, got %d | pktType=0x%02x", minLen, len(c.Payload), c.PktType)
			c.Abort()
			return
		}
		bind(c)
	}
}

// --------------------------------------------------
// 便捷组合中间件（常用搭配）
// --------------------------------------------------

// GameAction 返回 [RequireGame + handler] 的组合
func GameAction(handler PacketHandlerFunc) []PacketHandlerFunc {
	return []PacketHandlerFunc{RequireGame, handler}
}

// DuelAction 返回 [RequireDuel + handler] 的组合
func DuelAction(handler PacketHandlerFunc) []PacketHandlerFunc {
	return []PacketHandlerFunc{RequireDuel, handler}
}

// LobbyAction 返回 [RequireLobby + handler] 的组合
func LobbyAction(handler PacketHandlerFunc) []PacketHandlerFunc {
	return []PacketHandlerFunc{RequireLobby, handler}
}

// BindAction 返回 [RequireGame + ValidateLength + Bind + handler] 的完整组合
func BindAction(sample interface{}, minLen int, handler PacketHandlerFunc) []PacketHandlerFunc {
	return []PacketHandlerFunc{
		RequireGame,
		ValidateLength(minLen),
		Bind(sample),
		handler,
	}
}
