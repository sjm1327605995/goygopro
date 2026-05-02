package duel

import (
	"unicode/utf16"

	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// --------------------------------------------------
// CTOS_PLAYER_INFO — 玩家信息设置
// --------------------------------------------------
func HandlePlayerInfo(c *PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSPlayerInfo)
	utils.NullTerminate(pkt.Name[:], 0)
	copy(c.Player.Name[:], pkt.Name[:])
	c.Player.SetID(string(utf16.Decode(c.Player.Name[:])))
}

// --------------------------------------------------
// CTOS_CREATE_GAME — 创建房间
// --------------------------------------------------
func HandleCreateGame(c *PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSCreateGame)
	if c.Player.Game != nil {
		c.AbortWithError(ErrAlreadyInGameAction())
		return
	}

	// 规则修正
	if pkt.Info.Rule > CURRENT_RULE {
		pkt.Info.Rule = CURRENT_RULE
	}
	if pkt.Info.Mode > MODE_TAG {
		pkt.Info.Mode = MODE_SINGLE
	}

	// 校验禁卡表
	var found bool
	for _, lfList := range DeckManger.LFList {
		if pkt.Info.LFList == lfList.Hash {
			found = true
			break
		}
	}
	if !found {
		if len(DeckManger.LFList) > 0 {
			pkt.Info.LFList = DeckManger.LFList[0].Hash
		} else {
			pkt.Info.LFList = 0
		}
	}

	// 创建模式实例
	var mode IDuelMode
	switch pkt.Info.Mode {
	case MODE_SINGLE:
		mode = &SingleDuel{Observers: make(map[string]*DuelPlayer)}
	case MODE_MATCH:
		mode = &SingleDuel{Observers: make(map[string]*DuelPlayer), MatchMode: true}
	case MODE_TAG:
		mode = &TagDuel{Observers: make(map[string]*DuelPlayer)}
	default:
		return
	}

	mode.BaseMode().HostInfo = pkt.Info
	utils.NullTerminate(pkt.Name[:], 0)
	utils.NullTerminate(pkt.Pass[:], 0)
	copy(mode.BaseMode().Name[:], pkt.Name[:])
	copy(mode.BaseMode().Pass[:], pkt.Pass[:])

	roomId := string(utf16.Decode(pkt.Pass[:]))
	room, _ := DefaultManager.JoinRoom(roomId, c.Player, mode)
	c.Player.Game = room.DuelMode
	c.Player.Game.JoinGame(c.Player, nil, true)
}

// --------------------------------------------------
// CTOS_JOIN_GAME — 加入房间
// --------------------------------------------------
func HandleJoinGame(c *PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSJoinGame)
	roomId := string(utf16.Decode(pkt.Pass[:]))
	room, isCreator := DefaultManager.JoinRoom(roomId, c.Player, nil)
	c.Player.Game = room.DuelMode
	c.Player.Game.JoinGame(c.Player, pkt, isCreator)
}

// --------------------------------------------------
// CTOS_LEAVE_GAME — 离开房间（统一错误处理）
// --------------------------------------------------
func HandleLeaveGame(c *PacketContext) {
	if c.Player.Game == nil {
		c.AbortWithError(ErrNeedGame())
		return
	}
	c.Player.Game.LeaveGame(c.Player)
}

// --------------------------------------------------
// CTOS_SURRENDER — 投降（统一错误处理）
// --------------------------------------------------
func HandleSurrender(c *PacketContext) {
	if c.Player.Game == nil {
		c.AbortWithError(ErrNeedGame())
		return
	}
	c.Player.Game.Surrender(c.Player)
}

// --------------------------------------------------
// CTOS_RESPONSE — 响应/操作
// --------------------------------------------------
func HandleResponse(c *PacketContext) {
	c.Player.Game.GetResponse(c.Player, c.Payload)
}

// --------------------------------------------------
// CTOS_TIME_CONFIRM — 时间确认（统一错误处理）
// --------------------------------------------------
func HandleTimeConfirm(c *PacketContext) {
	if c.Player.Game == nil {
		c.AbortWithError(ErrNeedGame())
		return
	}
	if c.Player.Game.OCGDuel() == nil {
		c.AbortWithError(ErrNeedDuel())
		return
	}
	c.Player.Game.TimeConfirm(c.Player)
}

// --------------------------------------------------
// CTOS_CHAT — 聊天（统一错误处理）
// --------------------------------------------------
func HandleChat(c *PacketContext) {
	if c.Player.Game == nil {
		c.AbortWithError(ErrNeedGame())
		return
	}
	pData := c.Payload
	if len(pData) < 2 {
		c.AbortWithError(ErrPayloadShort(len(pData), 2, c.PktType))
		return
	}
	if len(pData) > protocol.LEN_CHAT_MSG*2 {
		c.AbortWithError(NewPacketError(ErrPayloadTooShort, "chat msg too long"))
		return
	}
	if len(pData)%2 != 0 {
		c.AbortWithError(NewPacketError(ErrBadRequest, "chat msg length must be even"))
		return
	}
	c.Player.Game.Chat(c.Player, pData)
}

// --------------------------------------------------
// CTOS_UPDATE_DECK — 更新卡组（统一错误处理）
// --------------------------------------------------
func HandleUpdateDeck(c *PacketContext) {
	pData := c.Payload
	if len(pData) < 8 {
		c.AbortWithError(ErrPayloadShort(len(pData), 8, c.PktType))
		return
	}
	if len(pData) > int(protocol.MAINC_MAX+protocol.SIDEC_MAX)*4+8 {
		c.AbortWithError(NewPacketError(ErrPayloadTooShort, "deck data too long"))
		return
	}
	c.Player.Game.UpdateDeck(c.Player, pData)
}

// --------------------------------------------------
// CTOS_HAND_RESULT — 猜拳结果
// --------------------------------------------------
func HandleHandResult(c *PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSHandResult)
	c.Player.Game.HandResult(c.Player, pkt.Res)
}

// --------------------------------------------------
// CTOS_TP_RESULT — 先后攻选择
// --------------------------------------------------
func HandleTPResult(c *PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSTPResult)
	c.Player.Game.TPResult(c.Player, pkt.Res)
}

// --------------------------------------------------
// CTOS_HS_TODUELIST — 切换为决斗者
// --------------------------------------------------
func HandleHsToDuelist(c *PacketContext) {
	c.Player.Game.ToDuelList(c.Player)
}

// --------------------------------------------------
// CTOS_HS_TOOBSERVER — 切换为观战者
// --------------------------------------------------
func HandleHsToObserver(c *PacketContext) {
	c.Player.Game.ToObserver(c.Player)
}

// --------------------------------------------------
// CTOS_HS_READY / CTOS_HS_NOTREADY — 准备/取消准备
// --------------------------------------------------
func HandleHsReady(c *PacketContext) {
	c.Player.Game.PlayerReady(c.Player, true)
}

func HandleHsNotReady(c *PacketContext) {
	c.Player.Game.PlayerReady(c.Player, false)
}

// --------------------------------------------------
// CTOS_HS_KICK — 踢人
// --------------------------------------------------
func HandleHsKick(c *PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSKick)
	c.Player.Game.PlayerKick(c.Player, pkt.Pos)
}

// --------------------------------------------------
// CTOS_HS_START — 开始决斗
// --------------------------------------------------
func HandleHsStart(c *PacketContext) {
	c.Player.Game.StartDuel(c.Player)
}

// --------------------------------------------------
// 房间中间件检查（使用统一错误处理）
// --------------------------------------------------

// RequireNotInGame 检查玩家不在任何游戏中
func RequireNotInGame(c *PacketContext) {
	if c.Player.Game != nil {
		c.AbortWithError(ErrAlreadyInGameAction())
		return
	}
	c.Next()
}

// RequireGameNotStarted 检查游戏未开始（大厅阶段）
func RequireGameNotStarted(c *PacketContext) {
	if c.Player.Game == nil || c.Player.Game.BaseMode() == nil || c.Player.Game.BaseMode().Duel != nil {
		c.AbortWithError(ErrNeedLobby())
		return
	}
	c.Next()
}

// --------------------------------------------------
// 构建完整路由器（推荐用法）
// --------------------------------------------------

// BuildPacketRouter 创建并配置完整的路由器
func BuildPacketRouter() *PacketRouter {
	router := NewPacketRouter()

	// 全局中间件：Recover + 状态检查
	router.Use(RecoverMiddleware, StateMiddleware)

	// 玩家信息设置（不需要在游戏内）
	router.Handle(network.CTOS_PLAYER_INFO,
		Bind(protocol.CTOSPlayerInfo{}),
		HandlePlayerInfo,
	)

	// 创建房间（不能已在游戏中）
	router.Handle(network.CTOS_CREATE_GAME,
		RequireNotInGame,
		Bind(protocol.CTOSCreateGame{}),
		HandleCreateGame,
	)

	// 加入房间（不能已在游戏中）
	router.Handle(network.CTOS_JOIN_GAME,
		RequireNotInGame,
		Bind(protocol.CTOSJoinGame{}),
		HandleJoinGame,
	)

	// 离开房间
	router.Handle(network.CTOS_LEAVE_GAME, RequireGame, HandleLeaveGame)

	// 投降（特殊：不检查 State）
	router.Handle(network.CTOS_SURRENDER, RequireGame, HandleSurrender)

	// 聊天（特殊：不检查 State）
	router.Handle(network.CTOS_CHAT, RequireGame, HandleChat)

	// 以下操作需要游戏已经开始（决斗中）
	gameGroup := router.Group("game", RequireGame)
	{
		gameGroup.Handle(network.CTOS_RESPONSE, HandleResponse)
		gameGroup.Handle(network.CTOS_TIME_CONFIRM, RequireDuel, HandleTimeConfirm)
		gameGroup.Handle(network.CTOS_UPDATE_DECK, HandleUpdateDeck)
		gameGroup.Handle(network.CTOS_HAND_RESULT,
			ValidateLength(int(1)),
			Bind(protocol.CTOSHandResult{}),
			HandleHandResult,
		)
		gameGroup.Handle(network.CTOS_TP_RESULT,
			ValidateLength(int(1)),
			Bind(protocol.CTOSTPResult{}),
			HandleTPResult,
		)
	}

	// 以下操作需要在大厅阶段（游戏未开始）
	lobbyGroup := router.Group("lobby", RequireGame, RequireGameNotStarted)
	{
		lobbyGroup.Handle(network.CTOS_HS_TODUELIST, HandleHsToDuelist)
		lobbyGroup.Handle(network.CTOS_HS_TOOBSERVER, HandleHsToObserver)
		lobbyGroup.Handle(network.CTOS_HS_READY, HandleHsReady)
		lobbyGroup.Handle(network.CTOS_HS_NOTREADY, HandleHsNotReady)
		lobbyGroup.Handle(network.CTOS_HS_KICK,
			ValidateLength(int(1)),
			Bind(protocol.CTOSKick{}),
			HandleHsKick,
		)
		lobbyGroup.Handle(network.CTOS_HS_START, HandleHsStart)
	}

	return router
}
