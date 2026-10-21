package duel

// 发送路径的语义锁定：sendFrame 的拷贝语义（共享 buff 是单飞工作区）与
// DuelPlayer.Write 的背压断开策略。fake conn 同步 emulate AsyncWrite 的
// 入队效果（生产路径由 gnet per-conn FIFO 保序）。

import (
	"bytes"
	"testing"

	"github.com/panjf2000/gnet/v2"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// fakeSendConn 记录 AsyncWrite 的字节，可配置 outbound 背压值与 Close 行为。
type fakeSendConn struct {
	gnet.Conn
	buf      []byte
	outbound int
	closed   bool
}

func (c *fakeSendConn) AsyncWrite(p []byte, _ gnet.AsyncCallback) error {
	c.buf = append(c.buf, p...)
	return nil
}

func (c *fakeSendConn) OutboundBuffered() int { return c.outbound }

func (c *fakeSendConn) Close() error {
	c.closed = true
	return nil
}

// TestSendFrameCopiesBuff：sendFrame 发出即拷贝——之后改写共享 buff 不得
// 影响已发出的帧；ReSend 语义（两次 sendFrame）各自持有独立拷贝。
func TestSendFrameCopiesBuff(t *testing.T) {
	conn := &fakeSendConn{}
	player := &DuelPlayer{Conn: conn, Type: 1}
	mode := &DuelMode{}

	mode.encodeFrame(network.STOC_GAME_MSG, []byte{0xaa, 0xbb})
	first := append([]byte(nil), mode.buff[:mode.buffOffset]...)
	mode.sendFrame(player)

	// 单飞工作区：改写 buff 不影响已发出的帧
	for i := 0; i < mode.buffOffset; i++ {
		mode.buff[i] = 0xee
	}
	if !bytes.Equal(conn.buf, first) {
		t.Fatalf("已发出的帧被共享 buff 改写污染:\n got %v\nwant %v", conn.buf, first)
	}

	// ReSend（不重新编码，直接复用当前 buff 内容）：第二次发送持有的是
	// 第二次拷贝，内容与当前 buff 一致
	second := append([]byte(nil), mode.buff[:mode.buffOffset]...)
	mode.ReSendToPlayer(player)
	if !bytes.Equal(conn.buf[len(first):], second) {
		t.Fatalf("ReSend 帧与发送时 buff 内容不符:\n got %v\nwant %v", conn.buf[len(first):], second)
	}
	if !bytes.Equal(conn.buf[:len(first)], first) {
		t.Fatalf("首帧在 ReSend 后被污染")
	}

	// 编码下一帧后，之前的所有已发帧保持各自的内容
	mode.encodeFrame(network.STOC_CHAT, []byte{1, 2, 3})
	third := append([]byte(nil), mode.buff[:mode.buffOffset]...)
	mode.sendFrame(player)
	if !bytes.Equal(conn.buf[2*len(first):], third) {
		t.Fatalf("第三帧内容不符")
	}
	if !bytes.Equal(conn.buf[:len(first)], first) || !bytes.Equal(conn.buf[len(first):2*len(first)], second) {
		t.Fatalf("历史帧被后续编码/发送污染")
	}
}

// TestSendFrameNilPlayer：nil 玩家静默跳过（原版语义），不 panic。
func TestSendFrameNilPlayer(t *testing.T) {
	mode := &DuelMode{}
	mode.encodeFrame(network.STOC_GAME_MSG, []byte{1})
	mode.sendFrame(nil) // 不得 panic
}

// TestWriteBackpressureClosesSlowClient：出站缓冲超过 maxOutboundBuffered
// 的慢客户端被断开（Conn.Close），Write 返回错误；正常连接不受影响。
func TestWriteBackpressureClosesSlowClient(t *testing.T) {
	slow := &fakeSendConn{outbound: maxOutboundBuffered + 1}
	slowPlayer := &DuelPlayer{Conn: slow, Type: 1}
	n, err := slowPlayer.Write([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("超限写应返回错误")
	}
	if n != 0 {
		t.Fatalf("失败写应返回 n=0，得到 %d", n)
	}
	if !slow.closed {
		t.Fatal("慢客户端应被 Close")
	}
	if len(slow.buf) != 0 {
		t.Fatalf("被断开连接的写不应入队，得到 %v", slow.buf)
	}

	// 阈值边界：恰好等于阈值不触发断开
	edge := &fakeSendConn{outbound: maxOutboundBuffered}
	edgePlayer := &DuelPlayer{Conn: edge, Type: 2}
	if n, err := edgePlayer.Write([]byte{9}); err != nil || n != 1 {
		t.Fatalf("阈值边界写应成功: n=%d err=%v", n, err)
	}
	if edge.closed || !bytes.Equal(edge.buf, []byte{9}) {
		t.Fatalf("阈值边界不应断开/丢字节: closed=%v buf=%v", edge.closed, edge.buf)
	}

	// 正常连接：字节入队、长度返回、不关闭
	ok := &fakeSendConn{}
	okPlayer := &DuelPlayer{Conn: ok, Type: 3}
	if n, err := okPlayer.Write([]byte{7, 7}); err != nil || n != 2 {
		t.Fatalf("正常写应成功: n=%d err=%v", n, err)
	}
	if ok.closed || !bytes.Equal(ok.buf, []byte{7, 7}) {
		t.Fatalf("正常写不应断开/丢字节: closed=%v buf=%v", ok.closed, ok.buf)
	}
}

// TestWriteNilConn：无连接玩家写返回错误（保持旧行为）。
func TestWriteNilConn(t *testing.T) {
	var p DuelPlayer
	if _, err := p.Write([]byte{1}); err == nil {
		t.Fatal("nil 连接应报错")
	}
}
