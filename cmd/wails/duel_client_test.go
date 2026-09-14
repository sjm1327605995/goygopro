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
