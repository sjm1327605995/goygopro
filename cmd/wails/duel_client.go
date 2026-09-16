package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf16"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// WailsDuelClient manages the TCP connection to YGOPro duel server
// and translates network packets to frontend events.
type WailsDuelClient struct {
	mu          sync.Mutex
	conn        net.Conn
	emitFunc    func(eventName string, optionalData ...interface{})
	isConnected bool
	gen         uint64 // 连接代际：每次 (重)连 +1，旧 readLoop 据此判断自己是否过期
	running     atomic.Bool

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
		replayDir: "replay",
	}
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
	c.gen++
	c.running.Store(true)
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
	c.running.Store(false)
	c.isConnected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
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
	// Pack 的返回值是写完后剩余的缓冲区（与 Unpack 返回剩余字节对称），
	// 而不是已写入的数据 —— 发包要截 buf 的前 SizeOf 字节。
	if _, err := deckData.Pack(buf, binary.LittleEndian); err != nil {
		return err
	}
	return c.sendPacket(network.CTOS_UPDATE_DECK, buf)
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
	// 在循环外固定连接引用与代际：Disconnect/Connect 会把 c.conn 置 nil 或
	// 换新连接，循环内若再读 c.conn，可能对 nil 接口调用 Read 而 panic。
	// 连接被关闭时 ReadFull 会自行报错退出。
	c.mu.Lock()
	conn := c.conn
	gen := c.gen
	c.mu.Unlock()
	if conn == nil {
		return
	}

	header := make([]byte, 2)
	for c.running.Load() {
		// 1. Read 2-byte packet length
		if _, err := io.ReadFull(conn, header); err != nil {
			if c.running.Load() {
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
			if c.running.Load() {
				log.Printf("[WailsDuelClient] Read body error: %v", err)
			}
			break
		}

		proto := body[0]
		payload := body[1:]
		c.handleSTOCPacket(proto, payload)
	}
	c.teardown(conn, gen)
}

// teardown 在一个 readLoop 退出时收尾连接，用代际守卫：只有本次循环持有的
// 连接仍是当前连接时才真正断开并广播 stoc:disconnected，避免重连后的旧
// readLoop 误断新连接（Disconnect 关的是 c.conn 而非它捕获的旧 conn）。
func (c *WailsDuelClient) teardown(conn net.Conn, gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.gen || c.conn != conn {
		return
	}
	c.running.Store(false)
	c.isConnected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.emit("stoc:disconnected", map[string]interface{}{})
}
