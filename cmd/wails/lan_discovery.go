package main

import (
	"encoding/binary"
	"net"
	"strconv"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/go-restruct/restruct"
	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// LAN 房间发现（原版 duelclient.cpp BeginRefreshHost / BroadcastReply）：
// 客户端向 255.255.255.255:7920 广播 HostRequest（NETWORK_CLIENT_ID），监听
// 本机 7921 收服务器的 HostPacket 应答，按 (IP, port) 去重、按
// identifier/version 过滤。服务端应答实现见 core/duel/broadcast.go。

// HostEntry 是发现到的一台 LAN 主机（一个房间）。
type HostEntry struct {
	IP            string `json:"ip"`
	Port          uint16 `json:"port"`
	Name          string `json:"name"`
	LFList        uint32 `json:"lflist"`
	Rule          uint8  `json:"rule"`
	Mode          uint8  `json:"mode"`
	DuelRule      uint8  `json:"duelRule"`
	NoCheckDeck   uint8  `json:"noCheckDeck"`
	NoShuffleDeck uint8  `json:"noShuffleDeck"`
	StartLp       int32  `json:"startLp"`
	StartHand     uint8  `json:"startHand"`
	DrawCount     uint8  `json:"drawCount"`
	TimeLimit     uint16 `json:"timeLimit"`
}

// refreshMu 串行化多次「刷新主机」：7921 端口同一时间只允许一个接收者
// （Windows 上 SO_REUSEADDR 语义不同，双收会互相抢包）。
var refreshMu sync.Mutex

// RefreshHosts 执行一轮 LAN 广播发现，返回应答的主机列表。
// timeoutMs 是收应答的窗口（原版 timeval{3,0}），缺省/越界时取 3000。
func (a *App) RefreshHosts(timeoutMs int) []HostEntry {
	return refreshHosts(timeoutMs, &net.UDPAddr{IP: net.IPv4bcast, Port: 7920})
}

// refreshHosts 是 RefreshHosts 的实现，目标地址可注入（测试指向 127.0.0.1
// 的假应答端；生产用 255.255.255.255 有限广播）。
func refreshHosts(timeoutMs int, dst *net.UDPAddr) []HostEntry {
	if timeoutMs <= 0 || timeoutMs > 10000 {
		timeoutMs = 3000
	}
	refreshMu.Lock()
	defer refreshMu.Unlock()

	// 应答接收 socket：绑定本机 7921（与原版 reply socket 同端口）
	replyAddr, err := net.ResolveUDPAddr("udp4", ":7921")
	if err != nil {
		return nil
	}
	conn, err := net.ListenUDP("udp4", replyAddr)
	if err != nil {
		return nil
	}
	defer conn.Close()

	broadcastRequests(dst)

	seen := map[string]bool{}
	var hosts []HostEntry
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	buf := make([]byte, 256)
	for {
		_ = conn.SetReadDeadline(deadline)
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			// 超时或端口错误都结束本轮收集
			break
		}
		var hp protocol.HostPacket
		if err := restruct.Unpack(buf[:n], binary.LittleEndian, &hp); err != nil {
			continue
		}
		// 原版 BroadcastReply：identifier/version 不符直接丢弃
		if hp.Identifier != network.NETWORK_SERVER_ID || hp.Version != duel.PRO_VERSION {
			continue
		}
		key := remoteAddr.IP.String() + ":" + strconv.Itoa(int(hp.Port))
		if seen[key] {
			continue
		}
		seen[key] = true
		hosts = append(hosts, HostEntry{
			IP:            remoteAddr.IP.String(),
			Port:          hp.Port,
			Name:          decodeHostName(hp.Name[:]),
			LFList:        hp.Host.LFList,
			Rule:          hp.Host.Rule,
			Mode:          hp.Host.Mode,
			DuelRule:      hp.Host.DuelRule,
			NoCheckDeck:   hp.Host.NoCheckDeck,
			NoShuffleDeck: hp.Host.NoShuffleDeck,
			StartLp:       hp.Host.StartLp,
			StartHand:     hp.Host.StartHand,
			DrawCount:     hp.Host.DrawCount,
			TimeLimit:     hp.Host.TimeLimit,
		})
	}
	return hosts
}

// broadcastRequests 向 dst（生产为 255.255.255.255:7920）发送 HostRequest。
// 原版遍历本机地址逐个绑定发送（多宿主机器每个出口一发）；Go 拨号广播地址
// 会自动设置 SO_BROADCAST，这里按本地 IPv4 接口逐个绑定发送，无接口时兜底发一发。
func broadcastRequests(dst *net.UDPAddr) {
	req := protocol.HostRequest{Identifier: network.NETWORK_CLIENT_ID}
	payload, err := restruct.Pack(binary.LittleEndian, &req)
	if err != nil {
		return
	}
	sent := false
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagBroadcast == 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip4 := ip.To4(); ip4 != nil && !ip4.IsLoopback() {
					sendBroadcastFrom(ip4, dst, payload)
					sent = true
				}
			}
		}
	}
	// 单独向回环发一发：同机自建的主机（本客户端「建立主机」流程）也应能
	// 自发现；C++ 靠按网卡绑定的有限广播被本栈环回，Windows 上不稳定。
	sendBroadcastFrom(net.IPv4(127, 0, 0, 1), dst, payload)
	if !sent {
		sendBroadcastFrom(nil, dst, payload)
	}
}

func sendBroadcastFrom(localIP net.IP, dst *net.UDPAddr, payload []byte) {
	var laddr *net.UDPAddr
	if localIP != nil {
		laddr = &net.UDPAddr{IP: localIP, Port: 0}
	}
	conn, err := net.DialUDP("udp4", laddr, dst)
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = conn.Write(payload)
}

// decodeHostName 把 HostPacket 的定长 UTF-16 名字解码成 Go 字符串（NUL 截断）。
func decodeHostName(name []uint16) string {
	runes := utf16.Decode(name)
	for i, r := range runes {
		if r == 0 {
			return string(runes[:i])
		}
	}
	return string(runes)
}
