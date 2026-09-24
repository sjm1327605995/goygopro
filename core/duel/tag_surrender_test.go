package duel

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestTagSurrenderResetOnNewTurn 守住原版 tag_duel.cpp:920-922 的语义：
// 每次 MSG_NEW_TURN 清空 4 个 surrender 标记——A 队队友本回合投降后，
// 新回合另一名队友再投降不应判负（两人投降判负只在同一回合内累积）。
// 旧 Go 实现只在 analyzeNewTurn 轮换 curPlayer，从不清投降标记。
func TestTagSurrenderResetOnNewTurn(t *testing.T) {
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
	for i := 0; i < 4; i++ {
		td.players[i].Type = uint8(i)
	}

	// 座位 0（A 队）本回合投降：只标记 + 通知队友，不判负。
	td.Surrender(td.players[0])
	if !td.surrender[0] {
		t.Fatal("座位 0 投降后 surrender[0] 未置位")
	}
	surrenderPkt := []byte{1, 0, network.STOC_TEAMMATE_SURRENDER}
	if !bytes.Contains(conns[0].buf, surrenderPkt) || !bytes.Contains(conns[1].buf, surrenderPkt) {
		t.Fatalf("座位 0 投降未通知本人与队友：seat0=%v seat1=%v", conns[0].buf, conns[1].buf)
	}

	// 新回合：投降标记必须全部复位。
	td.Analyze([]byte{ocgcore.MSG_NEW_TURN, 0})
	for i := 0; i < 4; i++ {
		if td.surrender[i] {
			t.Fatalf("MSG_NEW_TURN 后 surrender[%d] 仍为 true（投降标记未随回合复位）", i)
		}
	}

	// 新回合座位 1（座位 0 的队友）投降：因标记已复位，只应再次标记，
	// 不得触发「两人投降判负」的 MSG_WIN / EndDuel。
	for _, c := range conns {
		c.buf = nil
	}
	td.Surrender(td.players[1])
	if !td.surrender[1] || td.surrender[0] {
		t.Fatalf("新回合投降标记异常：surrender=%v", td.surrender)
	}
	if td.Duel == nil {
		t.Fatal("新回合单人投降后决斗被结束（surrender 未随回合复位导致误判负）")
	}
	for i, c := range conns {
		winPkt := []byte{ocgcore.MSG_WIN}
		if bytes.Contains(c.buf, winPkt) {
			t.Errorf("座位 %d 收到 MSG_WIN（跨回合投降被误判为两人投降判负）", i)
		}
	}

	// 同一回合内两人投降仍应判负（语义未变）。
	td.Surrender(td.players[0])
	if td.Duel != nil {
		t.Fatal("同回合两人投降未判负（surrender 语义被破坏）")
	}
	if !bytes.Contains(conns[0].buf, []byte{ocgcore.MSG_WIN, 1, 0}) {
		t.Errorf("两人投降后未广播 MSG_WIN(胜方=1)：seat0=%v", conns[0].buf)
	}
}
