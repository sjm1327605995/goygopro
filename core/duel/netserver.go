package duel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
	"unicode/utf16"
)

var (
	CURRENT_RULE uint8 = 5
	MODE_TAG     uint8 = 1
	MODE_SINGLE  uint8 = 1
	MODE_MATCH   uint8 = 0
)

func HandleCTOSPacket(dp *DuelPlayer, duelMode IDuelMode, data []byte) {

	pktType := data[0]
	pData := data[1:]
	//if pktType != network.CTOS_SURRENDER && pktType != network.CTOS_CHAT &&
	//	(dp.State == 0xff || dp.State != pktType) {
	//	return
	//}
	switch pktType {
	case network.CTOS_RESPONSE:
		if dp.Game == nil || duelMode.OCGDuel() == nil {
			return
		}
		duelMode.GetResponse(dp, pData)
	case network.CTOS_TIME_CONFIRM:
		if dp.Game == nil || duelMode.OCGDuel() == nil {
			return
		}
		duelMode.TimeConfirm(dp)
	case network.CTOS_CHAT:
		if dp.Game == nil {
			return
		}
		duelMode.Chat(dp, pData)
	case network.CTOS_UPDATE_DECK:
		if dp.Game == nil {
			return
		}
		duelMode.UpdateDeck(dp, pData)
	case network.CTOS_HAND_RESULT:
		if dp.Game == nil {
			return
		}
		var pkt protocol.CTOSHandResult
		binary.Decode(pData, binary.LittleEndian, &pkt)
		dp.Game.HandResult(dp, pkt.Res)
	case network.CTOS_TP_RESULT:
		if dp.Game == nil {
			return
		}
		var pkt protocol.CTOSTPResult
		binary.Decode(pData, binary.LittleEndian, &pkt)
		dp.Game.TPResult(dp, pkt.Res)
	case network.CTOS_PLAYER_INFO:
		var pkt protocol.CTOSPlayerInfo
		_, err := binary.Decode(pData, binary.LittleEndian, &pkt)
		if err != nil {
			fmt.Println(err)
			return
		}
		utils.NullTerminate(pkt.Name[:], 0)
		copy(dp.Name[:], pkt.Name[:])
		dp.SetID(string(utf16.Decode(dp.Name[:])))
	case network.CTOS_CREATE_GAME:
		if dp.Game != nil || duelMode != nil {
			return
		}
		var pkt protocol.CTOSCreateGame
		binary.Decode(pData, binary.LittleEndian, &pkt)
		if pkt.Info.Rule > CURRENT_RULE {
			pkt.Info.Rule = CURRENT_RULE
		}
		if pkt.Info.Mode > MODE_TAG {
			pkt.Info.Mode = MODE_SINGLE
		}
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
		if pkt.Info.Mode == MODE_SINGLE {
			duelMode = &SingleDuel{}
		} else if pkt.Info.Mode == MODE_MATCH {
			duelMode = &SingleDuel{MatchMode: true}
		} else if pkt.Info.Mode == MODE_TAG {
			//duel_mode = new TagDuel();
			//duel_mode->etimer = event_new(net_evbase, 0, EV_TIMEOUT | EV_PERSIST, TagDuel::TagTimer, duel_mode);
		} else {
			return
		}
		duelMode.BaseMode().HostInfo = pkt.Info
		utils.NullTerminate(pkt.Name[:], 0)
		utils.NullTerminate(pkt.Pass[:], 0)
		copy(duelMode.BaseMode().Name[:], pkt.Name[:])
		copy(duelMode.BaseMode().Pass[:], pkt.Pass[:])
		duelMode.JoinGame(dp, nil, true)
		//StartBroadcast()

	case network.CTOS_JOIN_GAME:
		if duelMode == nil {
			duelMode = &SingleDuel{}
			dp.Game = duelMode
			duelMode.JoinGame(dp, bytes.NewReader(pData), true)
		} else {
			dp.Game = duelMode
			duelMode.JoinGame(dp, bytes.NewReader(pData), false)
		}

	case network.CTOS_LEAVE_GAME:
		if duelMode == nil {
			return
		}
		duelMode.LeaveGame(dp)
	case network.CTOS_SURRENDER:
		if duelMode == nil {
			return
		}
		duelMode.Surrender(dp)
	case network.CTOS_HS_TODUELIST:
		if duelMode == nil || duelMode.BaseMode().Duel != nil {
			return
		}
		duelMode.ToDuelList(dp)
	case network.CTOS_HS_TOOBSERVER:
		if duelMode == nil || duelMode.BaseMode().Duel != nil {
			return
		}
		duelMode.ToObserver(dp)
	case network.CTOS_HS_READY, network.CTOS_HS_NOTREADY:
		if duelMode == nil || duelMode.BaseMode().Duel != nil {
			return
		}
		duelMode.PlayerReady(dp, (network.CTOS_HS_NOTREADY-pktType) != 0)
	case network.CTOS_HS_KICK:
		if duelMode == nil || duelMode.BaseMode().Duel != nil {
			return
		}
		var packet protocol.CTOSKick
		binary.Decode(pData, binary.LittleEndian, &packet)
		duelMode.PlayerKick(dp, packet.Pos)
	case network.CTOS_HS_START:
		if duelMode == nil || duelMode.BaseMode().Duel != nil {
			return
		}
		duelMode.StartDuel(dp)
	}

}

//size_t NetServer::CreateChatPacket(unsigned char* src, int src_size, unsigned char* dst, uint16_t dst_player_type) {
//	if (!check_msg_size(src_size))
//		return 0;
//	uint16_t src_msg[LEN_CHAT_MSG];
//	std::memcpy(src_msg, src, src_size);
//	const int src_len = src_size / sizeof(uint16_t);
//	if (src_msg[src_len - 1] != 0)
//		return 0;
//	// STOC_Chat packet
//	auto pdst = dst;
//	buffer_write<uint16_t>(pdst, dst_player_type);
//	buffer_write_block(pdst, src_msg, src_size);
//	return sizeof(dst_player_type) + src_size;
//}
//
//}
