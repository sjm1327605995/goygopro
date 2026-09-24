package main

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestRefreshHostsDiscoversLANRooms 锁定客户端 LAN 发现路径
// （duelclient.cpp BeginRefreshHost/BroadcastReply 的 Go 版）：向 7920 发
// HostRequest，收 7921 的 HostPacket 应答并解码/去重/过滤。
// 测试在 7920 上起假应答端，回两个包：一个合法、一个版本不符（应被过滤）。
func TestRefreshHostsDiscoversLANRooms(t *testing.T) {
	// 假服务器：收到请求后回两个 HostPacket 到发送方端口（7921）
	srv, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7920})
	if err != nil {
		t.Skipf("7920 被占用，跳过 LAN 发现测试: %v", err)
	}
	defer srv.Close()

	go func() {
		buf := make([]byte, 256)
		for {
			n, remote, err := srv.ReadFromUDP(buf)
			if err != nil {
				return
			}
			var req protocol.HostRequest
			if err := restruct.Unpack(buf[:n], binary.LittleEndian, &req); err != nil || req.Identifier != network.NETWORK_CLIENT_ID {
				continue
			}
			reply := func(hp protocol.HostPacket) {
				data, err := restruct.Pack(binary.LittleEndian, &hp)
				if err != nil {
					return
				}
				_, _ = srv.WriteToUDP(data, &net.UDPAddr{IP: remote.IP, Port: 7921})
			}
			good := protocol.HostPacket{
				Identifier: network.NETWORK_SERVER_ID,
				Version:    duel.PRO_VERSION,
				Port:       7911,
				Host: protocol.HostInfo{
					LFList: 0x7dfcee6a, Rule: 0, Mode: 2, DuelRule: 5,
					StartLp: 8000, StartHand: 5, DrawCount: 1, TimeLimit: 180,
				},
			}
			copy(good.Name[:], utf16.Encode([]rune("测试房间")))
			reply(good)
			// 同 (ip,port) 重复包：应被去重
			reply(good)
			// 版本不符：应被过滤
			bad := good
			bad.Version = duel.PRO_VERSION - 1
			reply(bad)
		}
	}()

	started := time.Now()
	hosts := refreshHosts(800, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7920})
	if len(hosts) != 1 {
		t.Fatalf("发现主机数 = %d, want 1: %+v", len(hosts), hosts)
	}
	h := hosts[0]
	if h.Name != "测试房间" || h.Port != 7911 {
		t.Fatalf("名字/端口错: %+v", h)
	}
	if h.Mode != 2 || h.LFList != 0x7dfcee6a || h.StartLp != 8000 || h.DuelRule != 5 {
		t.Fatalf("HostInfo 字段错: %+v", h)
	}
	if h.IP == "" {
		t.Fatalf("IP 为空")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("发现耗时异常: %v", elapsed)
	}
}
