package duel

import (
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
)

// TestSingleGetResponseDeductsTimeBank 守住原版 single_duel.cpp:1419-1423 的
// 计时银行语义：响应耗时必须先从 timeLimit 扣减、再把 timeElapsed 清零。
// 旧 Go 实现在扣减前把 timeElapsed 置 0，导致 timeLimit -= timeElapsed
// 恒减 0，限时模式的时间银行永不扣减。
func TestSingleGetResponseDeductsTimeBank(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := ocgcore.Init(ocgcore.WithRootPath(root)); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}

	sd := newSingleDuel(false)
	sd.DuelStage = network.DUEL_STAGE_DUELING
	sd.Duel = ocgcore.NewDuel(1)
	if sd.Duel == nil {
		t.Fatal("create_duel returned nil")
	}
	defer func() {
		if sd.Duel != nil {
			sd.Duel.End()
		}
	}()
	sd.lastReplay = NewReplay()
	sd.HostInfo.TimeLimit = 180
	sd.timeLimit = [2]int16{100, 100}
	sd.lastResponse = 0

	conn := &recordConn{}
	dp := &DuelPlayer{Conn: conn, Type: 0, State: network.CTOS_RESPONSE}
	sd.players[0] = dp
	sd.players[1] = &DuelPlayer{Conn: &recordConn{}, Type: 1}

	// 模拟玩家 0 响应耗时 7 秒。
	sd.timeElapsed = 7
	sd.GetResponse(dp, []byte{0, 0, 0, 0})

	if sd.timeLimit[0] != 93 {
		t.Errorf("timeLimit[0] = %d，期望 93（响应耗时未从计时银行扣减）", sd.timeLimit[0])
	}
	if sd.timeElapsed != 0 {
		t.Errorf("timeElapsed = %d，期望扣减后清零", sd.timeElapsed)
	}
	// 对方银行不受影响。
	if sd.timeLimit[1] != 100 {
		t.Errorf("timeLimit[1] = %d，期望 100（不应被动）", sd.timeLimit[1])
	}

	// 耗时超过剩余时间时银行清零而非变负（原版 else 分支）。
	sd.timeLimit[0] = 5
	sd.timeElapsed = 9
	dp.State = network.CTOS_RESPONSE
	sd.GetResponse(dp, []byte{0, 0, 0, 0})
	if sd.timeLimit[0] != 0 {
		t.Errorf("timeLimit[0] = %d，期望超时扣减后清零", sd.timeLimit[0])
	}
}
