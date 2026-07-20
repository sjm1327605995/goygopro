package protocol

import (
	"bytes"
	"testing"
)

// restruct 只认指针：给它一个非指针的结构体，它会打出一串零而**不报错**。
//
// 这个坑埋了很久：客户端所有 SendStruct 都是值传递，于是 CTOS_JOIN_GAME 的版本号、
// CTOS_PLAYER_INFO 的玩家名、CTOS_CREATE_GAME 的房间参数、猜拳与先后攻的选择
// 全部发成了 0 —— 连不上服务器，或者选了什么都不算数，而代码看上去完全正常。
//
// PackGameMsg 现在自己兜住这件事。这个测试钉住「值和指针打出来一样」。
func TestPackGameMsgAcceptsValueAndPointer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		val     interface{}
		ptr     interface{}
		wantLen int
	}{
		{"踢人", CTOSKick{Pos: 2}, &CTOSKick{Pos: 2}, 1},
		{"猜拳", CTOSHandResult{Res: 3}, &CTOSHandResult{Res: 3}, 1},
		{"先后攻", CTOSTPResult{Res: 1}, &CTOSTPResult{Res: 1}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			byVal := PackGameMsg(tc.val)
			byPtr := PackGameMsg(tc.ptr)

			if !bytes.Equal(byVal, byPtr) {
				t.Errorf("值传递 = %v, 指针传递 = %v —— 两者必须一致", byVal, byPtr)
			}
			if len(byVal) != tc.wantLen {
				t.Fatalf("长度 = %d, want %d", len(byVal), tc.wantLen)
			}
			// 关键：非零字段不能被打成 0
			if bytes.Equal(byVal, make([]byte, len(byVal))) {
				t.Error("打包结果全是 0 —— 字段没有被写进去")
			}
		})
	}
}

// 多字段结构体同样要能按值打包：房间参数有十几个字段，全 0 意味着建出来的房间
// 规则全错（LP 0、手牌 0），服务端多半直接拒绝。
func TestPackGameMsgMultiFieldByValue(t *testing.T) {
	info := HostInfo{
		LFList:    12345,
		Rule:      2,
		Mode:      1,
		StartLp:   8000,
		StartHand: 5,
		DrawCount: 1,
		TimeLimit: 180,
	}

	got := PackGameMsg(info)

	if len(got) == 0 {
		t.Fatal("打包结果为空")
	}
	if bytes.Equal(got, make([]byte, len(got))) {
		t.Fatal("房间参数被打成全 0 —— 建出来的房间规则全错")
	}
	// 往返验证：解回来应当与原值一致
	var back HostInfo
	if err := UnpackGameMsg(got, &back); err != nil {
		t.Fatalf("解包失败: %v", err)
	}
	if back.StartLp != info.StartLp || back.TimeLimit != info.TimeLimit ||
		back.LFList != info.LFList || back.StartHand != info.StartHand {
		t.Errorf("往返后 = %+v, 原值 = %+v", back, info)
	}
}
