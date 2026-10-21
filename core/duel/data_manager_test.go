package duel

import (
	"testing"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestCheckMsgSizeAllowsEmptyMessage 对齐原版 CreateChatPacket
//（netserver.cpp:374-386）：只校验 NUL 结尾，仅含终止符的空消息（2 字节）
// 照发；旧实现要求至少 4 字节，把空消息拒掉。
func TestCheckMsgSizeAllowsEmptyMessage(t *testing.T) {
	if !CheckMsgSize(2) {
		t.Fatal("空聊天消息（2 字节 NUL）应被接受（原版只查 NUL 结尾）")
	}
	for _, size := range []int{0, 1, 3, LenChatMsg*SizeOfUint16 + 2} {
		if CheckMsgSize(size) {
			t.Fatalf("非法聊天消息长度 %d 应被拒绝", size)
		}
	}
	if !CheckMsgSize(4) {
		t.Fatal("单字符消息（4 字节）应被接受")
	}
	_ = network.STOC_CHAT // 保持 import 与包契约关联
}
