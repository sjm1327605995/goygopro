package client

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/sjm1327605995/goygopro/core/duel"
)

var localServerRunning atomic.Bool

// serverDataPaths returns candidate paths for the card database and script directory.
func serverDataPaths() (dbPaths []string, scriptPaths []string, rootPaths []string) {
	dbPaths = []string{
		`cards.cdb`,
		`./cards.cdb`,
		`E:\ygopro\cards.cdb`,
		`E:\gopath\kimi\goygopro\cards.cdb`,
	}
	scriptPaths = []string{
		`script`,
		`./script`,
		`ygo`,
		`./ygo`,
		`E:\ygo`,
		`E:\gopath\kimi\goygopro\script`,
	}
	rootPaths = []string{
		`.`,
		`E:\Go\gopath\goygopro`,
		`E:\gopath\kimi\goygopro`,
	}
	return
}

func findExisting(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// StartLocalServer starts an embedded YGOPro server on the given port.
// It initializes the card database and ocgcore, then starts the gnet server
// and broadcast discovery service.
func StartLocalServer(port uint16) error {
	if localServerRunning.Load() {
		if isPortOpen("127.0.0.1", port) {
			return nil
		}
		localServerRunning.Store(false)
	}

	dbPaths, scriptPaths, rootPaths := serverDataPaths()
	dbPath := findExisting(dbPaths)
	scriptPath := findExisting(scriptPaths)
	rootPath := findExisting(rootPaths)

	if dbPath == "" {
		return fmt.Errorf("找不到卡片数据库 (cards.cdb)，请将其放置到项目目录或 E:\\ygopro\\cards.cdb")
	}
	if scriptPath == "" {
		// ocgcore does not validate script directory at init time,
		// so we can use a dummy path if none exists.
		scriptPath = "script"
	}
	if rootPath == "" {
		rootPath = "."
	}

	if err := duel.InitServerData(dbPath, scriptPath, rootPath); err != nil {
		return fmt.Errorf("初始化服务器数据失败: %w", err)
	}

	// Start the TCP server in a background goroutine.
	go func() {
		if err := duel.StartDuelServer(int(port), false); err != nil {
			fmt.Println("Local server stopped:", err)
		}
		localServerRunning.Store(false)
	}()

	// Wait for the server to actually start listening.
	if err := waitForPort("127.0.0.1", port, 2*time.Second); err != nil {
		return fmt.Errorf("本地服务器未在 2 秒内启动: %w", err)
	}

	// Start UDP broadcast discovery.
	bs, err := duel.StartBroadcast(port)
	if err != nil {
		fmt.Println("Failed to start broadcast:", err)
	} else {
		duel.BroadcastInstance = bs
	}

	localServerRunning.Store(true)
	return nil
}

// StopLocalServer stops the embedded server if it is running.
func StopLocalServer() {
	if !localServerRunning.Load() {
		return
	}
	if duel.NetServerEngine != nil {
		_ = duel.NetServerEngine.Stop(context.Background())
	}
	if duel.BroadcastInstance != nil {
		duel.BroadcastInstance.Stop()
	}
	localServerRunning.Store(false)
}

// isPortOpen checks whether a TCP port is accepting connections.
func isPortOpen(host string, port uint16) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 100*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// waitForPort polls the given TCP address until it accepts a connection
// or the timeout expires.
func waitForPort(host string, port uint16, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isPortOpen(host, port) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("端口 %d 在 %v 内未开放", port, timeout)
}
