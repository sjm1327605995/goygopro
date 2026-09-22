package duel

// 波 J：SingleSession 单人解谜驱动测试。
// 用仓库自带的谜题脚本「青眼一击」真实驱动引擎：响应通道喂入制胜走线
// （进战斗阶段 → 青眼攻击恶魔的召唤），断言引擎走到 MSG_WIN 且对局完结；
// 另用永不作答 + 关闭 stopCh 驱动「退出对局」的真实状态变化。

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

func initSingleEngine(t *testing.T) {
	t.Helper()
	// 单人解谜现在会录制录像到 ./replay（Prepare 时就 BeginRecord），测试后
	// 清理掉，避免在仓库里留下 core/duel/replay/。与 replay_test.go 的做法一致。
	t.Cleanup(func() { _ = os.RemoveAll("./replay") })
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultDataManager.LoadDB(filepath.Join(root, "cards.cdb")); err != nil {
		t.Fatalf("load cards.cdb: %v", err)
	}
	if err := ocgcore.Init(
		ocgcore.WithRootPath(root),
		ocgcore.WithScriptDirectory(filepath.Join(root, "script")),
		ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
			return DefaultDataManager.GetData(cardId)
		}),
	); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}
}

func newRandomSeed() [8]uint32 {
	var s [8]uint32
	for i := range s {
		s[i] = uint32(i*0x9e3779b9 + 12345)
	}
	return s
}

func TestSingleSessionPlayPuzzle(t *testing.T) {
	initSingleEngine(t)

	ss := NewSingleSession(newRandomSeed())
	if err := ss.Prepare("青眼一击.lua"); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// 谜题布场后：我方卡组 0 张（残局无卡组）、对方手牌 1 张
	if got := ss.DeckCount(0, ocgcore.LOCATION_DECK); got != 0 {
		t.Fatalf("player0 deck count = %d, want 0", got)
	}
	if got := ss.DeckCount(1, ocgcore.LOCATION_HAND); got != 1 {
		t.Fatalf("player1 hand count = %d, want 1", got)
	}

	respCh := make(chan []byte, 4)
	stopCh := make(chan struct{})
	var sawWin, sawIdleCmd, sawBattleCmd bool
	var lastMsg byte
	attacksLeft := 1

	// 保险丝：响应流卡死时自动退出，测试报错而不是挂死
	go func() {
		<-time.After(60 * time.Second)
		close(stopCh)
	}()

	err := ss.Run(func(msg []byte) {
		p := 0
		for p < len(msg) {
			op := msg[p]
			lastMsg = op
			switch op {
			case ocgcore.MSG_SELECT_IDLECMD:
				sawIdleCmd = true
				respCh <- encodeI(6) // (0<<16)|6 → 进入战斗阶段
				return
			case ocgcore.MSG_SELECT_BATTLECMD:
				sawBattleCmd = true
				if attacksLeft > 0 {
					attacksLeft--
					respCh <- encodeI(1) // (0<<16)|1 → 攻击列表第 0 个：青眼→恶魔
				} else {
					respCh <- encodeI(3) // (0<<16)|3 → 已无攻击目标，进结束阶段
				}
				return
			case ocgcore.MSG_SELECT_CHAIN:
				// 不连锁（playerop.cpp SelectChain：-1 = pass）
				respCh <- encodeI(-1)
				return
			case ocgcore.MSG_SELECT_CARD:
				// 攻击目标选择：count=1 + 序号 0（select_card 响应格式）
				respCh <- []byte{1, 0}
				return
			case ocgcore.MSG_WIN:
				sawWin = true
				return
			}
			// 本测试只需要识别这几个消息；其余消息按最小布局推进不现实，
			// 直接返回等下一批（handler 允许不解析完整批次）。
			return
		}
	}, respCh, stopCh)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !ss.Completed {
		t.Fatal("session did not complete")
	}
	if !sawIdleCmd || !sawBattleCmd {
		t.Fatalf("prompts not seen: idlecmd=%v battlecmd=%v", sawIdleCmd, sawBattleCmd)
	}
	if !sawWin {
		t.Fatalf("duel did not reach MSG_WIN (last msg 0x%02x)", lastMsg)
	}
	if ss.ResponsesConsumed < 2 {
		t.Fatalf("responses consumed = %d, want >= 2", ss.ResponsesConsumed)
	}
}

// StopPlay 语义：玩家退出 → 永不作答的等待被 stopCh 解除，返回 ErrSingleStopped。
func TestSingleSessionStopWhileWaiting(t *testing.T) {
	initSingleEngine(t)

	ss := NewSingleSession(newRandomSeed())
	if err := ss.Prepare("青眼一击.lua"); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	respCh := make(chan []byte, 4)
	stopCh := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- ss.Run(func([]byte) {}, respCh, stopCh) }()
	close(stopCh)
	if err := <-done; err != ErrSingleStopped {
		t.Fatalf("run error = %v, want ErrSingleStopped", err)
	}
	if ss.Completed {
		t.Fatal("aborted session must not be marked completed")
	}
}

func TestSingleSessionPrepareFailsOnMissingScript(t *testing.T) {
	initSingleEngine(t)
	ss := NewSingleSession(newRandomSeed())
	if err := ss.Prepare("不存在的谜题.lua"); err == nil {
		if _, err := os.Stat("single/不存在的谜题.lua"); err == nil {
			t.Fatal("unexpected script exists")
		}
		t.Fatal("prepare should fail on missing script")
	}
}

// 第二个谜题至少要能被引擎真实布场（手牌 2 张、卡组 3 张）。
func TestSingleSessionPrepareSecondPuzzle(t *testing.T) {
	initSingleEngine(t)
	ss := NewSingleSession(newRandomSeed())
	if err := ss.Prepare("魔导师的初阵.lua"); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if got := ss.DeckCount(0, ocgcore.LOCATION_HAND); got != 2 {
		t.Fatalf("player0 hand count = %d, want 2", got)
	}
	if got := ss.DeckCount(0, ocgcore.LOCATION_DECK); got != 3 {
		t.Fatalf("player0 deck count = %d, want 3", got)
	}
	if got := ss.DeckCount(1, ocgcore.LOCATION_MZONE); got != 0 {
		t.Fatalf("player1 mzone count = %d, want 0", got)
	}
}

func encodeI(v int32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(v))
	return buf
}

// driveBlueEyesWin 用制胜走线把「青眼一击」驱动到 MSG_WIN（与
// TestSingleSessionPlayPuzzle 的 handler 相同）：进战斗阶段 → 青眼攻击恶魔的
// 召唤 → 无目标后进结束阶段 → MSG_CHAIN 一律不连锁。
func driveBlueEyesWin(ss *SingleSession) (sawWin bool, err error) {
	respCh := make(chan []byte, 4)
	stopCh := make(chan struct{})
	attacksLeft := 1
	// 保险丝：响应流卡死时自动退出
	go func() {
		<-time.After(60 * time.Second)
		close(stopCh)
	}()
	err = ss.Run(func(msg []byte) {
		if len(msg) == 0 {
			return
		}
		switch msg[0] {
		case ocgcore.MSG_SELECT_IDLECMD:
			respCh <- encodeI(6) // 进入战斗阶段
		case ocgcore.MSG_SELECT_BATTLECMD:
			if attacksLeft > 0 {
				attacksLeft--
				respCh <- encodeI(1) // 攻击列表第 0 个
			} else {
				respCh <- encodeI(3) // 进结束阶段
			}
		case ocgcore.MSG_SELECT_CHAIN:
			respCh <- encodeI(-1) // 不连锁
		case ocgcore.MSG_SELECT_CARD:
			respCh <- []byte{1, 0} // 攻击目标：count=1 + 序号 0
		case ocgcore.MSG_WIN:
			sawWin = true
		}
	}, respCh, stopCh)
	return sawWin, err
}

// 单机录像落盘回读（原版 single_mode.cpp:114-162 的 last_replay 录制链）：
// 走完胜利后断言 SaveReplay 的 .yrp 能被 OpenReplay 原样读回 —— 头标带
// REPLAY_UNIFORM|REPLAY_SINGLE_MODE，昵称/空 client 名/8000·5·1 参数/脚本文件名
// 都正确，响应流能被 ReadNextResponse 从数据区起点反读。
func TestSingleSessionRecordRoundtrip(t *testing.T) {
	initSingleEngine(t)

	ss := NewSingleSession(newRandomSeed())
	ss.HostName = "ReplayTester"
	if err := ss.Prepare("青眼一击.lua"); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	sawWin, err := driveBlueEyesWin(ss)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !sawWin || !ss.Completed {
		t.Fatalf("duel did not complete (win=%v)", sawWin)
	}

	if !ss.SaveReplay("single_replay_roundtrip") {
		t.Fatal("SaveReplay failed")
	}

	r2 := NewReplay()
	if !r2.OpenReplay("single_replay_roundtrip.yrp") {
		t.Fatal("OpenReplay failed to read back single replay")
	}
	hdr := r2.ReadHeader()
	if hdr.Base.Flag&REPLAY_SINGLE_MODE == 0 || hdr.Base.Flag&REPLAY_UNIFORM == 0 {
		t.Fatalf("header flag = 0x%x, want REPLAY_SINGLE_MODE|REPLAY_UNIFORM", hdr.Base.Flag)
	}
	if len(r2.players) != 2 {
		t.Fatalf("players = %d, want 2", len(r2.players))
	}
	if r2.players[0] != "ReplayTester" {
		t.Fatalf("host name = %q, want ReplayTester", r2.players[0])
	}
	if r2.players[1] != "" {
		t.Fatalf("client name = %q, want empty", r2.players[1])
	}
	if r2.scriptName != "青眼一击.lua" {
		t.Fatalf("script name = %q, want 青眼一击.lua", r2.scriptName)
	}
	if r2.params.StartLP != 8000 || r2.params.StartHand != 5 || r2.params.DrawCount != 1 {
		t.Fatalf("params = LP%d/Hand%d/Draw%d, want 8000/5/1", r2.params.StartLP, r2.params.StartHand, r2.params.DrawCount)
	}

	// 响应流：SkipInfo 定位到信息区之后，第一段是 uint8 长度 + 4 字节 LE 的
	// encodeI(6)（进入战斗阶段）。
	r2.SkipInfo()
	var resp [8]byte
	if !r2.ReadNextResponse(resp[:]) {
		t.Fatal("ReadNextResponse failed at response stream start")
	}
	if resp[0] != 6 || resp[1] != 0 || resp[2] != 0 || resp[3] != 0 {
		t.Fatalf("first response = %v, want [6 0 0 0]", resp[:4])
	}
}
