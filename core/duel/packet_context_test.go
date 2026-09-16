package duel

import (
	"encoding/binary"
	"testing"

	"github.com/panjf2000/gnet/v2"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// recordConn 满足 gnet.Conn：嵌入空接口补齐方法，仅记录 Write 的字节，
// 用于断言 SendError/Reply 发出的线格式。
type recordConn struct {
	gnet.Conn
	buf []byte
}

func (r *recordConn) Write(p []byte) (int, error) {
	r.buf = append(r.buf, p...)
	return len(p), nil
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
	} {
		if _, ok := errmsgCode(code); ok {
			t.Errorf("errmsgCode(0x%02x) 不应有客户端错误码", code)
		}
	}
}

// TestSendErrorWireLayout 断言 STOC_ERROR_MSG 线格式：Msg 放 ERRMSG_*（非内部码），
// code 字段为 0，整体 8 字节；Reply 再加 2 字节包长前缀 + 1 字节 pktType。
func TestSendErrorWireLayout(t *testing.T) {
	conn := &recordConn{}
	ctx := NewPacketContext(&DuelPlayer{Conn: conn}, []byte{network.CTOS_JOIN_GAME})
	if err := ctx.SendError(ErrJoinFailed, "room not found"); err != nil {
		t.Fatalf("SendError: %v", err)
	}
	if len(conn.buf) != 2+1+8 {
		t.Fatalf("发送字节数 = %d, want %d", len(conn.buf), 2+1+8)
	}
	if pktLen := binary.LittleEndian.Uint16(conn.buf[0:2]); pktLen != 9 { // 1 + 8
		t.Fatalf("包长 = %d, want 9", pktLen)
	}
	if conn.buf[2] != network.STOC_ERROR_MSG {
		t.Fatalf("pktType = 0x%02x, want STOC_ERROR_MSG", conn.buf[2])
	}
	if conn.buf[3] != network.ERRMSG_JOINERROR {
		t.Fatalf("Msg = 0x%02x, want ERRMSG_JOINERROR", conn.buf[3])
	}
}

// TestSendErrorNoClientMsg 无映射的内部错误不产生客户端包（原版 netserver 对
// 协议违例同样只丢弃不回复，避免把内部分类码塞进协议）。
func TestSendErrorNoClientMsg(t *testing.T) {
	conn := &recordConn{}
	ctx := NewPacketContext(&DuelPlayer{Conn: conn}, []byte{network.CTOS_SURRENDER})
	if err := ctx.SendError(ErrBadRequest, "internal"); err != nil {
		t.Fatalf("SendError: %v", err)
	}
	if len(conn.buf) != 0 {
		t.Fatalf("不应发送客户端包，实际写入 %d 字节", len(conn.buf))
	}
}
