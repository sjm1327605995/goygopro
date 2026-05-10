package duel

import (
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// BroadcastServer UDP 广播发现服务器
// C++ 对应：NetServer::StartBroadcast / NetServer::StopBroadcast / NetServer::BroadcastEvent
type BroadcastServer struct {
	conn     *net.UDPConn
	stopCh   chan struct{}
	stopOnce sync.Once
}

// StartBroadcast 启动 UDP 广播监听
// C++: 创建 UDP socket -> bind(:7920) -> event_add(EV_READ|EV_PERSIST)
func StartBroadcast(serverPort uint16) (*BroadcastServer, error) {
	addr, err := net.ResolveUDPAddr("udp4", ":7920")
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}

	bs := &BroadcastServer{
		conn:   conn,
		stopCh: make(chan struct{}),
	}

	go bs.serve(serverPort)
	return bs, nil
}

func (bs *BroadcastServer) serve(serverPort uint16) {
	buf := make([]byte, 256)
	for {
		select {
		case <-bs.stopCh:
			return
		default:
		}

		_ = bs.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, remoteAddr, err := bs.conn.ReadFromUDP(buf)
		if err != nil {
			// 超时继续循环，其他错误退出
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			return
		}

		if n < 2 {
			continue
		}

		var req protocol.HostRequest
		if err := restruct.Unpack(buf[:n], binary.LittleEndian, &req); err != nil {
			continue
		}

		// C++: if(pHR->identifier == NETWORK_CLIENT_ID)
		if req.Identifier != network.NETWORK_CLIENT_ID {
			continue
		}

		// C++ 语义：NetServer::duel_mode 是唯一的房间。
		// Go 中遍历所有房间，为每个房间回复一个 HostPacket（兼容多房间但保持语义）。
		for _, room := range DefaultManager.AllRooms() {
			if room == nil || room.DuelMode == nil {
				continue
			}
			base := room.DuelMode.BaseMode()
			if base == nil {
				continue
			}

			hp := protocol.HostPacket{
				Identifier: network.NETWORK_SERVER_ID,
				Version:    PRO_VERSION,
				Port:       serverPort,
				Host:       base.HostInfo,
			}
			copy(hp.Name[:], base.Name[:])

			data, err := restruct.Pack(binary.LittleEndian, &hp)
			if err != nil {
				continue
			}

			// C++: sockTo.sin_port = htons(7921)
			replyAddr := &net.UDPAddr{
				IP:   remoteAddr.IP,
				Port: 7921,
			}
			_, _ = bs.conn.WriteToUDP(data, replyAddr)
		}
	}
}

// Stop 停止 UDP 广播监听
// C++: event_del -> evutil_closesocket -> event_free
func (bs *BroadcastServer) Stop() {
	bs.stopOnce.Do(func() {
		close(bs.stopCh)
		_ = bs.conn.Close()
	})
}
