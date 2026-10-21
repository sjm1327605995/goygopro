package duel

import (
	"github.com/panjf2000/gnet/v2"
)

// recordConn 满足 gnet.Conn：嵌入空接口补齐方法，记录 AsyncWrite 的
// 字节，用于断言 SendError/Reply/广播发出的线格式（各 handler/analyze
// 测试共用）。同步追加保持断言的简单性；生产路径是 DuelPlayer.Write →
// Conn.AsyncWrite（异步出站），此处 emulate 其入队效果。
type recordConn struct {
	gnet.Conn
	buf []byte
}

func (r *recordConn) AsyncWrite(p []byte, _ gnet.AsyncCallback) error {
	r.buf = append(r.buf, p...)
	return nil
}

// OutboundBuffered 恒 0：测试连接无背压。
func (r *recordConn) OutboundBuffered() int { return 0 }
