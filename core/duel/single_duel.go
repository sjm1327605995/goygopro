package duel

import (
	"encoding/binary"
	"slices"
	"time"

	"github.com/duke-git/lancet/v2/condition"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

func newSingleDuel(matchMode bool) *SingleDuel {
	sd := &SingleDuel{MatchMode: matchMode, DuelMode: DuelMode{
		Observers: make(map[string]*DuelPlayer),
		// 刷新参数表：single 的取值即原 *Def 系列默认参数（带缓存）；
		// newPhaseHand 留 0（single 的 MSG_NEW_PHASE 后不刷新手牌，与原版一致）；
		// skipCardQueryLe 留 false（边界语义 clen < LEN_HEADER）。
		refresh: refreshFlags{
			summonMzone:     0x881fff,
			summonSzone:     0x681fff,
			chainMzone:      0x881fff,
			chainSzone:      0x681fff,
			chainHand:       0x681fff,
			damageStepMzone: 0x881fff,
			newPhaseMzone:   0x881fff,
			newPhaseSzone:   0x681fff,
			singleMoved:     0xf81fff,
			singleFlip:      0xf81fff,
			graveAfterSwap:  0x81fff,
			useCache:        1,
		},
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

func (s *SingleDuel) JoinGame(dp *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) {

	if !checkJoinAllowed(s, dp, pkt, isCreator) {
		return
	}
	dp.Game = s
	if s.players[0] == nil && s.players[1] == nil && len(s.Observers) == 0 {
		s.HostPlayer = dp
	}
	var (
		joinGamePkt = protocol.STOCJoinGame{Info: s.HostInfo}
		typeChangePkt = protocol.STOCTypeChange{Type: condition.Ternary[bool, uint8](s.HostPlayer == dp, 0x10, 0)}
	)
	if s.players[0] == nil || s.players[1] == nil {
		var playerEnterPkt protocol.STOCHsPlayerEnter
		copy(playerEnterPkt.Name[:], dp.Name[:])
		if s.players[0] == nil {
			playerEnterPkt.Pos = 0
		} else {
			playerEnterPkt.Pos = 1
		}
		if s.players[0] != nil {
			s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		}
		if s.players[1] != nil {
			s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		}
		if s.players[0] == nil {
			s.players[0] = dp
			dp.Type = network.NETPLAYER_TYPE_PLAYER1
			typeChangePkt.Type |= network.NETPLAYER_TYPE_PLAYER1
		} else {
			s.players[1] = dp
			dp.Type = network.NETPLAYER_TYPE_PLAYER2
			typeChangePkt.Type |= network.NETPLAYER_TYPE_PLAYER2
		}
	} else {
		s.Observers[dp.ID] = dp
		dp.Type = network.NETPLAYER_TYPE_OBSERVER
		typeChangePkt.Type |= network.NETPLAYER_TYPE_OBSERVER
		var watchChangePkt protocol.STOCHsWatchChange
		watchChangePkt.WatchCount = uint16(len(s.Observers))
		if s.players[0] != nil {
			s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_WATCH_CHANGE, watchChangePkt)
		}
		if s.players[1] != nil {
			s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_WATCH_CHANGE, watchChangePkt)
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
		}
	}
	s.SendPacketDataToPlayer(dp, network.STOC_JOIN_GAME, joinGamePkt)
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, typeChangePkt)
	if s.players[0] != nil {
		var playerEnterPkt protocol.STOCHsPlayerEnter
		copy(playerEnterPkt.Name[:], s.players[0].Name[:])
		playerEnterPkt.Pos = 0
		s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		if s.ready[0] {
			var playerChangePkt protocol.STOCHsPlayerChange
			playerChangePkt.Status = network.PLAYERCHANGE_READY
			s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
		}
	}
	if s.players[1] != nil {
		var playerEnterPkt protocol.STOCHsPlayerEnter
		copy(playerEnterPkt.Name[:], s.players[1].Name[:])
		playerEnterPkt.Pos = 1
		s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		if s.ready[1] {
			var playerChangePkt protocol.STOCHsPlayerChange
			playerChangePkt.Status = 0x10 | network.PLAYERCHANGE_READY
			s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
		}
	}
	if len(s.Observers) > 0 {
		var watchChangePkt protocol.STOCHsWatchChange
		watchChangePkt.WatchCount = uint16(len(s.Observers))
		s.SendPacketDataToPlayer(dp, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
	}
}

func (s *SingleDuel) leaveAsPlayer(dp *DuelPlayer) {
	if s.DuelStage == network.DUEL_STAGE_BEGIN {
		var playerChangePkt protocol.STOCHsPlayerChange
		s.players[dp.Type] = nil
		s.ready[dp.Type] = false
		playerChangePkt.Status = dp.Type<<4 | network.PLAYERCHANGE_LEAVE
		sendHsPlayerChangeAll(s, playerChangePkt)
		_ = s.DisconnectPlayer(dp)
		return
	}
	if s.DuelStage == network.DUEL_STAGE_SIDING {
		if !s.ready[0] {
			s.SendPacketToPlayer(s.players[0], network.STOC_DUEL_START)
		}
		if !s.ready[1] {
			s.SendPacketToPlayer(s.players[1], network.STOC_DUEL_START)
		}
	}
	if s.DuelStage != network.DUEL_STAGE_END {
		wbuf := make([]byte, 3)
		wbuf[0] = network.MSG_WIN
		wbuf[1] = 1 - dp.Type
		wbuf[2] = 0x4
		broadcastData(s, network.MSG_WIN, wbuf)
		s.EndDuel()
		broadcastProto(s, network.STOC_DUEL_END)
	}
	_ = s.DisconnectPlayer(dp)
}

func (s *SingleDuel) ToDuelList(dp *DuelPlayer) {
	if dp.Type != network.NETPLAYER_TYPE_OBSERVER {
		return
	}
	if s.players[0] != nil && s.players[1] != nil {
		return
	}
	toDuelListObserver(s, dp)
}

func (s *SingleDuel) UpdateDeck(dp *DuelPlayer, pData []byte) {
	if dp.Type > 1 || s.ready[dp.Type] {
		return
	}
	deckBuf, ok := unpackDeckData(&s.DuelMode, dp, pData)
	if !ok {
		return
	}
	if s.pDeck[dp.Type] == nil {
		s.pDeck[dp.Type] = &Deck{}
	} else {
		s.pDeck[dp.Type].Clear()
	}
	if s.duelCount == 0 {
		s.DeckError[dp.Type] = DeckManager.LoadDeck(s.pDeck[dp.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC, false)
	} else {
		if DeckManager.LoadSide(s.pDeck[dp.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC) {
			s.ready[dp.Type] = true
			s.SendPacketToPlayer(dp, network.STOC_DUEL_START)
			if s.ready[0] && s.ready[1] {
				s.SendPacketToPlayer(s.players[s.tpPlayer], network.STOC_SELECT_TP)
				s.players[1-s.tpPlayer].State = 0xff
				s.players[s.tpPlayer].State = network.CTOS_TP_RESULT
				s.DuelStage = network.DUEL_STAGE_FIRSTGO
			}
		} else {
			var scem = protocol.STOCErrorMsg{
				Msg:  network.ERRMSG_DECKERROR,
				Code: 0,
			}
			s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
		}
	}
}

func (s *SingleDuel) StartDuel(dp *DuelPlayer) {
	startDuelCommon(s, dp)
}

func (s *SingleDuel) TPResult(dp *DuelPlayer, tp byte) {
	if dp.State != network.CTOS_TP_RESULT {
		return
	}
	s.DuelStage = network.DUEL_STAGE_DUELING
	var swapped bool
	s.pPlayers[0], s.pPlayers[1] = s.players[0], s.players[1]
	if (tp != 0 && dp.Type == 1) || (tp == 0 && dp.Type == 0) {
		s.players[0], s.players[1] = s.players[1], s.players[0]
		s.players[0].Type, s.players[1].Type = 0, 1
		s.pDeck[0], s.pDeck[1] = s.pDeck[1], s.pDeck[0]
		swapped = true
	}
	beginDuel(s, dp, swapped)
}

func (s *SingleDuel) DuelEndProc() {
	if !s.MatchMode {
		duelEndProcSimple(s)
		return
	}
	winc := make([]byte, 3)
	for i := uint8(0); i < s.duelCount; i++ {
		winc[s.matchResult[i]]++
	}
	if s.matchKill != 0 ||
		winc[0] == 2 || (winc[0] == 1 && winc[2] == 2) ||
		winc[1] == 2 || (winc[1] == 1 && winc[2] == 2) ||
		winc[2] == 3 || (winc[0] == 1 && winc[1] == 1 && winc[2] == 1) {
		duelEndProcSimple(s)
		return
	}
	if s.players[0] != s.pPlayers[0] {
		s.players[0] = s.pPlayers[0]
		s.players[1] = s.pPlayers[1]
		s.players[0].Type = 0
		s.players[1].Type = 1
		s.pDeck[0], s.pDeck[1] = s.pDeck[1], s.pDeck[0]
	}
	s.ready[0], s.ready[1] = false, false
	s.players[0].State = network.CTOS_UPDATE_DECK
	s.players[1].State = network.CTOS_UPDATE_DECK
	s.SendPacketToPlayer(s.players[0], network.STOC_CHANGE_SIDE)
	s.SendPacketToPlayer(s.players[1], network.STOC_CHANGE_SIDE)
	for _, v := range s.Observers {
		s.SendPacketToPlayer(v, network.STOC_WAITING_SIDE)
	}
	s.DuelStage = network.DUEL_STAGE_SIDING
}

// singleAnalyzeTable 在共享表之上追加 single 特有的消息
// （MSG_HAND_RES 为 single 独有；MSG_NEW_TURN / MSG_MATCH_KILL 两模式行为不同）。
var singleAnalyzeTable = copyAnalyzeTable(map[uint8]analyzeHandler{
	ocgcore.MSG_NEW_TURN: func(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		return m.(*SingleDuel).analyzeNewTurn(pbuf, offset)
	},
	ocgcore.MSG_MATCH_KILL: func(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		return m.(*SingleDuel).analyzeMatchKill(pbuf, offset)
	},
	ocgcore.MSG_HAND_RES: analyzeSkipBroadcast(ocgcore.MSG_HAND_RES),
})

func (s *SingleDuel) Analyze(msgBuffer []byte) int {
	return runAnalyze(s, singleAnalyzeTable, msgBuffer)
}

// analyzeNewTurn 新回合开始：先刷新全部区域防御状态，再广播并重置时间限制。
func (s *SingleDuel) analyzeNewTurn(pbuf, offset *utils.YGOBuffer) int {
	s.RefreshMzoneDef(0)
	s.RefreshMzoneDef(1)
	s.RefreshSzoneDef(0)
	s.RefreshSzoneDef(1)
	s.RefreshHandDef(0)
	s.RefreshHandDef(1)
	pbuf.Next(1)
	s.timeLimit[0] = int16(s.HostInfo.TimeLimit)
	s.timeLimit[1] = int16(s.HostInfo.TimeLimit)
	broadcastData(s, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	return 0
}

// analyzeMatchKill match 击杀：仅 match 模式记录击杀卡码并广播。
func (s *SingleDuel) analyzeMatchKill(pbuf, offset *utils.YGOBuffer) int {
	var code int32
	_ = pbuf.Read(&code)
	if s.MatchMode {
		s.matchKill = int(code)
		broadcastData(s, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	}
	return 0
}

func (s *SingleDuel) GetResponse(dp *DuelPlayer, msgBuffer []byte) {
	if s.Duel == nil {
		return
	}
	if dp.State != network.CTOS_RESPONSE {
		return
	}
	// 原版 single_duel.cpp:1419-1423：GetResponse 不回写 lastResponse
	// （lastResponse 由 WaitforResponse 记录，Go 在 waitForResponse 中同步，
	// 见 duel_common.go），也没有在扣减前清零 timeElapsed——旧代码先
	// timeElapsed=0 导致下方 timeLimit -= timeElapsed 恒减 0，计时银行
	// 永不扣减。
	length := len(msgBuffer)
	if length > ocgcore.SIZE_RETURN_VALUE {
		length = ocgcore.SIZE_RETURN_VALUE
	}
	recordResponseAndSet(&s.DuelMode, msgBuffer[:length])
	if s.HostInfo.TimeLimit != 0 {
		if s.timeLimit[dp.Type] > s.timeElapsed {
			s.timeLimit[dp.Type] -= s.timeElapsed
		} else {
			s.timeLimit[dp.Type] = 0
		}
		s.timeElapsed = 0
	}
	s.Process()
}

// applyWinResult 记录一局胜负（Surrender / 超时 / MSG_WIN 共用同一套
// matchResult/tpPlayer 记账逻辑）。
func (s *SingleDuel) applyWinResult(player uint8) {
	if s.players[player] == s.pPlayers[player] {
		s.matchResult[s.duelCount] = 1 - player
		s.duelCount++
		s.tpPlayer = player
	} else {
		s.matchResult[s.duelCount] = player
		s.duelCount++
		s.tpPlayer = 1 - player
	}
}

func (s *SingleDuel) Surrender(dp *DuelPlayer) {
	if dp.Type > 1 || s.Duel == nil {
		return
	}
	player := dp.Type
	wbuf := make([]byte, 3)
	wbuf[0] = ocgcore.MSG_WIN
	wbuf[1] = 1 - player
	wbuf[2] = 0
	broadcastData(s, network.STOC_GAME_MSG, wbuf)
	s.applyWinResult(player)
	s.EndDuel()
	s.DuelEndProc()
	if s.ETimer != nil {
		s.ETimer.Stop()
	}
}
func (s *SingleDuel) SingleTimer() {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	running, timeout := tickDuelTimerLocked(&s.DuelMode)
	if !running {
		if !timeout {
			return
		}
		player := s.lastResponse
		wbuf := make([]byte, 3)
		wbuf[0] = ocgcore.MSG_WIN
		wbuf[1] = 1 - player
		wbuf[2] = 0x3
		broadcastData(s, network.STOC_GAME_MSG, wbuf)
		s.applyWinResult(player)
		s.EndDuel()
		s.DuelEndProc()
		if s.ETimer != nil {
			s.ETimer.Stop()
			s.ETimer = nil
		}
		return
	}
	s.ETimer = timerWheel.AfterFunc(time.Second, s.SingleTimer)

}

// duelRoom 接口实现（差异点钩子）。

func (s *SingleDuel) allPlayers() []*DuelPlayer { return s.players[:] }

func (s *SingleDuel) currentPlayer(player int) *DuelPlayer { return s.players[player] }

func (s *SingleDuel) zoneFullPair(player int) (*DuelPlayer, *DuelPlayer) {
	return s.players[player], nil
}

func (s *SingleDuel) zoneMaskedPair(player int) (*DuelPlayer, *DuelPlayer) {
	return s.players[1-player], nil
}

func (s *SingleDuel) handMaskedRecipients(player int) []*DuelPlayer {
	return []*DuelPlayer{s.players[1-player]}
}

func (s *SingleDuel) waitingNotifyRecipients(player byte) []*DuelPlayer {
	return []*DuelPlayer{s.players[1-player]}
}

// 原版 single_duel.cpp:1451-1452 的 WaitforResponse 只向 players[0]/[1]
// 发 STOC_TIME_LIMIT，不重发观战者（与 tag 一致）。
func (s *SingleDuel) timeLimitToObservers() bool { return false }

func (s *SingleDuel) isResponder(dp *DuelPlayer) bool { return dp.Type == s.lastResponse }

func (s *SingleDuel) firstFreeSeat() int {
	if s.players[0] == nil {
		return 0
	}
	return 1
}

func (s *SingleDuel) assignSeat(pos int, dp *DuelPlayer) { s.players[pos] = dp }

func (s *SingleDuel) removeFromSeat(seat int) { s.players[seat] = nil }

func (s *SingleDuel) handResultIndex(seat uint8) int { return int(seat) }

func (s *SingleDuel) handSeats() (a, a2, b, b2 *DuelPlayer) {
	return s.players[0], nil, s.players[1], nil
}

func (s *SingleDuel) recordTpPlayer(bWins bool) {
	if bWins {
		s.tpPlayer = 1
	} else {
		s.tpPlayer = 0
	}
}

func (s *SingleDuel) onJoinPassDenied(dp *DuelPlayer) { _ = s.DisconnectPlayer(dp) }

func (s *SingleDuel) onDuelEnded() {}

func (s *SingleDuel) seatCount() int { return 2 }

func (s *SingleDuel) replayFlag() uint32 { return REPLAY_UNIFORM }

func (s *SingleDuel) extraDuelOpt() uint32 { return 0 }

// loadDecksToEngine 把两名玩家的卡组按原版顺序装入引擎并写入回放：
// Main 先整体 Reverse（与原版一致，从卡组顶端开始 load），再按
// 0 号主卡组/额外 → 1 号主卡组/额外 的固定序列 AddCard
//（single_duel.cpp TPResult 的 load 调用序列）。
func (s *SingleDuel) loadDecksToEngine(d *ocgcore.Duel, rp *Replay) {
	load := func(deckContainer []*CardDataC, p uint8, location uint8) {
		rp.WriteInt32(int32(len(deckContainer)), false)
		for _, v := range deckContainer {
			d.AddCard(v.Code, int(p), location)
			rp.WriteInt32(int32(v.Code), false)
		}
	}
	slices.Reverse(s.pDeck[0].Main)
	slices.Reverse(s.pDeck[1].Main)
	load(s.pDeck[0].Main, 0, ocgcore.LOCATION_DECK)
	load(s.pDeck[0].Extra, 0, ocgcore.LOCATION_EXTRA)
	load(s.pDeck[1].Main, 1, ocgcore.LOCATION_DECK)
	load(s.pDeck[1].Extra, 1, ocgcore.LOCATION_EXTRA)
}

// refreshOnDuelStart 是开局额外卡组刷新（原版 single RefreshExtraDef，
// 即 0xe81fff/缓存）。
func (s *SingleDuel) refreshOnDuelStart() {
	s.RefreshExtraDef(0)
	s.RefreshExtraDef(1)
}

// armDuelTimer 武装决斗秒表：SingleTimer 超时处理自行重新武装
//（与原版 SingleTimer 一致）。
func (s *SingleDuel) armDuelTimer() {
	s.ETimer = timerWheel.AfterFunc(time.Second, s.SingleTimer)
}

func (s *SingleDuel) onEngineWin(player uint8) {
	if player > 1 {
		s.matchResult[s.duelCount] = 2
		s.duelCount++
		s.tpPlayer = 1 - s.tpPlayer
	} else if s.players[player] == s.pPlayers[player] {
		s.matchResult[s.duelCount] = player
		s.duelCount++
		s.tpPlayer = 1 - player
	} else {
		s.matchResult[s.duelCount] = 1 - player
		s.duelCount++
		s.tpPlayer = player
	}
}

// refreshGraveAfterSwap / refreshAfterSummon / refreshAfterChain /
// refreshAfterDamageStep / refreshAfterNewPhase / refreshSingleMoved /
// refreshSingleFlip / skipCardQuery 原本在本文件以 *Def 常量实现，差异已收进
// DuelMode.refresh 参数表（构造时填写），统一由 DuelMode 基类读表实现
//（见 duel_refresh.go）。

func (s *SingleDuel) RefreshSingle(player uint8, location uint8, sequence uint8, flag int32) {
	flag |= ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION
	var (
		queryBuffer = make([]byte, 0x1000)
		qbuf        = utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	)
	qbuf.Write([]byte{ocgcore.MSG_UPDATE_CARD, player, location, sequence})
	length := s.Duel.QueryCard(player, location, sequence, flag, qbuf.Bytes(), false)
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
	if length <= ocgcore.LEN_HEADER {
		return
	}
	var (
		clen int32
	)
	qbuf.Read(&clen)
	position := network.GetPosition(qbuf.Bytes(), 8)
	if position&ocgcore.POS_FACEDOWN != 0 {
		qbuf.Write(int32(ocgcore.QUERY_CODE), int32(0), make([]byte, clen-12))
	}
	s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}
