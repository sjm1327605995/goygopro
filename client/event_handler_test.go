package client

import (
	"encoding/binary"
	"testing"

	"github.com/sjm1327605995/goygopro/protocol/network"
)

// 这些消息此前完全没有分发（ClientAnalyze 会落到 default 打印 "Unhandled MSG type"），
// 是照着 C++ duelclient.cpp 补的。测试喂真实字节，验证按协议解析到位。

func newTestField() {
	MainGame.DField = NewClientField()
	MainGame.DInfo = DuelInfo{}
	MainGame.DField.Initial(0, 40, 15, 0)
	MainGame.DField.Initial(1, 40, 15, 0)
}

// MSG_WIN 必须置 IsFinished —— 决斗结束后界面要停止接受操作。
func TestHandleWin(t *testing.T) {
	for _, tc := range []struct {
		name    string
		player  uint8
		winType uint8
		want    string
	}{
		{"自己获胜", 0, 0x00, "LP 归零"},
		{"平局", 2, 0x00, "平局"},
		{"与玩家无关的原因不带名字", 0, 0x10, "胜利条件 16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newTestField()
			MainGame.DInfo.ClientName = "我"
			MainGame.DInfo.HostName = "对手"

			Client.handleWin([]byte{tc.player, tc.winType})

			if !MainGame.DInfo.IsFinished {
				t.Error("MSG_WIN 之后 IsFinished 仍为 false，界面不会知道决斗已结束")
			}
			if MainGame.DInfo.WinPlayer != int(tc.player) {
				t.Errorf("WinPlayer = %d, want %d", MainGame.DInfo.WinPlayer, tc.player)
			}
			if got := MainGame.DInfo.VicString; got == "" || !contains(got, tc.want) {
				t.Errorf("VicString = %q, 期望含 %q", got, tc.want)
			}
		})
	}
}

// MSG_CARD_HINT 的 DESC_ADD/REMOVE 是配对的计数，减到 0 必须删键 ——
// 留着 0 值会让「这张卡有提示吗」的判断永远为真。
func TestHandleCardHintDescAddRemove(t *testing.T) {
	newTestField()
	card := NewClientCard()
	card.Controler, card.Location, card.Sequence = 0, 0x04, 0
	MainGame.DField.MZone[0][0] = card

	const desc = 1234
	Client.handleCardHint(cardHintBytes(0, 0x04, 0, 6 /*CHINT_DESC_ADD*/, desc))
	if card.DescHints[desc] != 1 {
		t.Fatalf("DESC_ADD 后计数 = %d, want 1", card.DescHints[desc])
	}

	Client.handleCardHint(cardHintBytes(0, 0x04, 0, 7 /*CHINT_DESC_REMOVE*/, desc))
	if _, ok := card.DescHints[desc]; ok {
		t.Errorf("DESC_REMOVE 减到 0 后键仍在（值 %d）—— 应当删除", card.DescHints[desc])
	}
}

// 其余 chtype 走的是另一条路：记在 CHint/ChValue 上，不进 DescHints。
func TestHandleCardHintOtherTypes(t *testing.T) {
	newTestField()
	card := NewClientCard()
	card.Controler, card.Location, card.Sequence = 0, 0x04, 0
	MainGame.DField.MZone[0][0] = card

	Client.handleCardHint(cardHintBytes(0, 0x04, 0, 1 /*CHINT_TURN*/, 3))

	if card.CHint != 1 || card.ChValue != 3 {
		t.Errorf("CHint/ChValue = %d/%d, want 1/3", card.CHint, card.ChValue)
	}
	if len(card.DescHints) != 0 {
		t.Error("CHINT_TURN 不该写进 DescHints")
	}
}

// CARD_QUESTION 是哨兵值，含义是「不能查看墓地」，而不是一条普通提示。
func TestHandlePlayerHintCardQuestionTogglesGrave(t *testing.T) {
	newTestField()

	Client.handlePlayerHint(playerHintBytes(0, 6 /*PHINT_DESC_ADD*/, CardQuestion))
	if !MainGame.DField.CantCheckGrave {
		t.Error("CARD_QUESTION + DESC_ADD 应当禁掉查看墓地")
	}
	if len(MainGame.DField.PlayerDescHints[0]) != 0 {
		t.Error("哨兵值不该混进 PlayerDescHints")
	}

	Client.handlePlayerHint(playerHintBytes(0, 7 /*PHINT_DESC_REMOVE*/, CardQuestion))
	if MainGame.DField.CantCheckGrave {
		t.Error("DESC_REMOVE 应当恢复查看墓地")
	}
}

func TestHandlePlayerHintNormalDesc(t *testing.T) {
	newTestField()
	const desc = 555

	Client.handlePlayerHint(playerHintBytes(0, 6, desc))
	Client.handlePlayerHint(playerHintBytes(0, 6, desc))
	if got := MainGame.DField.PlayerDescHints[0][desc]; got != 2 {
		t.Errorf("两次 ADD 后计数 = %d, want 2", got)
	}

	Client.handlePlayerHint(playerHintBytes(0, 7, desc))
	if got := MainGame.DField.PlayerDescHints[0][desc]; got != 1 {
		t.Errorf("一次 REMOVE 后计数 = %d, want 1", got)
	}
	Client.handlePlayerHint(playerHintBytes(0, 7, desc))
	if _, ok := MainGame.DField.PlayerDescHints[0][desc]; ok {
		t.Error("减到 0 后键仍在 —— 应当删除")
	}
}

// MSG_CONFIRM_EXTRATOP 从额外卡组末尾往前数，并跳过已翻开的（ExtraPCount）。
// 数错方向或漏掉这个偏移，翻开的就是别的卡。
func TestHandleConfirmExtratop(t *testing.T) {
	newTestField()
	extra := MainGame.DField.Extra[0]
	MainGame.DField.ExtraPCount[0] = 2 // 末尾 2 张是表侧的灵摆怪，应当跳过

	const code = 99999
	buf := []byte{0 /*player*/, 1 /*count*/}
	buf = binary.LittleEndian.AppendUint32(buf, code)
	buf = append(buf, 0, 0, 0) // 位置信息 3 字节

	Client.handleConfirmExtratop(buf)

	want := extra[len(extra)-1-0-2] // 末尾往前，跳过 2 张
	if want.Code != code {
		t.Errorf("被翻开的卡 code = %d, want %d —— 取卡位置算错了", want.Code, code)
	}
	if n := len(MainGame.DField.SelectableCards); n != 1 {
		t.Fatalf("SelectableCards 有 %d 张, want 1", n)
	}
	if MainGame.DField.SelectableCards[0] != want {
		t.Error("SelectableCards 里放的不是被翻开的那张")
	}
}

func TestHandleWaitingSetsHint(t *testing.T) {
	newTestField()
	MainGame.WaitFrame = 42
	Client.handleWaiting()
	if MainGame.WaitFrame != 0 {
		t.Errorf("WaitFrame = %d, want 0", MainGame.WaitFrame)
	}
	if MainGame.ShowingText == "" {
		t.Error("MSG_WAITING 应当给出等待提示文本")
	}
}

// 短包不能让客户端崩掉：网络上什么都可能来。
func TestHandlersRejectShortBuffers(t *testing.T) {
	newTestField()
	for name, fn := range map[string]func([]byte) bool{
		"win":             Client.handleWin,
		"cardHint":        Client.handleCardHint,
		"playerHint":      Client.handlePlayerHint,
		"confirmExtraTop": Client.handleConfirmExtratop,
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("空包导致 panic: %v", r)
				}
			}()
			fn(nil)
			fn([]byte{0})
		})
	}
}

func cardHintBytes(player, loc, seq, chType byte, value uint32) []byte {
	b := []byte{player, loc, seq, 0, chType}
	return binary.LittleEndian.AppendUint32(b, value)
}

func playerHintBytes(player, chType byte, value uint32) []byte {
	b := []byte{player, chType}
	return binary.LittleEndian.AppendUint32(b, value)
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// 「等待对方操作」的提示必须在下一条消息到来时撤掉，否则它会一直挂在界面上。
// 这条清理写在 ClientAnalyze 入口（对应 C++ duelclient.cpp 同一位置），
// 跨消息的时序行为最容易在重构里悄悄丢掉。
func TestWaitingHintClearedByNextMessage(t *testing.T) {
	newTestField()

	Client.ClientAnalyze([]byte{network.MSG_WAITING})
	if MainGame.ShowingText == "" {
		t.Fatal("MSG_WAITING 之后应当有等待提示")
	}

	// 任意一条别的消息（这里用 MSG_WIN）都该把它撤掉
	Client.ClientAnalyze([]byte{network.MSG_WIN, 0, 0})
	if MainGame.ShowingText != "" {
		t.Errorf("下一条消息到来后提示仍在: %q", MainGame.ShowingText)
	}
}

// MSG_CARD_SELECTED 是对方选卡过程中的播报，等待并没有结束 —— 提示要留着。
func TestWaitingHintSurvivesCardSelected(t *testing.T) {
	newTestField()

	Client.ClientAnalyze([]byte{network.MSG_WAITING})
	before := MainGame.ShowingText

	Client.ClientAnalyze([]byte{network.MSG_CARD_SELECTED, 0})
	if MainGame.ShowingText != before {
		t.Errorf("MSG_CARD_SELECTED 不该撤掉等待提示，现在是 %q", MainGame.ShowingText)
	}
}
