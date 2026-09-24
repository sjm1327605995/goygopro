package duel

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestJoinGameWrongPasswordReportsCode1 守住「密码即房间索引」设计下密码错误的
// 提示路径：服务器上存在房间但按密码查不到时，应回 JOINERROR code=1
// （原版 SysString 1404「密码错误」，前端 Lobby describeErrorMsg 的 code=1
// 分支），而不是 code=0 的「房间人数已满」。
func TestJoinGameWrongPasswordReportsCode1(t *testing.T) {
	sd := newSingleDuel(false)
	if _, created := DefaultManager.CreateRoom("secret", sd); !created {
		t.Fatal("setup: room already exists")
	}
	defer DefaultManager.RemoveRoom("secret")

	conn := &recordConn{}
	dp := &DuelPlayer{Conn: conn}
	c := NewPacketContext(dp, []byte{network.CTOS_JOIN_GAME})
	var pkt protocol.CTOSJoinGame
	copy(pkt.Pass[:], utf16.Encode([]rune("wrongpass")))
	pkt.Version = PRO_VERSION
	c.SetPayload(&pkt)

	HandleJoinGame(c)

	// 期望线格式：len(2)=9（含类型字节）| STOC_ERROR_MSG | ERRMSG_JOINERROR | pad(3) | code=1
	want := []byte{9, 0, network.STOC_ERROR_MSG, network.ERRMSG_JOINERROR, 0, 0, 0, 1, 0, 0, 0}
	if len(conn.buf) != len(want) {
		t.Fatalf("回包长度 = %d，期望 %d：%v", len(conn.buf), len(want), conn.buf)
	}
	for i := range want {
		if conn.buf[i] != want[i] {
			t.Fatalf("回包字节 %d = 0x%02x，期望 0x%02x：%v", i, conn.buf[i], want[i], conn.buf)
		}
	}
	if code := binary.LittleEndian.Uint32(conn.buf[7:]); code != 1 {
		t.Fatalf("JOINERROR code = %d，期望 1（密码错误）", code)
	}
}
