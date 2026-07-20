package client

import (
	"testing"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 「取消/完成」决定玩家怎么从各种选择里退出来。此前 Go 侧完全没有这套东西 ——
// 最要命的是连锁：服务器每问一次「要不要连锁」，客户端只列出可连锁的卡，
// 没有说「不」的出口，对方每发动一个效果玩家都被迫连锁一张。
//
// 这些测试断言的是「按了取消到底发出什么字节」，用 sendHook 录下来。

func newCancelField() *ClientField {
	MainGame.DField = NewClientField()
	MainGame.DInfo = DuelInfo{}
	return MainGame.DField
}

// 不连锁 = 回 -1。强制连锁时没得选，取消不该有任何动作。
func TestCancelChain(t *testing.T) {
	t.Run("可以不连锁", func(t *testing.T) {
		cf := newCancelField()
		got := captureSends(t)

		if !cf.CanCancel(network.MSG_SELECT_CHAIN) {
			t.Fatal("非强制连锁时应当能选择不连锁")
		}
		if cf.CancelOrFinish(network.MSG_SELECT_CHAIN) != CancelResponded {
			t.Fatal("不连锁应当回应服务器")
		}
		assertResponseI(t, *got, -1)
	})

	t.Run("强制连锁不能取消", func(t *testing.T) {
		cf := newCancelField()
		cf.ChainForced = true
		got := captureSends(t)

		if cf.CanCancel(network.MSG_SELECT_CHAIN) {
			t.Error("强制连锁时不该显示「不连锁」")
		}
		if cf.CancelOrFinish(network.MSG_SELECT_CHAIN) != CancelNothing {
			t.Error("强制连锁时取消不该有动作")
		}
		if len(*got) != 0 {
			t.Errorf("强制连锁时不该发包，却发了 %d 个", len(*got))
		}
	})

	t.Run("按钮文案是不连锁", func(t *testing.T) {
		cf := newCancelField()
		if got := cf.CancelLabel(network.MSG_SELECT_CHAIN); got != "不连锁" {
			t.Errorf("文案 = %q, want 不连锁", got)
		}
	})
}

// 问句（是否发动效果）取消 = 回「否」。
func TestCancelYesNo(t *testing.T) {
	for _, msg := range []int16{network.MSG_SELECT_YESNO, network.MSG_SELECT_EFFECTYN} {
		cf := newCancelField()
		got := captureSends(t)

		if cf.CancelOrFinish(msg) != CancelResponded {
			t.Fatalf("msg=%d 取消应当回应", msg)
		}
		assertResponseI(t, *got, 0)
	}
}

// 选卡：一张没选且允许取消 → 回 -1；已选够 → 提交已选的卡。
func TestCancelSelectCard(t *testing.T) {
	t.Run("没选卡时取消", func(t *testing.T) {
		cf := newCancelField()
		cf.SelectCancelable = true
		got := captureSends(t)

		cf.CancelOrFinish(network.MSG_SELECT_CARD)
		assertResponseI(t, *got, -1)
	})

	t.Run("不允许取消时无动作", func(t *testing.T) {
		cf := newCancelField()
		cf.SelectCancelable = false
		got := captureSends(t)

		if cf.CancelOrFinish(network.MSG_SELECT_CARD) != CancelNothing {
			t.Error("不可取消的选择不该被取消掉")
		}
		if len(*got) != 0 {
			t.Error("不该发包")
		}
	})

	t.Run("选够了是完成", func(t *testing.T) {
		cf := newCancelField()
		a, b := NewClientCard(), NewClientCard()
		a.SelectSeq, b.SelectSeq = 3, 7
		cf.SelectedCards = []*ClientCard{a, b}
		cf.SelectReady = true
		got := captureSends(t)

		if lbl := cf.CancelLabel(network.MSG_SELECT_CARD); lbl != "完成" {
			t.Errorf("选够时文案 = %q, want 完成", lbl)
		}
		cf.CancelOrFinish(network.MSG_SELECT_CARD)

		// 回的是 [张数, 各卡的 SelectSeq...]
		assertResponseB(t, *got, []byte{2, 3, 7})
	})
}

// 凑数值只有「完成」：没凑够时取消不该有动作，否则服务器会拒绝。
func TestCancelSelectSum(t *testing.T) {
	cf := newCancelField()
	cf.SelectReady = false
	got := captureSends(t)

	if cf.CanCancel(network.MSG_SELECT_SUM) {
		t.Error("没凑够时不该显示完成按钮")
	}
	if cf.CancelOrFinish(network.MSG_SELECT_SUM) != CancelNothing {
		t.Error("没凑够时不该提交")
	}
	if len(*got) != 0 {
		t.Error("不该发包")
	}
}

// 选择位置：取消回一个空位置。
func TestCancelSelectPlace(t *testing.T) {
	cf := newCancelField()
	cf.SelectCancelable = true
	cf.SelectableField = 0xff
	got := captureSends(t)

	cf.CancelOrFinish(network.MSG_SELECT_PLACE)

	assertResponseB(t, *got, []byte{0, 0, 0})
	if cf.SelectableField != 0 {
		t.Error("取消后应当清掉可选位置，否则格子还亮着")
	}
}

// SetResponseSelectedCards 回的是 SelectSeq 而不是数组下标 ——
// 凑数值那步两者不同（must_select 的卡编号都是 0）。
func TestSetResponseSelectedCardsUsesSelectSeq(t *testing.T) {
	cf := newCancelField()
	a, b, c := NewClientCard(), NewClientCard(), NewClientCard()
	a.SelectSeq, b.SelectSeq, c.SelectSeq = 0, 0, 5 // 两张 must_select + 一张可选
	cf.SelectedCards = []*ClientCard{a, b, c}
	got := captureSends(t)

	cf.SetResponseSelectedCards()
	Client.SendResponse()

	assertResponseB(t, *got, []byte{3, 0, 0, 5})
}

// ---- 断言辅助 ----

func assertResponseI(t *testing.T, sent []sentPacket, want int32) {
	t.Helper()
	if len(sent) != 1 {
		t.Fatalf("发出了 %d 个包, want 1", len(sent))
	}
	p := sent[0]
	if p.proto != network.CTOS_RESPONSE {
		t.Fatalf("包类型 = %d, want CTOS_RESPONSE", p.proto)
	}
	if len(p.payload) < 4 {
		t.Fatalf("负载 %d 字节, 整数响应应当 4 字节", len(p.payload))
	}
	got := int32(uint32(p.payload[0]) | uint32(p.payload[1])<<8 |
		uint32(p.payload[2])<<16 | uint32(p.payload[3])<<24)
	if got != want {
		t.Errorf("回应 = %d, want %d", got, want)
	}
}

func assertResponseB(t *testing.T, sent []sentPacket, want []byte) {
	t.Helper()
	if len(sent) != 1 {
		t.Fatalf("发出了 %d 个包, want 1", len(sent))
	}
	p := sent[0]
	if p.proto != network.CTOS_RESPONSE {
		t.Fatalf("包类型 = %d, want CTOS_RESPONSE", p.proto)
	}
	if len(p.payload) < len(want) {
		t.Fatalf("负载 = %v, want 前 %d 字节是 %v", p.payload, len(want), want)
	}
	for i := range want {
		if p.payload[i] != want[i] {
			t.Errorf("负载 = %v, want 前 %d 字节是 %v", p.payload[:len(want)], len(want), want)
			return
		}
	}
}
