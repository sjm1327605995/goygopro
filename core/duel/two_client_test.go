package duel

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestTwoClientsCreateJoinAndDuel is the end-to-end check for the server's
// multi-client support: two TCP clients connect to a live StartDuelServer,
// the first creates a single-mode room (CTOS_CREATE_GAME), the second joins
// it (CTOS_JOIN_GAME), and together they walk the lobby (deck + ready +
// start), the finger guess and the first/second choice until both receive
// MSG_START — proving a real two-player duel began over the wire.
//
// It also pins the single-room limit inherited from the C++ server: a third
// client that tries to CREATE a second room is rejected, and a JOIN with an
// unknown password gets ERRMSG_JOINERROR.
func TestTwoClientsCreateJoinAndDuel(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	ensureTwoClientEngine(t, root)

	// Fresh manager + accept flag: other tests in this package may have
	// touched the globals.
	DefaultManager = NewManager()
	AcceptingConnections.Store(true)

	port := freePort(t)
	go func() {
		// gnet.Run blocks until the engine stops; the error is surfaced via
		// NetServerEngine by the handlers, nothing actionable here.
		_ = StartDuelServer(port, false)
	}()
	t.Cleanup(func() {
		AcceptingConnections.Store(true)
		DefaultManager = NewManager()
		if NetServerEngine != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = NetServerEngine.Stop(ctx)
		}
	})
	waitForServer(t, port)

	alice := dialClient(t, port)
	bob := dialClient(t, port)

	// --- 大厅阶段：建房 + 加入 ---
	sendPlayerInfo(t, alice, "Alice")
	sendPlayerInfo(t, bob, "Bob")
	sendCreateGame(t, alice, "room", "pass123")

	// Alice（房主）应收到 JOIN_GAME + TYPE_CHANGE(0x10=host|player1) + 自己进房。
	p := expectProto(t, alice, network.STOC_JOIN_GAME)
	if len(p.payload) != 20 {
		t.Fatalf("STOC_JOIN_GAME payload len = %d, want 20", len(p.payload))
	}
	p = expectProto(t, alice, network.STOC_TYPE_CHANGE)
	if p.payload[0] != 0x10|network.NETPLAYER_TYPE_PLAYER1 {
		t.Fatalf("alice type = 0x%02x, want host|player1 (0x10)", p.payload[0])
	}
	expectProto(t, alice, network.STOC_HS_PLAYER_ENTER)

	sendJoinGame(t, bob, "pass123", 0x1361)

	// Bob 应收到 JOIN_GAME + TYPE_CHANGE(player2) + 两名玩家的 PLAYER_ENTER。
	expectProto(t, bob, network.STOC_JOIN_GAME)
	p = expectProto(t, bob, network.STOC_TYPE_CHANGE)
	if p.payload[0] != network.NETPLAYER_TYPE_PLAYER2 {
		t.Fatalf("bob type = 0x%02x, want player2 (0x01)", p.payload[0])
	}
	expectProto(t, bob, network.STOC_HS_PLAYER_ENTER)
	expectProto(t, bob, network.STOC_HS_PLAYER_ENTER)

	// Alice 应收到 Bob 的进房通知。
	expectProto(t, alice, network.STOC_HS_PLAYER_ENTER)

	// --- 第三客户端：单房间限制 + 密码校验 ---
	carol := dialClient(t, port)
	sendPlayerInfo(t, carol, "Carol")
	sendJoinGame(t, carol, "wrongpass", 0x1361)
	p = expectProto(t, carol, network.STOC_ERROR_MSG)
	if p.payload[0] != network.ERRMSG_JOINERROR {
		t.Fatalf("carol error msg = %d, want ERRMSG_JOINERROR", p.payload[0])
	}
	// 房主仍在游戏中时再建房：服务器静默拒绝（与 C++ 一致，不回复错误包）。
	sendCreateGame(t, carol, "room2", "other999")
	if _, err := readPacket(carol, 400*time.Millisecond); err == nil {
		t.Fatal("carol creating a second room should get no packets (single-room server)")
	}
	carol.Close()

	// --- 跨客户端广播：Alice 聊天，Bob 应收 STOC_CHAT ---
	sendPacket(t, alice, network.CTOS_CHAT, utf16NullTerm("hello bob"))
	p = expectProto(t, bob, network.STOC_CHAT)
	if len(p.payload) < 2+len("hello bob")*2 {
		t.Fatalf("STOC_CHAT payload too short: %d", len(p.payload))
	}

	// --- 双方上传卡组并准备，房主开始决斗 ---
	deck := deckPayload(40, 15)
	sendPacket(t, alice, network.CTOS_UPDATE_DECK, deck)
	sendPacket(t, bob, network.CTOS_UPDATE_DECK, deck)
	sendPacket(t, alice, network.CTOS_HS_READY, nil)
	sendPacket(t, bob, network.CTOS_HS_READY, nil)
	// 跨连接无顺序保证：必须等双方 ready 广播都落袋（意味着两人的
	// UPDATE_DECK/READY 均已被服务器处理）再发 HS_START，否则 START 可能
	// 先于 Bob 的准备到达，导致空转超时。
	waitBothReady(t, alice)
	sendPacket(t, alice, network.CTOS_HS_START, nil)

	// 双方应收到 DUEL_START + DECK_COUNT + SELECT_HAND（猜拳）。
	for _, c := range []struct {
		name string
		conn net.Conn
	}{{"alice", alice}, {"bob", bob}} {
		expectProtoNamed(t, c.name, c.conn, network.STOC_DUEL_START)
		p = expectProtoNamed(t, c.name, c.conn, network.STOC_DECK_COUNT)
		if len(p.payload) != 12 {
			t.Fatalf("%s STOC_DECK_COUNT payload len = %d, want 12", c.name, len(p.payload))
		}
		expectProtoNamed(t, c.name, c.conn, network.STOC_SELECT_HAND)
	}

	// 猜拳：Alice 出 1（石头）、Bob 出 2（剪刀）→ Bob 胜，获得先后攻选择权。
	sendPacket(t, alice, network.CTOS_HAND_RESULT, []byte{1})
	sendPacket(t, bob, network.CTOS_HAND_RESULT, []byte{2})
	expectProto(t, alice, network.STOC_HAND_RESULT)
	expectProto(t, bob, network.STOC_HAND_RESULT)
	expectProto(t, bob, network.STOC_SELECT_TP)

	// Bob 选择先攻（tp=1），随后双方都应收到 MSG_START —— 双客户端决斗正式开始。
	sendPacket(t, bob, network.CTOS_TP_RESULT, []byte{1})
	for _, c := range []struct {
		name string
		conn net.Conn
	}{{"alice", alice}, {"bob", bob}} {
		p = expectProtoNamed(t, c.name, c.conn, network.STOC_GAME_MSG)
		if len(p.payload) == 0 || p.payload[0] != byte(ocgcore.MSG_START) {
			t.Fatalf("%s first GAME_MSG = %v, want MSG_START", c.name, p.payload[:1])
		}
	}

	// 断线清理：双方 TCP 关闭后服务器应结束对局并摘除房间。
	alice.Close()
	bob.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if DefaultManager.RoomCount() == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("room not removed after both clients disconnected")
}

// --------------------------------------------------
// 测试辅助
// --------------------------------------------------

// ensureTwoClientEngine 加载卡库与 ocgcore（幂等，整个测试进程只初始化一次）。
func ensureTwoClientEngine(t *testing.T, root string) {
	t.Helper()
	if DefaultDataManager.GetData(89631139) == nil {
		if err := DefaultDataManager.LoadDB(filepath.Join(root, "cards.cdb")); err != nil {
			t.Fatalf("load cards.cdb: %v", err)
		}
	}
	if ocgcore.API == nil {
		if err := ocgcore.Init(
			ocgcore.WithRootPath(root),
			ocgcore.WithScriptDirectory(filepath.Join(root, "script")),
			ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
				return DefaultDataManager.GetData(cardId)
			}),
		); err != nil {
			t.Skipf("ocgcore library not available: %v", err)
		}
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForServer(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server on :%d did not come up", port)
}

func dialClient(t *testing.T, port int) net.Conn {
	t.Helper()
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

// sendPacket 写出一个完整报文：[uint16 LE 长度][proto][payload]。
func sendPacket(t *testing.T, conn net.Conn, proto byte, payload []byte) {
	t.Helper()
	buf := make([]byte, 3+len(payload))
	binary.LittleEndian.PutUint16(buf, uint16(1+len(payload)))
	buf[2] = proto
	copy(buf[3:], payload)
	if _, err := conn.Write(buf); err != nil {
		t.Fatalf("send proto 0x%02x: %v", proto, err)
	}
}

type recvPacket struct {
	proto   byte
	payload []byte
}

// readPacket 读取一个报文；超时返回错误。
func readPacket(conn net.Conn, timeout time.Duration) (recvPacket, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		return recvPacket{}, err
	}
	n := int(binary.LittleEndian.Uint16(hdr))
	body := make([]byte, n)
	if _, err := io.ReadFull(conn, body); err != nil {
		return recvPacket{}, err
	}
	if n == 0 {
		return recvPacket{}, fmt.Errorf("empty packet")
	}
	return recvPacket{proto: body[0], payload: body[1:]}, nil
}

// waitBothReady 读取 STOC_HS_PLAYER_CHANGE，直到 0/1 号位都广播了 READY。
func waitBothReady(t *testing.T, conn net.Conn) {
	t.Helper()
	ready := [2]bool{}
	deadline := time.Now().Add(5 * time.Second)
	for !ready[0] || !ready[1] {
		p, err := readPacket(conn, time.Until(deadline))
		if err != nil {
			t.Fatalf("waiting for both players ready: %v", err)
		}
		if p.proto != network.STOC_HS_PLAYER_CHANGE {
			continue
		}
		status := p.payload[0]
		if status&0x0f == network.PLAYERCHANGE_READY {
			ready[status>>4] = true
		}
	}
}

// expectProto 连续读包直到出现目标 proto；遇到 STOC_ERROR_MSG 直接失败。
func expectProto(t *testing.T, conn net.Conn, want byte) recvPacket {
	t.Helper()
	return expectProtoNamed(t, "client", conn, want)
}

func expectProtoNamed(t *testing.T, name string, conn net.Conn, want byte) recvPacket {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		p, err := readPacket(conn, time.Until(deadline))
		if err != nil {
			t.Fatalf("%s: waiting for proto 0x%02x: %v", name, want, err)
		}
		if p.proto == network.STOC_ERROR_MSG && want != network.STOC_ERROR_MSG {
			t.Fatalf("%s: got STOC_ERROR_MSG (msg=%d code=%d) while waiting for 0x%02x",
				name, p.payload[0], binary.LittleEndian.Uint32(p.payload[4:8]), want)
		}
		if p.proto == want {
			return p
		}
		// 其他报文（如 HS_PLAYER_CHANGE）跳过。
	}
}

func utf16z(s string) [20]uint16 {
	var out [20]uint16
	enc := utf16.Encode([]rune(s))
	copy(out[:], enc)
	return out
}

func utf16NullTerm(s string) []byte {
	enc := append(utf16.Encode([]rune(s)), 0)
	out := make([]byte, 0, len(enc)*2)
	for _, u := range enc {
		out = binary.LittleEndian.AppendUint16(out, u)
	}
	return out
}

func sendPlayerInfo(t *testing.T, conn net.Conn, name string) {
	t.Helper()
	buf := make([]byte, 0, 40)
	for _, u := range utf16z(name) {
		buf = binary.LittleEndian.AppendUint16(buf, u)
	}
	sendPacket(t, conn, network.CTOS_PLAYER_INFO, buf)
}

func sendCreateGame(t *testing.T, conn net.Conn, name, pass string) {
	t.Helper()
	payload := make([]byte, 0, 20+40+40)
	payload = binary.LittleEndian.AppendUint32(payload, 0) // LFList（服务器会替换为默认表）
	payload = append(payload, CURRENT_RULE)                // Rule
	payload = append(payload, MODE_SINGLE)                 // Mode
	payload = append(payload, 0)                           // DuelRule
	payload = append(payload, 1)                           // NoCheckDeck
	payload = append(payload, 0)                           // NoShuffleDeck
	payload = append(payload, 0, 0, 0)                     // padding
	payload = binary.LittleEndian.AppendUint32(payload, 8000)
	payload = append(payload, 5) // StartHand
	payload = append(payload, 1) // DrawCount
	payload = binary.LittleEndian.AppendUint16(payload, 0) // TimeLimit
	for _, u := range utf16z(name) {
		payload = binary.LittleEndian.AppendUint16(payload, u)
	}
	for _, u := range utf16z(pass) {
		payload = binary.LittleEndian.AppendUint16(payload, u)
	}
	sendPacket(t, conn, network.CTOS_CREATE_GAME, payload)
}

func sendJoinGame(t *testing.T, conn net.Conn, pass string, version uint16) {
	t.Helper()
	payload := make([]byte, 0, 44)
	payload = binary.LittleEndian.AppendUint16(payload, version)
	payload = append(payload, 0, 0) // padding
	payload = binary.LittleEndian.AppendUint32(payload, 0)
	for _, u := range utf16z(pass) {
		payload = binary.LittleEndian.AppendUint16(payload, u)
	}
	sendPacket(t, conn, network.CTOS_JOIN_GAME, payload)
}

// deckPayload 组一个 mainc 张主卡 + sidec 张副卡的测试卡组（青眼白龙，编号 89631139）。
func deckPayload(mainc, sidec int32) []byte {
	payload := make([]byte, 0, 8+int(mainc+sidec)*4)
	payload = binary.LittleEndian.AppendUint32(payload, uint32(mainc))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(sidec))
	for i := int32(0); i < mainc+sidec; i++ {
		payload = binary.LittleEndian.AppendUint32(payload, 89631139)
	}
	return payload
}

// 防止 unused 告警（protocol 仅用于常量文档引用）。
var _ = protocol.MAINC_MAX
