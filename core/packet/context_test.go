package packet

import (
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// stubPeer 满足 Peer 的最小桩：记录 Write 的字节，用于断言
// SendError/Reply 发出的线格式。Conn 层细节（gnet.Conn）与本包无关。
type stubPeer struct {
	buf         []byte
	shortWriteN int // >0 时 Write 只"写出"前 N 字节且 err=nil
}

func (p *stubPeer) GetID() string      { return "stub" }
func (p *stubPeer) LogName() string    { return "stub" }
func (p *stubPeer) PlayerState() uint8 { return 0 }
func (p *stubPeer) CurrentGame() Game  { return nil }
func (p *stubPeer) Write(b []byte) (int, error) {
	if p.shortWriteN > 0 {
		n := p.shortWriteN
		p.shortWriteN = 0
		return n, nil
	}
	p.buf = append(p.buf, b...)
	return len(b), nil
}

// TestErrmsgCode 守住「只有加入失败能映射到 ERRMSG_JOINERROR，其余内部码无
// 客户端可见错误码」——防止 SendError 又把内部分类码（0x10-0x18）塞进
// STOC_ERROR_MSG.Msg 造成协议错位。
func TestErrmsgCode(t *testing.T) {
	msg, ok := errmsgCode(ErrJoinFailed)
	if !ok || msg != network.ERRMSG_JOINERROR {
		t.Fatalf("errmsgCode(ErrJoinFailed) = 0x%02x, %v; want 0x%02x, true", msg, ok, network.ERRMSG_JOINERROR)
	}
	for _, code := range []uint8{
		ErrBadRequest, ErrPayloadTooShort, ErrBindFailed, ErrNotInGame,
		ErrDuelNotStarted, ErrDuelAlreadyStarted, ErrInvalidState, ErrAlreadyInGame,
		ErrPayloadTooLong,
	} {
		if _, ok := errmsgCode(code); ok {
			t.Errorf("errmsgCode(0x%02x) 不应有客户端错误码", code)
		}
	}
}

// TestSendErrorWireLayout 断言 STOC_ERROR_MSG 线格式：Msg 放 ERRMSG_*（非内部码），
// code 字段为 0，整体 8 字节；Reply 再加 2 字节包长前缀 + 1 字节 pktType。
func TestSendErrorWireLayout(t *testing.T) {
	peer := &stubPeer{}
	ctx := NewPacketContext(peer, []byte{network.CTOS_JOIN_GAME})
	if err := ctx.SendError(ErrJoinFailed, "room not found"); err != nil {
		t.Fatalf("SendError: %v", err)
	}
	if len(peer.buf) != 2+1+8 {
		t.Fatalf("发送字节数 = %d, want %d", len(peer.buf), 2+1+8)
	}
	if pktLen := binary.LittleEndian.Uint16(peer.buf[0:2]); pktLen != 9 { // 1 + 8
		t.Fatalf("包长 = %d, want 9", pktLen)
	}
	if peer.buf[2] != network.STOC_ERROR_MSG {
		t.Fatalf("pktType = 0x%02x, want STOC_ERROR_MSG", peer.buf[2])
	}
	if peer.buf[3] != network.ERRMSG_JOINERROR {
		t.Fatalf("Msg = 0x%02x, want ERRMSG_JOINERROR", peer.buf[3])
	}
}

// TestSendErrorNoClientMsg 无映射的内部错误不产生客户端包（原版 netserver 对
// 协议违例同样只丢弃不回复，避免把内部分类码塞进协议）。
func TestSendErrorNoClientMsg(t *testing.T) {
	peer := &stubPeer{}
	ctx := NewPacketContext(peer, []byte{network.CTOS_SURRENDER})
	if err := ctx.SendError(ErrBadRequest, "internal"); err != nil {
		t.Fatalf("SendError: %v", err)
	}
	if len(peer.buf) != 0 {
		t.Fatalf("不应发送客户端包，实际写入 %d 字节", len(peer.buf))
	}
}

// TestReplyPayloadLimit 守住 Reply 的 2 字节包长前缀上限：超限必须拒绝
// （返回错误且一个字节都不写），而不是长度回绕让客户端按错误长度切帧、
// 后续所有包解析错位。
func TestReplyPayloadLimit(t *testing.T) {
	peer := &stubPeer{}
	ctx := NewPacketContext(peer, []byte{network.CTOS_JOIN_GAME})
	if err := ctx.Reply(network.STOC_JOIN_GAME, make([]byte, 0xFFFF)); err == nil {
		t.Fatal("0xFFFF 字节 payload 应被拒绝")
	}
	if len(peer.buf) != 0 {
		t.Fatalf("拒绝后不应写连接，实际写入 %d 字节", len(peer.buf))
	}

	// 边界：0xFFFE 是允许的最大 payload（包长前缀恰好 0xFFFF 不回绕）
	bigPeer := &stubPeer{}
	bigCtx := NewPacketContext(bigPeer, []byte{network.CTOS_JOIN_GAME})
	if err := bigCtx.Reply(network.STOC_JOIN_GAME, make([]byte, 0xFFFE)); err != nil {
		t.Fatalf("0xFFFE 字节 payload 应允许: %v", err)
	}
	if got := binary.LittleEndian.Uint16(bigPeer.buf[0:2]); got != 0xFFFF {
		t.Fatalf("包长前缀 = 0x%04x, want 0xffff", got)
	}
}

// TestReplyShortWrite 守住 Reply 的短写检查：Write 返回的 n 小于缓冲
// 长度且无错误时必须报 io.ErrShortWrite——静默吞掉会让客户端拿到截断的帧。
func TestReplyShortWrite(t *testing.T) {
	peer := &stubPeer{shortWriteN: 2} // 只写出包长前缀的两个字节
	ctx := NewPacketContext(peer, []byte{network.CTOS_JOIN_GAME})
	err := ctx.Reply(network.STOC_JOIN_GAME, []byte{1, 2, 3})
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("err = %v, want io.ErrShortWrite", err)
	}
}
