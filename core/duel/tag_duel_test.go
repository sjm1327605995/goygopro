package duel

import (
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestTagUpdateDeckAllocatesPDeck 守住 tag 模式的卡组兜底分配：pDeck 是
// [4]*Deck 初值 nil（原版 tag_duel.cpp:281 的 pdeck 是值成员永远存在），
// 旧实现直接传 nil 给 LoadDeck 在首次 deck.Extra/Main 访问即 panic，
// 导致 tag 房间实际无法开赛。
func TestTagUpdateDeckAllocatesPDeck(t *testing.T) {
	td := newTagDuel()
	dp := &DuelPlayer{Type: 0, Conn: &recordConn{}}
	// 空卡组包：MainC=0 / SideC=0（8 字节头），不触碰卡库。
	td.UpdateDeck(dp, make([]byte, 8))
	if td.pDeck[0] == nil {
		t.Fatal("UpdateDeck 后 pDeck[0] 仍为 nil（tag_duel.cpp:281 值成员语义的兜底分配缺失）")
	}
	if td.DeckError[0] != 0 {
		t.Fatalf("空卡组不应报错：DeckError=%d", td.DeckError[0])
	}
	// 重复上传必须清空重载（原版 LoadDeck 入口即 deck.clear()），不得叠加。
	td.pDeck[0].Main = append(td.pDeck[0].Main, &CardDataC{})
	td.UpdateDeck(dp, make([]byte, 8))
	if len(td.pDeck[0].Main) != 0 {
		t.Fatal("重复 UpdateDeck 未清空旧卡组（原版 deck.clear() 语义缺失）")
	}
}

// TestTagGetResponseTruncatesOversize 守住 tag_duel.cpp:1519-1520 的响应
// 截断：超过 SIZE_RETURN_VALUE 的响应直接写回放缓冲区（resb 定长）会切片
// 越界 panic；截断后正常提交引擎。
func TestTagGetResponseTruncatesOversize(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := ocgcore.Init(ocgcore.WithRootPath(root)); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}

	td, _ := newTimedTagDuelForTest(t)
	td.Duel = ocgcore.NewDuel(1)
	if td.Duel == nil {
		t.Fatal("create_duel returned nil")
	}
	defer td.Duel.End()
	td.lastReplay = NewReplay()

	dp := td.players[0]
	dp.Type = 0
	dp.State = network.CTOS_RESPONSE
	td.curPlayer[0] = dp
	td.lastResponse = 0

	oversize := make([]byte, ocgcore.SIZE_RETURN_VALUE+64)
	for i := range oversize {
		oversize[i] = byte(i)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("超长响应未截断导致 panic：%v", r)
		}
	}()
	td.GetResponse(dp, oversize)
}

// TestWaitForResponseKeepsTimeElapsed 守住原版 WaitforResponse 不触碰
// time_elapsed 的语义（single_duel.cpp:1443-1456）：应答方已流逝的计时不
// 因进入等待而白赚重置；清零只发生在扣减后（GetResponse）与 TimeConfirm
// 的 10 秒兜底。
func TestWaitForResponseKeepsTimeElapsed(t *testing.T) {
	td, _ := newTimedTagDuelForTest(t)
	td.HostInfo.TimeLimit = 60
	td.timeLimit = [2]int16{60, 60}
	td.timeElapsed = 7

	waitForResponse(td, 0)
	if td.timeElapsed != 7 {
		t.Fatalf("waitForResponse 重置了 timeElapsed：%d（原版 WaitforResponse 不触碰它）", td.timeElapsed)
	}
}
