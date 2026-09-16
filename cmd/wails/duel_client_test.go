package main

import (
	"encoding/binary"
	"io"
	"net"
	"testing"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestSendKickPacket locks down the CTOS_HS_KICK wire layout: 2-byte LE length,
// proto 0x24, then one payload byte carrying the seat to kick
// (network.h CTOS_Kick; netserver.cpp:355-365 rejects it outside prep phase).
func TestSendKickPacket(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	c := &WailsDuelClient{conn: clientConn, isConnected: true}

	pktCh := make(chan []byte, 1)
	go func() {
		var header [2]byte
		if _, err := io.ReadFull(serverConn, header[:]); err != nil {
			close(pktCh)
			return
		}
		body := make([]byte, binary.LittleEndian.Uint16(header[:]))
		if _, err := io.ReadFull(serverConn, body); err != nil {
			close(pktCh)
			return
		}
		pktCh <- body
	}()

	if err := c.SendKick(1); err != nil {
		t.Fatalf("SendKick: %v", err)
	}
	body, ok := <-pktCh
	if !ok {
		t.Fatal("failed to read packet body")
	}
	want := []byte{network.CTOS_HS_KICK, 0x01}
	if len(body) != len(want) {
		t.Fatalf("packet body = %v, want %v", body, want)
	}
	for i := range want {
		if body[i] != want[i] {
			t.Fatalf("packet body = %v, want %v", body, want)
		}
	}
}

// TestTeardownGenerationGuard 锁定重连代际守卫：旧 readLoop 退出时调 teardown
// 只对「自己捕获的那条连接」生效——重连后换成新连接、代际 +1，旧循环不得
// 误断新连接；当前代际的循环才真正断开。
func TestTeardownGenerationGuard(t *testing.T) {
	_, connA := net.Pipe()
	defer connA.Close()
	_, connB := net.Pipe()
	defer connB.Close()

	c := NewWailsDuelClient(nil)
	c.mu.Lock()
	c.conn = connB
	c.isConnected = true
	c.gen = 2
	c.running.Store(true)
	c.mu.Unlock()

	// 旧 readLoop（持 connA/gen1）退出：代际不匹配，不得断开当前 connB
	c.teardown(connA, 1)
	c.mu.Lock()
	stillConnected := c.isConnected && c.conn == connB
	c.mu.Unlock()
	if !stillConnected {
		t.Fatal("旧 readLoop 的 teardown 误断了新连接")
	}

	// 当前 readLoop（持 connB/gen2）退出：应真正断开
	c.teardown(connB, 2)
	c.mu.Lock()
	connected := c.isConnected
	c.mu.Unlock()
	if connected {
		t.Fatal("当前 readLoop 的 teardown 未断开连接")
	}
}
