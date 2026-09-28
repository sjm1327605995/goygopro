package utils

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unicode/utf16"
)

// NetLogf 是联机诊断日志：向进程当前工作目录的 debug_net.log 追加一行
// 「[时间戳] [模块] 消息」。默认启用，环境变量 YGO_NETLOG=0 关闭。
// 线程安全；打开/写入失败一律静默忽略，绝不影响主流程。
var (
	netlogMu      sync.Mutex
	netlogInited  bool
	netlogEnabled bool
	netlogFile    *os.File
)

const netlogFileName = "debug_net.log"

// netlogReady 懒加载初始化（首次调用时读环境变量并打开文件）。
func netlogReady() bool {
	if !netlogInited {
		netlogInited = true
		netlogEnabled = os.Getenv("YGO_NETLOG") != "0"
		if netlogEnabled {
			f, err := os.OpenFile(netlogFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				netlogEnabled = false
			} else {
				netlogFile = f
			}
		}
	}
	return netlogEnabled
}

func NetLogf(module string, format string, args ...any) {
	netlogMu.Lock()
	defer netlogMu.Unlock()
	if !netlogReady() {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	_, _ = fmt.Fprintf(netlogFile, "[%s] [%s] %s\n", ts, module, fmt.Sprintf(format, args...))
}

// WideString 把 UTF-16 数组解码为 Go 字符串（截断到首个 NUL）。
// 供日志埋点打印玩家名/房间名等协议字符串字段。
func WideString(s []uint16) string {
	end := len(s)
	for i, v := range s {
		if v == 0 {
			end = i
			break
		}
	}
	return string(utf16.Decode(s[:end]))
}
