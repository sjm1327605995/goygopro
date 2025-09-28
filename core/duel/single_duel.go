package duel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/antlabs/timer"
	"github.com/duke-git/lancet/v2/condition"
	"github.com/ghostiam/binstruct"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
	"math/rand"
	"slices"
	"time"
)

const PRO_VERSION = 0x1361

type SingleDuel struct {
	DuelMode
	players      [2]*DuelPlayer
	pPlayers     [2]*DuelPlayer
	ready        [2]bool
	pDeck        [2]*Deck
	DeckError    [2]uint32
	handResult   [2]uint8
	lastResponse uint8
	Observers    map[string]*DuelPlayer
	//lastReplay Replay
	MatchMode   bool
	matchKill   int
	duelCount   uint8
	tpPlayer    uint8
	matchResult [3]uint8
	timeLimit   [2]int16

	timeElapsed int16
}

func (s *SingleDuel) Chat(dp *DuelPlayer, pData []byte) {
	var buff bytes.Buffer

	sccSize := s.CreateChatPacket(pData, &buff, uint16(dp.Type))
	if sccSize > 0 {
		return
	}
	s.SendPacketDataToPlayer(s.players[0], network.STOC_CHAT, buff.Bytes())
	s.ReSendToPlayer(s.players[1])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}

var timerWheel timer.Timer

func init() {
	timerWheel = timer.NewTimer(timer.WithTimeWheel())
	go timerWheel.Run()
}

func (s *SingleDuel) JoinGame(dp *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) {

	if !isCreator {
		//TODO
		//if dp.Type != 0xff {
		//	var scem = protocol.STOCErrorMsg{Msg: network.ERRMSG_JOINERROR}
		//	s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
		//	_ = s.DisconnetPlayer(dp)
		//	return
		//}
		//
		//if pkt.Version != PRO_VERSION {
		//	var scem = protocol.STOCErrorMsg{
		//		Msg:  network.ERRMSG_VERERROR,
		//		Code: PRO_VERSION,
		//	}
		//	s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
		//	_ = s.DisconnetPlayer(dp)
		//	return
		//}
		var jpass [20]uint16
		utils.NullTerminate(pkt.Pass[:], 0)
		copy(jpass[:], pkt.Pass[:])

		if utils.Wcscmp(jpass[:], pkt.Pass[:]) != 0 {
			var scem = protocol.STOCErrorMsg{
				Msg:  network.ERRMSG_JOINERROR,
				Code: 1,
			}
			s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
			_ = s.DisconnetPlayer(dp)
			return
		}

	}
	s.HostInfo.TimeLimit = 180

	utils.NullTerminate(pkt.Pass[:], 0)
	copy(dp.Game.BaseMode().Pass[:], pkt.Pass[:])
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
	if dp == s.HostPlayer {
		s.EndDuel()
		s.StopServer()
	} else if dp.Type == network.NETPLAYER_TYPE_OBSERVER {
		delete(s.Observers, dp.ID)
		if s.DuelStage == network.DUEL_STAGE_BEGIN {
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
		_ = s.DisconnetPlayer(dp)
	} else {
		if s.DuelStage == network.DUEL_STAGE_BEGIN {
			var scpc protocol.STOCHsPlayerChange
			s.players[dp.Type] = nil
			s.ready[dp.Type] = false
			scpc.Status = dp.Type<<4 | network.PLAYERCHANGE_LEAVE
			if s.players[0] != nil && dp.Type != 0 {
				s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_PLAYER_CHANGE, scpc)
			}
			if s.players[1] != nil && dp.Type != 1 {
				s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_PLAYER_CHANGE, scpc)
			}
			for _, v := range s.Observers {
				s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_CHANGE, scpc)
			}
			_ = s.DisconnetPlayer(dp)
		} else {
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
				wbuf[2] = 0x24
				s.SendPacketDataToPlayer(s.players[0], network.MSG_WIN, wbuf)
				s.ReSendToPlayer(s.players[1])
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
				s.EndDuel()
				s.SendPacketToPlayer(s.players[0], network.STOC_DUEL_END)
				s.ReSendToPlayer(s.players[1])
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			}
			_ = s.DisconnetPlayer(dp)
		}
	}
}

func (s *SingleDuel) ToDuelList(dp *DuelPlayer) {
	if dp.Type != network.NETPLAYER_TYPE_OBSERVER {
		return
	}
	if s.players[0] != nil && s.players[1] != nil {
		return
	}
	delete(s.Observers, dp.ID)
	var scpe protocol.STOCHsPlayerEnter
	copy(scpe.Name[:], dp.Name[:])
	if s.players[0] == nil {
		s.players[0] = dp
		dp.Type = network.NETPLAYER_TYPE_PLAYER1
		scpe.Pos = 0
	} else {
		s.players[1] = dp
		dp.Type = network.NETPLAYER_TYPE_PLAYER2
		scpe.Pos = 1
	}
	var scwc protocol.STOCHsWatchChange
	scwc.WatchCount = uint16(len(s.Observers))
	s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_PLAYER_ENTER, scpe)
	s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_PLAYER_ENTER, scwc)
	if s.players[1] != nil {
		s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_PLAYER_ENTER, scpe)
		s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_PLAYER_ENTER, scwc)
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_ENTER, scpe)
		s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_ENTER, scwc)
	}
	var sctc protocol.STOCTypeChange
	sctc.Type = condition.Ternary[bool, uint8](dp == s.HostPlayer, 0x10, 0) | dp.Type
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, sctc)
}

func (s *SingleDuel) ToObserver(dp *DuelPlayer) {
	if dp.Type > 1 {
		return
	}
	var scpc protocol.STOCHsPlayerChange
	scpc.Status = dp.Type<<4 | network.PLAYERCHANGE_OBSERVE
	if s.players[0] != nil {
		s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_PLAYER_CHANGE, scpc)
	}
	if s.players[1] != nil {
		s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_PLAYER_CHANGE, scpc)
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_CHANGE, scpc)
	}
	s.players[dp.Type] = nil
	s.ready[dp.Type] = false
	dp.Type = network.NETPLAYER_TYPE_OBSERVER
	s.Observers[dp.ID] = dp
	var sctc protocol.STOCTypeChange
	sctc.Type = condition.Ternary[bool, uint8](dp == s.HostPlayer, 0x10, 0) | dp.Type
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, sctc)
}

func (s *SingleDuel) PlayerReady(dp *DuelPlayer, isReady bool) {
	if dp.Type > 1 {
		return
	}
	if s.ready[dp.Type] == isReady {
		return
	}
	if isReady {
		var deckError uint32
		if s.HostInfo.NoCheckDeck == 0 {
			if s.DeckError[dp.Type] != 0 {
				deckError = network.DECKERROR_UNKNOWNCARD<<28 | s.DeckError[dp.Type]
			} else {
				deckError = DeckManger.CheckDeck(s.pDeck[dp.Type], s.HostInfo.LFList, int(s.HostInfo.Rule))
			}
		}
		if deckError != 0 {
			var scpc protocol.STOCHsPlayerChange
			scpc.Status = dp.Type<<4 | network.PLAYERCHANGE_NOTREADY
			s.SendPacketDataToPlayer(s.players[dp.Type], network.STOC_HS_PLAYER_CHANGE, scpc)
			var scem protocol.STOCErrorMsg
			scem.Msg = network.ERRMSG_DECKERROR
			scem.Code = deckError
			s.SendPacketDataToPlayer(s.players[dp.Type], network.STOC_ERROR_MSG, scem)
			return
		}
	}
	s.ready[dp.Type] = isReady
	var scpc protocol.STOCHsPlayerChange
	scpc.Status = dp.Type<<4 | condition.Ternary[bool, byte](isReady, network.PLAYERCHANGE_READY, network.PLAYERCHANGE_NOTREADY)
	s.SendPacketDataToPlayer(s.players[dp.Type], network.STOC_HS_PLAYER_CHANGE, scpc)
	if s.players[1-dp.Type] != nil {
		s.SendPacketDataToPlayer(s.players[1-dp.Type], network.STOC_HS_PLAYER_CHANGE, scpc)
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_CHANGE, scpc)
	}
}

func (s *SingleDuel) PlayerKick(dp *DuelPlayer, pos byte) {
	if pos > 1 || dp != s.HostPlayer || dp == s.players[pos] || s.players[pos] == nil {
		return
	}
	s.LeaveGame(s.players[pos])
}

func (s *SingleDuel) UpdateDeck(dp *DuelPlayer, pData []byte) {
	if dp.Type > 1 || s.ready[dp.Type] {
		return
	}
	var valid = true
	length := len(pData)
	var deckBuf protocol.CTOSDeckData
	err := binstruct.UnmarshalLE(pData, &deckBuf)
	if err != nil {
		fmt.Println(err)
		return
	}
	if deckBuf.MainC < 0 || deckBuf.MainC > protocol.MAINC_MAX {
		valid = false
	} else if deckBuf.SideC < 0 || deckBuf.SideC > protocol.SIDEC_MAX {
		valid = false
	} else if int32(length) < (2+deckBuf.MainC+deckBuf.SideC)*4 {
		valid = false
	}
	if !valid {
		var scem = protocol.STOCErrorMsg{
			Msg:  network.ERRMSG_DECKERROR,
			Code: 0,
		}
		s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
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
	if res > 3 {
		return
	}
	if dp.State != network.CTOS_HAND_RESULT {
		return
	}
	s.handResult[dp.Type] = res
	if s.handResult[0] != 0 && s.handResult[1] != 0 {
		var schr = protocol.STOCHandResult{
			Res1: s.handResult[0],
			Res2: s.handResult[1],
		}
		s.SendPacketDataToPlayer(s.players[0], network.STOC_HAND_RESULT, schr)
		for _, v := range s.Observers {
			s.ReSendToPlayer(v)
		}
		schr.Res1 = s.handResult[1]
		schr.Res2 = s.handResult[0]
		s.SendPacketDataToPlayer(s.players[1], network.STOC_HAND_RESULT, schr)
		if s.handResult[0] == s.handResult[1] {
			s.SendPacketToPlayer(s.players[0], network.STOC_SELECT_HAND)
			s.ReSendToPlayer(s.players[1])
			s.handResult[0], s.handResult[1] = 0, 0
			s.players[0].State = network.CTOS_HAND_RESULT
			s.players[1].State = network.CTOS_HAND_RESULT
		} else if (s.handResult[0] == 1 && s.handResult[1] == 2) ||
			(s.handResult[0] == 2 && s.handResult[1] == 3) ||
			(s.handResult[0] == 3 && s.handResult[1] == 1) {
			s.SendPacketToPlayer(s.players[1], network.STOC_SELECT_TP)
			s.tpPlayer = 1
			s.players[0].State = 0xff
			s.players[1].State = network.CTOS_TP_RESULT
			s.DuelStage = network.DUEL_STAGE_FIRSTGO
		} else {
			s.SendPacketToPlayer(s.players[0], network.STOC_SELECT_TP)
			s.players[1].State = 0xff
			s.players[0].State = network.CTOS_TP_RESULT
			s.tpPlayer = 0
			s.DuelStage = network.DUEL_STAGE_FIRSTGO
		}
	}

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
	//duelSeed := rnd
	//	rh.id = 0x31707279;
	//	rh.version = PRO_VERSION;
	//	rh.flag = REPLAY_UNIFORM;
	//	rh.seed = seed;
	//	rh.start_time = (unsigned int)std::time(nullptr);
	//	last_replay.BeginRecord();
	//	last_replay.WriteHeader(rh);
	//	last_replay.WriteData(players[0]->name, 40, false);
	//	last_replay.WriteData(players[1]->name, 40, false);
	if s.HostInfo.NoShuffleDeck == 0 {
		rnd.Shuffle(len(s.pDeck[0].Main), func(i, j int) {
			s.pDeck[0].Main[i], s.pDeck[0].Main[j] = s.pDeck[0].Main[j], s.pDeck[0].Main[i]
		})
		rnd.Shuffle(len(s.pDeck[1].Main), func(i, j int) {
			s.pDeck[1].Main[i], s.pDeck[1].Main[j] = s.pDeck[1].Main[j], s.pDeck[1].Main[i]
		})
	}
	s.timeLimit[0], s.timeLimit[1] = int16(s.HostInfo.TimeLimit), int16(s.HostInfo.TimeLimit)

	s.Duel = ocgcore.NewDuel(seed)
	//s.Duel.InitPlayers(s.HostInfo.StartLp, int32(s.HostInfo.StartHand), int32(s.HostInfo.DrawCount))
	s.Duel.InitPlayers(8000, 5, 1)

	opt := uint32(s.HostInfo.DuelRule) << 16
	//TODO 暂时不洗牌
	//if s.HostInfo.NoShuffleDeck != 0 {
	//	opt |= ocgcore.DUEL_PSEUDO_SHUFFLE
	//}
	opt |= ocgcore.DUEL_PSEUDO_SHUFFLE
	opt |= ocgcore.DUEL_TAG_MODE
	//last_replay.WriteInt32(host_info.start_lp, false);
	//last_replay.WriteInt32(host_info.start_hand, false);
	//last_replay.WriteInt32(host_info.draw_count, false);
	//last_replay.WriteInt32(opt, false);
	//last_replay.Flush();
	load := func(deckContainer []*CardDataC, p uint8, location uint8) {
		//	last_replay.WriteInt32(deck_container.size(), false);
		for _, v := range deckContainer {
			s.Duel.AddCard(v.Code, int(p), location)

		}
	}
	slices.Reverse(s.pDeck[0].Main)
	slices.Reverse(s.pDeck[1].Main)
	load(s.pDeck[0].Main, 0, ocgcore.LOCATION_DECK)
	load(s.pDeck[0].Extra, 0, ocgcore.LOCATION_EXTRA)
	load(s.pDeck[1].Main, 1, ocgcore.LOCATION_DECK)
	load(s.pDeck[1].Extra, 1, ocgcore.LOCATION_EXTRA)

	//	last_replay.Flush();
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
	fmt.Println("opt", opt)
	opt = 5
	s.Duel.Start(5)
	if s.HostInfo.TimeLimit != 0 {
		s.timeElapsed = 0
		s.ETimer = timerWheel.AfterFunc(time.Second, s.SingleTimer)

	}

	s.Process()
}

func (s *SingleDuel) Process() {
	var (
		buff    = make([]byte, ocgcore.SIZE_MESSAGE_BUFFER)
		engFlag uint32
		engLen  int
		stop    int
	)
	for stop == 0 {
		if engFlag == ocgcore.PROCESSOR_END {
			break
		}
		result := s.Duel.Process()
		engLen = int(result & ocgcore.PROCESSOR_BUFFER_LEN)
		engFlag = result & ocgcore.PROCESSOR_FLAG
		if engLen > 0 {
			if engLen > len(buff) {
				buff = make([]byte, engLen)
			}
			s.Duel.GetMessage(buff)
			stop = s.Analyze(buff[:engLen])
		}
	}
	if stop == 2 {
		s.DuelEndProc()
	}

}

func (s *SingleDuel) DuelEndProc() {
	if !s.MatchMode {
		s.SendPacketToPlayer(s.players[0], network.STOC_DUEL_END)
		s.ReSendToPlayer(s.players[1])
		for _, v := range s.Observers {
			s.ReSendToPlayer(v)
		}
		s.DuelStage = network.DUEL_STAGE_END
	} else {
		winc := make([]byte, 3)
		for i := uint8(0); i < s.duelCount; i++ {
			winc[s.matchResult[i]]++
		}
		if s.matchKill != 0 ||
			winc[0] == 2 || (winc[0] == 1 && winc[2] == 2) ||
			winc[1] == 2 || (winc[1] == 1 && winc[2] == 2) ||
			winc[2] == 3 || (winc[0] == 1 && winc[1] == 1 && winc[2] == 1) {
			s.SendPacketToPlayer(s.players[0], network.STOC_DUEL_END)
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.DuelStage = network.DUEL_STAGE_END
		} else {
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
	}
}

func (s *SingleDuel) Analyze(msgBuffer []byte) int {
	// 创建主缓冲区，用于解析游戏消息数据
	// msgBuffer: 原始的游戏消息字节数据
	// binary.LittleEndian: 使用小端字节序进行数据编解码
	pbuf := utils.NewYGOBuffer(msgBuffer, binary.LittleEndian)

	// 创建辅助缓冲区和偏移量跟踪器
	// pbufw: 用于写入修改后的数据（如隐藏对手手牌信息）
	// offset: 用于跟踪消息起始位置，便于提取完整消息数据
	pbufw, offset := pbuf.Clone(), pbuf.Clone()

	// 循环处理缓冲区中的所有消息，直到缓冲区为空
	for pbuf.Len() > 0 {
		// 记录当前消息的起始位置，用于后续提取完整消息数据
		offset = pbuf.Clone()

		// 读取消息类型（1字节无符号整数）
		var engType uint8
		err := pbuf.Read(&engType)
		if err != nil {
			panic(err)
		}
		fmt.Println("engType", engType)

		// 根据消息类型进行不同的处理逻辑
		switch engType {
		case ocgcore.MSG_RETRY:
			// 重试消息：让上次响应的玩家重新选择
			s.WaitforResponse(s.lastResponse)
			s.SendPacketDataToPlayer(s.players[s.lastResponse], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_HINT:
			// 提示消息：根据提示类型发送给不同的玩家
			var (
				typ    uint8
				player uint8
				data   int32
			)
			// 读取提示类型、玩家编号和提示数据
			pbuf.Read(&typ, &player, &data)
			switch typ {
			case 1, 2, 3, 5:
				// 类型1,2,3,5：只发送给指定玩家
				s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			case 4, 6, 7, 8, 9, 11:
				// 类型4,6,7,8,9,11：发送给对手玩家和所有观察者
				s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			case 10:
				// 类型10：发送给所有玩家和观察者
				s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				s.SendPacketDataToPlayer(s.players[1], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			}
		case ocgcore.MSG_WIN:
			// 胜利消息：处理游戏结束逻辑
			var (
				player uint8
				typ    uint8
			)
			// 读取胜利玩家和胜利类型
			_ = pbuf.Read(&player, &typ)

			// 发送胜利消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 处理比赛结果和回合玩家切换
			if player > 1 {
				s.matchResult[s.duelCount] = 2
				s.duelCount++
				s.tpPlayer = 1 - player
			} else if s.players[player] == s.pPlayers[player] {
				s.matchResult[s.duelCount] = player
				s.duelCount++
				s.tpPlayer = 1 - player
			} else {
				s.matchResult[s.duelCount] = 1 - player
				s.duelCount++
				s.tpPlayer = player
			}
			s.EndDuel()
			return 2
		case ocgcore.MSG_SELECT_BATTLECMD:
			// 战斗命令选择消息：处理战斗阶段的选择
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和可攻击怪物数量
			_ = pbuf.Read(&player, &count)

			// 跳过攻击怪物数据（每个怪物11字节）
			pbuf.Next(int(count) * 11)

			// 读取可发动效果数量
			_ = pbuf.Read(&count)

			// 跳过效果数据（每个效果8字节 + 2字节额外数据）
			pbuf.Next(int(count)*8 + 2)

			// 刷新所有区域防御状态
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_IDLECMD:
			// 空闲阶段命令选择消息：处理主要阶段的选择
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和第一个选项数量
			_ = pbuf.Read(&player, &count)

			// 跳过6组选项数据，每组包含不同数量的数据
			pbuf.Next(int(count * 7))    // 跳过第一组选项（每个选项7字节）
			_ = pbuf.Read(&count)        // 读取第二组选项数量
			pbuf.Next(int(count * 7))    // 跳过第二组选项
			_ = pbuf.Read(&count)        // 读取第三组选项数量
			pbuf.Next(int(count * 7))    // 跳过第三组选项
			_ = pbuf.Read(&count)        // 读取第四组选项数量
			pbuf.Next(int(count * 7))    // 跳过第四组选项
			_ = pbuf.Read(&count)        // 读取第五组选项数量
			pbuf.Next(int(count * 7))    // 跳过第五组选项
			_ = pbuf.Read(&count)        // 读取第六组选项数量
			pbuf.Next(int(count*11 + 3)) // 跳过第六组选项（每个选项11字节 + 3字节额外数据）

			// 刷新所有区域防御状态
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_EFFECTYN:
			// 效果发动确认消息：询问玩家是否发动效果
			var (
				player uint8
			)
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 跳过效果相关数据（12字节）
			pbuf.Next(12)

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_YESNO:
			// 是/否选择消息：询问玩家简单的是或否问题
			var (
				player uint8
			)
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 跳过问题相关数据（4字节）
			pbuf.Next(4)

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_OPTION:
			// 选项选择消息：让玩家从多个选项中选择
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和选项数量
			_ = pbuf.Read(&player, &count)

			// 跳过选项数据（每个选项4字节）
			pbuf.Next(int(count * 4))

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_CARD, ocgcore.MSG_SELECT_TRIBUTE:
			// 卡片选择/祭品选择消息：处理卡片选择逻辑
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 跳过3字节的额外数据
			pbuf.Next(3)

			// 读取可选卡片数量
			_ = pbuf.Read(&count)

			var c uint8
			// 遍历所有可选卡片
			for i := uint8(0); i < count; i++ {
				// 创建写入缓冲区副本，用于修改卡片数据
				pbufw = pbuf.Clone()
				var (
					code int32
					l    uint8
					s    uint8
					ss   uint8
				)
				// 读取卡片信息：卡片代码、位置、状态等
				_ = pbuf.Read(&code, &l, &s, &ss)

				// 如果不是当前玩家，隐藏卡片代码（设置为0）
				if c != player {
					pbufw.Write(int32(0))
				}
			}

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_UNSELECT_CARD:
			// 可选择/不可选择卡片消息：处理复杂的卡片选择逻辑
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 跳过4字节的额外数据
			pbuf.Next(4)

			// 读取可选择卡片数量
			_ = pbuf.Read(&count)

			var (
				code int32
				c    uint8
				l    uint8
				s1   uint8
				ss   uint8
			)

			// 处理可选择卡片列表
			for i := uint8(0); i < count; i++ {
				// 创建写入缓冲区副本
				pbufw = pbuf.Clone()
				// 读取卡片信息
				_ = pbuf.Read(&code, &c, &l, &s1, &ss)
				// 如果不是当前玩家，隐藏卡片代码
				if c != player {
					pbufw.Write(int32(0))
				}
			}

			// 读取不可选择卡片数量
			_ = pbuf.Read(&count)

			// 处理不可选择卡片列表
			for i := uint8(0); i < count; i++ {
				// 创建写入缓冲区副本
				pbufw = pbuf.Clone()
				// 读取卡片信息
				_ = pbuf.Read(&code, &c, &l, &s1, &ss)
				// 如果不是当前玩家，隐藏卡片代码
				if c != player {
					pbufw.Write(int32(0))
				}
			}

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_CHAIN:
			// 连锁选择消息：处理连锁发动选择
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和连锁数量
			_ = pbuf.Read(&player, &count)

			// 跳过连锁数据（9字节固定数据 + 每个连锁14字节）
			pbuf.Next(int(9 + count*14))

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_PLACE, ocgcore.MSG_SELECT_DISFIELD:
			// 放置位置选择/场地选择消息
			var player uint8
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 跳过位置相关数据（5字节）
			pbuf.Next(5)

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_POSITION:
			// 表示形式选择消息：选择怪物的表示形式
			var player uint8
			// 读取玩家编号
			pbuf.Read(&player)

			// 跳过位置数据（5字节）
			pbuf.Next(5)

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_COUNTER:
			// 计数器选择消息：处理计数器移除选择
			var player uint8
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 跳过4字节的额外数据
			pbuf.Next(4)

			var count uint8
			// 读取可移除计数器数量
			_ = pbuf.Read(&count)

			// 跳过计数器数据（每个计数器9字节）
			pbuf.Next(int(count * 9))

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_SUM:
			// 数值合计选择消息：处理等级/阶级合计选择
			// 跳过1字节的额外数据
			pbuf.Next(1)

			var player uint8
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 跳过6字节的额外数据
			pbuf.Next(6)

			var count uint8
			// 读取第一组卡片数量
			_ = pbuf.Read(&count)

			// 跳过第一组卡片数据（每个卡片11字节）
			pbuf.Next(int(count * 11))

			// 读取第二组卡片数量
			_ = pbuf.Read(&count)

			// 跳过第二组卡片数据（每个卡片11字节）
			pbuf.Next(int(count * 11))

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SORT_CARD:
			// 卡片排序消息：处理手牌排序
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和卡片数量
			pbuf.Read(&player, &count)

			// 跳过卡片数据（每个卡片7字节）
			pbuf.Next(int(count) * 7)

			// 等待玩家响应并发送选择消息
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_CONFIRM_DECKTOP:
			// 卡组顶部确认消息：显示卡组顶部的卡片
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和卡片数量
			_ = pbuf.Read(&player, &count)

			// 跳过卡片数据（每个卡片7字节）
			pbuf.Next(int(count * 7))

			// 发送确认消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CONFIRM_EXTRATOP:
			// 额外卡组顶部确认消息：显示额外卡组顶部的卡片
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和卡片数量
			_ = pbuf.Read(&player, &count)

			// 跳过卡片数据（每个卡片7字节）
			pbuf.Next(int(count * 7))

			// 发送确认消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CONFIRM_CARDS:
			// 卡片确认消息：确认特定位置的卡片
			var (
				player uint8
				n      uint8
				count  uint8
			)
			// 读取玩家编号、未知参数和卡片数量
			_ = pbuf.Read(&player, &n, &count)

			// 检查卡片位置，如果不是卡组则发送给所有玩家
			if pbuf.At(5) != ocgcore.LOCATION_DECK {
				// 跳过卡片数据（每个卡片7字节）
				pbuf.Next(int(count) * 7)

				// 发送给指定玩家、对手和所有观察者
				s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				s.ReSendToPlayer(s.players[1-player])
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			} else {
				// 如果是卡组位置，只发送给指定玩家
				pbuf.Next(int(count * 7))
				s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			}
		case ocgcore.MSG_SHUFFLE_DECK:
			// 卡组洗牌消息：洗牌并发送给所有玩家
			var (
				player uint8
			)
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 发送洗牌消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SHUFFLE_HAND:
			// 手牌洗牌消息：洗牌并分别发送给玩家和对手
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和卡片数量
			_ = pbuf.Read(&player, &count)

			// 发送给指定玩家（跳过count*4字节的卡片数据）
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlicesOffset(pbuf, int(count)*4))

			// 为对手写入0值隐藏卡片信息
			for i := uint8(0); i < count; i++ {
				pbuf.Write(int32(0))
			}

			// 发送给对手玩家（包含隐藏的卡片信息）
			s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshHand(int(player), 0x781fff, 0)
		case ocgcore.MSG_SHUFFLE_EXTRA:
			// 额外卡组洗牌消息：洗牌并分别发送给玩家和对手
			var (
				player uint8
				count  uint8
			)
			// 读取玩家编号和卡片数量
			_ = pbuf.Read(&player, &count)

			// 发送给指定玩家（跳过count*4字节的卡片数据）
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlicesOffset(pbuf, int(count)*4))

			// 为对手写入0值隐藏卡片信息
			for i := uint8(0); i < count; i++ {
				pbuf.Write(int32(0))
			}

			// 发送给对手玩家（包含隐藏的卡片信息）
			s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_REFRESH_DECK:
			// 卡组刷新消息：刷新卡组显示
			// 跳过1字节的额外数据
			pbuf.Next(1)

			// 发送刷新消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SWAP_GRAVE_DECK:
			// 墓地卡组交换消息：交换墓地和卡组
			var (
				player uint8
			)
			// 读取玩家编号
			_ = pbuf.Read(&player)

			// 发送交换消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 刷新墓地区域显示
			s.RefreshGraveDef(int(player))
		case ocgcore.MSG_REVERSE_DECK:
			// 卡组反转消息：反转卡组顺序
			// 发送反转消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DECK_TOP:
			// 卡组顶部消息：显示卡组顶部的卡片
			// 跳过6字节的卡片位置数据
			pbuf.Next(6)

			// 发送卡组顶部消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SHUFFLE_SET_CARD:
			// 场上卡片洗牌消息：洗牌场上设置的卡片
			var (
				loc   uint8
				count uint8
			)
			// 读取位置类型和卡片数量
			_ = pbuf.Read(&loc, &count)

			// 跳过卡片数据（每个卡片8字节）
			pbuf.Next(int(count) * 8)

			// 发送洗牌消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 根据位置类型刷新相应的区域
			if ocgcore.LOCATION_MZONE == loc {
				// 刷新怪兽区域
				s.RefreshMzone(0, 0x181fff, 0)
				s.RefreshMzone(1, 0x181fff, 0)
			} else {
				// 刷新魔法陷阱区域
				s.RefreshSzone(0, 0x181fff, 0)
				s.RefreshSzone(1, 0x181fff, 0)
			}
		case ocgcore.MSG_NEW_TURN:
			// 新回合开始消息：处理回合开始逻辑
			// 刷新所有区域显示
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)

			// 跳过1字节的回合编号数据
			pbuf.Next(1)

			// 重置时间限制
			s.timeLimit[0] = int16(s.HostInfo.TimeLimit)
			s.timeLimit[1] = int16(s.HostInfo.TimeLimit)

			// 发送新回合消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_NEW_PHASE:
			// 新阶段开始消息：处理阶段开始逻辑
			// 跳过2字节的阶段编号数据
			pbuf.Next(2)

			// 发送新阶段消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 刷新所有区域显示
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)
		case ocgcore.MSG_MOVE:
			// 卡片移动消息：处理卡片位置移动
			// 克隆缓冲区用于修改数据
			pbufw = pbuf.Clone()

			// 读取卡片移动相关参数
			var (
				pc = pbuf.At(4)  // 前一个控制者
				pl = pbuf.At(5)  // 前一个位置
				cc = pbuf.At(8)  // 当前控制者
				cl = pbuf.At(9)  // 当前位置
				cs = pbuf.At(10) // 当前序列号
				cp = pbuf.At(11) // 当前位置表示
			)

			// 跳过16字节的移动数据
			pbuf.Next(16)

			// 发送给当前控制者玩家
			s.SendPacketDataToPlayer(s.players[cc], network.STOC_GAME_MSG, offset.SubSlices(pbuf))

			// 如果卡片移动到隐藏位置（卡组、手牌或里侧表示），为对手隐藏卡片信息
			if (cl&(ocgcore.LOCATION_GRAVE+ocgcore.LOCATION_OVERLAY)) == 0 &&
				((cl&(ocgcore.LOCATION_DECK+ocgcore.LOCATION_HAND)) != 0 || cp&ocgcore.POS_FACEDOWN != 0) {
				pbufw.Write(int32(0))
			}

			// 发送给对手玩家
			s.SendPacketDataToPlayer(s.players[1-cc], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 如果卡片位置发生变化且不是叠放，刷新单个卡片显示
			if cl != 0 && (cl&ocgcore.LOCATION_OVERLAY) == 0 && (cl != pl || pc != cc) {
				s.RefreshSingleDef(cc, cl, cs)
			}
		case ocgcore.MSG_POS_CHANGE:
			// 位置表示变更消息：处理卡片表示形式变更
			// 读取位置变更相关参数
			var (
				cc = pbuf.At(4) // 控制者
				cl = pbuf.At(5) // 位置
				cs = pbuf.At(6) // 序列号
				pp = pbuf.At(7) // 前一个位置表示
				cp = pbuf.At(8) // 当前位置表示
			)

			// 跳过9字节的位置变更数据
			pbuf.Next(9)

			// 发送位置变更消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 如果从里侧表示变为表侧表示，刷新单个卡片显示
			if (pp&ocgcore.POS_FACEDOWN != 0) && (cp&ocgcore.POS_FACEUP != 0) {
				s.RefreshSingleDef(cc, cl, cs)
			}
		case ocgcore.MSG_SET:
			// 卡片设置消息：处理卡片设置到场上
			// 写入0值隐藏卡片信息
			pbuf.Write(int32(0))

			// 跳过4字节的额外数据
			pbuf.Next(4)

			// 发送设置消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SWAP:
			// 卡片交换消息：处理两个卡片位置交换
			// 读取交换卡片的相关参数
			var (
				c1 = pbuf.At(4)  // 第一个卡片的控制者
				l1 = pbuf.At(5)  // 第一个卡片的位置
				s1 = pbuf.At(6)  // 第一个卡片的序列号
				c2 = pbuf.At(12) // 第二个卡片的控制者
				l2 = pbuf.At(13) // 第二个卡片的位置
				s2 = pbuf.At(14) // 第二个卡片的序列号
			)

			// 跳过16字节的交换数据
			pbuf.Next(16)

			// 发送交换消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 刷新两个交换卡片的显示
			s.RefreshSingleDef(c1, l1, s1)
			s.RefreshSingleDef(c2, l2, s2)
		case ocgcore.MSG_FIELD_DISABLED:
			// 场地禁用消息：处理场地效果禁用
			// 跳过4字节的禁用数据
			pbuf.Next(4)

			// 发送场地禁用消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SUMMONING:
			// 通常召唤中消息：处理通常召唤过程
			// 跳过8字节的召唤数据
			pbuf.Next(8)

			// 发送召唤中消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SUMMONED:
			// 通常召唤完成消息：处理召唤完成逻辑
			// 发送召唤完成消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 刷新所有区域显示
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
		case ocgcore.MSG_SPSUMMONING:
			// 特殊召唤中消息：处理特殊召唤过程
			// 跳过8字节的特殊召唤数据
			pbuf.Next(8)

			// 发送特殊召唤中消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SPSUMMONED:
			// 特殊召唤完成消息：处理特殊召唤完成逻辑
			// 发送特殊召唤完成消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 刷新所有区域显示
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
		case ocgcore.MSG_FLIPSUMMONING:
			// 反转召唤中消息：处理反转召唤过程
			// 刷新单个卡片显示（控制者、位置、序列号）
			s.RefreshSingleDef(pbuf.At(4), pbufw.At(5), pbuf.At(6))

			// 跳过8字节的反转召唤数据
			pbuf.Next(8)

			// 发送反转召唤中消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_FLIPSUMMONED:
			// 反转召唤完成消息：处理反转召唤完成逻辑
			// 发送反转召唤完成消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 刷新所有区域显示
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
		case ocgcore.MSG_CHAINING:
			// 连锁发动中消息：处理连锁发动过程
			// 跳过16字节的连锁数据
			pbuf.Next(16)

			// 发送连锁发动中消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAINED:
			// 连锁发动完成消息：处理连锁发动完成逻辑
			// 跳过1字节的连锁编号数据
			pbuf.Next(1)

			// 发送连锁发动完成消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 刷新所有区域显示
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)
		case ocgcore.MSG_CHAIN_SOLVING:
			// 连锁处理中消息：处理连锁效果解决过程
			// 跳过1字节的连锁编号数据
			pbuf.Next(1)

			// 发送连锁处理中消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAIN_SOLVED:
			// 连锁处理完成消息：处理连锁效果解决完成逻辑
			// 跳过1字节的连锁编号数据
			pbuf.Next(1)

			// 发送连锁处理完成消息给所有玩家和观察者
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}

			// 刷新所有区域显示
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)
		case ocgcore.MSG_CHAIN_END:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)
		case ocgcore.MSG_CHAIN_NEGATED:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAIN_DISABLED:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CARD_SELECTED:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 4)
		case ocgcore.MSG_RANDOM_SELECTED:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 4)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_BECOME_TARGET:
			var (
				count uint8
			)
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 4)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DRAW:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbufw = pbuf.Clone()
			pbuf.Next(int(count) * 4)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			for i := uint8(0); i < count; i++ {
				if pbufw.At(3)&0x80 == 0 {
					pbufw.Write(int32(0))
				} else {
					pbufw.Next(4)
				}
			}
			s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DAMAGE:
			pbuf.Next(5)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_RECOVER:
			pbuf.Next(5)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_EQUIP:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_LPUPDATE:
			pbuf.Next(5)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_UNEQUIP:
			pbuf.Next(4)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CARD_TARGET:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CANCEL_TARGET:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_PAY_LPCOST:
			pbuf.Next(5)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ADD_COUNTER:
			pbuf.Next(7)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_REMOVE_COUNTER:
			pbuf.Next(7)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ATTACK:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_BATTLE:
			pbuf.Next(26)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ATTACK_DISABLED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DAMAGE_STEP_START:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
		case ocgcore.MSG_DAMAGE_STEP_END:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
		case ocgcore.MSG_MISSED_EFFECT:
			var (
				player = pbuf.At(0)
			)
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
		case ocgcore.MSG_TOSS_COIN:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count))
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_TOSS_DICE:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count))
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ROCK_PAPER_SCISSORS:
			var (
				player uint8
			)
			_ = pbuf.Read(&player)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_HAND_RES:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ANNOUNCE_RACE:
			var (
				player uint8
			)
			_ = pbuf.Read(&player)
			pbuf.Next(5)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_ANNOUNCE_ATTRIB:
			var (
				player uint8
			)
			_ = pbuf.Read(&player)
			pbuf.Next(5)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_ANNOUNCE_CARD, ocgcore.MSG_ANNOUNCE_NUMBER:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 4)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_CARD_HINT:
			pbuf.Next(9)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_PLAYER_HINT:
			pbuf.Next(6)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_MATCH_KILL:
			var (
				code int32
			)
			_ = pbuf.Read(&code)
			if s.MatchMode {
				s.matchKill = int(code)
				s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				s.ReSendToPlayer(s.players[1])
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			}
		}

	}
	return 0
}
func (s *SingleDuel) WaitforResponse(player byte) {
	s.lastResponse = player
	msg := ocgcore.MSG_WAITING
	s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, msg)
	if s.HostInfo.TimeLimit != 0 {
		var sctl protocol.STOCTimeLimit
		sctl.Player = player
		sctl.LeftTime = uint16(s.timeLimit[player])
		s.SendPacketDataToPlayer(s.players[0], network.STOC_TIME_LIMIT, sctl)
		s.SendPacketDataToPlayer(s.players[1], network.STOC_TIME_LIMIT, sctl)
		s.players[player].State = network.CTOS_TIME_CONFIRM
	} else {
		s.players[player].State = network.CTOS_RESPONSE
	}
}
func (s *SingleDuel) TimeConfirm(dp *DuelPlayer) {
	if s.HostInfo.TimeLimit == 0 {
		return
	}
	if dp.Type != s.lastResponse {
		return
	}
	s.players[s.lastResponse].State = network.CTOS_RESPONSE
	if s.timeElapsed < 10 {
		s.timeElapsed = 0
	}
}
func (s *SingleDuel) GetResponse(dp *DuelPlayer, msgBuffer []byte) {
	length := len(msgBuffer)
	if length > ocgcore.SIZE_RETURN_VALUE {
		length = ocgcore.SIZE_RETURN_VALUE
	}
	var resb = make([]byte, ocgcore.SIZE_RETURN_VALUE)
	copy(resb, msgBuffer[:length])
	//	last_replay.Write<uint8_t>(len);
	//	last_replay.WriteData(resb, len);
	s.Duel.SetResponseBytes(resb)
	s.players[dp.Type].State = 0xff
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
	if s.Duel == nil {
		return
	}
	//	last_replay.EndRecord();
	//	char replaybuf[0x2000], *pbuf = replaybuf;
	//	std::memcpy(pbuf, &last_replay.pheader, sizeof(ReplayHeader));
	//	pbuf += sizeof(ReplayHeader);
	//	std::memcpy(pbuf, last_replay.comp_data, last_replay.comp_size);
	//	NetServer::SendBufferToPlayer(players[0], STOC_REPLAY, replaybuf, sizeof(ReplayHeader) + last_replay.comp_size);
	//s.ReSendToPlayer(s.players[1]);
	//for _, v := range s.Observers {
	//	s.ReSendToPlayer(v)
	//}
	s.Duel.End()
	if s.ETimer != nil {
		s.ETimer.Stop()
	}
	s.ETimer.Stop()
	s.ETimer = nil
	s.Duel = nil
}

func (s *SingleDuel) Surrender(dp *DuelPlayer) {
	if dp.Type > 1 || s.Duel == nil {
		return
	}
	var wbuf = make([]byte, 3)
	player := dp.Type
	wbuf[0] = ocgcore.MSG_WIN
	wbuf[1] = 1 - player
	wbuf[2] = 0
	s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, wbuf)
	s.ReSendToPlayer(s.players[1])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
	if s.players[player] == s.pPlayers[player] {
		s.matchResult[s.duelCount] = 1 - player
		s.duelCount++
		s.tpPlayer = player
	} else {
		s.matchResult[s.duelCount] = player
		s.duelCount++
		s.tpPlayer = 1 - player
	}
	s.EndDuel()
	s.DuelEndProc()
	if s.ETimer != nil {
		s.ETimer.Stop()
	}
}
func (s *SingleDuel) SingleTimer() {
	s.timeElapsed++
	if s.timeElapsed >= s.timeLimit[s.lastResponse] || s.timeLimit[s.lastResponse] <= 0 {
		wbuf := make([]byte, 3)
		player := s.lastResponse
		wbuf[0] = ocgcore.MSG_WIN
		wbuf[1] = 1 - player
		wbuf[2] = 0x3
		s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, wbuf)
		s.ReSendToPlayer(s.players[1])
		for _, v := range s.Observers {
			s.ReSendToPlayer(v)
		}
		if s.players[player] == s.pPlayers[player] {
			s.matchResult[s.duelCount] = 1 - player
			s.duelCount++
			s.tpPlayer = player
		} else {
			s.matchResult[s.duelCount] = player
			s.duelCount++
			s.tpPlayer = 1 - player
		}
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
	s.RefreshExtra(player, 0xe81fff, 1)
}
func (s *SingleDuel) RefreshExtra(player int, flag uint32, useCache int) {
	buff := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	length := s.writeUpdateData(player, int(ocgcore.LOCATION_EXTRA), flag, buff, useCache)
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, buff[:length+3])
}
func (s *SingleDuel) RefreshMzoneDef(player int) {
	s.RefreshMzone(player, 0x881fff, 1)

}
func (s *SingleDuel) RefreshMzone(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_MZONE), flag, qbuf.Bytes(), useCache))
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, queryBuffer[:length+3])

	var (
		qLen int32
	)
	qbuf.Next(3)
	for qLen < length {
		var clen int32
		qbuf.Read(&clen)
		qLen += clen
		if clen < ocgcore.LEN_HEADER {
			continue
		}
		data := qbuf.Bytes()
		position := network.GetPosition(data, 8)
		if position&ocgcore.POS_FACEDOWN != 0 {
			copy(data[:clen-4], make([]byte, clen-4))
		}
		qbuf.Next(int(clen) - 4)
	}
	s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, queryBuffer[:length+3])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}

}
func (s *SingleDuel) writeUpdateData(player int, location int, flag uint32, qbuf []byte, use_cache int) int {
	flag |= ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION
	qbuf[0] = ocgcore.MSG_UPDATE_DATA
	qbuf[1] = byte(player)
	qbuf[2] = byte(location)

	n := int(s.Duel.QueryFieldCard(uint8(player), uint8(location), flag, qbuf[3:], use_cache != 0))
	return n
}
func (s *SingleDuel) RefreshSzoneDef(player int) {
	s.RefreshSzone(player, 0x681fff, 1)
}
func (s *SingleDuel) RefreshSzone(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_SZONE), flag, qbuf.Bytes(), useCache))
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, queryBuffer[:length+3])

	var (
		qLen int32
	)
	qbuf.Next(3)
	for qLen < length {
		var clen int32
		qbuf.Read(&clen)
		qLen += clen
		if clen < ocgcore.LEN_HEADER {
			continue
		}
		data := qbuf.Bytes()
		position := network.GetPosition(data, 8)
		if position&ocgcore.POS_FACEDOWN != 0 {
			copy(data[:clen-4], make([]byte, clen-4))
		}
		qbuf.Next(int(clen) - 4)
	}
	s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, queryBuffer[:length+3])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}
func (s *SingleDuel) RefreshHandDef(player int) {
	s.RefreshHand(player, 0x681fff, 1)
}
func (s *SingleDuel) RefreshHand(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_HAND), flag, qbuf.Bytes(), useCache))
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, queryBuffer[:length+3])
	qbuf.Next(3)
	var (
		qLen int32
	)

	for qLen < length {
		var slen int32
		qbuf.Read(&slen)
		qLen += slen
		if slen < ocgcore.LEN_HEADER {
			continue
		}
		data := qbuf.Bytes()
		position := network.GetPosition(qbuf.Bytes(), 8)
		if position&ocgcore.POS_FACEUP == 0 {
			copy(data[:slen-4], make([]byte, slen-4))
		}
		qbuf.Next(int(slen) - 4)
	}
	s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, queryBuffer[:length+3])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}
func (s *SingleDuel) RefreshSingleDef(player uint8, location uint8, sequence uint8) {
	s.RefreshSingle(player, location, sequence, 0xf81fff)
}
func (s *SingleDuel) RefreshGraveDef(player int) {
	s.RefreshGrave(player, 0x81fff, 1)
}
func (s *SingleDuel) RefreshGrave(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_MZONE), flag, qbuf.Bytes(), useCache))
	s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, queryBuffer[:length+3])
	s.ReSendToPlayer(s.players[1])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}
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
