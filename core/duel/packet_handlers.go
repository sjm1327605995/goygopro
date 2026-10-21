package duel

import (
	"unicode/utf16"

	"github.com/sjm1327605995/goygopro/core/data"
	"github.com/sjm1327605995/goygopro/core/packet"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// playerOf 取回 PacketContext 中的具体玩家（Peer 的决斗侧实现类型，
// core/packet 只认识窄接口 packet.Peer）。
func playerOf(c *packet.PacketContext) *DuelPlayer {
	return c.Peer.(*DuelPlayer)
}

// --------------------------------------------------
// CTOS_PLAYER_INFO — 玩家信息设置
// --------------------------------------------------
func HandlePlayerInfo(c *packet.PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSPlayerInfo)
	player := playerOf(c)
	utils.NullTerminate(pkt.Name[:], 0)
	copy(player.Name[:], pkt.Name[:])
	player.SetID(string(utf16.Decode(player.Name[:])))
	utils.NetLogf("server", "PLAYER_INFO: name=%q", utils.WideString(pkt.Name[:]))
}

// normalizeCreateGameInfo 修正 CREATE_GAME 的主机信息：规则/模式截断到服务器
// 支持的上限，禁卡表哈希不在服务器列表时回退为列表首项（服务器无列表时置 0）。
// 对应原版 netserver.cpp CTOS_CREATE_GAME 分支的逐项修正。
func normalizeCreateGameInfo(info *protocol.HostInfo) {
	if info.Rule > CURRENT_RULE {
		info.Rule = CURRENT_RULE
	}
	if info.Mode > MODE_TAG {
		info.Mode = MODE_SINGLE
	}
	for _, lfList := range data.DeckManager.LFList {
		if info.LFList == lfList.Hash {
			return
		}
	}
	info.LFList = 0
	if len(data.DeckManager.LFList) > 0 {
		info.LFList = data.DeckManager.LFList[0].Hash
	}
}

// --------------------------------------------------
// CTOS_CREATE_GAME — 创建房间
// --------------------------------------------------
func HandleCreateGame(c *packet.PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSCreateGame)
	player := playerOf(c)
	if player.Game != nil {
		c.AbortWithError(packet.ErrAlreadyInGameAction())
		return
	}

	normalizeCreateGameInfo(&pkt.Info)

	roomID := string(utf16.Decode(pkt.Pass[:]))

	// C++: if(dp->game || duel_mode) return;
	// 玩家已在游戏中（RequireNotInGame 已拦）或服务器已有房间，拒绝创建。
	// 单房间约束的权威判定在 CreateRoomIfEmpty（预检 + SetIfAbsent 同一临界区，
	// 不同密码的并发 CREATE_GAME 不能都建成），这里只是免分配的快速预检。
	if DefaultManager.RoomCount() > 0 {
		utils.NetLogf("server", "CREATE_GAME rejected: room already exists (count=%d)", DefaultManager.RoomCount())
		c.AbortWithError(packet.ErrAlreadyInGameAction())
		return
	}

	// 创建模式实例
	var mode IDuelMode
	switch pkt.Info.Mode {
	case MODE_SINGLE:
		mode = newSingleDuel(false)
	case MODE_MATCH:
		mode = newSingleDuel(true)
	case MODE_TAG:
		mode = newTagDuel()
	default:
		return
	}

	mode.BaseMode().HostInfo = pkt.Info
	utils.NullTerminate(pkt.Name[:], 0)
	utils.NullTerminate(pkt.Pass[:], 0)
	copy(mode.BaseMode().Name[:], pkt.Name[:])
	copy(mode.BaseMode().Pass[:], pkt.Pass[:])

	utils.NetLogf("server", "CREATE_GAME: room=%q pass=%q mode=%d rule=%d duelRule=%d player=%q",
		utils.WideString(pkt.Name[:]), utils.WideString(pkt.Pass[:]), pkt.Info.Mode, pkt.Info.Rule, pkt.Info.DuelRule,
		utils.WideString(player.Name[:]))

	room, created := DefaultManager.CreateRoomIfEmpty(roomID, mode)
	if !created {
		utils.NetLogf("server", "CREATE_GAME rejected: CreateRoomIfEmpty failed (roomId=%q)", roomID)
		c.AbortWithError(packet.ErrAlreadyInGameAction())
		return
	}
	mode.BaseMode().RoomID = roomID

	player.Game = room.DuelMode
	// 对齐原版 netserver.cpp:313（CTOS_CREATE_GAME 即 StartBroadcast）：
	// 建房时若广播应答器未运行（如上一场决斗开始时被 StopListen 连带停止）
	// 则恢复 LAN 发现应答；已在运行时为空操作，端口被占只记日志。
	EnsureBroadcast()
	// create 未挂 RoomLockMiddleware（RequireNotInGame 保证进来时 c.Game()==nil，
	// 该中间件是 no-op），而 JoinGame 写房间的共享玩家表，需显式持锁，
	// 避免 --multicore=true 下与并发加入者竞争。
	room.DuelMode.BaseMode().Mu.Lock()
	player.Game.JoinGame(player, nil, true)
	room.DuelMode.BaseMode().Mu.Unlock()
}

// --------------------------------------------------
// CTOS_JOIN_GAME — 加入房间
// --------------------------------------------------
func HandleJoinGame(c *packet.PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSJoinGame)
	player := playerOf(c)
	// 与 HandleCreateGame 对齐：客户端只填前 N 个 uint16，剩余槽位是 0，
	// 不解 NUL 截断会把尾随 \x00 算进 roomId，导致按密码永远查不到房间。
	utils.NullTerminate(pkt.Pass[:], 0)
	roomID := string(utf16.Decode(pkt.Pass[:]))
	utils.NetLogf("server", "JOIN_GAME: player=%q pass=%q version=0x%04x gameId=%d",
		utils.WideString(player.Name[:]), utils.WideString(pkt.Pass[:]), pkt.Version, pkt.GameID)

	room, found := DefaultManager.GetRoom(roomID)
	if !found {
		utils.NetLogf("server", "JOIN_GAME rejected: room pass=%q not found (rooms=%d)", utils.WideString(pkt.Pass[:]), DefaultManager.RoomCount())
		// Go 设计密码即房间索引：GetRoom 失败最常见的原因是密码不对。
		// 对齐原版进房后密码不符的提示（JOINERROR code=1，SysString 1404
		// 「密码错误」，前端 Lobby describeErrorMsg 的 code=1 分支自动生效）：
		// 只要服务器上还有任意房间就按密码错误提示；一个房间都没有时才回
		// 通用 JOINERROR（code=0），单房间语义不变。
		if DefaultManager.RoomCount() > 0 {
			c.AbortWithErrorCode(packet.ErrWrongRoomPassword(), 1)
			return
		}
		c.AbortWithError(packet.ErrJoinError())
		return
	}

	// 注意不能在这里提前把 player.Game 设为房间：checkJoinAllowed 把
	// dp.Game != nil 视为「已在其他房间」直接拒绝（C++ 是先校验后赋值，
	// netserver.cpp CTOS_JOIN_GAME 分支）。JoinGame 校验通过后会自行设置。
	// join 未挂 RoomLockMiddleware（RequireNotInGame 保证进来时 c.Game()==nil），
	// JoinGame 写房间共享玩家表需显式持锁。
	room.DuelMode.BaseMode().Mu.Lock()
	room.DuelMode.JoinGame(player, pkt, false)
	room.DuelMode.BaseMode().Mu.Unlock()
}

// --------------------------------------------------
// CTOS_LEAVE_GAME — 离开房间（统一错误处理）
// --------------------------------------------------
func HandleLeaveGame(c *packet.PacketContext) {
	if c.Game() == nil {
		c.AbortWithError(packet.ErrNeedGame())
		return
	}
	playerOf(c).leaveGameOnce()
}

// --------------------------------------------------
// CTOS_SURRENDER — 投降（统一错误处理）
// --------------------------------------------------
func HandleSurrender(c *packet.PacketContext) {
	player := playerOf(c)
	if player.Game == nil {
		c.AbortWithError(packet.ErrNeedGame())
		return
	}
	player.Game.Surrender(player)
}

// --------------------------------------------------
// CTOS_RESPONSE — 响应/操作
// --------------------------------------------------
func HandleResponse(c *packet.PacketContext) {
	player := playerOf(c)
	player.Game.GetResponse(player, c.Payload)
}

// --------------------------------------------------
// CTOS_TIME_CONFIRM — 时间确认（统一错误处理）
// --------------------------------------------------
func HandleTimeConfirm(c *packet.PacketContext) {
	player := playerOf(c)
	if player.Game == nil {
		c.AbortWithError(packet.ErrNeedGame())
		return
	}
	if player.Game.OCGDuel() == nil {
		c.AbortWithError(packet.ErrNeedDuel())
		return
	}
	player.Game.TimeConfirm(player)
}

// --------------------------------------------------
// CTOS_CHAT — 聊天（统一错误处理）
// --------------------------------------------------
func HandleChat(c *packet.PacketContext) {
	player := playerOf(c)
	if player.Game == nil {
		c.AbortWithError(packet.ErrNeedGame())
		return
	}
	payload := c.Payload
	if len(payload) < 2 {
		c.AbortWithError(packet.ErrPayloadShort(len(payload), 2, c.PktType))
		return
	}
	if len(payload) > protocol.LEN_CHAT_MSG*2 {
		c.AbortWithError(packet.ErrPayloadLong(len(payload), protocol.LEN_CHAT_MSG*2, c.PktType))
		return
	}
	if len(payload)%2 != 0 {
		c.AbortWithError(packet.NewPacketError(packet.ErrBadRequest, "chat msg length must be even"))
		return
	}
	player.Game.Chat(player, payload)
}

// --------------------------------------------------
// CTOS_UPDATE_DECK — 更新卡组（统一错误处理）
// --------------------------------------------------
func HandleUpdateDeck(c *packet.PacketContext) {
	player := playerOf(c)
	payload := c.Payload
	if len(payload) < deckDataHeaderLen {
		c.AbortWithError(packet.ErrPayloadShort(len(payload), deckDataHeaderLen, c.PktType))
		return
	}
	if len(payload) > deckDataMaxPayload {
		c.AbortWithError(packet.ErrPayloadLong(len(payload), deckDataMaxPayload, c.PktType))
		return
	}
	player.Game.UpdateDeck(player, payload)
}

// --------------------------------------------------
// CTOS_HAND_RESULT — 猜拳结果
// --------------------------------------------------
func HandleHandResult(c *packet.PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSHandResult)
	player := playerOf(c)
	player.Game.HandResult(player, pkt.Res)
}

// --------------------------------------------------
// CTOS_TP_RESULT — 先后攻选择
// --------------------------------------------------
func HandleTPResult(c *packet.PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSTPResult)
	player := playerOf(c)
	player.Game.TPResult(player, pkt.Res)
}

// --------------------------------------------------
// CTOS_HS_TODUELIST — 切换为决斗者
// --------------------------------------------------
func HandleHsToDuelist(c *packet.PacketContext) {
	player := playerOf(c)
	player.Game.ToDuelList(player)
}

// --------------------------------------------------
// CTOS_HS_TOOBSERVER — 切换为观战者
// --------------------------------------------------
func HandleHsToObserver(c *packet.PacketContext) {
	player := playerOf(c)
	player.Game.ToObserver(player)
}

// --------------------------------------------------
// CTOS_HS_READY / CTOS_HS_NOTREADY — 准备/取消准备
// --------------------------------------------------
func HandleHsReady(c *packet.PacketContext) {
	player := playerOf(c)
	player.Game.PlayerReady(player, true)
}

func HandleHsNotReady(c *packet.PacketContext) {
	player := playerOf(c)
	player.Game.PlayerReady(player, false)
}

// --------------------------------------------------
// CTOS_HS_KICK — 踢人
// --------------------------------------------------
func HandleHsKick(c *packet.PacketContext) {
	pkt := c.MustPayload().(*protocol.CTOSKick)
	player := playerOf(c)
	player.Game.PlayerKick(player, pkt.Pos)
}

// --------------------------------------------------
// CTOS_HS_START — 开始决斗
// --------------------------------------------------
func HandleHsStart(c *packet.PacketContext) {
	player := playerOf(c)
	player.Game.StartDuel(player)
}

// --------------------------------------------------
// 房间中间件检查（使用统一错误处理）
// --------------------------------------------------

// RequireNotInGame 检查玩家不在任何游戏中
func RequireNotInGame(c *packet.PacketContext) {
	if c.Game() != nil {
		c.AbortWithError(packet.ErrAlreadyInGameAction())
		return
	}
	c.Next()
}

// RequireGameNotStarted 检查游戏未开始（大厅阶段）
func RequireGameNotStarted(c *packet.PacketContext) {
	game := c.Game()
	if game == nil || game.BaseRoom() == nil || game.BaseRoom().DuelStarted() {
		c.AbortWithError(packet.ErrNeedLobby())
		return
	}
	c.Next()
}

// --------------------------------------------------
// 构建完整路由器（推荐用法）
// --------------------------------------------------

// BuildPacketRouter 创建并配置完整的路由器
func BuildPacketRouter() *packet.PacketRouter {
	router := packet.NewPacketRouter()

	// 全局中间件：Recover + 状态检查
	router.Use(packet.RecoverMiddleware, packet.StateMiddleware)

	// 玩家信息设置（不需要在游戏内）
	router.Handle(network.CTOS_PLAYER_INFO,
		packet.Bind(protocol.CTOSPlayerInfo{}),
		HandlePlayerInfo,
	)

	// 创建房间（不能已在游戏中）
	router.Handle(network.CTOS_CREATE_GAME,
		RequireNotInGame,
		packet.Bind(protocol.CTOSCreateGame{}),
		HandleCreateGame,
	)

	// 加入房间（不能已在游戏中）
	router.Handle(network.CTOS_JOIN_GAME,
		RequireNotInGame,
		packet.Bind(protocol.CTOSJoinGame{}),
		HandleJoinGame,
	)

	// 离开房间
	// 离开房间：锁由 leaveGameOnce 内部统一获取（OnClose 断线路径不走路由，
	// middleware 罩不到，见 leaveGameOnce 注释），此处不再挂 RoomLockMiddleware。
	router.Handle(network.CTOS_LEAVE_GAME, packet.RequireGame, HandleLeaveGame)

	// 投降（特殊：不检查 State）
	router.Handle(network.CTOS_SURRENDER, packet.RequireGame, packet.RoomLockMiddleware, HandleSurrender)

	// 聊天（特殊：不检查 State）
	router.Handle(network.CTOS_CHAT, packet.RequireGame, packet.RoomLockMiddleware, HandleChat)

	// 以下操作需要游戏已经开始（决斗中）
	gameGroup := router.Group(packet.RequireGame, packet.RoomLockMiddleware)
	{
		gameGroup.Handle(network.CTOS_RESPONSE, HandleResponse)
		gameGroup.Handle(network.CTOS_TIME_CONFIRM, packet.RequireDuel, HandleTimeConfirm)
		gameGroup.Handle(network.CTOS_UPDATE_DECK, HandleUpdateDeck)
		gameGroup.Handle(network.CTOS_HAND_RESULT,
			packet.ValidateLength(1),
			packet.Bind(protocol.CTOSHandResult{}),
			HandleHandResult,
		)
		gameGroup.Handle(network.CTOS_TP_RESULT,
			packet.ValidateLength(1),
			packet.Bind(protocol.CTOSTPResult{}),
			HandleTPResult,
		)
	}

	// 以下操作需要在大厅阶段（游戏未开始）
	lobbyGroup := router.Group(packet.RequireGame, packet.RoomLockMiddleware, RequireGameNotStarted)
	{
		lobbyGroup.Handle(network.CTOS_HS_TODUELIST, HandleHsToDuelist)
		lobbyGroup.Handle(network.CTOS_HS_TOOBSERVER, HandleHsToObserver)
		lobbyGroup.Handle(network.CTOS_HS_READY, HandleHsReady)
		lobbyGroup.Handle(network.CTOS_HS_NOTREADY, HandleHsNotReady)
		lobbyGroup.Handle(network.CTOS_HS_KICK,
			packet.ValidateLength(1),
			packet.Bind(protocol.CTOSKick{}),
			HandleHsKick,
		)
		lobbyGroup.Handle(network.CTOS_HS_START, HandleHsStart)
	}

	return router
}
