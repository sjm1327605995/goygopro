package duel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/antlabs/timer"
	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
	"math/rand"
	"time"
)

type TagDuel struct {
	DuelMode
	players      [4]*DuelPlayer
	pplayer      [4]*DuelPlayer
	curPlayer    [2]*DuelPlayer
	ready        [4]bool
	pDeck        [4]*Deck
	DeckError    [4]uint32
	handResult   [2]uint8
	lastResponse uint8
	Observers    map[string]*DuelPlayer
	turnCount    uint8
	timeLimit    [2]int16
	timeElapsed  int16
}

func NewTagDuel() *TagDuel {
	s := &TagDuel{
		Observers: make(map[string]*DuelPlayer),
	}
	for i := 0; i < 4; i++ {
		s.ready[i] = false
	}
	return s
}

func (s *TagDuel) Chat(dp *DuelPlayer, pData []byte) {
	var dst bytes.Buffer
	size := s.CreateChatPacket(pData, &dst, uint16(dp.Type))
	if size == 0 {
		return
	}
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.SendPacketDataToPlayer(s.players[i], network.STOC_CHAT, dst.Bytes())
		}
	}
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}

func (s *TagDuel) JoinGame(dp *DuelPlayer, pkt *protocol.CTOSJoinGame, isCreator bool) {
	if !isCreator {
		if dp.Game != nil && dp.Type != 0xff {
			var scem protocol.STOCErrorMsg
			scem.Msg = network.ERRMSG_JOINERROR
			scem.Code = 0
			s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
			s.DisconnetPlayer(dp)
			return
		}
		if pkt.Version != PRO_VERSION {
			var scem protocol.STOCErrorMsg
			scem.Msg = network.ERRMSG_VERERROR
			scem.Code = PRO_VERSION
			s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
			s.DisconnetPlayer(dp)
			return
		}
		var jpass [20]uint16
		utils.NullTerminate(pkt.Pass[:], uint16(0))
		copy(jpass[:], pkt.Pass[:])
		if utils.Wcscmp(jpass[:], s.Pass[:]) != 0 {
			var scem protocol.STOCErrorMsg
			scem.Msg = network.ERRMSG_JOINERROR
			scem.Code = 1
			s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
			return
		}
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
	if dp == s.HostPlayer {
		s.EndDuel()
		// NetServer::StopServer()
	} else if dp.Type == network.NETPLAYER_TYPE_OBSERVER {
		delete(s.Observers, dp.ID)
		if s.DuelStage == network.DUEL_STAGE_BEGIN {
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
		s.DisconnetPlayer(dp)
	} else {
		if s.DuelStage == network.DUEL_STAGE_BEGIN {
			var scpc protocol.STOCHsPlayerChange
			s.players[dp.Type] = nil
			s.ready[dp.Type] = false
			scpc.Status = uint8(dp.Type<<4) | network.PLAYERCHANGE_LEAVE
			for i := 0; i < 4; i++ {
				if s.players[i] != nil {
					s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_PLAYER_CHANGE, scpc)
				}
			}
			for _, v := range s.Observers {
				s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_CHANGE, scpc)
			}
		} else if s.DuelStage != network.DUEL_STAGE_END {
			s.EndDuel()
			s.DuelEndProc()
		}
		s.DisconnetPlayer(dp)
	}
}

func (s *TagDuel) ToDuelList(dp *DuelPlayer) {
	if s.players[0] != nil && s.players[1] != nil && s.players[2] != nil && s.players[3] != nil {
		return
	}
	if dp.Type == network.NETPLAYER_TYPE_OBSERVER {
		delete(s.Observers, dp.ID)
		var scpe protocol.STOCHsPlayerEnter
		copy(scpe.Name[:], dp.Name[:])
		var newType uint8
		if s.players[0] == nil {
			newType = 0
		} else if s.players[1] == nil {
			newType = 1
		} else if s.players[2] == nil {
			newType = 2
		} else {
			newType = 3
		}
		dp.Type = newType
		s.players[newType] = dp
		scpe.Pos = newType
		var scwc protocol.STOCHsWatchChange
		scwc.WatchCount = uint16(len(s.Observers))
		for i := 0; i < 4; i++ {
			if s.players[i] != nil {
				s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_PLAYER_ENTER, scpe)
				s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_WATCH_CHANGE, scwc)
			}
		}
		for _, v := range s.Observers {
			s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_ENTER, scpe)
			s.SendPacketDataToPlayer(v, network.STOC_HS_WATCH_CHANGE, scwc)
		}
		var sctc protocol.STOCTypeChange
		sctc.Type = 0
		if dp == s.HostPlayer {
			sctc.Type = 0x10
		}
		sctc.Type |= dp.Type
		s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, sctc)
	} else {
		if s.ready[dp.Type] {
			return
		}
		dptype := (dp.Type + 1) % 4
		for s.players[dptype] != nil {
			dptype = (dptype + 1) % 4
		}
		var scpc protocol.STOCHsPlayerChange
		scpc.Status = uint8(dp.Type<<4) | dptype
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
		sctc.Type |= dptype
		s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, sctc)
		s.players[dp.Type] = nil
		s.players[dptype] = dp
		dp.Type = dptype
	}
}

func (s *TagDuel) ToObserver(dp *DuelPlayer) {
	if dp.Type > 3 {
		return
	}
	var scpc protocol.STOCHsPlayerChange
	scpc.Status = uint8(dp.Type<<4) | network.PLAYERCHANGE_OBSERVE
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_PLAYER_CHANGE, scpc)
		}
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_CHANGE, scpc)
	}
	s.players[dp.Type] = nil
	s.ready[dp.Type] = false
	dp.Type = network.NETPLAYER_TYPE_OBSERVER
	s.Observers[dp.ID] = dp
	var sctc protocol.STOCTypeChange
	if dp == s.HostPlayer {
		sctc.Type = 0x10
	}
	sctc.Type |= dp.Type
	s.SendPacketDataToPlayer(dp, network.STOC_TYPE_CHANGE, sctc)
}

func (s *TagDuel) PlayerReady(dp *DuelPlayer, isReady bool) {
	if dp.Type > 3 || s.ready[dp.Type] == isReady {
		return
	}
	if isReady {
		var deckerror uint32
		if s.HostInfo.NoCheckDeck == 0 {
			if s.DeckError[dp.Type] != 0 {
				deckerror = (network.DECKERROR_UNKNOWNCARD << 28) | s.DeckError[dp.Type]
			} else {
				deckerror = DeckManger.CheckDeck(s.pDeck[dp.Type], s.HostInfo.LFList, int(s.HostInfo.Rule))
			}
		}
		if deckerror != 0 {
			var scpc protocol.STOCHsPlayerChange
			scpc.Status = uint8(dp.Type<<4) | network.PLAYERCHANGE_NOTREADY
			s.SendPacketDataToPlayer(dp, network.STOC_HS_PLAYER_CHANGE, scpc)
			var scem protocol.STOCErrorMsg
			scem.Msg = network.ERRMSG_DECKERROR
			scem.Code = deckerror
			s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
			return
		}
	}
	s.ready[dp.Type] = isReady
	var scpc protocol.STOCHsPlayerChange
	if isReady {
		scpc.Status = uint8(dp.Type<<4) | network.PLAYERCHANGE_READY
	} else {
		scpc.Status = uint8(dp.Type<<4) | network.PLAYERCHANGE_NOTREADY
	}
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.SendPacketDataToPlayer(s.players[i], network.STOC_HS_PLAYER_CHANGE, scpc)
		}
	}
	for _, v := range s.Observers {
		s.SendPacketDataToPlayer(v, network.STOC_HS_PLAYER_CHANGE, scpc)
	}
}

func (s *TagDuel) PlayerKick(dp *DuelPlayer, pos byte) {
	if pos > 3 || dp != s.HostPlayer || dp == s.players[pos] || s.players[pos] == nil {
		return
	}
	s.LeaveGame(s.players[pos])
}

func (s *TagDuel) UpdateDeck(dp *DuelPlayer, pData []byte) {
	if dp.Type > 3 || s.ready[dp.Type] {
		return
	}
	length := len(pData)
	if length < 8 || length > 2008 {
		return
	}
	var valid = true
	var deckBuf protocol.CTOSDeckData
	err := restruct.Unpack(pData, binary.LittleEndian, &deckBuf)
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
		var scem protocol.STOCErrorMsg
		scem.Msg = network.ERRMSG_DECKERROR
		scem.Code = 0
		s.SendPacketDataToPlayer(dp, network.STOC_ERROR_MSG, scem)
		return
	}
	s.DeckError[dp.Type] = DeckManger.LoadDeck(s.pDeck[dp.Type], deckBuf.List, deckBuf.MainC, deckBuf.SideC, false)
}

func (s *TagDuel) StartDuel(dp *DuelPlayer) {
	if dp != s.HostPlayer {
		return
	}
	if !s.ready[0] || !s.ready[1] || !s.ready[2] || !s.ready[3] {
		return
	}
	// NetServer::StopListen()
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
	s.SendPacketDataToPlayer(s.players[0], network.STOC_DECK_COUNT, deckbuff[:6])
	s.ReSendToPlayer(s.players[1])
	var tempbuff [6]byte
	copy(tempbuff[:], deckbuff[:6])
	copy(deckbuff[:6], deckbuff[6:12])
	copy(deckbuff[6:12], tempbuff[:])
	s.SendPacketDataToPlayer(s.players[2], network.STOC_DECK_COUNT, deckbuff[:6])
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
	if res > 3 {
		return
	}
	if dp.State != network.CTOS_HAND_RESULT {
		return
	}
	if dp.Type == 0 {
		s.handResult[0] = res
	} else {
		s.handResult[1] = res
	}
	if s.handResult[0] != 0 && s.handResult[1] != 0 {
		var schr protocol.STOCHandResult
		schr.Res1 = s.handResult[0]
		schr.Res2 = s.handResult[1]
		s.SendPacketDataToPlayer(s.players[0], network.STOC_HAND_RESULT, schr)
		s.ReSendToPlayer(s.players[1])
		for _, v := range s.Observers {
			s.ReSendToPlayer(v)
		}
		schr.Res1 = s.handResult[1]
		schr.Res2 = s.handResult[0]
		s.SendPacketDataToPlayer(s.players[2], network.STOC_HAND_RESULT, schr)
		s.ReSendToPlayer(s.players[3])
		if s.handResult[0] == s.handResult[1] {
			s.SendPacketToPlayer(s.players[0], network.STOC_SELECT_HAND)
			s.ReSendToPlayer(s.players[2])
			s.handResult[0], s.handResult[1] = 0, 0
			s.players[0].State = network.CTOS_HAND_RESULT
			s.players[2].State = network.CTOS_HAND_RESULT
		} else if (s.handResult[0] == 1 && s.handResult[1] == 2) ||
			(s.handResult[0] == 2 && s.handResult[1] == 3) ||
			(s.handResult[0] == 3 && s.handResult[1] == 1) {
			s.SendPacketToPlayer(s.players[2], network.STOC_SELECT_TP)
			s.players[0].State = 0xff
			s.players[2].State = network.CTOS_TP_RESULT
			s.DuelStage = network.DUEL_STAGE_FIRSTGO
		} else {
			s.SendPacketToPlayer(s.players[0], network.STOC_SELECT_TP)
			s.players[2].State = 0xff
			s.players[0].State = network.CTOS_TP_RESULT
			s.DuelStage = network.DUEL_STAGE_FIRSTGO
		}
	}
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
	s.Duel = ocgcore.NewDuel(seed)
	s.Duel.InitPlayers(s.HostInfo.StartLp, int32(s.HostInfo.StartHand), int32(s.HostInfo.DrawCount))
	opt := uint32(s.HostInfo.DuelRule) << 16
	if s.HostInfo.NoShuffleDeck != 0 {
		opt |= ocgcore.DUEL_PSEUDO_SHUFFLE
	}
	opt |= ocgcore.DUEL_TAG_MODE
	slices.Reverse(s.pDeck[0].Main)
	slices.Reverse(s.pDeck[1].Main)
	slices.Reverse(s.pDeck[2].Main)
	slices.Reverse(s.pDeck[3].Main)
	loadSingle := func(deckContainer []*CardDataC, p uint8, location uint8) {
		for _, v := range deckContainer {
			s.Duel.AddCard(v.Code, int(p), location)
		}
	}
	loadTag := func(deckContainer []*CardDataC, p uint8, location uint8) {
		for _, v := range deckContainer {
			s.Duel.AddCard(v.Code, int(p), location)
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
	var engineBuffer []byte
	var engFlag uint32
	var engLen int
	var stop int
	for stop == 0 {
		if engFlag == ocgcore.PROCESSOR_END {
			break
		}
		result := s.Duel.Process()
		engLen = int(result & ocgcore.PROCESSOR_BUFFER_LEN)
		engFlag = result & ocgcore.PROCESSOR_FLAG
		if engLen > 0 {
			if len(engineBuffer) < engLen {
				engineBuffer = make([]byte, engLen)
			}
			s.Duel.GetMessage(engineBuffer)
			stop = s.Analyze(engineBuffer[:engLen])
		}
	}
	if stop == 2 {
		s.DuelEndProc()
	}
}

func (s *TagDuel) DuelEndProc() {
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.SendPacketToPlayer(s.players[i], network.STOC_DUEL_END)
		}
	}
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
	s.DuelStage = network.DUEL_STAGE_END
}

func (s *TagDuel) Surrender(dp *DuelPlayer) {
	if dp.Type > 3 || s.Duel == nil {
		return
	}
	player := dp.Type
	teammate := uint8(1)
	if player == 0 {
		teammate = 1
	} else if player == 1 {
		teammate = 0
	} else if player == 2 {
		teammate = 3
	} else {
		teammate = 2
	}
	_ = teammate
	var winplayer uint8
	if player < 2 {
		winplayer = 1
	} else {
		winplayer = 0
	}
	var wbuf [3]byte
	wbuf[0] = ocgcore.MSG_WIN
	wbuf[1] = winplayer
	wbuf[2] = 0
	s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, wbuf[:])
	s.ReSendToPlayer(s.players[1])
	s.ReSendToPlayer(s.players[2])
	s.ReSendToPlayer(s.players[3])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
	s.EndDuel()
	s.DuelEndProc()
	if s.ETimer != nil {
		s.ETimer.Stop()
	}
}


func (s *TagDuel) Analyze(msgBuffer []byte) int {
	pbuf := utils.NewYGOBuffer(msgBuffer, binary.LittleEndian)
	var offset *utils.YGOBuffer
	var pbufw *utils.YGOBuffer
	for pbuf.Offset() < len(msgBuffer) {
		offset = pbuf.Clone()
		var engType uint8
		_ = pbuf.Read(&engType)
		switch engType {
		case ocgcore.MSG_RETRY:
			s.WaitforResponse(s.lastResponse)
			s.SendPacketDataToPlayer(s.curPlayer[s.lastResponse], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_HINT:
			var (
				typ    uint8
				player uint8
				data   int32
			)
			_ = pbuf.Read(&typ, &player, &data)
			switch typ {
			case 1, 2, 3, 5:
				s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			case 4, 6, 7, 8, 9, 11:
				for i := 0; i < 4; i++ {
					if s.players[i] != s.curPlayer[player] {
						s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
					}
				}
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			case 10:
				for i := 0; i < 4; i++ {
					if s.players[i] != nil {
						s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
					}
				}
				for _, v := range s.Observers {
					s.ReSendToPlayer(v)
				}
			}
		case ocgcore.MSG_WIN:
			var (
				player uint8
				typ    uint8
			)
			_ = pbuf.Read(&player, &typ)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.EndDuel()
			return 2
		case ocgcore.MSG_SELECT_BATTLECMD:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 11)
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 8)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_IDLECMD:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 11)
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 8)
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 8)
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 8)
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 8)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_EFFECTYN:
			var player uint8
			_ = pbuf.Read(&player)
			pbuf.Next(9)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_YESNO:
			var player uint8
			_ = pbuf.Read(&player)
			pbuf.Next(5)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_OPTION:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 4)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_CARD, ocgcore.MSG_SELECT_TRIBUTE:
			var (
				player uint8
			)
			_ = pbuf.Read(&player)
			pbuf.Next(3)
			var count uint8
			_ = pbuf.Read(&count)
			for i := uint8(0); i < count; i++ {
				pbufw = pbuf.Clone()
				var code int32
				_ = pbuf.Read(&code)
				var c uint8
				_ = pbuf.Read(&c)
				pbuf.Next(3)
				if c != player {
					binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
				}
			}
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_UNSELECT_CARD:
			var (
				player uint8
			)
			_ = pbuf.Read(&player)
			pbuf.Next(4)
			var count uint8
			_ = pbuf.Read(&count)
			for i := uint8(0); i < count; i++ {
				pbufw = pbuf.Clone()
				var code int32
				_ = pbuf.Read(&code)
				var c uint8
				_ = pbuf.Read(&c)
				pbuf.Next(3)
				if c != player {
					binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
				}
			}
			_ = pbuf.Read(&count)
			for i := uint8(0); i < count; i++ {
				pbufw = pbuf.Clone()
				var code int32
				_ = pbuf.Read(&code)
				var c uint8
				_ = pbuf.Read(&c)
				pbuf.Next(3)
				if c != player {
					binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
				}
			}
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_CHAIN:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(9 + int(count)*14)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_PLACE, ocgcore.MSG_SELECT_DISFIELD:
			var player uint8
			_ = pbuf.Read(&player)
			pbuf.Next(5)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_POSITION:
			var player uint8
			_ = pbuf.Read(&player)
			pbuf.Next(5)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_COUNTER:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player)
			pbuf.Next(4)
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 9)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SELECT_SUM:
			var player uint8
			_ = pbuf.Read(&player)
			pbuf.Next(7)
			var count uint8
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 13)
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 13)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_SORT_CARD:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 7)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_CONFIRM_DECKTOP:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 7)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			for i := 0; i < 4; i++ {
				if s.players[i] != s.curPlayer[player] {
					s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				}
			}
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CONFIRM_CARDS:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbufw = pbuf.Clone()
			pbuf.Next(int(count) * 7)
			for i := uint8(0); i < count; i++ {
				var position uint32
				_ = pbufw.Read(&position)
				position >>= 24
				if player == 1 && position&ocgcore.POS_FACEDOWN != 0 {
					binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
				}
				pbufw.Next(3)
			}
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			for i := 0; i < 4; i++ {
				if s.players[i] != s.curPlayer[player] {
					s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				}
			}
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SHUFFLE_DECK:
			var player uint8
			_ = pbuf.Read(&player)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SHUFFLE_HAND:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlicesOffset(pbuf, int(count)*4))
			for i := uint8(0); i < count; i++ {
				binary.LittleEndian.PutUint32(pbuf.ReadNext(4), 0)
			}
			for i := 0; i < 4; i++ {
				if s.players[i] != s.curPlayer[player] {
					s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				}
			}
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshHand(int(player), 0x781fff, 0)
		case ocgcore.MSG_SHUFFLE_EXTRA:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlicesOffset(pbuf, int(count)*4))
			for i := uint8(0); i < count; i++ {
				binary.LittleEndian.PutUint32(pbuf.ReadNext(4), 0)
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
		case ocgcore.MSG_REFRESH_DECK:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SWAP_GRAVE_DECK:
			var player uint8
			_ = pbuf.Read(&player)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshGrave(int(player), 0x81fff4, 0)
		case ocgcore.MSG_REVERSE_DECK:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DECK_TOP:
			pbuf.Next(6)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SHUFFLE_SET_CARD:
			var loc uint8
			_ = pbuf.Read(&loc)
			var count uint8
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			if loc == uint8(ocgcore.LOCATION_MZONE) {
				s.RefreshMzone(0, 0x181fff, 0)
				s.RefreshMzone(1, 0x181fff, 0)
			} else {
				s.RefreshSzone(0, 0x181fff, 0)
				s.RefreshSzone(1, 0x181fff, 0)
			}
		case ocgcore.MSG_NEW_TURN:
			pbuf.Next(1)
			s.timeLimit[0] = int16(s.HostInfo.TimeLimit)
			s.timeLimit[1] = int16(s.HostInfo.TimeLimit)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
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
		case ocgcore.MSG_NEW_PHASE:
			pbuf.Next(2)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
			s.RefreshSzone(0, 0x81fff4, 0)
			s.RefreshSzone(1, 0x81fff4, 0)
			s.RefreshHand(0, 0x781fff, 0)
			s.RefreshHand(1, 0x781fff, 0)
		case ocgcore.MSG_MOVE:
			pbufw = pbuf.Clone()
			var pc = pbuf.At(4)
			var pl = pbuf.At(5)
			var cc = pbuf.At(8)
			var cl = pbuf.At(9)
			var cs = pbuf.At(10)
			var cp = pbuf.At(11)
			pbuf.Next(16)
			s.SendPacketDataToPlayer(s.curPlayer[cc], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			if cl&(uint8(ocgcore.LOCATION_GRAVE+ocgcore.LOCATION_OVERLAY)) == 0 && ((cl&(uint8(ocgcore.LOCATION_DECK+ocgcore.LOCATION_HAND))) != 0 || (cp&ocgcore.POS_FACEDOWN) != 0) {
				binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
			}
			for i := 0; i < 4; i++ {
				if s.players[i] != s.curPlayer[cc] {
					s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
				}
			}
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			if cl != 0 && (cl&uint8(ocgcore.LOCATION_OVERLAY)) == 0 && (cl != pl || pc != cc) {
				s.RefreshSingle(cc, cl, cs, 0x81fff4)
			}
		case ocgcore.MSG_POS_CHANGE:
			var cc = pbuf.At(4)
			var cl = pbuf.At(5)
			var cs = pbuf.At(6)
			var pp = pbuf.At(7)
			var cp = pbuf.At(8)
			pbuf.Next(9)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			if (pp&ocgcore.POS_FACEDOWN) != 0 && (cp&ocgcore.POS_FACEUP) != 0 {
				s.RefreshSingle(cc, cl, cs, 0x81fff4)
			}
		case ocgcore.MSG_SET:
			binary.LittleEndian.PutUint32(pbuf.ReadNext(4), 0)
			pbuf.Next(4)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SWAP:
			var c1 = pbuf.At(4)
			var l1 = pbuf.At(5)
			var s1 = pbuf.At(6)
			var c2 = pbuf.At(12)
			var l2 = pbuf.At(13)
			var s2 = pbuf.At(14)
			pbuf.Next(16)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshSingle(c1, l1, s1, 0x81fff4)
			s.RefreshSingle(c2, l2, s2, 0x81fff4)
		case ocgcore.MSG_FIELD_DISABLED:
			pbuf.Next(4)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SUMMONING:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SUMMONED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
			s.RefreshSzone(0, 0x81fff4, 0)
			s.RefreshSzone(1, 0x81fff4, 0)
		case ocgcore.MSG_SPSUMMONING:
			pbufw = pbuf.Clone()
			var cc = pbuf.At(4)
			var cp = pbuf.At(7)
			pbuf.Next(8)
			var pid int
			if cc == 0 {
				pid = 0
			} else {
				pid = 2
			}
			s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[pid+1])
			if cp&ocgcore.POS_FACEDOWN != 0 {
				binary.LittleEndian.PutUint32(pbufw.ReadNext(4), 0)
			}
			pid = 2 - pid
			s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[pid+1])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_SPSUMMONED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
			s.RefreshSzone(0, 0x81fff4, 0)
			s.RefreshSzone(1, 0x81fff4, 0)
		case ocgcore.MSG_FLIPSUMMONING:
			s.RefreshSingle(pbuf.At(4), pbuf.At(5), pbuf.At(6), 0x81fff4)
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_FLIPSUMMONED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
			s.RefreshSzone(0, 0x81fff4, 0)
			s.RefreshSzone(1, 0x81fff4, 0)
		case ocgcore.MSG_CHAINING:
			pbuf.Next(16)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAINED:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
			s.RefreshSzone(0, 0x81fff4, 0)
			s.RefreshSzone(1, 0x81fff4, 0)
			s.RefreshHand(0, 0x781fff, 0)
			s.RefreshHand(1, 0x781fff, 0)
		case ocgcore.MSG_CHAIN_SOLVING:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAIN_SOLVED:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
			s.RefreshSzone(0, 0x81fff4, 0)
			s.RefreshSzone(1, 0x81fff4, 0)
			s.RefreshHand(0, 0x781fff, 0)
			s.RefreshHand(1, 0x781fff, 0)
		case ocgcore.MSG_CHAIN_END:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
			s.RefreshSzone(0, 0x81fff4, 0)
			s.RefreshSzone(1, 0x81fff4, 0)
			s.RefreshHand(0, 0x781fff, 0)
			s.RefreshHand(1, 0x781fff, 0)
		case ocgcore.MSG_CHAIN_NEGATED:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CHAIN_DISABLED:
			pbuf.Next(1)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CARD_SELECTED:
			var player uint8
			_ = pbuf.Read(&player)
			var count uint8
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 4)
		case ocgcore.MSG_RANDOM_SELECTED:
			var player uint8
			_ = pbuf.Read(&player)
			var count uint8
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 4)
			s.SendPacketDataToPlayer(s.players[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_BECOME_TARGET:
			var count uint8
			_ = pbuf.Read(&count)
			pbuf.Next(int(count) * 4)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
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
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			for i := uint8(0); i < count; i++ {
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
		case ocgcore.MSG_DAMAGE:
			pbuf.Next(5)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_RECOVER:
			pbuf.Next(5)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_EQUIP:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_LPUPDATE:
			pbuf.Next(5)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_UNEQUIP:
			pbuf.Next(4)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CARD_TARGET:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_CANCEL_TARGET:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_PAY_LPCOST:
			pbuf.Next(5)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ADD_COUNTER:
			pbuf.Next(7)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_REMOVE_COUNTER:
			pbuf.Next(7)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ATTACK:
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_BATTLE:
			pbuf.Next(26)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ATTACK_DISABLED:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_DAMAGE_STEP_START:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
		case ocgcore.MSG_DAMAGE_STEP_END:
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
			s.RefreshMzone(0, 0x81fff4, 0)
			s.RefreshMzone(1, 0x81fff4, 0)
		case ocgcore.MSG_MISSED_EFFECT:
			var player = pbuf.At(0)
			pbuf.Next(8)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
		case ocgcore.MSG_TOSS_COIN:
			var player uint8
			_ = pbuf.Read(&player)
			var count uint8
			_ = pbuf.Read(&count)
			pbuf.Next(int(count))
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_TOSS_DICE:
			var player uint8
			_ = pbuf.Read(&player)
			var count uint8
			_ = pbuf.Read(&count)
			pbuf.Next(int(count))
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_ANNOUNCE_RACE:
			var player uint8
			_ = pbuf.Read(&player)
			pbuf.Next(5)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_ANNOUNCE_ATTRIB:
			var player uint8
			_ = pbuf.Read(&player)
			pbuf.Next(5)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_ANNOUNCE_CARD, ocgcore.MSG_ANNOUNCE_NUMBER:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			pbuf.Next(int(count) * 4)
			s.WaitforResponse(player)
			s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			return 1
		case ocgcore.MSG_CARD_HINT:
			pbuf.Next(9)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_PLAYER_HINT:
			pbuf.Next(6)
			s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, offset.SubSlices(pbuf))
			s.ReSendToPlayer(s.players[1])
			s.ReSendToPlayer(s.players[2])
			s.ReSendToPlayer(s.players[3])
			for _, v := range s.Observers {
				s.ReSendToPlayer(v)
			}
		case ocgcore.MSG_MATCH_KILL:
			pbuf.Next(4)
		case ocgcore.MSG_TAG_SWAP:
			var player uint8
			_ = pbuf.Read(&player)
			pbuf.Next(3)
			var ecount uint8
			_ = pbuf.Read(&ecount)
			var hcount uint8
			_ = pbuf.Read(&hcount)
			pbufw = pbuf.Clone()
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
		}
	}
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
	resb := make([]byte, ocgcore.SIZE_RETURN_VALUE)
	copy(resb, msgBuffer)
	s.Duel.SetResponseb(resb)
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
	if s.HostInfo.TimeLimit == 0 {
		return
	}
	if dp != s.curPlayer[s.lastResponse] {
		return
	}
	s.curPlayer[s.lastResponse].State = network.CTOS_RESPONSE
	if s.timeElapsed < 10 {
		s.timeElapsed = 0
	}
}

func (s *TagDuel) EndDuel() {
	if s.Duel == nil {
		return
	}
	s.Duel.End()
	if s.ETimer != nil {
		s.ETimer.Stop()
		s.ETimer = nil
	}
	s.Duel = nil
	for i := 0; i < 4; i++ {
		if s.players[i] != nil {
			s.players[i].State = 0xff
		}
	}
}

func (s *TagDuel) WaitforResponse(player byte) {
	s.lastResponse = player
	msg := ocgcore.MSG_WAITING
	for i := 0; i < 4; i++ {
		if s.players[i] != s.curPlayer[player] {
			s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, msg)
		}
	}
	if s.HostInfo.TimeLimit != 0 {
		s.timeElapsed = 0
		var sctl protocol.STOCTimeLimit
		sctl.Player = player
		sctl.LeftTime = uint16(s.timeLimit[player])
		s.SendPacketDataToPlayer(s.players[0], network.STOC_TIME_LIMIT, sctl)
		s.ReSendToPlayer(s.players[1])
		s.ReSendToPlayer(s.players[2])
		s.ReSendToPlayer(s.players[3])
		s.curPlayer[player].State = network.CTOS_TIME_CONFIRM
	} else {
		s.curPlayer[player].State = network.CTOS_RESPONSE
	}
}

func (s *TagDuel) writeUpdateData(player int, location int, flag uint32, qbuf []byte, useCache int) int32 {
	flag |= ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION
	wbuf := utils.NewYGOBuffer(qbuf, binary.LittleEndian)
	wbuf.Write(uint8(ocgcore.MSG_UPDATE_DATA), uint8(player), uint8(location))
	return s.Duel.QueryFieldCard(player, location, flag, wbuf.Bytes(), useCache)
}

func (s *TagDuel) RefreshMzoneDef(player int) {
	s.RefreshMzone(player, 0x881fff, 1)
}

func (s *TagDuel) RefreshMzone(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_MZONE), flag, qbuf.Bytes(), useCache))
	pid := 0
	if player == 0 {
		pid = 0
	} else {
		pid = 2
	}
	s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, queryBuffer[:length+3])
	s.ReSendToPlayer(s.players[pid+1])
	var qLen int32
	qbuf.Next(3)
	for qLen < length {
		var clen int32
		qbuf.Read(&clen)
		qLen += clen
		if clen <= ocgcore.LEN_HEADER {
			continue
		}
		data := qbuf.Bytes()
		position := network.GetPosition(data, 8)
		if position&ocgcore.POS_FACEDOWN != 0 {
			copy(data[:clen-4], make([]byte, clen-4))
		}
		qbuf.Next(int(clen) - 4)
	}
	pid = 2 - pid
	s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, queryBuffer[:length+3])
	s.ReSendToPlayer(s.players[pid+1])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}

func (s *TagDuel) RefreshSzoneDef(player int) {
	s.RefreshSzone(player, 0x681fff, 1)
}

func (s *TagDuel) RefreshSzone(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_SZONE), flag, qbuf.Bytes(), useCache))
	pid := 0
	if player == 0 {
		pid = 0
	} else {
		pid = 2
	}
	s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, queryBuffer[:length+3])
	s.ReSendToPlayer(s.players[pid+1])
	var qLen int32
	qbuf.Next(3)
	for qLen < length {
		var clen int32
		qbuf.Read(&clen)
		qLen += clen
		if clen <= ocgcore.LEN_HEADER {
			continue
		}
		data := qbuf.Bytes()
		position := network.GetPosition(data, 8)
		if position&ocgcore.POS_FACEDOWN != 0 {
			copy(data[:clen-4], make([]byte, clen-4))
		}
		qbuf.Next(int(clen) - 4)
	}
	pid = 2 - pid
	s.SendPacketDataToPlayer(s.players[pid], network.STOC_GAME_MSG, queryBuffer[:length+3])
	s.ReSendToPlayer(s.players[pid+1])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}

func (s *TagDuel) RefreshHandDef(player int) {
	s.RefreshHand(player, 0x681fff, 1)
}

func (s *TagDuel) RefreshHand(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_HAND), flag, qbuf.Bytes(), useCache))
	s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, queryBuffer[:length+3])
	qbuf.Next(3)
	var qLen int32
	for qLen < length {
		var slen int32
		qbuf.Read(&slen)
		qLen += slen
		if slen <= ocgcore.LEN_HEADER {
			continue
		}
		position := network.GetPosition(qbuf.Bytes(), 8)
		if position&ocgcore.POS_FACEUP == 0 {
			copy(qbuf.Bytes()[:slen-4], make([]byte, slen-4))
		}
		qbuf.Next(int(slen) - 4)
	}
	for i := 0; i < 4; i++ {
		if s.players[i] != s.curPlayer[player] {
			s.SendPacketDataToPlayer(s.players[i], network.STOC_GAME_MSG, queryBuffer[:length+3])
		}
	}
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}

func (s *TagDuel) RefreshGraveDef(player int) {
	s.RefreshGrave(player, 0x81fff, 1)
}

func (s *TagDuel) RefreshGrave(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_GRAVE), flag, qbuf.Bytes(), useCache))
	s.SendPacketDataToPlayer(s.players[0], network.STOC_GAME_MSG, queryBuffer[:length+3])
	s.ReSendToPlayer(s.players[1])
	s.ReSendToPlayer(s.players[2])
	s.ReSendToPlayer(s.players[3])
	for _, v := range s.Observers {
		s.ReSendToPlayer(v)
	}
}

func (s *TagDuel) RefreshExtraDef(player int) {
	s.RefreshExtra(player, 0xe81fff, 1)
}

func (s *TagDuel) RefreshExtra(player int, flag uint32, useCache int) {
	queryBuffer := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	qbuf := utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	length := int32(s.writeUpdateData(player, int(ocgcore.LOCATION_EXTRA), flag, qbuf.Bytes(), useCache))
	s.SendPacketDataToPlayer(s.curPlayer[player], network.STOC_GAME_MSG, queryBuffer[:length+3])
}

func (s *TagDuel) RefreshSingleDef(player uint8, location uint8, sequence uint8) {
	s.RefreshSingle(player, location, sequence, 0xf81fff)
}

func (s *TagDuel) RefreshSingle(player uint8, location uint8, sequence uint8, flag int32) {
	flag |= int32(ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION)
	var queryBuffer = make([]byte, 0x1000)
	var qbuf = utils.NewYGOBuffer(queryBuffer, binary.LittleEndian)
	qbuf.Write([]byte{ocgcore.MSG_UPDATE_CARD, player, location, sequence})
	length := s.Duel.QueryCard(player, location, sequence, uint32(flag), qbuf.Bytes(), false)
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
	s.timeElapsed++
	if int(s.timeElapsed) >= int(s.timeLimit[s.lastResponse]) || s.timeLimit[s.lastResponse] <= 0 {
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
	}
}
