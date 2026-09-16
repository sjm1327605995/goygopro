package duel

import (
	"encoding/binary"
	"math/rand"
	"slices"
	"time"

	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

func newTagDuel() *TagDuel {
	return &TagDuel{DuelMode: DuelMode{Observers: make(map[string]*DuelPlayer)}}
}

type TagDuel struct {
	DuelMode
	players   [4]*DuelPlayer
	pplayer   [4]*DuelPlayer
	curPlayer [2]*DuelPlayer
	surrender [4]bool
	turnCount uint8
}

func (s *TagDuel) Chat(dp *DuelPlayer, pData []byte) {
	duelChat(s, dp, pData)
}

func (s *TagDuel) JoinGame(dp *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) {
	if !checkJoinAllowed(s, dp, pkt, isCreator) {
		return
	}
	dp.Game = s
	if s.players[0] == nil && s.players[1] == nil && s.players[2] == nil && s.players[3] == nil && len(s.Observers) == 0 {
		s.HostPlayer = dp
	}
	var scjg protocol.STOCJoinGame
	scjg.Info = s.HostInfo
	var sctc protocol.STOCTypeChange
	if s.HostPlayer == dp {
		sctc.Type = 0x10
	} else {
		sctc.Type = 0
	}
	if s.players[0] == nil || s.players[1] == nil || s.players[2] == nil || s.players[3] == nil {
		var scpe protocol.STOCHsPlayerEnter
		copy(scpe.Name[:], dp.Name[:])
		var pos uint8
		if s.players[0] == nil {
			pos = 0
		} else if s.players[1] == nil {
			pos = 1
		} else if s.players[2] == nil {
			pos = 2
		} else {
			pos = 3
		}
		scpe.Pos = pos
		for i := 0; i < 4; i++ {
			if s.players[i] != nil {
				s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_PLAYER_ENTER, scpe)
			}
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_ENTER, scpe)
		}
		s.players[pos] = dp
		dp.Type = pos
		sctc.Type |= pos
	} else {
		s.Observers[dp.ID] = dp
		dp.Type = network.NETPLAYER_TYPE_OBSERVER
		sctc.Type |= network.NETPLAYER_TYPE_OBSERVER
		var scwc protocol.STOCHsWatchChange
		scwc.WatchCount = uint16(len(s.Observers))
		for i := 0; i < 4; i++ {
			if s.players[i] != nil {
				s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_WATCH_CHANGE, scwc)
			}
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_WATCH_CHANGE, scwc)
		}
	}
	s.SendPacketDataToPlayer(dp, network.STOC_JOIN_GAME, scjg)
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, sctc)
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			var scpe protocol.STOCHsPlayerEnter
			copy(scpe.Name[:], s.players[i].Name[:])
			scpe.Pos = uint8(i)
			s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_ENTER, scpe)
			if s.ready[i] {
				var scpc protocol.STOCHsPlayerChange
				scpc.Status = uint8(i<<4) | network.PLAYERCHANGE_READY
				s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_CHANGE, scpc)
			}
		}
	}
	if len(s.Observers) > 0 {
		var scwc protocol.STOCHsWatchChange
		scwc.WatchCount = uint16(len(s.Observers))
		s.SendPacketDataToPlayer(dp, network.STOC_HS_WATCH_CHANGE, scwc)
	}
}

func (s *TagDuel) LeaveGame(dp *DuelPlayer) {
	leaveGame(s, dp)
}

func (s *TagDuel) leaveAsPlayer(dp *DuelPlayer) {
	if s.DuelStage == network.DUEL_STAGE_BEGIN {
		var scpc protocol.STOCHsPlayerChange
		s.players[dp.Type] = nil
		s.ready[dp.Type] = false
		scpc.Status = uint8(dp.Type<<4) | network.PLAYERCHANGE_LEAVE
		sendHsPlayerChangeAll(s, scpc)
	} else if s.DuelStage != network.DUEL_STAGE_END {
		s.EndDuel()
		s.DuelEndProc()
	}
	s.DisconnetPlayer(dp)
}

func (s *TagDuel) ToDuelList(dp *DuelPlayer) {
	if s.players[0] != nil && s.players[1] != nil && s.players[2] != nil && s.players[3] != nil {
		return
	}
	if dp.Type == network.NETPLAYER_TYPE_OBSERVER {
		toDuelListObserver(s, dp)
		return
	}
	// 以下为本模式特有的换座逻辑（观战者之外的大厅玩家移到下一个空位）。
	if s.ready[dp.Type] {
		return
	}
	dptype := (dp.Type + 1) % 4
	for s.players[dptype] != nil {
		dptype = (dptype + 1) % 4
	}
	var scpc protocol.STOCHsPlayerChange
	scpc.Status = uint8(dp.Type<<4) | uint8(dptype)
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_PLAYER_CHANGE, scpc)
		}
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_CHANGE, scpc)
	}
	var sctc protocol.STOCTypeChange
	sctc.Type = 0
	if dp == s.HostPlayer {
		sctc.Type = 0x10
	}
	sctc.Type |= uint8(dptype)
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, sctc)
	s.players[dp.Type] = nil
	s.players[dptype] = dp
	dp.Type = dptype
}

func (s *TagDuel) ToObserver(dp *DuelPlayer) {
	toObserver(s, dp)
}

func (s *TagDuel) PlayerReady(dp *DuelPlayer, isReady bool) {
	playerReady(s, dp, isReady)
}

func (s *TagDuel) PlayerKick(dp *DuelPlayer, pos byte) {
	playerKick(s, dp, pos)
}

func (s *TagDuel) UpdateDeck(dp *DuelPlayer, pData []byte) {
	if dp.Type > 3 || s.ready[dp.Type] {
		return
	}
	deckBuf, ok := unpackDeckData(&s.DuelMode, dp, pData)
	if !ok {
		return
	}
	s.DeckError[dp.Type] = DeckManger.LoadDeck(s.pDeck[dp.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC, false)
}

func (s *TagDuel) StartDuel(dp *DuelPlayer) {
	if dp != s.HostPlayer {
		return
	}
	if !s.ready[0] || !s.ready[1] || !s.ready[2] || !s.ready[3] {
		return
	}
	s.StopListen()
	for i := 0; i < 4; i++ {
		s.SendPacketToPlayer(s.players[i], network.STOC_DUEL_START)
	}
	for _, v := range s.Observers {
		v.State = network.CTOS_LEAVE_GAME
		s.ReSendToPlayer(v)
	}
	var deckbuff [12]byte
	pbuf := utils.NewYGOBuffer(deckbuff[:], binary.LittleEndian)
	pbuf.Write(uint16(len(s.pDeck[0].Main)), uint16(len(s.pDeck[0].Extra)), uint16(len(s.pDeck[0].Side)))
	pbuf.Write(uint16(len(s.pDeck[2].Main)), uint16(len(s.pDeck[2].Extra)), uint16(len(s.pDeck[2].Side)))
	s.SendPacketDataToPlayer(s.players[0], network.STOC_DECK_COUNT, deckbuff[:12])
	s.ReSendToPlayer(s.players[1])
	var tempbuff [6]byte
	copy(tempbuff[:], deckbuff[:6])
	copy(deckbuff[:6], deckbuff[6:12])
	copy(deckbuff[6:12], tempbuff[:])
	s.SendPacketDataToPlayer(s.players[2], network.STOC_DECK_COUNT, deckbuff[:12])
	s.ReSendToPlayer(s.players[3])
	s.SendPacketToPlayer(s.players[0], network.STOC_SELECT_HAND)
	s.ReSendToPlayer(s.players[2])
	s.handResult[0] = 0
	s.handResult[1] = 0
	s.players[0].State = network.CTOS_HAND_RESULT
	s.players[2].State = network.CTOS_HAND_RESULT
	s.DuelStage = network.DUEL_STAGE_FINGER
}

func (s *TagDuel) HandResult(dp *DuelPlayer, res byte) {
	handResult(s, dp, res)
}

func (s *TagDuel) TPResult(dp *DuelPlayer, tp byte) {
	if dp.State != network.CTOS_TP_RESULT {
		return
	}
	s.DuelStage = network.DUEL_STAGE_DUELING
	var swapped bool
	s.pplayer[0] = s.players[0]
	s.pplayer[1] = s.players[1]
	s.pplayer[2] = s.players[2]
	s.pplayer[3] = s.players[3]
	if (tp != 0 && dp.Type == 2) || (tp == 0 && dp.Type == 0) {
		s.players[0], s.players[2] = s.players[2], s.players[0]
		s.players[1], s.players[3] = s.players[3], s.players[1]
		s.players[0].Type, s.players[1].Type, s.players[2].Type, s.players[3].Type = 0, 1, 2, 3
		s.pDeck[0], s.pDeck[2] = s.pDeck[2], s.pDeck[0]
		s.pDeck[1], s.pDeck[3] = s.pDeck[3], s.pDeck[1]
		swapped = true
	}
	s.turnCount = 0
	s.curPlayer[0] = s.players[0]
	s.curPlayer[1] = s.players[3]
	dp.State = network.CTOS_RESPONSE
	seed := rand.Uint32()

	var rnd = rand.New(rand.NewSource(int64(seed)))
	rh := ExtendedReplayHeader{}
	rh.Base.ID = REPLAY_ID_YRP2
	rh.Base.Version = PRO_VERSION
	rh.Base.Flag = REPLAY_UNIFORM | REPLAY_TAG
	rh.Base.Seed = seed
	for i := 0; i < SEED_COUNT; i++ {
		rh.SeedSequence[i] = rand.Uint32()
	}
	rh.Base.StartTime = uint32(time.Now().Unix())
	s.lastReplay = NewReplay()
	s.lastReplay.BeginRecord()
	s.lastReplay.WriteHeader(rh)
	for i := 0; i < 4; i++ {
		name := make([]byte, 40)
		for j := 0; j < 20; j++ {
			binary.LittleEndian.PutUint16(name[j*2:], s.players[i].Name[j])
		}
		s.lastReplay.WriteData(name, false)
	}
	if s.HostInfo.NoShuffleDeck == 0 {
		rnd.Shuffle(len(s.pDeck[0].Main), func(i, j int) {
			s.pDeck[0].Main[i], s.pDeck[0].Main[j] = s.pDeck[0].Main[j], s.pDeck[0].Main[i]
		})
		rnd.Shuffle(len(s.pDeck[1].Main), func(i, j int) {
			s.pDeck[1].Main[i], s.pDeck[1].Main[j] = s.pDeck[1].Main[j], s.pDeck[1].Main[i]
		})
		rnd.Shuffle(len(s.pDeck[2].Main), func(i, j int) {
			s.pDeck[2].Main[i], s.pDeck[2].Main[j] = s.pDeck[2].Main[j], s.pDeck[2].Main[i]
		})
		rnd.Shuffle(len(s.pDeck[3].Main), func(i, j int) {
			s.pDeck[3].Main[i], s.pDeck[3].Main[j] = s.pDeck[3].Main[j], s.pDeck[3].Main[i]
		})
	}
	s.timeLimit[0], s.timeLimit[1] = int16(s.HostInfo.TimeLimit), int16(s.HostInfo.TimeLimit)
	s.Duel = ocgcore.NewDuelV2(rh.SeedSequence)
	s.Duel.InitPlayers(s.HostInfo.StartLp, int32(s.HostInfo.StartHand), int32(s.HostInfo.DrawCount))
	opt := uint32(s.HostInfo.DuelRule) << 16
	if s.HostInfo.NoShuffleDeck != 0 {
		opt |= ocgcore.DUEL_PSEUDO_SHUFFLE
	}
	opt |= ocgcore.DUEL_TAG_MODE
	s.lastReplay.WriteInt32(s.HostInfo.StartLp, false)
	s.lastReplay.WriteInt32(int32(s.HostInfo.StartHand), false)
	s.lastReplay.WriteInt32(int32(s.HostInfo.DrawCount), false)
	s.lastReplay.WriteInt32(int32(opt), false)
	s.lastReplay.Flush()
	slices.Reverse(s.pDeck[0].Main)
	slices.Reverse(s.pDeck[1].Main)
	slices.Reverse(s.pDeck[2].Main)
	slices.Reverse(s.pDeck[3].Main)
	loadSingle := func(deckContainer []*CardDataC, p uint8, location uint8) {
		s.lastReplay.WriteInt32(int32(len(deckContainer)), false)
		for _, v := range deckContainer {
			s.Duel.AddCard(v.Code, int(p), location)
			s.lastReplay.WriteInt32(int32(v.Code), false)
		}
	}
	loadTag := func(deckContainer []*CardDataC, p uint8, location uint8) {
		s.lastReplay.WriteInt32(int32(len(deckContainer)), false)
		for _, v := range deckContainer {
			s.Duel.AddTagCard(v.Code, p, location)
			s.lastReplay.WriteInt32(int32(v.Code), false)
		}
	}
	loadSingle(s.pDeck[0].Main, 0, ocgcore.LOCATION_DECK)
	loadSingle(s.pDeck[0].Extra, 0, ocgcore.LOCATION_EXTRA)
	loadTag(s.pDeck[1].Main, 0, ocgcore.LOCATION_DECK)
	loadTag(s.pDeck[1].Extra, 0, ocgcore.LOCATION_EXTRA)
	loadSingle(s.pDeck[3].Main, 1, ocgcore.LOCATION_DECK)
	loadSingle(s.pDeck[3].Extra, 1, ocgcore.LOCATION_EXTRA)
	loadTag(s.pDeck[2].Main, 1, ocgcore.LOCATION_DECK)
	loadTag(s.pDeck[2].Extra, 1, ocgcore.LOCATION_EXTRA)
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
	s.ReSendToPlayer(s.players[1])
	startBuf[1] = 1
	s.SendPacketDataToPlayer(s.players[2], network.STOC_GAME_MSG, startBuf[:19])
	s.ReSendToPlayer(s.players[3])
	if !swapped {
		startBuf[1] = 0x10
	} else {
		startBuf[1] = 0x11
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_GAME_MSG, startBuf[:19])
	}
	s.RefreshExtra(0, 0x81fff4, 0)
	s.RefreshExtra(1, 0x81fff4, 0)
	s.Duel.Start(int32(opt))
	if s.HostInfo.TimeLimit != 0 {
		s.timeElapsed = 0
		s.ETimer = timerWheel.AfterFunc(time.Second, s.TagTimer)
	}
	s.Process()
}

func (s *TagDuel) Process() {
	processDuel(s)
}

func (s *TagDuel) DuelEndProc() {
	duelEndProcSimple(s)
}

func (s *TagDuel) Surrender(dp *DuelPlayer) {
	if dp.Type > 3 || s.Duel == nil {
		return
	}
	player := dp.Type
	if s.surrender[player] {
		return
	}
	teammateMap := [4]uint8{1, 0, 3, 2}
	teammate := teammateMap[player]
	if !s.surrender[teammate] {
		s.surrender[player] = true
		s.SendPacketToPlayer(s.players[player], network.STOC_TEAMMATE_SURRENDER)
		s.SendPacketToPlayer(s.players[teammate], network.STOC_TEAMMATE_SURRENDER)
		return
	}
	winPlayerMap := [4]uint8{1, 1, 0, 0}
	var wbuf [3]byte
	wbuf[0] = ocgcore.MSG_WIN
	wbuf[1] = winPlayerMap[player]
	wbuf[2] = 0
	broadcastData(s, network.STOC_GAME_MSG, wbuf[:])
	s.EndDuel()
	s.DuelEndProc()
	if s.ETimer != nil {
		s.ETimer.Stop()
	}
}

// tagAnalyzeTable 在共享表之上追加 tag 特有的消息
// （MSG_TAG_SWAP 为 tag 独有；MSG_NEW_TURN / MSG_MATCH_KILL 两模式行为不同）。
var tagAnalyzeTable = copyAnalyzeTable(map[uint8]analyzeHandler{
	ocgcore.MSG_NEW_TURN: func(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		return m.(*TagDuel).analyzeNewTurn(pbuf, offset)
	},
	ocgcore.MSG_TAG_SWAP: func(m duelRoom, _ uint8, pbuf, offset *utils.YGOBuffer) int {
		return m.(*TagDuel).analyzeTagSwap(pbuf, offset)
	},
	ocgcore.MSG_MATCH_KILL: func(_ duelRoom, _ uint8, pbuf, _ *utils.YGOBuffer) int {
		pbuf.Next(4)
		return 0
	},
})

func (s *TagDuel) Analyze(msgBuffer []byte) int {
	return runAnalyze(s, tagAnalyzeTable, msgBuffer)
}

// analyzeNewTurn 新回合开始：广播、重置时间限制，并按回合数奇偶轮换
// 双方的当前操作者（curPlayer）。
func (s *TagDuel) analyzeNewTurn(pbuf, offset *utils.YGOBuffer) int {
	pbuf.Next(1)
	s.timeLimit[0] = int16(s.HostInfo.TimeLimit)
	s.timeLimit[1] = int16(s.HostInfo.TimeLimit)
	broadcastData(s, network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	if s.turnCount > 0 {
		if s.turnCount%2 == 0 {
			if s.curPlayer[0] == s.players[0] {
				s.curPlayer[0] = s.players[1]
			} else {
				s.curPlayer[0] = s.players[0]
			}
		} else {
			if s.curPlayer[1] == s.players[2] {
				s.curPlayer[1] = s.players[3]
			} else {
				s.curPlayer[1] = s.players[2]
			}
		}
	}
	s.turnCount++
	return 0
}

// analyzeTagSwap 队友交换（TAG_SWAP）：当前操作者收完整手牌/额外信息，
// 未公开（0x80 标记）的卡码对其他人就地抹零后转发，然后整区刷新。
func (s *TagDuel) analyzeTagSwap(pbuf, offset *utils.YGOBuffer) int {
	var player uint8
	_ = pbuf.Read(&player)
	pbuf.Next(1) // skip main_size
	var ecount uint8
	_ = pbuf.Read(&ecount) // extra_size
	pbuf.Next(1)           // skip extra_p_count
	var hcount uint8
	_ = pbuf.Read(&hcount) // hand_size
	pbufw := pbuf.Clone()
	pbufw.Next(4)
	pbuf.Next(int(hcount)*4 + int(ecount)*4 + 4)
	s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
	for i := uint8(0); i < hcount; i++ {
		if pbufw.At(3)&0x80 == 0 {
			binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
		} else {
			pbufw.Next(4)
		}
	}
	for i := uint8(0); i < ecount; i++ {
		if pbufw.At(3)&0x80 == 0 {
			binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
		} else {
			pbufw.Next(4)
		}
	}
	for i := 0; i < 4; i++ {
		if s.players[i] != s.curPlayer[player] {
			s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
		}
	}
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
	s.RefreshExtra(int(player), 0x81fff4, 0)
	s.RefreshMzone(0, 0x81fff, 0)
	s.RefreshMzone(1, 0x81fff, 0)
	s.RefreshSzone(0, 0x681fff, 0)
	s.RefreshSzone(1, 0x681fff, 0)
	s.RefreshHand(0, 0x781fff, 0)
	s.RefreshHand(1, 0x781fff, 0)
	return 0
}

func (s *TagDuel) GetResponse(dp *DuelPlayer, msgBuffer []byte) {
	if dp.State != network.CTOS_RESPONSE {
		return
	}
	if s.Duel == nil {
		return
	}
	if s.DuelStage != network.DUEL_STAGE_DUELING {
		return
	}
	if dp != s.curPlayer[s.lastResponse] {
		return
	}
	recordResponseAndSet(&s.DuelMode, msgBuffer)
	s.players[dp.Type].State = 0xff
	if s.HostInfo.TimeLimit != 0 {
		respType := 0
		if dp.Type >= 2 {
			respType = 1
		}
		if s.timeLimit[respType] >= s.timeElapsed {
			s.timeLimit[respType] -= s.timeElapsed
		} else {
			s.timeLimit[respType] = 0
		}
		s.timeElapsed = 0
	}
	s.Process()
}

func (s *TagDuel) TimeConfirm(dp *DuelPlayer) {
	timeConfirm(s, dp)
}

func (s *TagDuel) EndDuel() {
	endDuel(s)
}

func (s *TagDuel) WaitforResponse(player byte) {
	waitForResponse(s, player)
}

func (s *TagDuel) RefreshMzoneDef(player int) {
	refreshMzoneDef(s, player)
}

func (s *TagDuel) RefreshMzone(player int, flag uint32, useCache int) {
	refreshZone(s, player, int(ocgcore.LOCATION_MZONE), flag, useCache)
}

func (s *TagDuel) RefreshSzoneDef(player int) {
	refreshSzoneDef(s, player)
}

func (s *TagDuel) RefreshSzone(player int, flag uint32, useCache int) {
	refreshZone(s, player, int(ocgcore.LOCATION_SZONE), flag, useCache)
}

func (s *TagDuel) RefreshHandDef(player int) {
	refreshHandDef(s, player)
}

func (s *TagDuel) RefreshHand(player int, flag uint32, useCache int) {
	refreshHand(s, player, flag, useCache)
}

func (s *TagDuel) RefreshGraveDef(player int) {
	refreshGraveDef(s, player)
}

func (s *TagDuel) RefreshGrave(player int, flag uint32, useCache int) {
	refreshGrave(s, player, flag, useCache)
}

func (s *TagDuel) RefreshExtraDef(player int) {
	refreshExtraDef(s, player)
}

func (s *TagDuel) RefreshExtra(player int, flag uint32, useCache int) {
	refreshExtra(s, player, flag, useCache)
}

// duelRoom 接口实现（差异点钩子）。

func (s *TagDuel) allPlayers() []*DuelPlayer { return s.players[:] }

func (s *TagDuel) currentPlayer(player int) *DuelPlayer { return s.curPlayer[player] }

func (s *TagDuel) zonePair(player int) (int, int) {
	if player == 0 {
		return 0, 2
	}
	return 2, 0
}

func (s *TagDuel) zoneFullPair(player int) (*DuelPlayer, *DuelPlayer) {
	pid, _ := s.zonePair(player)
	return s.players[pid], s.players[pid+1]
}

func (s *TagDuel) zoneMaskedPair(player int) (*DuelPlayer, *DuelPlayer) {
	_, pid := s.zonePair(player)
	return s.players[pid], s.players[pid+1]
}

func (s *TagDuel) handMaskedRecipients(player int) []*DuelPlayer {
	recipients := make([]*DuelPlayer, 0, 3)
	for i := 0; i < 4; i++ {
		if s.players[i] != s.curPlayer[player] {
			recipients = append(recipients, s.players[i])
		}
	}
	return recipients
}

func (s *TagDuel) skipCardQuery(clen int32) bool { return clen <= ocgcore.LEN_HEADER }

func (s *TagDuel) waitingNotifyRecipients(player byte) []*DuelPlayer {
	recipients := make([]*DuelPlayer, 0, 3)
	for i := 0; i < 4; i++ {
		if s.players[i] != s.curPlayer[player] {
			recipients = append(recipients, s.players[i])
		}
	}
	return recipients
}

func (s *TagDuel) timeLimitToObservers() bool { return false }

func (s *TagDuel) isResponder(dp *DuelPlayer) bool { return dp == s.curPlayer[s.lastResponse] }

func (s *TagDuel) firstFreeSeat() int {
	for i := 0; i < 4; i++ {
		if s.players[i] == nil {
			return i
		}
	}
	return 3
}

func (s *TagDuel) assignSeat(pos int, dp *DuelPlayer) { s.players[pos] = dp }

func (s *TagDuel) removeFromSeat(seat int) { s.players[seat] = nil }

func (s *TagDuel) handResultIndex(seat uint8) int {
	if seat == 0 {
		return 0
	}
	return 1
}

func (s *TagDuel) handSeats() (a, a2, b, b2 *DuelPlayer) {
	return s.players[0], s.players[1], s.players[2], s.players[3]
}

func (s *TagDuel) recordTpPlayer(bWins bool) {}

func (s *TagDuel) onJoinPassDenied(dp *DuelPlayer) {}

func (s *TagDuel) onEngineWin(player uint8) {}

func (s *TagDuel) refreshGraveAfterSwap(player int) { s.RefreshGrave(player, 0x81fff4, 0) }

func (s *TagDuel) refreshAfterSummon() {
	s.RefreshMzone(0, 0x81fff4, 0)
	s.RefreshMzone(1, 0x81fff4, 0)
	s.RefreshSzone(0, 0x81fff4, 0)
	s.RefreshSzone(1, 0x81fff4, 0)
}

func (s *TagDuel) refreshAfterChain() {
	s.RefreshMzone(0, 0x81fff4, 0)
	s.RefreshMzone(1, 0x81fff4, 0)
	s.RefreshSzone(0, 0x81fff4, 0)
	s.RefreshSzone(1, 0x81fff4, 0)
	s.RefreshHand(0, 0x781fff, 0)
	s.RefreshHand(1, 0x781fff, 0)
}

func (s *TagDuel) refreshAfterDamageStep() {
	s.RefreshMzone(0, 0x81fff4, 0)
	s.RefreshMzone(1, 0x81fff4, 0)
}

func (s *TagDuel) refreshAfterNewPhase() {
	s.RefreshMzone(0, 0x81fff4, 0)
	s.RefreshMzone(1, 0x81fff4, 0)
	s.RefreshSzone(0, 0x81fff4, 0)
	s.RefreshSzone(1, 0x81fff4, 0)
	s.RefreshHand(0, 0x781fff, 0)
	s.RefreshHand(1, 0x781fff, 0)
}

func (s *TagDuel) refreshSingleMoved(cc, cl, cs uint8) { s.RefreshSingle(cc, cl, cs, 0x81fff4) }

func (s *TagDuel) refreshSingleFlip(cc, cl, cs uint8) { s.RefreshSingle(cc, cl, cs, 0x81fff4) }

func (s *TagDuel) onDuelEnded() {
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.players[i].State = 0xff
		}
	}
}

func (s *TagDuel) RefreshSingleDef(player uint8, location uint8, sequence uint8) {
	s.RefreshSingle(player, location, sequence, 0xf81fff)
}

func (s *TagDuel) RefreshSingle(player uint8, location uint8, sequence uint8, flag int32) {
	flag |= int32(ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION)
	var queryBuffer = make([]byte, 0x1000)
	var qbuf = utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	qbuf.Write([]byte{ocgcore.MSG_UPDATE_CARD, player, location, sequence})
	length := s.Duel.QueryCard(player, location, sequence, int32(flag), qbuf.Bytes(), false)
	position := network.GetPosition(qbuf.Bytes(), 12)
	if location&uint8(ocgcore.LOCATION_ONFIELD) != 0 {
		pid := 0
		if player == 0 {
			pid = 0
		} else {
			pid = 2
		}
		s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
		s.ReSendToPlayer(s.players[pid+1])
		if position&ocgcore.POS_FACEUP != 0 {
			pid = 2 - pid
			s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
			s.ReSendToPlayer(s.players[pid+1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		}
	} else {
		pid := 0
		if player == 0 {
			pid = 0
		} else {
			pid = 2
		}
		s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, queryBuffer[:int(length)+4])
		s.ReSendToPlayer(s.players[pid+1])
		if location == uint8(ocgcore.LOCATION_REMOVED) && (position&ocgcore.POS_FACEDOWN) != 0 {
			return
		}
		if location&0x90 != 0 {
			for i := 0; i < 4; i++ {
				if s.players[i] != s.curPlayer[int(player)] {
					s.ReSendToPlayer(s.players[i])
				}
			}
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		}
	}
}

func (s *TagDuel) TagTimer() {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	running, timeout := tickDuelTimerLocked(&s.DuelMode)
	if !running {
		if !timeout {
			return
		}
		var wbuf [3]byte
		player := s.lastResponse
		wbuf[0] = ocgcore.MSG_WIN
		wbuf[1] = 1 - player
		wbuf[2] = 0x3
		s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, wbuf[:])
		s.ReSendToPlayer(s.players[1])
		s.ReSendToPlayer(s.players[2])
		s.ReSendToPlayer(s.players[3])
		s.EndDuel()
		s.DuelEndProc()
		if s.ETimer != nil {
			s.ETimer.Stop()
		}
		return
	}
	// 注意：与原实现一致，TagTimer 非超时路径不重新武装定时器。
}
