package client

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// DuelClient corresponds to C++ class DuelClient in duelclient.h
// Manages the TCP connection to a YGOPro server and handles STOC packets.
type DuelClient struct {
	mu sync.RWMutex

	conn       net.Conn
	connected  atomic.Bool
	isHost     bool
	selfType   uint8
	watching   uint32
	selectHint int
	selectUnselectHint int
	lastSelectHint     int
	isClosing  bool
	isSwapping bool
	isRefreshing bool
	matchKill  int

	responseBuf [256]byte
	responseLen int

	hosts []protocol.HostPacket

	// Channels for UI communication
	stocChan   chan []byte
	msgChan    chan []byte
	closeChan  chan struct{}
}

var Client = &DuelClient{
	stocChan:  make(chan []byte, 64),
	msgChan:   make(chan []byte, 64),
	closeChan: make(chan struct{}),
}

// StartClient connects to a YGOPro server.
func (dc *DuelClient) StartClient(addr string, port uint16, createGame bool) bool {
	if dc.connected.Load() {
		return false
	}
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		fmt.Println("StartClient: connection failed:", err)
		return false
	}
	dc.conn = conn
	dc.connected.Store(true)
	dc.isClosing = false

	// Send player info
	dc.SendPlayerInfo(MainGame.Config.Nickname)

	if createGame {
		dc.SendCreateGame()
	} else {
		dc.SendJoinGame()
	}

	go dc.readLoop()
	go dc.msgLoop()
	return true
}

func (dc *DuelClient) StopClient() {
	if !dc.connected.Load() {
		return
	}
	dc.isClosing = true
	dc.connected.Store(false)
	if dc.conn != nil {
		dc.conn.Close()
	}
	close(dc.closeChan)
}

func (dc *DuelClient) msgLoop() {
	for dc.connected.Load() {
		select {
		case msg := <-dc.msgChan:
			if dc.connected.Load() {
				dc.ClientAnalyze(msg)
			}
		case <-dc.closeChan:
			return
		}
	}
}

func (dc *DuelClient) readLoop() {
	buf := make([]byte, 4096)
	for dc.connected.Load() {
		n, err := dc.conn.Read(buf)
		if err != nil {
			if !dc.isClosing {
				fmt.Println("Read error:", err)
			}
			break
		}
		if n > 0 {
			dc.handlePacket(buf[:n])
		}
	}
	dc.connected.Store(false)
}

func (dc *DuelClient) handlePacket(data []byte) {
	// YGOPro packet format: [2 bytes len] [1 byte type] [payload]
	// Multiple packets may be in one read.
	offset := 0
	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}
		pktLen := int(binary.BigEndian.Uint16(data[offset:offset+2]))
		if offset+2+pktLen > len(data) {
			break
		}
		pktType := data[offset+2]
		payload := data[offset+3 : offset+2+pktLen]
		offset += 2 + pktLen

		switch pktType {
		case network.STOC_GAME_MSG:
			// Game messages are forwarded to ClientAnalyze
			select {
			case dc.msgChan <- payload:
			default:
			}
		default:
			// Non-game STOC messages
			dc.HandleSTOCPacket(append([]byte{pktType}, payload...))
		}
	}
}

// SendPacketToServer sends a packet with just the proto byte.
func (dc *DuelClient) SendPacketToServer(proto uint8) {
	buf := make([]byte, 3)
	binary.BigEndian.PutUint16(buf, 1)
	buf[2] = proto
	dc.send(buf)
}

// SendBufferToServer sends a packet with payload.
func (dc *DuelClient) SendBufferToServer(proto uint8, payload []byte) {
	if len(payload) > 0x7FFF {
		payload = payload[:0x7FFF]
	}
	buf := make([]byte, 3+len(payload))
	binary.BigEndian.PutUint16(buf, uint16(1+len(payload)))
	buf[2] = proto
	copy(buf[3:], payload)
	dc.send(buf)
}

func (dc *DuelClient) send(buf []byte) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if dc.conn != nil {
		dc.conn.Write(buf)
	}
}

func (dc *DuelClient) SendPlayerInfo(name string) {
	var pkt protocol.CTOSPlayerInfo
	copy(pkt.Name[:], StringToUtf16(name, 20))
	dc.SendStruct(network.CTOS_PLAYER_INFO, pkt)
}

func (dc *DuelClient) SendCreateGame() {
	var pkt protocol.CTOSCreateGame
	copy(pkt.Name[:], StringToUtf16(MainGame.Config.GameName, 20))
	copy(pkt.Pass[:], StringToUtf16(MainGame.Config.RoomPass, 20))
	pkt.Info = MainGame.HostInfo
	dc.SendStruct(network.CTOS_CREATE_GAME, pkt)
}

func (dc *DuelClient) SendJoinGame() {
	var pkt protocol.CTOSJoinGame
	pkt.Version = network.NETWORK_CLIENT_ID
	pkt.GameID = 0
	copy(pkt.Pass[:], StringToUtf16(MainGame.Config.RoomPass, 20))
	dc.SendStruct(network.CTOS_JOIN_GAME, pkt)
}

func (dc *DuelClient) SendStruct(proto uint8, v interface{}) {
	data := protocol.PackGameMsg(v)
	dc.SendBufferToServer(proto, data)
}

func (dc *DuelClient) SendResponse() {
	if dc.responseLen > 0 {
		dc.SendBufferToServer(network.CTOS_RESPONSE, dc.responseBuf[:dc.responseLen])
	}
}

func (dc *DuelClient) SetResponseI(resp int32) {
	binary.BigEndian.PutUint32(dc.responseBuf[:4], uint32(resp))
	dc.responseLen = 4
}

func (dc *DuelClient) SetResponseB(resp []byte) {
	dc.responseLen = copy(dc.responseBuf[:], resp)
}



func (dc *DuelClient) SendChat(msg string) {
	// CTOS_CHAT format: uint16_t array (player type + message UTF-16 LE)
	// Max message length: 256 uint16_t units
	maxLen := 256
	runes := []rune(msg)
	if len(runes) > maxLen {
		runes = runes[:maxLen]
	}
	data := make([]byte, 2+len(runes)*2)
	// player type = selfType
	binary.LittleEndian.PutUint16(data, uint16(dc.selfType))
	for i, r := range runes {
		binary.LittleEndian.PutUint16(data[2+i*2:], uint16(r))
	}
	dc.SendBufferToServer(network.CTOS_CHAT, data)
}

// DiscoverHosts sends a UDP broadcast to discover YGOPro hosts on the LAN.
// Returns a list of HostPacket responses received within the timeout.
func (dc *DuelClient) DiscoverHosts() []protocol.HostPacket {
	addr, err := net.ResolveUDPAddr("udp", "255.255.255.255:7911")
	if err != nil {
		return nil
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil
	}
	defer conn.Close()

	req := protocol.HostRequest{Identifier: network.NETWORK_SERVER_ID}
	_, err = conn.Write(protocol.PackGameMsg(req))
	if err != nil {
		return nil
	}

	// Listen for replies
	listenAddr, _ := net.ResolveUDPAddr("udp", ":7911")
	listenConn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return nil
	}
	defer listenConn.Close()
	listenConn.SetReadDeadline(time.Now().Add(1 * time.Second))

	var hosts []protocol.HostPacket
	buf := make([]byte, 256)
	for {
		n, _, err := listenConn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if n < 8 {
			continue
		}
		// Quick check: first 2 bytes should be NETWORK_SERVER_ID
		if binary.LittleEndian.Uint16(buf) != network.NETWORK_SERVER_ID {
			continue
		}
		var pkt protocol.HostPacket
		if protocol.UnpackGameMsg(buf[:n], &pkt) == nil {
			hosts = append(hosts, pkt)
		}
	}
	return hosts
}

func StringToUtf16(s string, maxLen int) []uint16 {
	runes := []rune(s)
	res := make([]uint16, maxLen)
	for i := 0; i < len(runes) && i < maxLen; i++ {
		res[i] = uint16(runes[i])
	}
	return res
}

// SendUpdateDeck 把当前卡组发给服务器。对应 C++ DuelClient::SendUpdateDeck。
//
// 这是联机的必经一步，此前完全没有实现：服务器在两处等这个包 ——
// 进房间准备时（core/duel 会 LoadDeck + CheckDeck），以及 match 换副卡组后
// （STOC_CHANGE_SIDE 之后玩家 state 被设成 CTOS_UPDATE_DECK，走 LoadSide 校验）。
// 不发的话，前者点了准备也开不了局，后者第二局直接卡死。
//
// 线上格式：mainc(int32) = 主卡组+额外卡组张数，sidec(int32) = 副卡组张数，
// 之后是三段卡号（主、额外、副）依次排开 —— 额外卡组并进 mainc 里，
// 服务端靠卡片类型再把它们拆回去（见 core/duel 的 LoadDeck）。
func (dc *DuelClient) SendUpdateDeck(deck *Deck) {
	if deck == nil {
		return
	}
	dc.SendBufferToServer(network.CTOS_UPDATE_DECK, buildUpdateDeckPayload(deck))
}

// buildUpdateDeckPayload 拼 CTOS_UPDATE_DECK 的负载。
// 单独拆出来是为了能直接对着 protocol 的解包器做往返测试 —— 这条格式错了，
// 症状是「点了准备开不了局」，从界面上根本看不出是哪一步出的问题。
func buildUpdateDeckPayload(deck *Deck) []byte {
	mainc := len(deck.Main) + len(deck.Extra)
	sidec := len(deck.Side)

	payload := make([]byte, 0, 8+(mainc+sidec)*4)
	payload = binary.LittleEndian.AppendUint32(payload, uint32(mainc))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(sidec))
	for _, list := range [][]uint32{deck.Main, deck.Extra, deck.Side} {
		for _, code := range list {
			payload = binary.LittleEndian.AppendUint32(payload, code)
		}
	}
	return payload
}
