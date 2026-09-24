package duel

import (
	"encoding/binary"
	"slices"
	"time"

	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

func newTagDuel() *TagDuel {
	td := &TagDuel{DuelMode: DuelMode{
		Observers: make(map[string]*DuelPlayer),
		// 刷新参数表：tag 的取值为原版 TagDuel 的 0x81fff4 / 0x781fff（无缓存）；
		// skipCardQueryLe = true（边界语义 clen <= LEN_HEADER，与 single 的 < 不同）。
		refresh: refreshFlags{
			summonMzone:     0x81fff4,
			summonSzone:     0x81fff4,
			chainMzone:      0x81fff4,
			chainSzone:      0x81fff4,
			chainHand:       0x781fff,
			damageStepMzone: 0x81fff4,
			newPhaseMzone:   0x81fff4,
			newPhaseSzone:   0x81fff4,
			newPhaseHand:    0x781fff,
			singleMoved:     0x81fff4,
			singleFlip:      0x81fff4,
			graveAfterSwap:  0x81fff4,
			useCache:        0,
			skipCardQueryLe: true,
		},
	}}
	td.room = td
	return td
}

type TagDuel struct {
	DuelMode
	players   [4]*DuelPlayer
	pplayer   [4]*DuelPlayer
	curPlayer [2]*DuelPlayer
	surrender [4]bool
	turnCount uint8
}

func (s *TagDuel) JoinGame(dp *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) {
	if !checkJoinAllowed(s, dp, pkt, isCreator) {
		return
	}
	dp.Game = s
	if s.players[0] == nil && s.players[1] == nil && s.players[2] == nil && s.players[3] == nil && len(s.Observers) == 0 {
		s.HostPlayer = dp
	}
	var joinGamePkt protocol.STOCJoinGame
	joinGamePkt.Info = s.HostInfo
	var typeChangePkt protocol.STOCTypeChange
	if s.HostPlayer == dp {
		typeChangePkt.Type = 0x10
	} else {
		typeChangePkt.Type = 0
	}
	if s.players[0] == nil || s.players[1] == nil || s.players[2] == nil || s.players[3] == nil {
		var playerEnterPkt protocol.STOCHsPlayerEnter
		copy(playerEnterPkt.Name[:], dp.Name[:])
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
		playerEnterPkt.Pos = pos
		for i := 0; i < 4; i++ {
			if s.players[i] != nil {
				s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
			}
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
		}
		s.players[pos] = dp
		dp.Type = pos
		typeChangePkt.Type |= pos
	} else {
		s.Observers[dp.ID] = dp
		dp.Type = network.NETPLAYER_TYPE_OBSERVER
		typeChangePkt.Type |= network.NETPLAYER_TYPE_OBSERVER
		var watchChangePkt protocol.STOCHsWatchChange
		watchChangePkt.WatchCount = uint16(len(s.Observers))
		for i := 0; i < 4; i++ {
			if s.players[i] != nil {
				s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_WATCH_CHANGE, watchChangePkt)
			}
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
		}
	}
	s.SendPacketDataToPlayer(dp, network.STOC_JOIN_GAME, joinGamePkt)
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, typeChangePkt)
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			var playerEnterPkt protocol.STOCHsPlayerEnter
			copy(playerEnterPkt.Name[:], s.players[i].Name[:])
			playerEnterPkt.Pos = uint8(i)
			s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_ENTER, playerEnterPkt)
			if s.ready[i] {
				var playerChangePkt protocol.STOCHsPlayerChange
				playerChangePkt.Status = uint8(i<<4) | network.PLAYERCHANGE_READY
				s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
			}
		}
	}
	if len(s.Observers) > 0 {
		var watchChangePkt protocol.STOCHsWatchChange
		watchChangePkt.WatchCount = uint16(len(s.Observers))
		s.SendPacketDataToPlayer(dp, network.STOC_HS_WATCH_CHANGE, watchChangePkt)
	}
}

func (s *TagDuel) leaveAsPlayer(dp *DuelPlayer) {
	if s.DuelStage == network.DUEL_STAGE_BEGIN {
		var playerChangePkt protocol.STOCHsPlayerChange
		s.players[dp.Type] = nil
		s.ready[dp.Type] = false
		playerChangePkt.Status = uint8(dp.Type<<4) | network.PLAYERCHANGE_LEAVE
		sendHsPlayerChangeAll(s, playerChangePkt)
	} else if s.DuelStage != network.DUEL_STAGE_END {
		s.EndDuel()
		s.DuelEndProc()
	}
	s.DisconnectPlayer(dp)
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
	var playerChangePkt protocol.STOCHsPlayerChange
	playerChangePkt.Status = uint8(dp.Type<<4) | uint8(dptype)
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
		}
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_CHANGE, playerChangePkt)
	}
	var typeChangePkt protocol.STOCTypeChange
	typeChangePkt.Type = 0
	if dp == s.HostPlayer {
		typeChangePkt.Type = 0x10
	}
	typeChangePkt.Type |= uint8(dptype)
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, typeChangePkt)
	s.players[dp.Type] = nil
	s.players[dptype] = dp
	dp.Type = dptype
}

func (s *TagDuel) UpdateDeck(dp *DuelPlayer, pData []byte) {
	if dp.Type > 3 || s.ready[dp.Type] {
		return
	}
	deckBuf, ok := unpackDeckData(&s.DuelMode, dp, pData)
	if !ok {
		return
	}
	s.DeckError[dp.Type] = DeckManager.LoadDeck(s.pDeck[dp.Type], deckBuf.List[:], deckBuf.MainC, deckBuf.SideC, false)
}

func (s *TagDuel) StartDuel(dp *DuelPlayer) {
	startDuelCommon(s, dp)
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
	beginDuel(s, dp, swapped)
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
	// 原版 tag_duel.cpp:920-922：每次 MSG_NEW_TURN 清空全部投降标记，
	// 「队友两人投降判负」只在同一回合内累积，跨回合重新计数。
	for i := range s.surrender {
		s.surrender[i] = false
	}
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

func (s *TagDuel) seatCount() int { return 4 }

func (s *TagDuel) replayFlag() uint32 { return REPLAY_UNIFORM | REPLAY_TAG }

func (s *TagDuel) extraDuelOpt() uint32 { return ocgcore.DUEL_TAG_MODE }

// loadDecksToEngine 把四个座位的卡组按原版顺序装入引擎并写入回放：
// 各座位 Main 先整体 Reverse，再按「0 号主队 AddCard、1 号队tag AddTagCard、
// 3 号主队 AddCard、2 号队tag AddTagCard」的固定序列装载
//（tag_duel.cpp TPResult 的 loadSingle/loadTag 调用序列；curPlayer 已在
// TPResult 换位后归位，0 号队即 players[0]/players[1]，1 号队即 players[2]/players[3]）。
func (s *TagDuel) loadDecksToEngine(d *ocgcore.Duel, rp *Replay) {
	loadSingle := func(deckContainer []*CardDataC, p uint8, location uint8) {
		rp.WriteInt32(int32(len(deckContainer)), false)
		for _, v := range deckContainer {
			d.AddCard(v.Code, int(p), location)
			rp.WriteInt32(int32(v.Code), false)
		}
	}
	loadTag := func(deckContainer []*CardDataC, p uint8, location uint8) {
		rp.WriteInt32(int32(len(deckContainer)), false)
		for _, v := range deckContainer {
			d.AddTagCard(v.Code, p, location)
			rp.WriteInt32(int32(v.Code), false)
		}
	}
	slices.Reverse(s.pDeck[0].Main)
	slices.Reverse(s.pDeck[1].Main)
	slices.Reverse(s.pDeck[2].Main)
	slices.Reverse(s.pDeck[3].Main)
	loadSingle(s.pDeck[0].Main, 0, ocgcore.LOCATION_DECK)
	loadSingle(s.pDeck[0].Extra, 0, ocgcore.LOCATION_EXTRA)
	loadTag(s.pDeck[1].Main, 0, ocgcore.LOCATION_DECK)
	loadTag(s.pDeck[1].Extra, 0, ocgcore.LOCATION_EXTRA)
	loadSingle(s.pDeck[3].Main, 1, ocgcore.LOCATION_DECK)
	loadSingle(s.pDeck[3].Extra, 1, ocgcore.LOCATION_EXTRA)
	loadTag(s.pDeck[2].Main, 1, ocgcore.LOCATION_DECK)
	loadTag(s.pDeck[2].Extra, 1, ocgcore.LOCATION_EXTRA)
}

// refreshOnDuelStart 是开局额外卡组刷新（原版 tag RefreshExtra，0x81fff4/无缓存）。
func (s *TagDuel) refreshOnDuelStart() {
	s.RefreshExtra(0, 0x81fff4, 0)
	s.RefreshExtra(1, 0x81fff4, 0)
}

// armDuelTimer 武装决斗秒表：TagTimer 非超时路径每秒自行重新武装
//（与原版 TagTimer 一致，见 TagTimer 方法注释）。
func (s *TagDuel) armDuelTimer() {
	s.ETimer = timerWheel.AfterFunc(time.Second, s.TagTimer)
}

// refreshGraveAfterSwap / refreshAfterSummon / refreshAfterChain /
// refreshAfterDamageStep / refreshAfterNewPhase / refreshSingleMoved /
// refreshSingleFlip / skipCardQuery 原本在本文件以 0x81fff4 等常量实现，
// 差异已收进 DuelMode.refresh 参数表（构造时填写），统一由 DuelMode 基类
// 读表实现（见 duel_refresh.go）。

func (s *TagDuel) onDuelEnded() {
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.players[i].State = 0xff
		}
	}
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
	// 原版 TagTimer 非超时路径每秒 event_add 重新武装定时器
	//（source/ygopro/gframe/tag_duel.cpp:1740-1741）；漏掉这行会让
	// tag 计时器走一秒后停走，超时判负永不触发。
	s.ETimer = timerWheel.AfterFunc(time.Second, s.TagTimer)
}
