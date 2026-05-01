package duel

import (
	"encoding/binary"
	"fmt"
	"github.com/go-restruct/restruct"
	"github.com/panjf2000/gnet/v2"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
	"unicode/utf16"
)

type DuelPlayer struct {
	ID    string
	Type  uint8
	Name  [20]uint16
	Game  IDuelMode
	Conn  gnet.Conn
	State uint8
}

func (d *DuelPlayer) SetHostPlayer() {
	//TODO implement me
	panic("implement me")
}

func (d *DuelPlayer) GetID() string {
	return d.ID
}
func (d *DuelPlayer) SetID(id string) {
	d.ID = id
}
func (d *DuelPlayer) Disconnect() {
	d.Conn.Close()
}

func (d *DuelPlayer) Write(data []byte) (int, error) {
	return d.Conn.Write(data)
}
func (d *DuelPlayer) Close() error {
	return d.Conn.Close()
}

var (
	CURRENT_RULE uint8 = 5
	MODE_SINGLE  uint8 = 0
	MODE_MATCH   uint8 = 1
	MODE_TAG     uint8 = 2
)

func (d *DuelPlayer) HandleCTOSPacket(data []byte) {

	pktType := data[0]
	pData := data[1:]
	if pktType != network.CTOS_SURRENDER && pktType != network.CTOS_CHAT {
		if d.State == 0xff || (d.State != 0 && d.State != pktType) {
			return
		}
	}
	switch pktType {
	case network.CTOS_RESPONSE:
		if d.Game == nil {
			return
		}
		d.Game.GetResponse(d, pData)
	case network.CTOS_TIME_CONFIRM:
		if d.Game == nil || d.Game.OCGDuel() == nil {
			return
		}
		d.Game.TimeConfirm(d)
	case network.CTOS_CHAT:
		if d.Game == nil {
			return
		}
		if len(pData) < 2 {
			return
		}
		if len(pData) > protocol.LEN_CHAT_MSG*2 {
			return
		}
		if len(pData)%2 != 0 {
			return
		}
		d.Game.Chat(d, pData)
	case network.CTOS_UPDATE_DECK:
		if d.Game == nil {
			return
		}
		if len(pData) < 8 {
			return
		}
		if len(pData) > int(binary.Size(protocol.CTOSDeckData{})) {
			return
		}
		d.Game.UpdateDeck(d, pData)
	case network.CTOS_HAND_RESULT:
		if d.Game == nil {
			return
		}
		if len(pData) < int(binary.Size(protocol.CTOSHandResult{})) {
			return
		}
		var pkt protocol.CTOSHandResult
		restruct.Unpack(pData, binary.LittleEndian, &pkt)
		d.Game.HandResult(d, pkt.Res)
	case network.CTOS_TP_RESULT:
		if d.Game == nil {
			return
		}
		if len(pData) < int(binary.Size(protocol.CTOSTPResult{})) {
			return
		}
		var pkt protocol.CTOSTPResult
		restruct.Unpack(pData, binary.LittleEndian, &pkt)
		d.Game.TPResult(d, pkt.Res)
	case network.CTOS_PLAYER_INFO:
		var pkt protocol.CTOSPlayerInfo
		err := restruct.Unpack(pData, binary.LittleEndian, &pkt)
		if err != nil {
			fmt.Println(err)
			return
		}
		utils.NullTerminate(pkt.Name[:], 0)
		copy(d.Name[:], pkt.Name[:])
		d.SetID(string(utf16.Decode(d.Name[:])))
	case network.CTOS_CREATE_GAME:
		if d.Game != nil {
			return
		}
		if len(data) < 1+int(binary.Size(protocol.CTOSCreateGame{})) {
			return
		}
		var pkt protocol.CTOSCreateGame
		restruct.Unpack(pData, binary.LittleEndian, &pkt)
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
		var mode IDuelMode
		if pkt.Info.Mode == MODE_SINGLE {
			mode = &SingleDuel{Observers: make(map[string]*DuelPlayer)}
		} else if pkt.Info.Mode == MODE_MATCH {
			mode = &SingleDuel{Observers: make(map[string]*DuelPlayer), MatchMode: true}
		} else if pkt.Info.Mode == MODE_TAG {
			mode = &TagDuel{Observers: make(map[string]*DuelPlayer)}
		} else {
			return
		}
		mode.BaseMode().HostInfo = pkt.Info
		utils.NullTerminate(pkt.Name[:], 0)
		utils.NullTerminate(pkt.Pass[:], 0)
		copy(mode.BaseMode().Name[:], pkt.Name[:])
		copy(mode.BaseMode().Pass[:], pkt.Pass[:])
		roomId := string(utf16.Decode(pkt.Pass[:]))
		room, _ := DefaultManager.JoinRoom(roomId, d, mode)
		d.Game = room.DuelMode
		d.Game.JoinGame(d, nil, true)

	case network.CTOS_JOIN_GAME:
		var pkt protocol.CTOSJoinGame
		err := restruct.Unpack(pData, binary.LittleEndian, &pkt)
		if err != nil {
			panic(err)
			return
		}
		roomId := string(utf16.Decode(pkt.Pass[:]))
		room, isCreator := DefaultManager.JoinRoom(roomId, d, nil)
		d.Game = room.DuelMode
		d.Game.JoinGame(d, &pkt, isCreator)
	case network.CTOS_LEAVE_GAME:
		if d.Game == nil {
			return
		}
		d.Game.LeaveGame(d)
	case network.CTOS_SURRENDER:
		if d.Game == nil {
			return
		}
		d.Game.Surrender(d)
	case network.CTOS_HS_TODUELIST:
		if d.Game == nil || d.Game.BaseMode().Duel != nil {
			return
		}
		d.Game.ToDuelList(d)
	case network.CTOS_HS_TOOBSERVER:
		if d.Game == nil || d.Game.BaseMode().Duel != nil {
			return
		}
		d.Game.ToObserver(d)
	case network.CTOS_HS_READY, network.CTOS_HS_NOTREADY:
		if d.Game == nil || d.Game.BaseMode().Duel != nil {
			return
		}
		d.Game.PlayerReady(d, (network.CTOS_HS_NOTREADY-pktType) != 0)
	case network.CTOS_HS_KICK:
		if d.Game == nil || d.Game.BaseMode().Duel != nil {
			return
		}
		var packet protocol.CTOSKick
		restruct.Unpack(pData, binary.LittleEndian, &packet)
		d.Game.PlayerKick(d, packet.Pos)
	case network.CTOS_HS_START:
		if d.Game == nil || d.Game.BaseMode().Duel != nil {
			return
		}
		d.Game.StartDuel(d)
	}

}
