package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// WailsDuelClient manages the TCP connection to YGOPro duel server
// and translates network packets to frontend events.
type WailsDuelClient struct {
	mu         sync.Mutex
	conn       net.Conn
	ctx        context.Context
	emitFunc   func(eventName string, optionalData ...interface{})
	isConnected bool
	playerType uint8
	running    bool
	stopChan   chan struct{}
}

func NewWailsDuelClient(emitFunc func(eventName string, optionalData ...interface{})) *WailsDuelClient {
	return &WailsDuelClient{
		emitFunc: emitFunc,
		stopChan: make(chan struct{}),
	}
}

func (c *WailsDuelClient) SetContext(ctx context.Context) {
	c.ctx = ctx
}

func (c *WailsDuelClient) Connect(addr string, username string, pass string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isConnected && c.conn != nil {
		c.conn.Close()
	}

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	c.conn = conn
	c.isConnected = true
	c.running = true
	c.stopChan = make(chan struct{})

	// Start background read loop
	go c.readLoop()

	// Send PlayerInfo packet
	c.sendPlayerInfo(username)
	return nil
}

func (c *WailsDuelClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected {
		return
	}
	c.running = false
	c.isConnected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	select {
	case <-c.stopChan:
	default:
		close(c.stopChan)
	}
	c.emit("stoc:disconnected", map[string]interface{}{})
}

func (c *WailsDuelClient) emit(eventName string, data interface{}) {
	if c.emitFunc != nil {
		c.emitFunc(eventName, data)
	}
}

// ------------------------------------------------------------------
// Outgoing CTOS Packets
// ------------------------------------------------------------------

func (c *WailsDuelClient) sendPacket(proto byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected || c.conn == nil {
		return fmt.Errorf("not connected")
	}

	pktLen := uint16(1 + len(payload))
	buf := make([]byte, 2+int(pktLen))
	binary.LittleEndian.PutUint16(buf[0:2], pktLen)
	buf[2] = proto
	if len(payload) > 0 {
		copy(buf[3:], payload)
	}

	_, err := c.conn.Write(buf)
	return err
}

func (c *WailsDuelClient) sendPlayerInfo(name string) {
	var pkt protocol.CTOSPlayerInfo
	encoded := utf16.Encode([]rune(name))
	for i := 0; i < len(encoded) && i < 19; i++ {
		pkt.Name[i] = encoded[i]
	}
	pkt.Name[19] = 0
	data, _ := restruct.Pack(binary.LittleEndian, &pkt)
	_ = c.sendPacket(network.CTOS_PLAYER_INFO, data)
}

func (c *WailsDuelClient) CreateGame(hostInfo protocol.HostInfo, roomName string, pass string) error {
	var pkt protocol.CTOSCreateGame
	pkt.Info = hostInfo
	encodedName := utf16.Encode([]rune(roomName))
	for i := 0; i < len(encodedName) && i < 19; i++ {
		pkt.Name[i] = encodedName[i]
	}
	encodedPass := utf16.Encode([]rune(pass))
	for i := 0; i < len(encodedPass) && i < 19; i++ {
		pkt.Pass[i] = encodedPass[i]
	}
	data, err := restruct.Pack(binary.LittleEndian, &pkt)
	if err != nil {
		return err
	}
	return c.sendPacket(network.CTOS_CREATE_GAME, data)
}

func (c *WailsDuelClient) JoinGame(version uint16, gameID uint32, pass string) error {
	var pkt protocol.CTOSJoinGame
	pkt.Version = version
	pkt.GameID = gameID
	encodedPass := utf16.Encode([]rune(pass))
	for i := 0; i < len(encodedPass) && i < 19; i++ {
		pkt.Pass[i] = encodedPass[i]
	}
	data, err := restruct.Pack(binary.LittleEndian, &pkt)
	if err != nil {
		return err
	}
	return c.sendPacket(network.CTOS_JOIN_GAME, data)
}

func (c *WailsDuelClient) LeaveGame() error {
	return c.sendPacket(network.CTOS_LEAVE_GAME, nil)
}

func (c *WailsDuelClient) Surrender() error {
	return c.sendPacket(network.CTOS_SURRENDER, nil)
}

func (c *WailsDuelClient) SetReady(isReady bool) error {
	if isReady {
		return c.sendPacket(network.CTOS_HS_READY, nil)
	}
	return c.sendPacket(network.CTOS_HS_NOTREADY, nil)
}

func (c *WailsDuelClient) StartDuel() error {
	return c.sendPacket(network.CTOS_HS_START, nil)
}

func (c *WailsDuelClient) ToObserver() error {
	return c.sendPacket(network.CTOS_HS_TOOBSERVER, nil)
}

func (c *WailsDuelClient) ToDuelist() error {
	return c.sendPacket(network.CTOS_HS_TODUELIST, nil)
}

func (c *WailsDuelClient) SendChat(msg string) error {
	encoded := utf16.Encode([]rune(msg))
	if len(encoded) > 255 {
		encoded = encoded[:255]
	}
	buf := make([]byte, (len(encoded)+1)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	binary.LittleEndian.PutUint16(buf[len(encoded)*2:], 0)
	return c.sendPacket(network.CTOS_CHAT, buf)
}

func (c *WailsDuelClient) SendHandResult(res byte) error {
	var pkt protocol.CTOSHandResult
	pkt.Res = res
	data, _ := restruct.Pack(binary.LittleEndian, &pkt)
	return c.sendPacket(network.CTOS_HAND_RESULT, data)
}

func (c *WailsDuelClient) SendTPResult(res byte) error {
	var pkt protocol.CTOSTPResult
	pkt.Res = res
	data, _ := restruct.Pack(binary.LittleEndian, &pkt)
	return c.sendPacket(network.CTOS_TP_RESULT, data)
}

func (c *WailsDuelClient) SendResponseI(val int32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(val))
	return c.sendPacket(network.CTOS_RESPONSE, buf)
}

func (c *WailsDuelClient) SendResponseB(data []byte) error {
	return c.sendPacket(network.CTOS_RESPONSE, data)
}

func (c *WailsDuelClient) SendTimeConfirm() error {
	return c.sendPacket(network.CTOS_TIME_CONFIRM, nil)
}

func (c *WailsDuelClient) UpdateDeck(mainList []uint32, sideList []uint32) error {
	var deckData protocol.CTOSDeckData
	deckData.MainC = int32(len(mainList))
	deckData.SideC = int32(len(sideList))
	deckData.List = append(deckData.List, mainList...)
	deckData.List = append(deckData.List, sideList...)
	buf := make([]byte, deckData.SizeOf())
	data, err := deckData.Pack(buf, binary.LittleEndian)
	if err != nil {
		return err
	}
	return c.sendPacket(network.CTOS_UPDATE_DECK, data)
}

// ------------------------------------------------------------------
// Incoming STOC Packet Processing Loop
// ------------------------------------------------------------------

func (c *WailsDuelClient) readLoop() {
	header := make([]byte, 2)
	for c.running {
		// 1. Read 2-byte packet length
		if _, err := io.ReadFull(c.conn, header); err != nil {
			if c.running {
				log.Printf("[WailsDuelClient] Read header error: %v", err)
			}
			break
		}

		packetLen := binary.LittleEndian.Uint16(header)
		if packetLen == 0 {
			continue
		}

		// 2. Read packet body (1 byte proto + payload)
		body := make([]byte, packetLen)
		if _, err := io.ReadFull(c.conn, body); err != nil {
			if c.running {
				log.Printf("[WailsDuelClient] Read body error: %v", err)
			}
			break
		}

		proto := body[0]
		payload := body[1:]
		c.handleSTOCPacket(proto, payload)
	}
	c.Disconnect()
}

func (c *WailsDuelClient) handleSTOCPacket(proto byte, payload []byte) {
	switch proto {
	case network.STOC_JOIN_GAME:
		var pkt protocol.STOCJoinGame
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		c.emit("stoc:join_game", pkt)

	case network.STOC_TYPE_CHANGE:
		var pkt protocol.STOCTypeChange
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		c.playerType = pkt.Type & 0x0f
		c.emit("stoc:type_change", map[string]interface{}{
			"type":   pkt.Type,
			"isHost": (pkt.Type & 0x10) != 0,
			"pos":    pkt.Type & 0x0f,
		})

	case network.STOC_HS_PLAYER_ENTER:
		var pkt protocol.STOCHsPlayerEnter
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		name := string(utf16.Decode(pkt.Name[:]))
		c.emit("stoc:player_enter", map[string]interface{}{
			"pos":  pkt.Pos,
			"name": name,
		})

	case network.STOC_HS_PLAYER_CHANGE:
		var pkt protocol.STOCHsPlayerChange
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		pos := (pkt.Status >> 4) & 0x0f
		state := pkt.Status & 0x0f
		c.emit("stoc:player_change", map[string]interface{}{
			"pos":    pos,
			"status": state,
			"ready":  state == network.PLAYERCHANGE_READY,
		})

	case network.STOC_HS_WATCH_CHANGE:
		var pkt protocol.STOCHsWatchChange
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		c.emit("stoc:watch_change", map[string]interface{}{
			"count": pkt.WatchCount,
		})

	case network.STOC_DUEL_START:
		c.emit("stoc:duel_start", map[string]interface{}{})

	case network.STOC_DECK_COUNT:
		if len(payload) >= 12 {
			c.emit("stoc:deck_count", map[string]interface{}{
				"deck0":  int16(binary.LittleEndian.Uint16(payload[0:2])),
				"extra0": int16(binary.LittleEndian.Uint16(payload[2:4])),
				"side0":  int16(binary.LittleEndian.Uint16(payload[4:6])),
				"deck1":  int16(binary.LittleEndian.Uint16(payload[6:8])),
				"extra1": int16(binary.LittleEndian.Uint16(payload[8:10])),
				"side1":  int16(binary.LittleEndian.Uint16(payload[10:12])),
			})
		}

	case network.STOC_SELECT_HAND:
		c.emit("stoc:select_hand", map[string]interface{}{})

	case network.STOC_HAND_RESULT:
		var pkt protocol.STOCHandResult
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		c.emit("stoc:hand_result", map[string]interface{}{
			"res1": pkt.Res1,
			"res2": pkt.Res2,
		})

	case network.STOC_SELECT_TP:
		c.emit("stoc:select_tp", map[string]interface{}{})

	case network.STOC_TIME_LIMIT:
		var pkt protocol.STOCTimeLimit
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		c.emit("stoc:time_limit", map[string]interface{}{
			"player":   pkt.Player,
			"leftTime": pkt.LeftTime,
		})

	case network.STOC_CHAT:
		if len(payload) >= 2 {
			player := binary.LittleEndian.Uint16(payload[0:2])
			msgRunes := make([]uint16, (len(payload)-2)/2)
			for i := range msgRunes {
				msgRunes[i] = binary.LittleEndian.Uint16(payload[2+i*2 : 4+i*2])
			}
			msg := string(utf16.Decode(msgRunes))
			c.emit("stoc:chat", map[string]interface{}{
				"player": player,
				"msg":    msg,
			})
		}

	case network.STOC_ERROR_MSG:
		var pkt protocol.STOCErrorMsg
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		c.emit("stoc:error_msg", map[string]interface{}{
			"msg":  pkt.Msg,
			"code": pkt.Code,
		})

	case network.STOC_DUEL_END:
		c.emit("stoc:duel_end", map[string]interface{}{})

	case network.STOC_CHANGE_SIDE:
		c.emit("stoc:change_side", map[string]interface{}{})

	case network.STOC_WAITING_SIDE:
		c.emit("stoc:waiting_side", map[string]interface{}{})

	case network.STOC_GAME_MSG:
		c.handleGameMessage(payload)
	}
}

// handleGameMessage parses YGOPRO engine MSG_* stream into structured 3D client events
func (c *WailsDuelClient) handleGameMessage(msgBuffer []byte) {
	pbuf := utils.NewYGOBuffer(msgBuffer, binary.LittleEndian)

	for pbuf.Len() > 0 {
		var engType uint8
		if err := pbuf.Read(&engType); err != nil {
			break
		}

		switch engType {
		case ocgcore.MSG_START:
			var (
				playertype uint8
				duelrule   uint8
				lp0, lp1   int32
				deck0, extra0, deck1, extra1 uint16
			)
			_ = pbuf.Read(&playertype, &duelrule, &lp0, &lp1, &deck0, &extra0, &deck1, &extra1)
			c.emit("duel:start", map[string]interface{}{
				"playerType": playertype,
				"duelRule":   duelrule,
				"lp0":        lp0,
				"lp1":        lp1,
				"deck0":      deck0,
				"extra0":     extra0,
				"deck1":      deck1,
				"extra1":     extra1,
			})

		case ocgcore.MSG_DRAW:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			cards := make([]int32, count)
			for i := 0; i < int(count); i++ {
				_ = pbuf.Read(&cards[i])
			}
			c.emit("duel:draw", map[string]interface{}{
				"player": player,
				"count":  count,
				"cards":  cards,
			})

		case ocgcore.MSG_NEW_TURN:
			var player uint8
			_ = pbuf.Read(&player)
			c.emit("duel:new_turn", map[string]interface{}{
				"player": player,
			})

		case ocgcore.MSG_NEW_PHASE:
			var phase uint16
			_ = pbuf.Read(&phase)
			c.emit("duel:new_phase", map[string]interface{}{
				"phase": phase,
			})

		case ocgcore.MSG_MOVE:
			var msg protocol.MoveMsg
			_ = pbuf.Unpack(&msg)
			c.emit("duel:move", map[string]interface{}{
				"code":   msg.Code,
				"pc":     msg.PC,
				"pl":     msg.PL,
				"ps":     msg.PS,
				"pp":     msg.PP,
				"cc":     msg.CC,
				"cl":     msg.CL,
				"cs":     msg.CS,
				"cp":     msg.CP,
				"reason": msg.Reason,
			})

		case ocgcore.MSG_POS_CHANGE:
			var (
				code uint32
				cc, cl, cs, pp, cp uint8
			)
			_ = pbuf.Read(&code, &cc, &cl, &cs, &pp, &cp)
			c.emit("duel:pos_change", map[string]interface{}{
				"code": code,
				"cc":   cc,
				"cl":   cl,
				"cs":   cs,
				"pp":   pp,
				"cp":   cp,
			})

		case ocgcore.MSG_SET:
			var (
				code uint32
				cc, cl, cs, cp uint8
			)
			_ = pbuf.Read(&code, &cc, &cl, &cs, &cp)
			c.emit("duel:set", map[string]interface{}{
				"code": code,
				"cc":   cc,
				"cl":   cl,
				"cs":   cs,
				"cp":   cp,
			})

		case ocgcore.MSG_SUMMONING:
			var (
				code uint32
				cc, cl, cs, cp uint8
			)
			_ = pbuf.Read(&code, &cc, &cl, &cs, &cp)
			c.emit("duel:summoning", map[string]interface{}{
				"code": code,
				"cc":   cc,
				"cl":   cl,
				"cs":   cs,
				"cp":   cp,
			})

		case ocgcore.MSG_SUMMONED:
			c.emit("duel:summoned", map[string]interface{}{})

		case ocgcore.MSG_SPSUMMONING:
			var (
				code uint32
				cc, cl, cs, cp uint8
			)
			_ = pbuf.Read(&code, &cc, &cl, &cs, &cp)
			c.emit("duel:spsummoning", map[string]interface{}{
				"code": code,
				"cc":   cc,
				"cl":   cl,
				"cs":   cs,
				"cp":   cp,
			})

		case ocgcore.MSG_SPSUMMONED:
			c.emit("duel:spsummoned", map[string]interface{}{})

		case ocgcore.MSG_FLIPSUMMONING:
			var (
				code uint32
				cc, cl, cs, cp uint8
			)
			_ = pbuf.Read(&code, &cc, &cl, &cs, &cp)
			c.emit("duel:flipsummoning", map[string]interface{}{
				"code": code,
				"cc":   cc,
				"cl":   cl,
				"cs":   cs,
				"cp":   cp,
			})

		case ocgcore.MSG_FLIPSUMMONED:
			c.emit("duel:flipsummoned", map[string]interface{}{})

		case ocgcore.MSG_CHAINING:
			var (
				code uint32
				p1, p2, p3, cc, cl, cs, cp uint8
			)
			_ = pbuf.Read(&code, &p1, &p2, &p3, &cc, &cl, &cs, &cp)
			_ = pbuf.Next(5)
			c.emit("duel:chaining", map[string]interface{}{
				"code": code,
				"cc":   cc,
				"cl":   cl,
				"cs":   cs,
				"cp":   cp,
			})

		case ocgcore.MSG_CHAINED:
			var count uint8
			_ = pbuf.Read(&count)
			c.emit("duel:chained", map[string]interface{}{"count": count})

		case ocgcore.MSG_CHAIN_SOLVING:
			var count uint8
			_ = pbuf.Read(&count)
			c.emit("duel:chain_solving", map[string]interface{}{"count": count})

		case ocgcore.MSG_CHAIN_SOLVED:
			var count uint8
			_ = pbuf.Read(&count)
			c.emit("duel:chain_solved", map[string]interface{}{"count": count})

		case ocgcore.MSG_CHAIN_END:
			c.emit("duel:chain_end", map[string]interface{}{})

		case ocgcore.MSG_DAMAGE:
			var (
				player uint8
				amount int32
			)
			_ = pbuf.Read(&player, &amount)
			c.emit("duel:damage", map[string]interface{}{
				"player": player,
				"amount": amount,
			})

		case ocgcore.MSG_RECOVER:
			var (
				player uint8
				amount int32
			)
			_ = pbuf.Read(&player, &amount)
			c.emit("duel:recover", map[string]interface{}{
				"player": player,
				"amount": amount,
			})

		case ocgcore.MSG_LPUPDATE:
			var (
				player uint8
				lp     int32
			)
			_ = pbuf.Read(&player, &lp)
			c.emit("duel:lp_update", map[string]interface{}{
				"player": player,
				"lp":     lp,
			})

		case ocgcore.MSG_ATTACK:
			var (
				attackerCC, attackerCL, attackerCS, attackerCP uint8
				targetCC, targetCL, targetCS, targetCP         uint8
			)
			_ = pbuf.Read(&attackerCC, &attackerCL, &attackerCS, &attackerCP,
				&targetCC, &targetCL, &targetCS, &targetCP)
			c.emit("duel:attack", map[string]interface{}{
				"attacker": map[string]interface{}{"c": attackerCC, "l": attackerCL, "s": attackerCS},
				"target":   map[string]interface{}{"c": targetCC, "l": targetCL, "s": targetCS},
			})

		case ocgcore.MSG_BATTLE:
			_ = pbuf.Next(26)
			c.emit("duel:battle", map[string]interface{}{})

		case ocgcore.MSG_WIN:
			var (
				player uint8
				typ    uint8
			)
			_ = pbuf.Read(&player, &typ)
			c.emit("duel:win", map[string]interface{}{
				"winner": player,
				"type":   typ,
			})

		case ocgcore.MSG_SELECT_IDLECMD:
			c.parseSelectIdleCmd(pbuf)

		case ocgcore.MSG_SELECT_BATTLECMD:
			c.parseSelectBattleCmd(pbuf)

		case ocgcore.MSG_SELECT_EFFECTYN:
			var (
				player uint8
				code   uint32
				loc    uint32
				desc   uint32
			)
			_ = pbuf.Read(&player, &code, &loc, &desc)
			c.emit("duel:select_effectyn", map[string]interface{}{
				"player": player,
				"code":   code,
				"loc":    loc,
				"desc":   desc,
			})

		case ocgcore.MSG_SELECT_YESNO:
			var (
				player uint8
				desc   uint32
			)
			_ = pbuf.Read(&player, &desc)
			c.emit("duel:select_yesno", map[string]interface{}{
				"player": player,
				"desc":   desc,
			})

		case ocgcore.MSG_SELECT_OPTION:
			var (
				player uint8
				count  uint8
			)
			_ = pbuf.Read(&player, &count)
			opts := make([]int32, count)
			for i := 0; i < int(count); i++ {
				_ = pbuf.Read(&opts[i])
			}
			c.emit("duel:select_option", map[string]interface{}{
				"player":  player,
				"options": opts,
			})

		case ocgcore.MSG_SELECT_CARD, ocgcore.MSG_SELECT_TRIBUTE:
			var (
				player uint8
				cancelable uint8
				min, max uint8
				count uint8
			)
			_ = pbuf.Read(&player, &cancelable, &min, &max, &count)
			cards := make([]map[string]interface{}, count)
			for i := 0; i < int(count); i++ {
				var code int32
				var c, l, s, p uint8
				_ = pbuf.Read(&code, &c, &l, &s, &p)
				cards[i] = map[string]interface{}{
					"code": code,
					"c":    c,
					"l":    l,
					"s":    s,
					"p":    p,
				}
			}
			c.emit("duel:select_card", map[string]interface{}{
				"player":     player,
				"cancelable": cancelable != 0,
				"min":        min,
				"max":        max,
				"cards":      cards,
			})

		case ocgcore.MSG_SELECT_POSITION:
			var (
				player    uint8
				code      uint32
				positions uint8
			)
			_ = pbuf.Read(&player, &code, &positions)
			c.emit("duel:select_position", map[string]interface{}{
				"player":    player,
				"code":      code,
				"positions": positions,
			})

		case ocgcore.MSG_SELECT_PLACE, ocgcore.MSG_SELECT_DISFIELD:
			var (
				player uint8
				count  uint8
				flag   uint32
			)
			_ = pbuf.Read(&player, &count, &flag)
			c.emit("duel:select_place", map[string]interface{}{
				"player": player,
				"count":  count,
				"flag":   flag,
			})

		case ocgcore.MSG_SELECT_CHAIN:
			var (
				player uint8
				count  uint8
				spec   uint8
				forced uint8
				hint0, hint1 uint32
			)
			_ = pbuf.Read(&player, &count, &spec, &forced, &hint0, &hint1)
			chains := make([]map[string]interface{}, count)
			for i := 0; i < int(count); i++ {
				var flag uint8
				var code uint32
				var cc, cl, cs, cp uint8
				var desc uint32
				_ = pbuf.Read(&flag, &code, &cc, &cl, &cs, &cp, &desc)
				chains[i] = map[string]interface{}{
					"flag": flag,
					"code": code,
					"cc":   cc,
					"cl":   cl,
					"cs":   cs,
					"cp":   cp,
					"desc": desc,
				}
			}
			c.emit("duel:select_chain", map[string]interface{}{
				"player": player,
				"count":  count,
				"chains": chains,
			})

		case ocgcore.MSG_UPDATE_DATA:
			var (
				player   uint8
				location uint8
			)
			_ = pbuf.Read(&player, &location)
			// Read the rest of buffer as raw update data
			raw := pbuf.Bytes()
			c.emit("duel:update_data", map[string]interface{}{
				"player":   player,
				"location": location,
				"data":     raw,
			})
			return

		case ocgcore.MSG_UPDATE_CARD:
			var (
				player, location, sequence uint8
			)
			_ = pbuf.Read(&player, &location, &sequence)
			raw := pbuf.Bytes()
			c.emit("duel:update_card", map[string]interface{}{
				"player":   player,
				"location": location,
				"sequence": sequence,
				"data":     raw,
			})
			return

		case ocgcore.MSG_WAITING:
			c.emit("duel:waiting", map[string]interface{}{})
		}
	}
}

func (c *WailsDuelClient) parseSelectIdleCmd(pbuf *utils.YGOBuffer) {
	var player uint8
	var count uint8
	_ = pbuf.Read(&player)

	// Summonable
	_ = pbuf.Read(&count)
	summon := make([]map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		var code uint32
		var c, l, s uint8
		_ = pbuf.Read(&code, &c, &l, &s)
		summon[i] = map[string]interface{}{"code": code, "c": c, "l": l, "s": s, "idx": i}
	}

	// Special summonable
	_ = pbuf.Read(&count)
	spsummon := make([]map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		var code uint32
		var c, l, s uint8
		_ = pbuf.Read(&code, &c, &l, &s)
		spsummon[i] = map[string]interface{}{"code": code, "c": c, "l": l, "s": s, "idx": i}
	}

	// Reposable
	_ = pbuf.Read(&count)
	repos := make([]map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		var code uint32
		var c, l, s uint8
		_ = pbuf.Read(&code, &c, &l, &s)
		repos[i] = map[string]interface{}{"code": code, "c": c, "l": l, "s": s, "idx": i}
	}

	// Monster setable
	_ = pbuf.Read(&count)
	mset := make([]map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		var code uint32
		var c, l, s uint8
		_ = pbuf.Read(&code, &c, &l, &s)
		mset[i] = map[string]interface{}{"code": code, "c": c, "l": l, "s": s, "idx": i}
	}

	// Spell/trap setable
	_ = pbuf.Read(&count)
	sset := make([]map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		var code uint32
		var c, l, s uint8
		_ = pbuf.Read(&code, &c, &l, &s)
		sset[i] = map[string]interface{}{"code": code, "c": c, "l": l, "s": s, "idx": i}
	}

	// Activatable
	_ = pbuf.Read(&count)
	activate := make([]map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		var code uint32
		var c, l, s uint8
		var desc uint32
		_ = pbuf.Read(&code, &c, &l, &s, &desc)
		activate[i] = map[string]interface{}{"code": code, "c": c, "l": l, "s": s, "desc": desc, "idx": i}
	}

	var toBP, toEP, shuffle uint8
	_ = pbuf.Read(&toBP, &toEP, &shuffle)

	c.emit("duel:select_idlecmd", map[string]interface{}{
		"player":   player,
		"summon":   summon,
		"spsummon": spsummon,
		"repos":    repos,
		"mset":     mset,
		"sset":     sset,
		"activate": activate,
		"toBP":     toBP != 0,
		"toEP":     toEP != 0,
		"shuffle":  shuffle != 0,
	})
}

func (c *WailsDuelClient) parseSelectBattleCmd(pbuf *utils.YGOBuffer) {
	var player uint8
	var count uint8
	_ = pbuf.Read(&player)

	// Activatable
	_ = pbuf.Read(&count)
	activate := make([]map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		var code uint32
		var c, l, s uint8
		var desc uint32
		_ = pbuf.Read(&code, &c, &l, &s, &desc)
		activate[i] = map[string]interface{}{"code": code, "c": c, "l": l, "s": s, "desc": desc, "idx": i}
	}

	// Attackable
	_ = pbuf.Read(&count)
	attack := make([]map[string]interface{}, count)
	for i := 0; i < int(count); i++ {
		var code uint32
		var c, l, s, diratt uint8
		_ = pbuf.Read(&code, &c, &l, &s, &diratt)
		attack[i] = map[string]interface{}{"code": code, "c": c, "l": l, "s": s, "diratt": diratt != 0, "idx": i}
	}

	var toM2, toEP uint8
	_ = pbuf.Read(&toM2, &toEP)

	c.emit("duel:select_battlecmd", map[string]interface{}{
		"player":   player,
		"activate": activate,
		"attack":   attack,
		"toM2":     toM2 != 0,
		"toEP":     toEP != 0,
	})
}
