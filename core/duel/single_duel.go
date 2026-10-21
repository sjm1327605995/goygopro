package duel

import (
	"slices"
	"time"

	"github.com/duke-git/lancet/v2/condition"
	"github.com/sjm1327605995/goygopro/core/data"
	"github.com/sjm1327605995/goygopro/core/replay"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

func newSingleDuel(matchMode bool) *SingleDuel {
	sd := &SingleDuel{MatchMode: matchMode, DuelMode: DuelMode{
		Observers: make(map[string]*DuelPlayer),
		// 刷新参数表即原版 *Def 系列默认参数（带缓存）；tag 的默认值与之
		// 完全一致（tag_duel.h:35-40 == single_duel.h:35-40），见
		// defaultRefreshFlags。MSG_NEW_PHASE 后按原版 single_duel.cpp:942-943
		// 同样刷新手牌。
		refresh: defaultRefreshFlags(),
	}}
	sd.room = sd
	return sd
}

type SingleDuel struct {
	DuelMode
	players   [2]*DuelPlayer
	pPlayers  [2]*DuelPlayer
	MatchMode bool
	matchKill int
	duelCount uint8
	tpPlayer  uint8
	//lastReplay Replay
	matchResult [3]uint8
}

// JoinGame 的全部逻辑在 joinGameCommon：single 与 tag 的 JoinGame 除座位数
// 外逐字节一致（对照原版 single_duel.cpp:25-100 / tag_duel.cpp:38-112），
// 已用 duelRoom 钩子（firstFreeSeat/assignSeat/seatCount/allPlayers）泛化。
func (single *SingleDuel) JoinGame(player *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) {
	joinGameCommon(single, player, pkt, isCreator, "single")
}

func (single *SingleDuel) leaveAsPlayer(player *DuelPlayer) {
	if single.DuelStage == network.DUEL_STAGE_BEGIN {
		var playerChangePkt protocol.STOCHsPlayerChange
		single.players[player.Type] = nil
		single.ready[player.Type] = false
		playerChangePkt.Status = player.Type<<4 | network.PLAYERCHANGE_LEAVE
		sendHsPlayerChangeAll(single, playerChangePkt)
		_ = single.DisconnectPlayer(player)
		return
	}
	if single.DuelStage == network.DUEL_STAGE_SIDING {
		if !single.ready[0] {
			single.SendPacketToPlayer(single.players[0], network.STOC_DUEL_START)
		}
		if !single.ready[1] {
			single.SendPacketToPlayer(single.players[1], network.STOC_DUEL_START)
		}
	}
	if single.DuelStage != network.DUEL_STAGE_END {
		wbuf := make([]byte, 3)
		wbuf[0] = network.MSG_WIN
		wbuf[1] = 1 - player.Type
		wbuf[2] = 0x4
		broadcastData(single, network.STOC_GAME_MSG, wbuf)
		single.EndDuel()
		broadcastProto(single, network.STOC_DUEL_END)
	}
	_ = single.DisconnectPlayer(player)
}

func (single *SingleDuel) ToDuelList(player *DuelPlayer) {
	if player.Type != network.NETPLAYER_TYPE_OBSERVER {
		return
	}
	if single.players[0] != nil && single.players[1] != nil {
		return
	}
	toDuelListObserver(single, player)
}

func (single *SingleDuel) UpdateDeck(player *DuelPlayer, pData []byte) {
	deckBuf, ok := updateDeckSlot(single, player, pData)
	if !ok {
		return
	}
	if single.duelCount == 0 {
		single.DeckError[player.Type] = data.DeckManager.LoadDeck(single.pDeck[player.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC, false)
	} else {
		if data.DeckManager.LoadSide(single.pDeck[player.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC) {
			single.ready[player.Type] = true
			single.SendPacketToPlayer(player, network.STOC_DUEL_START)
			if single.ready[0] && single.ready[1] {
				enterSelectTp(&single.DuelMode, single.players[single.tpPlayer], single.players[1-single.tpPlayer])
			}
		} else {
			var errMsg = protocol.STOCErrorMsg{
				Msg:  network.ERRMSG_DECKERROR,
				Code: 0,
			}
			single.SendPacketDataToPlayer(player, network.STOC_ERROR_MSG, &errMsg)
		}
	}
}

func (single *SingleDuel) StartDuel(player *DuelPlayer) {
	startDuelCommon(single, player)
}

func (single *SingleDuel) TPResult(player *DuelPlayer, tp byte) {
	if player.State != network.CTOS_TP_RESULT {
		return
	}
	single.DuelStage = network.DUEL_STAGE_DUELING
	var swapped bool
	single.pPlayers[0], single.pPlayers[1] = single.players[0], single.players[1]
	if (tp != 0 && player.Type == 1) || (tp == 0 && player.Type == 0) {
		single.players[0], single.players[1] = single.players[1], single.players[0]
		single.players[0].Type, single.players[1].Type = 0, 1
		single.pDeck[0], single.pDeck[1] = single.pDeck[1], single.pDeck[0]
		swapped = true
	}
	beginDuel(single, player, swapped)
}

func (single *SingleDuel) DuelEndProc() {
	if !single.MatchMode {
		duelEndProcSimple(single)
		return
	}
	winc := make([]byte, 3)
	for i := uint8(0); i < single.duelCount; i++ {
		winc[single.matchResult[i]]++
	}
	if single.matchDecided(winc) {
		duelEndProcSimple(single)
		return
	}
	if single.players[0] != single.pPlayers[0] {
		single.players[0] = single.pPlayers[0]
		single.players[1] = single.pPlayers[1]
		single.players[0].Type = 0
		single.players[1].Type = 1
		single.pDeck[0], single.pDeck[1] = single.pDeck[1], single.pDeck[0]
	}
	single.ready[0], single.ready[1] = false, false
	single.players[0].State = network.CTOS_UPDATE_DECK
	single.players[1].State = network.CTOS_UPDATE_DECK
	single.SendPacketToPlayer(single.players[0], network.STOC_CHANGE_SIDE)
	single.SendPacketToPlayer(single.players[1], network.STOC_CHANGE_SIDE)
	for _, observer := range single.Observers {
		single.SendPacketToPlayer(observer, network.STOC_WAITING_SIDE)
	}
	single.DuelStage = network.DUEL_STAGE_SIDING
}

// matchDecided 报告 match 是否已决出最终胜负（原版 single_duel.cpp:517-520
// 的 match_kill / 连胜 / 三局分布判定，逐条件一致）。winc 按 matchResult
// 记账：下标 0/1 为两方胜局数，下标 2 为平局数。
func (single *SingleDuel) matchDecided(winc []byte) bool {
	return single.matchKill != 0 ||
		winc[0] == 2 || (winc[0] == 1 && winc[2] == 2) ||
		winc[1] == 2 || (winc[1] == 1 && winc[2] == 2) ||
		winc[2] == 3 || (winc[0] == 1 && winc[1] == 1 && winc[2] == 1)
}

// singleAnalyzeTable 在共享表之上追加 single 特有的消息
// （MSG_HAND_RES 为 single 独有；MSG_NEW_TURN / MSG_MATCH_KILL 两模式行为不同）。
var singleAnalyzeTable = copyAnalyzeTable(map[uint8]analyzeHandler{
	ocgcore.MSG_NEW_TURN: func(room duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		return room.(*SingleDuel).analyzeNewTurn(pbuf, offset)
	},
	ocgcore.MSG_MATCH_KILL: func(room duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		return room.(*SingleDuel).analyzeMatchKill(pbuf, offset)
	},
	ocgcore.MSG_HAND_RES: analyzeSkipBroadcast(ocgcore.MSG_HAND_RES),
})

func (single *SingleDuel) Analyze(msgBuffer []byte) int {
	return runAnalyze(single, singleAnalyzeTable, msgBuffer)
}

// analyzeNewTurn 新回合开始：先刷新全部区域防御状态，再广播并重置时间限制。
func (single *SingleDuel) analyzeNewTurn(pbuf, offset *utils.YGOBuffer) int {
	single.RefreshMzoneDef(0)
	single.RefreshMzoneDef(1)
	single.RefreshSzoneDef(0)
	single.RefreshSzoneDef(1)
	single.RefreshHandDef(0)
	single.RefreshHandDef(1)
	_ = mustReadMsg[protocol.NewTurnMsg](pbuf)
	single.timeLimit[0] = int16(single.HostInfo.TimeLimit)
	single.timeLimit[1] = int16(single.HostInfo.TimeLimit)
	broadcastData(single, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	return 0
}

// analyzeMatchKill match 击杀：仅 match 模式记录击杀卡码并广播。
func (single *SingleDuel) analyzeMatchKill(pbuf, offset *utils.YGOBuffer) int {
	msg := mustReadMsg[protocol.MatchKillMsg](pbuf)
	if single.MatchMode {
		single.matchKill = int(msg.Code)
		broadcastData(single, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	}
	return 0
}

func (single *SingleDuel) GetResponse(player *DuelPlayer, msgBuffer []byte) {
	if single.Duel == nil {
		return
	}
	if player.State != network.CTOS_RESPONSE {
		return
	}
	// 原版 single_duel.cpp:1419-1423：GetResponse 不回写 lastResponse
	// （lastResponse 由 WaitforResponse 记录，Go 在 waitForResponse 中同步，
	// 见 duel_common.go），也没有在扣减前清零 timeElapsed——旧代码先
	// timeElapsed=0 导致下方 timeLimit -= timeElapsed 恒减 0，计时银行
	// 永不扣减。
	//
	// 与 tag 版 GetResponse 的对照（为何保留两份实现）见 tag_duel.go。
	recordResponseAndSet(&single.DuelMode, truncateResponse(msgBuffer))
	if single.HostInfo.TimeLimit != 0 {
		if single.timeLimit[player.Type] > single.timeElapsed {
			single.timeLimit[player.Type] -= single.timeElapsed
		} else {
			single.timeLimit[player.Type] = 0
		}
		single.timeElapsed = 0
	}
	single.Process()
}

// applyWinResult 记录一局胜负（Surrender / 超时 / MSG_WIN 共用同一套
// matchResult/tpPlayer 记账逻辑）。
func (single *SingleDuel) applyWinResult(player uint8) {
	if single.players[player] == single.pPlayers[player] {
		single.matchResult[single.duelCount] = 1 - player
		single.duelCount++
		single.tpPlayer = player
	} else {
		single.matchResult[single.duelCount] = player
		single.duelCount++
		single.tpPlayer = 1 - player
	}
}

func (single *SingleDuel) Surrender(surrenderer *DuelPlayer) {
	if surrenderer.Type > 1 || single.Duel == nil {
		return
	}
	player := surrenderer.Type
	wbuf := make([]byte, 3)
	wbuf[0] = ocgcore.MSG_WIN
	wbuf[1] = 1 - player
	wbuf[2] = 0
	broadcastData(single, network.STOC_GAME_MSG, wbuf)
	single.applyWinResult(player)
	single.EndDuel()
	single.DuelEndProc()
	if single.ETimer != nil {
		single.ETimer.Stop()
	}
}
func (single *SingleDuel) SingleTimer() {
	single.Mu.Lock()
	defer single.Mu.Unlock()
	running, timeout := tickDuelTimerLocked(&single.DuelMode)
	if !running {
		if !timeout {
			return
		}
		player := single.lastResponse
		wbuf := make([]byte, 3)
		wbuf[0] = ocgcore.MSG_WIN
		wbuf[1] = 1 - player
		wbuf[2] = 0x3
		broadcastData(single, network.STOC_GAME_MSG, wbuf)
		single.applyWinResult(player)
		single.EndDuel()
		single.DuelEndProc()
		if single.ETimer != nil {
			single.ETimer.Stop()
			single.ETimer = nil
		}
		return
	}
	single.ETimer = timerWheel.AfterFunc(time.Second, single.SingleTimer)

}

// duelRoom 接口实现（差异点钩子）。

func (single *SingleDuel) allPlayers() []*DuelPlayer { return single.players[:] }

func (single *SingleDuel) currentPlayer(player int) *DuelPlayer { return single.players[player] }

func (single *SingleDuel) zoneFullPair(player int) (*DuelPlayer, *DuelPlayer) {
	return single.players[player], nil
}

func (single *SingleDuel) zoneMaskedPair(player int) (*DuelPlayer, *DuelPlayer) {
	return single.players[1-player], nil
}

func (single *SingleDuel) handMaskedRecipients(player int) []*DuelPlayer {
	return []*DuelPlayer{single.players[1-player]}
}

func (single *SingleDuel) waitingNotifyRecipients(player byte) []*DuelPlayer {
	return []*DuelPlayer{single.players[1-player]}
}

// 原版 single_duel.cpp:1451-1452 的 WaitforResponse 只向 players[0]/[1]
// 发 STOC_TIME_LIMIT，不重发观战者（与 tag 一致）。
func (single *SingleDuel) timeLimitToObservers() bool { return false }

func (single *SingleDuel) isResponder(player *DuelPlayer) bool {
	return player.Type == single.lastResponse
}

func (single *SingleDuel) firstFreeSeat() int {
	if single.players[0] == nil {
		return 0
	}
	return 1
}

func (single *SingleDuel) assignSeat(pos int, player *DuelPlayer) { single.players[pos] = player }

func (single *SingleDuel) removeFromSeat(seat int) { single.players[seat] = nil }

func (single *SingleDuel) handResultIndex(seat uint8) int { return int(seat) }

// handSeats 返回猜拳双方的代表座位与各自队友：host 为先手队代表、
// hostPartner 为其队友，guest 为后手队代表、guestPartner 为其队友。
// single 无队友概念：host=players[0]、guest=players[1]，
// hostPartner/guestPartner 为 nil（对应原版不向第二人重发）；
// tag：host=players[0]、hostPartner=players[1]（0 号队），
// guest=players[2]、guestPartner=players[3]（1 号队）。
func (single *SingleDuel) handSeats() (host, hostPartner, guest, guestPartner *DuelPlayer) {
	return single.players[0], nil, single.players[1], nil
}

func (single *SingleDuel) recordTpPlayer(bWins bool) {
	if bWins {
		single.tpPlayer = 1
	} else {
		single.tpPlayer = 0
	}
}

func (single *SingleDuel) onJoinPassDenied(player *DuelPlayer) { _ = single.DisconnectPlayer(player) }

func (single *SingleDuel) onDuelEnded() {}

func (single *SingleDuel) seatCount() int { return 2 }

func (single *SingleDuel) replayFlag() uint32 { return replay.REPLAY_UNIFORM }

func (single *SingleDuel) extraDuelOpt() uint32 { return 0 }

// loadDecksToEngine 把两名玩家的卡组按原版顺序装入引擎并写入回放：
// Main 先整体 Reverse（与原版一致，从卡组顶端开始 load），再按
// 0 号主卡组/额外 → 1 号主卡组/额外 的固定序列 AddCard
// （single_duel.cpp TPResult 的 load 调用序列）。
func (single *SingleDuel) loadDecksToEngine(engine *ocgcore.Duel, rec *replay.Replay) {
	load := func(deckContainer []*data.CardDataC, player uint8, location uint8) {
		rec.WriteInt32(int32(len(deckContainer)), false)
		for _, card := range deckContainer {
			engine.AddCard(card.Code, int(player), location)
			rec.WriteInt32(int32(card.Code), false)
		}
	}
	slices.Reverse(single.pDeck[0].Main)
	slices.Reverse(single.pDeck[1].Main)
	load(single.pDeck[0].Main, 0, ocgcore.LOCATION_DECK)
	load(single.pDeck[0].Extra, 0, ocgcore.LOCATION_EXTRA)
	load(single.pDeck[1].Main, 1, ocgcore.LOCATION_DECK)
	load(single.pDeck[1].Extra, 1, ocgcore.LOCATION_EXTRA)
}

// refreshOnDuelStart 是开局额外卡组刷新（原版 single RefreshExtraDef，
// 即 0xe81fff/缓存）。
func (single *SingleDuel) refreshOnDuelStart() {
	single.RefreshExtraDef(0)
	single.RefreshExtraDef(1)
}

// armDuelTimer 武装决斗秒表：SingleTimer 超时处理自行重新武装
// （与原版 SingleTimer 一致）。
func (single *SingleDuel) armDuelTimer() {
	single.ETimer = timerWheel.AfterFunc(time.Second, single.SingleTimer)
}

func (single *SingleDuel) onEngineWin(player uint8) {
	if player > 1 {
		single.matchResult[single.duelCount] = 2
		single.duelCount++
		single.tpPlayer = 1 - single.tpPlayer
	} else if single.players[player] == single.pPlayers[player] {
		single.matchResult[single.duelCount] = player
		single.duelCount++
		single.tpPlayer = 1 - player
	} else {
		single.matchResult[single.duelCount] = 1 - player
		single.duelCount++
		single.tpPlayer = player
	}
}

// refreshGraveAfterSwap / refreshAfterSummon / refreshAfterChain /
// refreshAfterDamageStep / refreshAfterNewPhase / refreshSingleMoved /
// refreshSingleFlip / skipCardQuery 原本在本文件以 *Def 常量实现，差异已收进
// DuelMode.refresh 参数表（构造时填写），统一由 DuelMode 基类读表实现
//（见 duel_refresh.go）。

func (single *SingleDuel) RefreshSingle(player uint8, location uint8, sequence uint8, flag int32) {
	queryBuffer, length, position := querySingleCard(&single.DuelMode, player, location, sequence, flag)
	single.SendPacketDataToPlayer(single.players[player], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
	if length <= ocgcore.LEN_HEADER {
		return
	}
	if position&ocgcore.POS_FACEDOWN != 0 {
		maskSingleQueryCode(queryBuffer)
	}
	single.SendPacketDataToPlayer(single.players[1-player], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
	single.ReSendToObservers()
}

// 以下为 SingleDuel / TagDuel 的公共实现段（经 duelRoom 钩子分发）。
// 按项目惯例这类共享逻辑应放 duel_common.go；本次清理限定文件范围，
// 故暂驻本文件，后续解封时可整体迁移。

// joinGameCommon 是 SingleDuel / TagDuel JoinGame 的公共实现（原版
// single_duel.cpp:25-100 与 tag_duel.cpp:38-112 除座位数外逐字节一致，
// 已核对）：入席找座（第一个空位）或转入观战、回发 JOIN_GAME/TYPE_CHANGE、
// 向新加入者重放大厅现状（在席玩家 + ready 状态 + 观战人数）。
// modeLabel 仅用于日志区分模式。
func joinGameCommon(room duelRoom, player *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool, modeLabel string) {
	if !checkJoinAllowed(room, player, pkt, isCreator) {
		return
	}
	base := room.BaseMode()
	player.Game = room
	players := room.allPlayers()
	allSeatsFree := true
	for _, seat := range players {
		if seat != nil {
			allSeatsFree = false
			break
		}
	}
	if allSeatsFree && len(base.Observers) == 0 {
		base.HostPlayer = player
	}
	var (
		joinGamePkt   = protocol.STOCJoinGame{Info: base.HostInfo}
		typeChangePkt = protocol.STOCTypeChange{Type: condition.Ternary[bool, uint8](base.HostPlayer == player, 0x10, 0)}
	)
	seatAvailable := false
	for _, seat := range players {
		if seat == nil {
			seatAvailable = true
			break
		}
	}
	if seatAvailable {
		var playerEnterPkt protocol.STOCHsPlayerEnter
		copy(playerEnterPkt.Name[:], player.Name[:])
		pos := room.firstFreeSeat()
		playerEnterPkt.Pos = uint8(pos)
		sendDataToAll(room, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		room.assignSeat(pos, player)
		player.Type = uint8(pos)
		typeChangePkt.Type |= player.Type
	} else {
		base.Observers[player.ID] = player
		player.Type = network.NETPLAYER_TYPE_OBSERVER
		typeChangePkt.Type |= network.NETPLAYER_TYPE_OBSERVER
		var watchChangePkt protocol.STOCHsWatchChange
		watchChangePkt.WatchCount = uint16(len(base.Observers))
		sendDataToAll(room, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
	}
	base.SendPacketDataToPlayer(player, network.STOC_JOIN_GAME, joinGamePkt)
	base.SendPacketDataToPlayer(player, network.STOC_TYPE_CHANGE, typeChangePkt)
	for i := 0; i < room.seatCount(); i++ {
		if players[i] != nil {
			var playerEnterPkt protocol.STOCHsPlayerEnter
			copy(playerEnterPkt.Name[:], players[i].Name[:])
			playerEnterPkt.Pos = uint8(i)
			base.SendPacketDataToPlayer(player, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
			if base.ready[i] {
				var playerChangePkt protocol.STOCHsPlayerChange
				playerChangePkt.Status = uint8(i<<4) | network.PLAYERCHANGE_READY
				base.SendPacketDataToPlayer(player, network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
			}
		}
	}
	if len(base.Observers) > 0 {
		var watchChangePkt protocol.STOCHsWatchChange
		watchChangePkt.WatchCount = uint16(len(base.Observers))
		base.SendPacketDataToPlayer(player, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
	}
	utils.NetLogf("server", "JOIN_GAME ok: player=%q seated as type=%d (%s, room=%q)",
		utils.WideString(player.Name[:]), player.Type, modeLabel, utils.WideString(base.Name[:]))
}

// updateDeckSlot 是 UpdateDeck 的公共前导（原版 single_duel.cpp:278 与
// tag_duel.cpp:260 除座位数外逐字节一致）：座位/ready 校验、解包卡组数据、
// pDeck 指针兜底分配并清空槽位。ok=false 时调用方应直接返回。
func updateDeckSlot(room duelRoom, player *DuelPlayer, pData []byte) (protocol.CTOSDeckData, bool) {
	base := room.BaseMode()
	if int(player.Type) >= room.seatCount() || base.ready[player.Type] {
		return protocol.CTOSDeckData{}, false
	}
	deckBuf, ok := unpackDeckData(base, player, pData)
	if !ok {
		return protocol.CTOSDeckData{}, false
	}
	// pDeck 是指针数组、初值为 nil（原版 pdeck 是值成员，永远存在；
	// tag_duel.cpp:281 的 pdeck 同理）；兜底分配，重复上传时清空重载
	// （原版 LoadDeck 入口即 deck.clear()）。
	if base.pDeck[player.Type] == nil {
		base.pDeck[player.Type] = &data.Deck{}
	} else {
		base.pDeck[player.Type].Clear()
	}
	return deckBuf, true
}

// truncateResponse 把客户端响应截断到引擎定长缓冲上限（原版
// single_duel.cpp:1411-1412 / tag_duel.cpp:1519-1520 的
// len > SIZE_RETURN_VALUE 截断；超长响应写回定长缓冲会越界）。
// 返回切片与传入缓冲共享底层数组。
func truncateResponse(msgBuffer []byte) []byte {
	if len(msgBuffer) > ocgcore.SIZE_RETURN_VALUE {
		return msgBuffer[:ocgcore.SIZE_RETURN_VALUE]
	}
	return msgBuffer
}
