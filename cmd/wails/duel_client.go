package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/core/utils"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// WailsDuelClient manages the TCP connection to YGOPro duel server
// and translates network packets to frontend events.
type WailsDuelClient struct {
	mu          sync.Mutex
	conn        net.Conn
	ctx         context.Context
	emitFunc    func(eventName string, optionalData ...interface{})
	isConnected bool
	playerType  uint8
	running     bool
	stopChan    chan struct{}

	// STOC_REPLAY 落盘：服务器在决斗结束时推送完整录像
	// （ExtendedReplayHeader/ReplayHeader + 压缩数据，single_duel.go:1801），
	// 先缓存到内存，前端确认（或 auto_save_replay=1 自动）后经 SaveReplay 写入。
	// replayDir 可注入以便测试，默认 "replay"。
	lastReplay []byte
	replayDir  string
}

func NewWailsDuelClient(emitFunc func(eventName string, optionalData ...interface{})) *WailsDuelClient {
	return &WailsDuelClient{
		emitFunc:  emitFunc,
		stopChan:  make(chan struct{}),
		replayDir: "replay",
	}
}

func (c *WailsDuelClient) SetContext(ctx context.Context) {
	c.ctx = ctx
}

func (c *WailsDuelClient) Connect(addr string, username string, pass string) error {
	// 不要在持有 c.mu 的状态下发送 PlayerInfo：sendPacket 也需要 c.mu
	// （sync.Mutex 不可重入，嵌套加锁会自死锁）。先完成拨号和状态更新，
	// 释放锁后再发送。
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	c.mu.Lock()
	if c.isConnected && c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn
	c.isConnected = true
	c.running = true
	c.stopChan = make(chan struct{})
	c.mu.Unlock()

	// Start background read loop
	go c.readLoop()

	// Send PlayerInfo packet
	return c.sendPlayerInfo(username)
}

// IsConnected 在锁内报告连接状态（StartSingle 的守卫用）。
func (c *WailsDuelClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isConnected
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

func (c *WailsDuelClient) sendPlayerInfo(name string) error {
	var pkt protocol.CTOSPlayerInfo
	encoded := utf16.Encode([]rune(name))
	for i := 0; i < len(encoded) && i < 19; i++ {
		pkt.Name[i] = encoded[i]
	}
	pkt.Name[19] = 0
	data, err := restruct.Pack(binary.LittleEndian, &pkt)
	if err != nil {
		return err
	}
	return c.sendPacket(network.CTOS_PLAYER_INFO, data)
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

func (c *WailsDuelClient) SendKick(pos byte) error {
	// CTOS_HS_KICK 携带一个字节：被踢的座位号（netserver.cpp:355-365 仅
	// 准备阶段允许，宿主专用）。
	return c.sendPacket(network.CTOS_HS_KICK, []byte{pos})
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

// SaveReplay 把最近一次 STOC_REPLAY 的包体原样写入 replayDir/name.yrp
// （gframe Replay::SaveReplay 的语义：包体本身就是 header+compData 的完整
// 文件格式）。name 为空时按 gframe 的兜底名 _LastReplay 保存。路径分隔符
// 一律替换为 _，防止越出 replay 目录。返回实际写入的文件名（不含 .yrp）。
func (c *WailsDuelClient) SaveReplay(name string) (string, error) {
	c.mu.Lock()
	data := c.lastReplay
	c.mu.Unlock()
	if len(data) == 0 {
		return "", fmt.Errorf("no replay received from server")
	}
	base := strings.TrimSpace(name)
	if base == "" {
		base = "_LastReplay"
	}
	base = strings.ReplaceAll(base, "/", "_")
	base = strings.ReplaceAll(base, "\\", "_")
	base = strings.TrimSuffix(base, ".yrp")
	if err := os.MkdirAll(c.replayDir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(c.replayDir, base+".yrp")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return base, nil
}

// ------------------------------------------------------------------
// Incoming STOC Packet Processing Loop
// ------------------------------------------------------------------

func (c *WailsDuelClient) readLoop() {
	// 在循环外固定连接引用：Disconnect 会把 c.conn 置 nil，
	// 循环内若再读 c.conn，可能对 nil 接口调用 Read 而 panic。
	// 连接被关闭时 ReadFull 会自行报错退出。
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return
	}

	header := make([]byte, 2)
	for c.running {
		// 1. Read 2-byte packet length
		if _, err := io.ReadFull(conn, header); err != nil {
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
		if _, err := io.ReadFull(conn, body); err != nil {
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
		// payload = int16_t[6]，且服务器已按接收方玩家交换过前后半
		// （single_duel.cpp:340-345），直接按相对序透传。
		var pkt protocol.STOCDeckCount
		_ = restruct.Unpack(payload, binary.LittleEndian, &pkt)
		c.emit("stoc:deck_count", pkt)

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
		// 布局 = player_type(2) + NUL 结尾的 UTF-16 变长串
		// （netserver.cpp:380-385：只写入 player + 原始消息字节，非定长 256），
		// restruct 的定长数组表达不了 NUL 结尾，保留手写解码。
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

	case network.STOC_TEAMMATE_SURRENDER:
		// 组队赛队友请求投降（duelclient.cpp:947-952）：无包体；原版把
		// btnLeaveGame 文案换成 SysString 1355「投降(1/2)」，这里交给
		// 右侧控制组同语义处理。
		c.emit("stoc:teammate_surrender", map[string]interface{}{})

	case network.STOC_REPLAY:
		// 决斗结束的完整录像（duelclient.cpp:727-774）：header + 压缩数据。
		// gframe 把 Base.StartTime（REPLAY_UNIFORM）或 Seed 当 time_t 格式化成
		// 文件名；这里同样推导建议文件名，落盘时机交给前端（auto_save_replay
		// 或确认弹窗）调 SaveReplay。
		if len(payload) < 32 {
			return
		}
		c.mu.Lock()
		c.lastReplay = append([]byte(nil), payload...)
		c.mu.Unlock()
		flag := binary.LittleEndian.Uint32(payload[8:12])
		startTime := binary.LittleEndian.Uint32(payload[12:16]) // Seed
		if flag&duel.REPLAY_UNIFORM != 0 {
			startTime = binary.LittleEndian.Uint32(payload[20:24]) // StartTime
		}
		suggested := time.Unix(int64(startTime), 0).Format("2006-01-02 15-04-05")
		c.emit("stoc:replay", map[string]interface{}{
			"name": suggested,
			"size": len(payload),
		})

	case network.STOC_GAME_MSG:
		if err := c.handleGameMessage(payload); err != nil {
			log.Printf("[WailsDuelClient] game message parse error: %v", err)
		}
	}
}

// handleGameMessage parses YGOPRO engine MSG_* stream into structured 3D client
// events. It returns an error when the stream cannot be fully parsed to byte
// alignment (unknown message layout or truncated body); the caller decides
// whether to log, drop the batch, or surface the failure.
//
// 每种消息的解析与转发行为由 engineBindings 声明（engine_bindings.go）：
// 查表 → pbuf.Unpack(消息结构体) → decorate 派生字段 → emit。
// 表内没有的消息走 skipEngineMessageBody 的残余分支保持对齐。
func (c *WailsDuelClient) handleGameMessage(msgBuffer []byte) error {
	pbuf := utils.NewYGOBuffer(msgBuffer, binary.LittleEndian)

	for pbuf.Len() > 0 {
		var engType uint8
		if err := pbuf.Read(&engType); err != nil {
			return fmt.Errorf("read opcode at offset %d: %w", pbuf.Offset(), err)
		}

		binding, ok := engineBindings[engType]
		if !ok {
			// Messages the UI does not decode still occupy bytes in the stream.
			// Skip their bodies so the parser keeps message alignment; on an
			// unknown layout, stop parsing this batch rather than desync.
			if err := skipEngineMessageBody(pbuf, engType); err != nil {
				return fmt.Errorf("opcode 0x%02x at offset %d: %w", engType, pbuf.Offset(), err)
			}
			continue
		}

		if binding.newMsg == nil {
			// 无消息体：只发空事件
			if binding.event != "" {
				c.emit(binding.event, map[string]interface{}{})
			}
			continue
		}

		msg := binding.newMsg()
		if err := pbuf.Unpack(msg); err != nil {
			return fmt.Errorf("%s at offset %d: %w", binding.name, pbuf.Offset(), err)
		}
		if binding.decorate != nil {
			if err := binding.decorate(c, engType, pbuf, msg); err != nil {
				return fmt.Errorf("%s at offset %d: %w", binding.name, pbuf.Offset(), err)
			}
			continue
		}
		if binding.event != "" {
			c.emit(binding.event, msg)
		}
	}
	return nil
}
