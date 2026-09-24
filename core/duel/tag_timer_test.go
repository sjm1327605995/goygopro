package duel

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// newTimedTagDuelForTest 构造一个限时开启、四名座位均为记录型桩连接的
// tag 房间（处于 DUELING 阶段，回放已就绪），供 TagTimer 测试使用。
// recordConn 复用 packet_context_test.go 的定义。
func newTimedTagDuelForTest(t *testing.T) (*TagDuel, []*recordConn) {
	t.Helper()
	td := newTagDuel()
	td.DuelStage = network.DUEL_STAGE_DUELING
	conns := make([]*recordConn, 4)
	for i := 0; i < 4; i++ {
		conns[i] = &recordConn{}
		td.players[i] = &DuelPlayer{Conn: conns[i]}
	}
	td.curPlayer[0] = td.players[0]
	td.curPlayer[1] = td.players[2]
	td.Observers = map[string]*DuelPlayer{}
	td.lastReplay = NewReplay()
	return td, conns
}

// TestTagTimerRearmsEverySecond 守住原版 TagTimer 的每秒重武装语义
//（source/ygopro/gframe/tag_duel.cpp:1740-1741 的 event_add）：非超时路径
// 必须重新武装定时器，否则计时器走一秒后停走、超时判负永不触发。
// 这正是旧 Go 实现的回归场景。
func TestTagTimerRearmsEverySecond(t *testing.T) {
	td, _ := newTimedTagDuelForTest(t)
	// 哨兵 Duel 指针即可：timeLimit 足够大，本测试窗口内不会进超时路径，
	// 因而不会触碰引擎；超时路径的引擎交互由 TestTagTimerTimeoutEndsDuel 覆盖。
	td.Duel = &ocgcore.Duel{}
	td.timeLimit = [2]int16{60, 60}
	td.lastResponse = 0

	td.TagTimer() // timeElapsed 0→1，未超时

	// 重武装的定时器必须继续走秒：等到 timeElapsed 推进到 3
	//（初始 1 tick + 两次重武装后的 tick）。旧实现不重武装，计时器
	// 永远停在 1。轮询在锁内同时读 armed，区分「回归（tick 后未武装）」
	// 与「到点未触发（纯环境慢）」。
	deadline := time.Now().Add(10 * time.Second)
	for {
		td.Mu.Lock()
		elapsed, armed := td.timeElapsed, td.ETimer != nil
		td.Mu.Unlock()
		if elapsed >= 3 {
			break
		}
		if !armed {
			t.Fatalf("非超时 tick 后定时器未重新武装（原版 tag_duel.cpp:1740 的 event_add 等价物缺失）：elapsed=%d Duel=%p stage=%d limit=%v lastResponse=%d",
				elapsed, td.Duel, td.DuelStage, td.timeLimit, td.lastResponse)
		}
		if time.Now().After(deadline) {
			t.Fatalf("重武装后计时器停走：timeElapsed=%d，期望推进到 3", elapsed)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 清理：掐停重武装链。已入队未触发的回调拿到锁后见 Duel==nil
	// 走「决斗已结束」早退分支，不会再次武装。
	td.Mu.Lock()
	if td.ETimer != nil {
		td.ETimer.Stop()
		td.ETimer = nil
	}
	td.Duel = nil
	td.Mu.Unlock()
}

// TestTagTimerTimeoutEndsDuel 走通超时判负全链路：timeElapsed 达到限时后
// TagTimer 必须广播 MSG_WIN(原因 0x3)、收尾决斗（STOC_REPLAY + STOC_DUEL_END，
// 阶段推进到 END）、停表并清空引擎引用——与原版 tag_duel.cpp:1725-1738 一致。
func TestTagTimerTimeoutEndsDuel(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := ocgcore.Init(ocgcore.WithRootPath(root)); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}

	td, conns := newTimedTagDuelForTest(t)
	td.Duel = ocgcore.NewDuel(1)
	if td.Duel == nil {
		t.Fatal("create_duel returned nil")
	}
	td.timeLimit = [2]int16{2, 2}
	td.timeElapsed = 1
	td.lastResponse = 0

	td.TagTimer() // 1→2 达到限时 → 超时判负

	td.Mu.Lock()
	duelCleared := td.Duel == nil
	stage := td.DuelStage
	td.Mu.Unlock()

	if !duelCleared {
		t.Error("超时后引擎引用未清空（EndDuel 未生效）")
	}
	if stage != network.DUEL_STAGE_END {
		t.Errorf("超时后阶段 = %d，期望 DUEL_STAGE_END(%d)", stage, network.DUEL_STAGE_END)
	}

	// 座位 0 收到完整判负收尾：WIN 包 + 回放广播 + DUEL_END。
	got := conns[0].buf
	winPkt := []byte{4, 0, network.STOC_GAME_MSG, ocgcore.MSG_WIN, 1, 0x3}
	if !bytes.Contains(got, winPkt) {
		t.Errorf("座位 0 未收到 MSG_WIN 判负包 %v，实际写出 %d 字节", winPkt, len(got))
	}
	duelEndPkt := []byte{1, 0, network.STOC_DUEL_END}
	if !bytes.Contains(got, duelEndPkt) {
		t.Errorf("座位 0 未收到 STOC_DUEL_END 包 %v", duelEndPkt)
	}
	// 座位 1-3 经 ReSend 收到同样的 WIN 与 DUEL_END。
	for i := 1; i < 4; i++ {
		if peer := conns[i].buf; !bytes.Contains(peer, winPkt) || !bytes.Contains(peer, duelEndPkt) {
			t.Errorf("座位 %d 未收到判负广播（WIN/DUEL_END 缺失）", i)
		}
	}
}
