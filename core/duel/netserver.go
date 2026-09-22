package duel

import (
	"sync/atomic"

	"github.com/panjf2000/gnet/v2"
)

// NetServerEngine 全局服务器引擎引用
// C++ 语义：NetServer 是静态类，所有方法直接访问全局状态。
// Go 中通过全局变量模拟，在 server.go 的 OnBoot 中注入。
var NetServerEngine *gnet.Engine

// BroadcastInstance 全局广播服务器引用
var BroadcastInstance *BroadcastServer

// AcceptingConnections 是否接受新连接。atomic.Bool：OnOpen 在 gnet 事件循环
// goroutine 读取，StopListen 在决斗 goroutine 写入，--multicore=true 下是跨
// goroutine 访问，普通 bool 会数据竞争。
// C++ 中 StopListen() 通过 evconnlistener_disable 停止接受新连接。
// Go 中通过该标志在 OnOpen 中拒绝新连接。
var AcceptingConnections atomic.Bool

func init() {
	AcceptingConnections.Store(true)
}
