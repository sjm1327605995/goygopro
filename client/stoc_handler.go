package client

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// HandleSTOCPacket corresponds to C++ DuelClient::HandleSTOCPacketLan
// Processes server-to-client packets (STOC_*).
func (dc *DuelClient) HandleSTOCPacket(data []byte) {
	if len(data) == 0 {
		return
	}
	pktType := data[0]
	payload := data[1:]

	switch pktType {
	case network.STOC_GAME_MSG:
		// Handled by msgLoop -> ClientAnalyze
	case network.STOC_ERROR_MSG:
		dc.handleSTOCErrorMsg(payload)
	case network.STOC_SELECT_HAND:
		// Show rock-paper-scissors selection dialog
		MainGame.Dialog.ShowOption([]int32{1, 2, 3})
		MainGame.Dialog.OnOptionSelected = func(index int) {
			res := uint8(index + 1)
			dc.SendStruct(network.CTOS_HAND_RESULT, protocol.CTOSHandResult{Res: res})
		}
	case network.STOC_SELECT_TP:
		// Show first/second selection dialog
		MainGame.Dialog.ShowOption([]int32{1, 0})
		MainGame.Dialog.OnOptionSelected = func(index int) {
			res := uint8(1)
			if index == 1 {
				res = 0
			}
			dc.SendStruct(network.CTOS_TP_RESULT, protocol.CTOSTPResult{Res: res})
		}
	case network.STOC_HAND_RESULT:
		if len(payload) >= 2 {
			var pkt protocol.STOCHandResult
			pkt.Res1 = payload[0]
			pkt.Res2 = payload[1]
			fmt.Printf("Hand result: P1=%d P2=%d\n", pkt.Res1, pkt.Res2)
		}
	case network.STOC_TP_RESULT:
		// reserved
	case network.STOC_CHANGE_SIDE:
		MainGame.DInfo.IsStarted = false
		MainGame.DInfo.IsInDuel = false
		MainGame.DField.Clear()
		MainGame.IsSiding = true
	case network.STOC_WAITING_SIDE:
		MainGame.DInfo.IsInDuel = false
		MainGame.DField.Clear()
	case network.STOC_DECK_COUNT:
		dc.handleSTOCDeckCount(payload)
	case network.STOC_JOIN_GAME:
		dc.handleSTOCJoinGame(payload)
	case network.STOC_TYPE_CHANGE:
		dc.handleSTOCTypeChange(payload)
	case network.STOC_DUEL_START:
		dc.handleSTOCDuelStart()
	case network.STOC_DUEL_END:
		dc.handleSTOCDuelEnd()
	case network.STOC_REPLAY:
		// Save replay data if auto-save is enabled
		if MainGame.Config.AutoSaveReplay != 0 && len(payload) > 0 {
			timestamp := time.Now().Unix()
			filename := fmt.Sprintf("replay_%d.yrp", timestamp)
			if err := os.WriteFile(filename, payload, 0644); err == nil {
				fmt.Printf("Replay saved: %s\n", filename)
			}
		}
	case network.STOC_TIME_LIMIT:
		dc.handleSTOCTimeLimit(payload)
	case network.STOC_CHAT:
		dc.handleSTOCChat(payload)
	case network.STOC_HS_PLAYER_ENTER:
		dc.handleSTOCHsPlayerEnter(payload)
	case network.STOC_HS_PLAYER_CHANGE:
		dc.handleSTOCHsPlayerChange(payload)
	case network.STOC_HS_WATCH_CHANGE:
		dc.handleSTOCHsWatchChange(payload)
	case network.STOC_TEAMMATE_SURRENDER:
		MainGame.DField.TagTeammateSurrender = true
	case network.STOC_FIELD_FINISH:
		fmt.Println("Field finish received")
	default:
		fmt.Printf("Unhandled STOC type: 0x%02x\n", pktType)
	}
}

func (dc *DuelClient) handleSTOCErrorMsg(payload []byte) {
	if len(payload) < 8 {
		return
	}
	msg := payload[0]
	code := binary.LittleEndian.Uint32(payload[4:])
	switch msg {
	case network.ERRMSG_JOINERROR:
		fmt.Println("Join error, code:", code)
	case network.ERRMSG_DECKERROR:
		fmt.Printf("Deck error, code: %d\n", code)
	case network.ERRMSG_SIDEERROR:
		fmt.Println("Side error")
	case network.ERRMSG_VERERROR:
		fmt.Printf("Version error: %d.%d.%d\n", code>>12, (code>>4)&0xff, code&0xf)
	}
}

func (dc *DuelClient) handleSTOCDeckCount(payload []byte) {
	if len(payload) < 12 {
		return
	}
	deckc0 := binary.LittleEndian.Uint16(payload)
	extrac0 := binary.LittleEndian.Uint16(payload[2:])
	sidec0 := binary.LittleEndian.Uint16(payload[4:])
	deckc1 := binary.LittleEndian.Uint16(payload[6:])
	extrac1 := binary.LittleEndian.Uint16(payload[8:])
	sidec1 := binary.LittleEndian.Uint16(payload[10:])
	MainGame.DField.Initial(0, int(deckc0), int(extrac0), int(sidec0))
	MainGame.DField.Initial(1, int(deckc1), int(extrac1), int(sidec1))
}

func (dc *DuelClient) handleSTOCJoinGame(payload []byte) {
	if len(payload) < 20 {
		return
	}
	var pkt protocol.STOCJoinGame
	// Read HostInfo from payload
	pkt.Info.LFList = binary.LittleEndian.Uint32(payload)
	pkt.Info.Rule = payload[4]
	pkt.Info.Mode = payload[5]
	pkt.Info.DuelRule = payload[6]
	pkt.Info.NoCheckDeck = payload[7]
	pkt.Info.NoShuffleDeck = payload[8]
	// padding[3] at 9,10,11
	pkt.Info.StartLp = int32(binary.LittleEndian.Uint32(payload[12:]))
	pkt.Info.StartHand = payload[16]
	pkt.Info.DrawCount = payload[17]
	pkt.Info.TimeLimit = binary.LittleEndian.Uint16(payload[18:])

	MainGame.DInfo.IsTag = pkt.Info.Mode == 2
	MainGame.DInfo.DuelRule = int(pkt.Info.DuelRule)
	MainGame.DInfo.StartLP = int(pkt.Info.StartLp)
	MainGame.DInfo.TimeLimit = pkt.Info.TimeLimit

	fmt.Printf("Joined game: Rule=%d Mode=%d LP=%d Time=%d\n",
		pkt.Info.Rule, pkt.Info.Mode, pkt.Info.StartLp, pkt.Info.TimeLimit)

	// Switch to lobby scene
	PushScene("lobby")
}

func (dc *DuelClient) handleSTOCTypeChange(payload []byte) {
	if len(payload) < 1 {
		return
	}
	typ := payload[0]
	dc.selfType = typ & 0xf
	dc.isHost = ((typ >> 4) & 0xf) != 0
	MainGame.DInfo.PlayerType = dc.selfType

	fmt.Printf("Type change: selfType=%d isHost=%v\n", dc.selfType, dc.isHost)
}

func (dc *DuelClient) handleSTOCDuelStart() {
	MainGame.DField.Clear()
	MainGame.DInfo.IsStarted = true
	MainGame.DInfo.IsFinished = false
	MainGame.DInfo.LP[0] = 0
	MainGame.DInfo.LP[1] = 0
	MainGame.DInfo.Turn = 0
	MainGame.DInfo.TimePlayer = 2
	MainGame.DInfo.IsReplaySwapped = false
	MainGame.IsBuilding = false
	PushScene("duelField")
}

func (dc *DuelClient) handleSTOCDuelEnd() {
	MainGame.DInfo.IsStarted = false
	MainGame.DInfo.IsInDuel = false
	MainGame.DInfo.IsFinished = true
	MainGame.IsBuilding = false
	MainGame.DField.Clear()
	PushScene("lanWindow")
}

func (dc *DuelClient) handleSTOCTimeLimit(payload []byte) {
	if len(payload) < 4 {
		return
	}
	player := payload[0]
	leftTime := binary.LittleEndian.Uint16(payload[2:])
	lplayer := MainGame.LocalPlayer(int(player))
	if lplayer == 0 {
		dc.SendPacketToServer(network.CTOS_TIME_CONFIRM)
	}
	MainGame.DInfo.TimePlayer = uint8(lplayer)
	MainGame.DInfo.TimeLeft[lplayer] = leftTime
}

func (dc *DuelClient) handleSTOCChat(payload []byte) {
	if len(payload) < 4 {
		return
	}
	chatPlayerType := binary.LittleEndian.Uint16(payload)
	msgBytes := payload[2:]
	msg := Utf16ToString(msgBytes)
	player := int(chatPlayerType)
	MainGame.AddChatMsg(msg, player, false)
}

func (dc *DuelClient) handleSTOCHsPlayerEnter(payload []byte) {
	if len(payload) < 41 {
		return
	}
	var pkt protocol.STOCHsPlayerEnter
	for i := 0; i < 20 && i*2+1 < len(payload); i++ {
		pkt.Name[i] = uint16(payload[i*2]) | uint16(payload[i*2+1])<<8
	}
	pkt.Pos = payload[40]
	if pkt.Pos > 3 {
		return
	}
	name := Utf16ToString(payload[:40])
	fmt.Printf("Player entered pos %d: %s\n", pkt.Pos, name)

	// Update lobby scene state via MainGame
	MainGame.SetHostPrepName(int(pkt.Pos), name)
}

func (dc *DuelClient) handleSTOCHsPlayerChange(payload []byte) {
	if len(payload) < 1 {
		return
	}
	status := payload[0]
	pos := (status >> 4) & 0xf
	state := status & 0xf
	if pos > 3 {
		return
	}

	switch state {
	case network.PLAYERCHANGE_OBSERVE:
		dc.watching++
		MainGame.SetHostPrepName(int(pos), "")
	case network.PLAYERCHANGE_READY:
		MainGame.SetHostPrepReady(int(pos), true)
	case network.PLAYERCHANGE_NOTREADY:
		MainGame.SetHostPrepReady(int(pos), false)
	case network.PLAYERCHANGE_LEAVE:
		MainGame.SetHostPrepName(int(pos), "")
		MainGame.SetHostPrepReady(int(pos), false)
	default:
		// Player moved to another position (< 8)
		MainGame.SetHostPrepName(int(pos), "")
		MainGame.SetHostPrepReady(int(pos), false)
	}
}

func (dc *DuelClient) handleSTOCHsWatchChange(payload []byte) {
	if len(payload) < 2 {
		return
	}
	dc.watching = uint32(binary.LittleEndian.Uint16(payload))
	MainGame.ObserverCount = int(dc.watching)
}

// Utf16ToString decodes a UTF-16 LE byte slice to a Go string.
func Utf16ToString(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	var runes []rune
	for i := 0; i < len(data); i += 2 {
		r := uint16(data[i]) | uint16(data[i+1])<<8
		if r == 0 {
			break
		}
		runes = append(runes, rune(r))
	}
	return string(runes)
}
