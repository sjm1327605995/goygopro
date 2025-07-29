package duel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/duke-git/lancet/v2/condition"
	"github.com/sjm1327605995/goygopro/core/duel/room"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
	"io"
	"math/rand"
	"time"
	"unicode/utf16"
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

func (s *SingleDuel) JoinGame(dp *DuelPlayer, pData io.Reader, isCreator bool) {
	var pkt protocol.CTOSJoinGame
	err := binary.Read(pData, binary.LittleEndian, &pkt)
	if err != nil {
		panic(err)
		return
	}
	roomId := string(utf16.Decode(pkt.Pass[:]))
	_, isCreator = room.DefaultManager.JoinRoom(roomId, dp)

	if !isCreator {
		if dp.Type != 0xff {
			var scem = protocol.STOCErrorMsg{Msg: network.ERRMSG_JOINERROR}
			s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
			_ = s.DisconnetPlayer(dp)
			return
		}

		if pkt.Version != PRO_VERSION {
			var scem = protocol.STOCErrorMsg{
				Msg:  network.ERRMSG_VERERROR,
				Code: PRO_VERSION,
			}
			s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
			_ = s.DisconnetPlayer(dp)
			return
		}
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
		s.SendPacketDataToPlayer(s.players[0], network.STOC_HS_PLAYER_ENTER, scpe)
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
		s.SendPacketDataToPlayer(s.players[1], network.STOC_HS_PLAYER_ENTER, scpe)
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

	_, err := binary.Decode(pData, binary.LittleEndian, &deckBuf.CTOSDeckDataBase)
	if err != nil {
		fmt.Println(err)
		return
	}
	reader := bytes.NewReader(pData[8:])
	deckBuf.List = make([]uint32, deckBuf.MainC+deckBuf.SideC)
	err = binary.Read(reader, binary.LittleEndian, deckBuf.List)
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
	pBuf := bytes.NewBuffer(deckBuff[:0])
	utils.BatchWrite(pBuf, binary.LittleEndian,
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
	//	set_script_reader(DataManager::ScriptReaderEx);
	//	set_card_reader(DataManager::CardReader);
	//	set_message_handler(SingleDuel::MessageHandler);
	s.Duel = ocgcore.NewDuel(seed)
	s.Duel.InitPlayers(s.HostInfo.StartLp, int32(s.HostInfo.StartHand), int32(s.HostInfo.DrawCount))
	opt := uint32(s.HostInfo.DuelRule) << 16
	if s.HostInfo.NoShuffleDeck != 0 {
		opt |= ocgcore.DUEL_PSEUDO_SHUFFLE
	}
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
	load(s.pDeck[0].Main, 0, ocgcore.LOCATION_DECK)
	load(s.pDeck[0].Extra, 0, ocgcore.LOCATION_EXTRA)
	load(s.pDeck[1].Main, 1, ocgcore.LOCATION_DECK)
	load(s.pDeck[1].Extra, 1, ocgcore.LOCATION_EXTRA)
	//	last_replay.Flush();
	startBuf := make([]byte, 32)
	pBuf := bytes.NewBuffer(startBuf[:0])
	utils.BatchWrite(pBuf, binary.LittleEndian,
		int8(ocgcore.MSG_START), int8(0), int32(s.HostInfo.DuelRule),
		s.HostInfo.StartLp, s.HostInfo.StartLp,
		int16(s.Duel.QueryFieldCount(0, ocgcore.LOCATION_DECK)),
		int16(s.Duel.QueryFieldCount(0, ocgcore.LOCATION_EXTRA)),
		int16(s.Duel.QueryFieldCount(1, ocgcore.LOCATION_DECK)),
		int16(s.Duel.QueryFieldCount(1, ocgcore.LOCATION_EXTRA)),
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
		s.ETimer.Reset(time.Second)
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
			s.Duel.GetMessage(buff[:engLen])
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
	pbufw, pbuf := 0, 0

	for pbuf < len(msgBuffer) {
		var engType uint8
		//offset = pbuf
		_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &engType)
		switch engType {
		case ocgcore.MSG_RETRY:
			s.WaitforResponse(s.lastResponse)
			s.SendPacketDataToPlayer(s.players[s.lastResponse], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_HINT:
			var (
				typ    uint8
				player uint8
				data   int32
			)
			utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &typ, &player, &data)
			switch typ {
			case 1, 2, 3, 5:
				s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			case 4, 6, 7, 8, 9, 11:
				s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			case 10:
				s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
				s.SendPacketDataToPlayer(s.players[1], network.STOC_GAME_MSG, msgBuffer[:pbuf])
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			}
		case ocgcore.MSG_WIN:
			var (
				player uint8
				typ    uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &typ)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			if player > 1 {
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
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count * 11)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count * +2)
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_IDLECMD:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count * 7)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count * 7)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count * 7)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count * 7)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count * 7)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count*11 + 3)
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_EFFECTYN:
			var (
				player uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 12
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_YESNO:
			var (
				player uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 4
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_OPTION:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count * 4)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_CARD, ocgcore.MSG_SELECT_TRIBUTE:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 3
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			var c uint8
			for i := uint8(0); i < count; i++ {
				pbufw = pbuf
				var (
					code int32
					l    uint8
					s    uint8
					ss   uint8
				)
				_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &code, &l, &s, &ss)
				if c != player {
					binary.Encode(msgBuffer[pbufw:], binary.LittleEndian, int32(0))
				}
			}
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_UNSELECT_CARD:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 4
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			var c uint8
			for i := uint8(0); i < count; i++ {
				pbufw = pbuf
				var (
					code int32
					l    uint8
					s    uint8
					ss   uint8
				)
				_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &code, &l, &s, &ss)
				if c != player {
					binary.Encode(msgBuffer[pbufw:], binary.LittleEndian, int32(0))
				}
			}
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			for i := uint8(0); i < count; i++ {
				pbufw = pbuf
				var (
					code int32
					l    uint8
					s    uint8
					ss   uint8
				)
				_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &code, &l, &s, &ss)
				if c != player {
					binary.Encode(msgBuffer[pbufw:], binary.LittleEndian, int32(0))
				}
			}
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_CHAIN:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(10 + count*13)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[pbuf])
			return 1
		case ocgcore.MSG_SELECT_PLACE, ocgcore.MSG_SELECT_DISFIELD:
			var player uint8
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 5
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_COUNTER:
			var player uint8
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 4
			var count uint8
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count * 9)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_SELECT_SUM:
			pbuf += 1
			var player uint8
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 6
			var count uint8
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count * 11)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count * 11)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_CONFIRM_DECKTOP:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count * 7)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CONFIRM_EXTRATOP:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count * 7)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CONFIRM_CARDS:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			if msgBuffer[pbuf+5] != ocgcore.LOCATION_DECK {
				pbuf += int(count) * 7
				s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
				s.ReSendToPlayer(s.players[1-player])
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			} else {
				pbuf += int(count * 7)
				s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			}
		case ocgcore.MSG_SHUFFLE_DECK:
			var (
				player uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SHUFFLE_HAND:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			for i := uint8(0); i < count; i++ {
				//TODO
			}
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf+int(count*4)])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshHand(int(player), 0x781fff, 0)
		case ocgcore.MSG_SHUFFLE_EXTRA:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf+int(count*4)])

			for i := uint8(0); i < count; i++ {
				_ = utils.BatchEncode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, int32(0))
			}
			s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_REFRESH_DECK:
			pbuf++
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SWAP_GRAVE_DECK:
			var (
				player uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_REVERSE_DECK:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DECK_TOP:
			pbuf += 6
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SHUFFLE_SET_CARD:
			var (
				loc   uint8
				count uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &loc, &count)
			pbuf += int(count) * 8
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			if ocgcore.LOCATION_MZONE == loc {
				s.RefreshMzone(0, 0x181fff, 0)
				s.RefreshMzone(1, 0x181fff, 0)
			} else {
				s.RefreshSzone(0, 0x181fff, 0)
				s.RefreshSzone(1, 0x181fff, 0)
			}
		case ocgcore.MSG_NEW_TURN:
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
			s.RefreshHandDef(0)
			s.RefreshHandDef(1)
			pbuf++
			s.timeLimit[0] = int16(s.HostInfo.TimeLimit)
			s.timeLimit[1] = int16(s.HostInfo.TimeLimit)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_NEW_PHASE:
			pbuf += 2
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
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
		case ocgcore.MSG_MOVE:
			pbufw = pbuf
			var (
				pc = msgBuffer[pbuf+4]
				pl = msgBuffer[pbuf+5]
				cc = msgBuffer[pbuf+8]
				cl = msgBuffer[pbuf+9]
				cs = msgBuffer[pbuf+10]
				cp = msgBuffer[pbuf+11]
			)
			pbuf += 16
			s.SendPacketDataToPlayer(s.players[cc], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			if (cl&(ocgcore.LOCATION_GRAVE+ocgcore.LOCATION_OVERLAY)) == 0 &&
				((cl&(ocgcore.LOCATION_DECK+ocgcore.LOCATION_HAND)) != 0 || cp&ocgcore.POS_FACEDOWN != 0) {
				_ = utils.BatchDecode(msgBuffer[pbufw:], &pbufw, binary.LittleEndian, int32(0))
			}
			s.SendPacketDataToPlayer(s.players[1-cc], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			if cl != 0 && (cl&ocgcore.LOCATION_OVERLAY) == 0 && (cl != pl || pc != cc) {
				s.RefreshSingleDef(cc, cl, cs)
			}
		case ocgcore.MSG_POS_CHANGE:
			var (
				cc = msgBuffer[pbuf+4]
				cl = msgBuffer[pbuf+5]
				cs = msgBuffer[pbuf+6]
				pp = msgBuffer[pbuf+7]
				cp = msgBuffer[pbuf+8]
			)
			pbuf += 9
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			if (pp&ocgcore.POS_FACEDOWN != 0) && (cp&ocgcore.POS_FACEUP != 0) {
				s.RefreshSingleDef(cc, cl, cs)
			}
		case ocgcore.MSG_SET:
			_ = utils.BatchEncode(msgBuffer[pbufw:], &pbuf, binary.LittleEndian, int32(0))
			pbuf += 4
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SWAP:
			var (
				c1 = msgBuffer[pbuf+4]
				l1 = msgBuffer[pbuf+5]
				s1 = msgBuffer[pbuf+6]
				c2 = msgBuffer[pbuf+12]
				l2 = msgBuffer[pbuf+13]
				s2 = msgBuffer[pbuf+14]
			)
			pbuf += 16
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshSingleDef(c1, l1, s1)
			s.RefreshSingleDef(c2, l2, s2)
		case ocgcore.MSG_FIELD_DISABLED:
			pbuf += 4
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SUMMONING:
			pbuf += 8
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SUMMONED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
		case ocgcore.MSG_SPSUMMONING:
			pbuf += 8
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SPSUMMONED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
		case ocgcore.MSG_FLIPSUMMONING:
			s.RefreshSingleDef(msgBuffer[pbuf+4], msgBuffer[pbuf+5], msgBuffer[pbuf+6])
			pbuf += 8
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_FLIPSUMMONED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
			s.RefreshSzoneDef(0)
			s.RefreshSzoneDef(1)
		case ocgcore.MSG_CHAINING:
			pbuf += 16
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAINED:
			pbuf++
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
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
		case ocgcore.MSG_CHAIN_SOLVING:
			pbuf++
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAIN_SOLVED:
			pbuf++
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
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
		case ocgcore.MSG_CHAIN_END:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
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
			pbuf++
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAIN_DISABLED:
			pbuf++
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CARD_SELECTED:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count) * 4
		case ocgcore.MSG_RANDOM_SELECTED:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count) * 4
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_BECOME_TARGET:
			var (
				count uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &count)
			pbuf += int(count) * 4
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DRAW:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbufw = pbuf
			pbuf += int(count) * 4
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			for i := uint8(0); i < count; i++ {
				if msgBuffer[pbufw+3]&0x80 == 0 {
					_ = utils.BatchEncode(msgBuffer[pbufw:], &pbufw, binary.LittleEndian, int32(0))
				} else {
					pbufw += 4
				}
			}
			s.SendPacketDataToPlayer(s.players[1], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DAMAGE:
			pbuf += 5
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_RECOVER:
			pbuf += 5
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_EQUIP:
			pbuf += 8
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_LPUPDATE:
			pbuf += 5
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_UNEQUIP:
			pbuf += 4
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CARD_TARGET:
			pbuf += 8
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CANCEL_TARGET:
			pbuf += 8
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ADD_COUNTER:
			pbuf += 7
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_REMOVE_COUNTER:
			pbuf += 7
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ATTACK:
			pbuf += 8
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_BATTLE:
			pbuf += 26
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ATTACK_DISABLED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DAMAGE_STEP_START:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
		case ocgcore.MSG_DAMAGE_STEP_END:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzoneDef(0)
			s.RefreshMzoneDef(1)
		case ocgcore.MSG_MISSED_EFFECT:
			var (
				player = msgBuffer[pbuf]
			)
			pbuf += 8
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
		case ocgcore.MSG_TOSS_COIN:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_TOSS_DICE:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ROCK_PAPER_SCISSORS:
			var (
				player uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_HAND_RES:
			pbuf += 1
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ANNOUNCE_RACE:
			var (
				player uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 5
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_ANNOUNCE_ATTRIB:
			var (
				player uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player)
			pbuf += 5
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_ANNOUNCE_CARD, ocgcore.MSG_ANNOUNCE_NUMBER:
			var (
				player uint8
				count  uint8
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &player, &count)
			pbuf += int(count) * 4
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			return 1
		case ocgcore.MSG_CARD_HINT:
			pbuf += 9
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_PLAYER_HINT:
			pbuf += 6
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
			s.ReSendToPlayer(s.players[1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_MATCH_KILL:
			var (
				code int32
			)
			_ = utils.BatchDecode(msgBuffer[pbuf:], &pbuf, binary.LittleEndian, &code)
			if s.MatchMode {
				s.matchKill = int(code)
				s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, msgBuffer[:pbuf])
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
		}
		return
	}
	s.ETimer.Reset(time.Second)
}
func (s *SingleDuel) RefreshExtraDef(player int) {
	s.RefreshExtra(player, 0xe81fff, 1)
}
func (s *SingleDuel) RefreshExtra(player int, flag int32, useCache int) {
	buff := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	length := s.writeUpdateData(player, int(ocgcore.LOCATION_EXTRA), flag, buff, useCache)
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, buff[:length+3])
}
func (s *SingleDuel) RefreshMzoneDef(player int) {
	s.RefreshMzone(player, 0x881fff, 1)

}
func (s *SingleDuel) RefreshMzone(player int, flag int32, useCache int) {
	buff := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	data := s.writeUpdateData(player, int(ocgcore.LOCATION_MZONE), flag, buff, useCache)
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, data)

}
func (s *SingleDuel) writeUpdateData(player int, location int, flag int32, qbuf []byte, use_cache int) int {
	flag |= ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION
	utils.BatchWrite(bytes.NewBuffer(qbuf[:0]),
		binary.LittleEndian,
		int8(ocgcore.MSG_UPDATE_DATA),
		int8(player), int8(location))
	return int(s.Duel.QueryFieldCard(uint8(player), uint8(location), flag, qbuf, use_cache != 0))
}
func (s *SingleDuel) RefreshSzoneDef(player int) {
	s.RefreshSzone(player, 0x681fff, 1)
}
func (s *SingleDuel) RefreshSzone(player int, flag int32, useCache int) {
	buff := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_SZONE), flag, buff, useCache))
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, buff[:length+3])
	var (
		qLen int32
		qBuf int
	)

	for qLen < length {
		var clen int32
		_ = utils.BatchDecode(buff[qBuf:], &qBuf, binary.LittleEndian, &clen)
		qLen += clen
		if clen < ocgcore.LEN_HEADER {
			continue
		}
		position := network.GetPosition(buff[qBuf:], 8)
		if position&ocgcore.POS_FACEDOWN != 0 {
			for i := int32(0); i < clen-4; i++ {
				buff[int32(qBuf)+i] = 0
			}
		}
		qBuf += int(clen) - 4
	}
	s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, buff[:length+3])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}
func (s *SingleDuel) RefreshHandDef(player int) {
	s.RefreshHand(player, 0x681fff, 1)
}
func (s *SingleDuel) RefreshHand(player int, flag int32, useCache int) {
	buff := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_HAND), flag, buff, useCache))
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, buff[:length+3])
	var (
		qLen int32
		qBuf int
	)

	for qLen < length {
		var clen int32
		_ = utils.BatchDecode(buff[qBuf:], &qBuf, binary.LittleEndian, &clen)
		qLen += clen
		if clen < ocgcore.LEN_HEADER {
			continue
		}
		position := network.GetPosition(buff[qBuf:], 8)
		if position&ocgcore.POS_FACEDOWN != 0 {
			for i := int32(0); i < clen-4; i++ {
				buff[int32(qBuf)+i] = 0
			}
		}
		qBuf += int(clen) - 4
	}
	s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, buff[:length+3])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}
func (s *SingleDuel) RefreshSingleDef(player uint8, location uint8, sequence uint8) {
	s.RefreshSingle(player, location, sequence, 0xf81fff)
}
func (s *SingleDuel) RefreshSingle(player uint8, location uint8, sequence uint8, flag int32) {
	flag |= ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION
	var (
		qbuf   = make([]byte, 0x1000)
		offset int
	)

	_ = utils.BatchDecode(qbuf, &offset, binary.LittleEndian,
		int8(ocgcore.MSG_UPDATE_CARD),
		int8(player),
		int8(location),
		int8(sequence))
	length := s.Duel.QueryCard(player, location, sequence, flag, qbuf, false)
	s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, qbuf[:int(length)+4])
	if length <= ocgcore.LEN_HEADER {
		return
	}
	var (
		clen int32
	)
	_ = utils.BatchDecode(qbuf[offset:], &offset, binary.LittleEndian, &clen)
	position := network.GetPosition(qbuf, 8)
	if position&ocgcore.POS_FACEDOWN != 0 {
		_ = utils.BatchEncode(qbuf[offset:], &offset, binary.LittleEndian, int32(ocgcore.QUERY_CODE), int32(0), make([]byte, clen-12))
	}
	s.SendPacketDataToPlayer(s.players[1-player], network.STOC_GAME_MSG, qbuf[:int(length)+4])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}
