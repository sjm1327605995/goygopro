package duel

import (
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sjm1327605995/goygopro/core/codec"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// BroadcastServer UDP 广播发现服务器
// C++ 对应：NetServer::StartBroadcast / NetServer::StopBroadcast / NetServer::BroadcastEvent
type BroadcastServer struct {
	conn     *net.UDPConn
	stopCh   chan struct{}
	stopOnce sync.Once
	stopped  atomic.Bool
}

// broadcastMu 串行化广播应答器实例指针与端口的全部读写。触达方至少四类
// 且分属不同 goroutine：CTOS_CREATE_GAME 处理器里的 EnsureBroadcast（gnet
// --multicore 下不同连接跑在不同 event loop，可并发）、决斗 goroutine 的
// StopListen/StopServer、前端调用的 StartLocalServer（重建主机先停旧实例）、
// cmd/server 启动登记。此前实例指针是裸全局变量，检查-启动-赋值与 Stop
// 交错存在数据竞争（2026-10 并发修复轮遗漏的最后一个无锁共享状态）。
var (
	broadcastMu       sync.Mutex
	broadcastInstance *BroadcastServer
	// lastBroadcastPort 记录最近一次（尝试）启动广播时的服务器监听端口，
	// 供 EnsureBroadcast 在广播被 StopListen 停止后按同端口重启。
	lastBroadcastPort uint16
)

// CurrentBroadcast 返回当前全局广播应答器实例（nil 安全）。只读快照，
// 调用方不得绕过 SetBroadcast/StopBroadcast 直接改指针。
func CurrentBroadcast() *BroadcastServer {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	return broadcastInstance
}

// SetBroadcast 替换全局广播应答器实例（旧实例由调用方负责 Stop）。
// 与 CurrentBroadcast 配对，避免对实例指针的裸读裸写。
func SetBroadcast(bs *BroadcastServer) {
	broadcastMu.Lock()
	broadcastInstance = bs
	broadcastMu.Unlock()
}

// StopBroadcast 停止并清空当前全局广播应答器（nil 安全）。
func StopBroadcast() {
	broadcastMu.Lock()
	bs := broadcastInstance
	broadcastInstance = nil
	broadcastMu.Unlock()
	if bs != nil {
		bs.Stop()
	}
}

// StartBroadcast 启动 UDP 广播监听
// C++: 创建 UDP socket -> bind(:7920) -> event_add(EV_READ|EV_PERSIST)
func StartBroadcast(serverPort uint16) (*BroadcastServer, error) {
	broadcastMu.Lock()
	lastBroadcastPort = serverPort
	broadcastMu.Unlock()

	return startBroadcast(serverPort)
}

// startBroadcast 是 StartBroadcast 的不登记端口、不触碰全局状态的实现体，
// 供持锁的 EnsureBroadcast 复用（避免同类锁重入）。
func startBroadcast(serverPort uint16) (*BroadcastServer, error) {
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

// Running 报告广播应答器是否仍在运行（Stop 后或从未启动为 false）。
func (bs *BroadcastServer) Running() bool {
	return bs != nil && !bs.stopped.Load()
}

// EnsureBroadcast 在广播应答器未运行（从未启动成功，或已被 StopListen /
// StopServer 停止）时按上次记录的端口重启。对齐原版 netserver.cpp:313
// CTOS_CREATE_GAME 即 StartBroadcast 的语义——决斗开始会连带停掉广播，
// 下一次建房时恢复应答。绑定 :7920 失败（端口被占）只记录日志，不影响建房。
// 整个「检查-启动-登记」持 broadcastMu 串行化，与 StopBroadcast/SetBroadcast
// 的并发触达互斥。
func EnsureBroadcast() {
	broadcastMu.Lock()
	defer broadcastMu.Unlock()
	if lastBroadcastPort == 0 || (broadcastInstance != nil && broadcastInstance.Running()) {
		return
	}
	bs, err := startBroadcast(lastBroadcastPort)
	if err != nil {
		log.Printf("[duel] restart broadcast failed: %v", err)
		return
	}
	broadcastInstance = bs
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
		if err := codec.Unmarshal(buf[:n], &req); err != nil {
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

			data, err := codec.Marshal(&hp)
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
		bs.stopped.Store(true)
		close(bs.stopCh)
		_ = bs.conn.Close()
	})
}
