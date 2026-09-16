package duel

import (
	"encoding/binary"
	"math/rand"
	"slices"
	"time"

	"github.com/duke-git/lancet/v2/condition"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

const PRO_VERSION = 0x1361

func newSingleDuel(matchMode bool) *SingleDuel {
	return &SingleDuel{MatchMode: matchMode, DuelMode: DuelMode{Observers: make(map[string]*DuelPlayer)}}
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

func (s *SingleDuel) Chat(dp *DuelPlayer, pData []byte) {
	duelChat(s, dp, pData)
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
		scjg = protocol.STOCJoinGame{Info: s.HostInfo}
		sctc = protocol.STOCTypeChange{Type: condition.Ternary[bool, uint8](s.HostPlayer == dp, 0x10, 0)}
	)
	if s.players[0] == nil || s.players[1] == nil {
		var scpe protocol.STOCHsPlayerEnter
		copy(scpe.Name[:], dp.Name[:])
		if s.players[0] == nil {
			scpe.Pos = 0
		} else {
			scpe.Pos = 1
		}
		if s.players[0] != nil {
			s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_PLAYER_ENTER, scpe)
		}
		if s.players[1] != nil {
			s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_PLAYER_ENTER, scpe)
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_ENTER, scpe)
		}
		if s.players[0] == nil {
			s.players[0] = dp
			dp.Type = network.NETPLAYER_TYPE_PLAYER1
			sctc.Type |= network.NETPLAYER_TYPE_PLAYER1
		} else {
			s.players[1] = dp
			dp.Type = network.NETPLAYER_TYPE_PLAYER2
			sctc.Type |= network.NETPLAYER_TYPE_PLAYER2
		}
	} else {
		s.Observers[dp.ID] = dp
		dp.Type = network.NETPLAYER_TYPE_OBSERVER
		sctc.Type |= network.NETPLAYER_TYPE_OBSERVER
		var scwc protocol.STOCHsWatchChange
		scwc.WatchCount = uint16(len(s.Observers))
		if s.players[0] != nil {
			s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_WATCH_CHANGE, scwc)
		}
		if s.players[1] != nil {
			s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_WATCH_CHANGE, scwc)
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_WATCH_CHANGE, scwc)
		}
	}
	s.SendPacketDataToPlayer(dp, network.STOC_JOIN_GAME, scjg)
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, sctc)
	if s.players[0] != nil {
		var scpe protocol.STOCHsPlayerEnter
		copy(scpe.Name[:], s.players[0].Name[:])
		scpe.Pos = 0
		s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_ENTER, scpe)
		if s.ready[0] {
			var scpc protocol.STOCHsPlayerChange
			scpc.Status = network.PLAYERCHANGE_READY
			s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_CHANGE, scpc)
		}
	}
	if s.players[1] != nil {
		var scpe protocol.STOCHsPlayerEnter
		copy(scpe.Name[:], s.players[1].Name[:])
		scpe.Pos = 1
		s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_ENTER, scpe)
		if s.ready[1] {
			var scpc protocol.STOCHsPlayerChange
			scpc.Status = 0x10 | network.PLAYERCHANGE_READY
			s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_CHANGE, scpc)
		}
	}
	if len(s.Observers) > 0 {
		var scwc protocol.STOCHsWatchChange
		scwc.WatchCount = uint16(len(s.Observers))
		s.SendPacketDataToPlayer(dp, network.STOC_HS_WATCH_CHANGE, scwc)
	}
}

func (s *SingleDuel) LeaveGame(dp *DuelPlayer) {
	leaveGame(s, dp)
}

func (s *SingleDuel) leaveAsPlayer(dp *DuelPlayer) {
	if s.DuelStage == network.DUEL_STAGE_BEGIN {
		var scpc protocol.STOCHsPlayerChange
		s.players[dp.Type] = nil
		s.ready[dp.Type] = false
		scpc.Status = dp.Type<<4 | network.PLAYERCHANGE_LEAVE
		sendHsPlayerChangeAll(s, scpc)
		_ = s.DisconnetPlayer(dp)
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
	_ = s.DisconnetPlayer(dp)
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

func (s *SingleDuel) ToObserver(dp *DuelPlayer) {
	toObserver(s, dp)
}

func (s *SingleDuel) PlayerReady(dp *DuelPlayer, isReady bool) {
	playerReady(s, dp, isReady)
}

func (s *SingleDuel) PlayerKick(dp *DuelPlayer, pos byte) {
	playerKick(s, dp, pos)
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
		s.DeckError[dp.Type] = DeckManger.LoadDeck(s.pDeck[dp.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC, false)
	} else {
		if DeckManger.LoadSide(s.pDeck[dp.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC) {
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
	if dp != s.HostPlayer {
		return
	}
	if !s.ready[0] || !s.ready[1] {
		return
	}
	s.StopListen()
	s.SendPacketToPlayer(s.players[0], network.STOC_DUEL_START)
	s.ReSendToPlayer(s.players[1])
	for _, v := range s.Observers {
		v.State = network.CTOS_LEAVE_GAME
		s.ReSendToPlayer(v)
	}
	var deckBuff = make([]byte, 12)
	pBuf := utils.NewYGOBuffer(deckBuff, binary.LittleEndian)
	pBuf.Write(
		int16(len(s.pDeck[0].Main)), int16(len(s.pDeck[0].Extra)), int16(len(s.pDeck[0].Side)),
		int16(len(s.pDeck[1].Main)), int16(len(s.pDeck[1].Extra)), int16(len(s.pDeck[1].Side)))
	s.SendPacketDataToPlayer(s.players[0], network.STOC_DECK_COUNT, deckBuff)

	// 交换前6字节和后6字节
	temp := make([]byte, 6)
	copy(temp, deckBuff[:6])
	copy(deckBuff[:6], deckBuff[6:])
	copy(deckBuff[6:], temp)
	s.SendPacketDataToPlayer(s.players[1], network.STOC_DECK_COUNT, deckBuff)
	s.SendPacketToPlayer(s.players[0], network.STOC_SELECT_HAND)
	s.ReSendToPlayer(s.players[1])
	s.handResult[0] = 0
	s.handResult[1] = 0
	s.players[0].State = network.CTOS_HAND_RESULT
	s.players[1].State = network.CTOS_HAND_RESULT
	s.DuelStage = network.DUEL_STAGE_FINGER
}

func (s *SingleDuel) HandResult(dp *DuelPlayer, res byte) {
	handResult(s, dp, res)
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
	dp.State = network.CTOS_RESPONSE
	seed := rand.Uint32()

	var rnd = rand.New(rand.NewSource(int64(seed)))
	rh := ExtendedReplayHeader{}
	rh.Base.ID = REPLAY_ID_YRP2
	rh.Base.Version = PRO_VERSION
	rh.Base.Flag = REPLAY_UNIFORM
	rh.Base.Seed = seed
	for i := 0; i < SEED_COUNT; i++ {
		rh.SeedSequence[i] = rand.Uint32()
	}
	rh.Base.StartTime = uint32(time.Now().Unix())
	s.lastReplay = NewReplay()
	s.lastReplay.BeginRecord()
	s.lastReplay.WriteHeader(rh)
	name0 := make([]byte, 40)
	for i := 0; i < 20; i++ {
		binary.LittleEndian.PutUint16(name0[i*2:], s.players[0].Name[i])
	}
	s.lastReplay.WriteData(name0, false)
	name1 := make([]byte, 40)
	for i := 0; i < 20; i++ {
		binary.LittleEndian.PutUint16(name1[i*2:], s.players[1].Name[i])
	}
	s.lastReplay.WriteData(name1, false)
	if s.HostInfo.NoShuffleDeck == 0 {
		rnd.Shuffle(len(s.pDeck[0].Main), func(i, j int) {
			s.pDeck[0].Main[i], s.pDeck[0].Main[j] = s.pDeck[0].Main[j], s.pDeck[0].Main[i]
		})
		rnd.Shuffle(len(s.pDeck[1].Main), func(i, j int) {
			s.pDeck[1].Main[i], s.pDeck[1].Main[j] = s.pDeck[1].Main[j], s.pDeck[1].Main[i]
		})
	}
	s.timeLimit[0], s.timeLimit[1] = int16(s.HostInfo.TimeLimit), int16(s.HostInfo.TimeLimit)

	s.Duel = ocgcore.NewDuelV2(rh.SeedSequence)
	s.Duel.InitPlayers(s.HostInfo.StartLp, int32(s.HostInfo.StartHand), int32(s.HostInfo.DrawCount))

	opt := uint32(s.HostInfo.DuelRule) << 16
	if s.HostInfo.NoShuffleDeck != 0 {
		opt |= ocgcore.DUEL_PSEUDO_SHUFFLE
	}
	s.lastReplay.WriteInt32(s.HostInfo.StartLp, false)
	s.lastReplay.WriteInt32(int32(s.HostInfo.StartHand), false)
	s.lastReplay.WriteInt32(int32(s.HostInfo.DrawCount), false)
	s.lastReplay.WriteInt32(int32(opt), false)
	s.lastReplay.Flush()
	load := func(deckContainer []*CardDataC, p uint8, location uint8) {
		s.lastReplay.WriteInt32(int32(len(deckContainer)), false)
		for _, v := range deckContainer {
			s.Duel.AddCard(v.Code, int(p), location)
			s.lastReplay.WriteInt32(int32(v.Code), false)
		}
	}
	slices.Reverse(s.pDeck[0].Main)
	slices.Reverse(s.pDeck[1].Main)
	load(s.pDeck[0].Main, 0, ocgcore.LOCATION_DECK)
	load(s.pDeck[0].Extra, 0, ocgcore.LOCATION_EXTRA)
	load(s.pDeck[1].Main, 1, ocgcore.LOCATION_DECK)
	load(s.pDeck[1].Extra, 1, ocgcore.LOCATION_EXTRA)

	startBuf := make([]byte, 32)
	pBuf := utils.NewYGOBuffer(startBuf, binary.LittleEndian)
	pBuf.Write(
		uint8(ocgcore.MSG_START), uint8(0), uint8(s.HostInfo.DuelRule),
		s.HostInfo.StartLp, s.HostInfo.StartLp,
		uint16(s.Duel.QueryFieldCount(0, ocgcore.LOCATION_DECK)),
		uint16(s.Duel.QueryFieldCount(0, ocgcore.LOCATION_EXTRA)),
		uint16(s.Duel.QueryFieldCount(1, ocgcore.LOCATION_DECK)),
		uint16(s.Duel.QueryFieldCount(1, ocgcore.LOCATION_EXTRA)),
	)
	s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, startBuf[:19])
	startBuf[1] = 1
	s.SendPacketDataToPlayer(s.players[1], network.STOC_GAME_MSG, startBuf[:19])
	if !swapped {
		startBuf[1] = 0x10
	} else {
		startBuf[1] = 0x11
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_GAME_MSG, startBuf[:19])
	}
	s.RefreshExtraDef(0)
	s.RefreshExtraDef(1)
	s.Duel.Start(int32(opt))
	if s.HostInfo.TimeLimit != 0 {
		s.timeElapsed = 0
		s.ETimer = timerWheel.AfterFunc(time.Second, s.SingleTimer)

	}

	s.Process()
}

func (s *SingleDuel) Process() {
	processDuel(s)
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

func (s *SingleDuel) WaitforResponse(player byte) {
	waitForResponse(s, player)
}
func (s *SingleDuel) TimeConfirm(dp *DuelPlayer) {
	timeConfirm(s, dp)
}
func (s *SingleDuel) GetResponse(dp *DuelPlayer, msgBuffer []byte) {
	if s.Duel == nil {
		return
	}
	if dp.State != network.CTOS_RESPONSE {
		return
	}
	if s.DuelStage == network.DUEL_STAGE_DUELING {
		s.lastResponse = dp.Type
		s.timeElapsed = 0
	}
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
func (s *SingleDuel) EndDuel() {
	endDuel(s)
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
func (s *SingleDuel) RefreshExtraDef(player int) {
	refreshExtraDef(s, player)
}
func (s *SingleDuel) RefreshExtra(player int, flag uint32, useCache int) {
	refreshExtra(s, player, flag, useCache)
}
func (s *SingleDuel) RefreshMzoneDef(player int) {
	refreshMzoneDef(s, player)
}
func (s *SingleDuel) RefreshMzone(player int, flag uint32, useCache int) {
	refreshZone(s, player, int(ocgcore.LOCATION_MZONE), flag, useCache)
}
func (s *SingleDuel) RefreshSzoneDef(player int) {
	refreshSzoneDef(s, player)
}
func (s *SingleDuel) RefreshSzone(player int, flag uint32, useCache int) {
	refreshZone(s, player, int(ocgcore.LOCATION_SZONE), flag, useCache)
}
func (s *SingleDuel) RefreshHandDef(player int) {
	refreshHandDef(s, player)
}
func (s *SingleDuel) RefreshHand(player int, flag uint32, useCache int) {
	refreshHand(s, player, flag, useCache)
}
func (s *SingleDuel) RefreshSingleDef(player uint8, location uint8, sequence uint8) {
	s.RefreshSingle(player, location, sequence, 0xf81fff)
}
func (s *SingleDuel) RefreshGraveDef(player int) {
	refreshGraveDef(s, player)
}
func (s *SingleDuel) RefreshGrave(player int, flag uint32, useCache int) {
	refreshGrave(s, player, flag, useCache)
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

func (s *SingleDuel) skipCardQuery(clen int32) bool { return clen < ocgcore.LEN_HEADER }

func (s *SingleDuel) waitingNotifyRecipients(player byte) []*DuelPlayer {
	return []*DuelPlayer{s.players[1-player]}
}

func (s *SingleDuel) timeLimitToObservers() bool { return true }

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

func (s *SingleDuel) onJoinPassDenied(dp *DuelPlayer) { _ = s.DisconnetPlayer(dp) }

func (s *SingleDuel) onDuelEnded() {}

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

func (s *SingleDuel) refreshGraveAfterSwap(player int) { s.RefreshGraveDef(player) }

func (s *SingleDuel) refreshAfterSummon() {
	s.RefreshMzoneDef(0)
	s.RefreshMzoneDef(1)
	s.RefreshSzoneDef(0)
	s.RefreshSzoneDef(1)
}

func (s *SingleDuel) refreshAfterChain() {
	s.RefreshMzoneDef(0)
	s.RefreshMzoneDef(1)
	s.RefreshSzoneDef(0)
	s.RefreshSzoneDef(1)
	s.RefreshHandDef(0)
	s.RefreshHandDef(1)
}

func (s *SingleDuel) refreshAfterDamageStep() {
	s.RefreshMzoneDef(0)
	s.RefreshMzoneDef(1)
}

func (s *SingleDuel) refreshAfterNewPhase() {
	s.RefreshMzoneDef(0)
	s.RefreshMzoneDef(1)
	s.RefreshSzoneDef(0)
	s.RefreshSzoneDef(1)
}

func (s *SingleDuel) refreshSingleMoved(cc, cl, cs uint8) { s.RefreshSingleDef(cc, cl, cs) }

func (s *SingleDuel) refreshSingleFlip(cc, cl, cs uint8) { s.RefreshSingle(cc, cl, cs, 0xf81fff) }

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
