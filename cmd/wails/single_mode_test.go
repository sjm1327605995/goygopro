package main

// 波 J：单人模式的 cmd/wails 层测试。TestStartSingleDrivesPuzzleToWin 用真实
// 引擎走完整制胜线：StartSingle →（经 EmitWailsEvent 截获事件）→ 玩家响应
// 经 routeResponseB 投回 respCh → single:ended completed=true，并断言
// reload_field/ai_name/update_data 事件把谜题布场送到了前端。
// 守卫与退出（StopSingle）各驱动一次真实状态变化。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// testApp 构造一个指向仓库根数据文件的 App，并把 cwd 切到仓库根（生产环境
// 这些路径就是 exe 目录下的相对路径）。
func testApp(t *testing.T) *App {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	a := NewApp()
	a.dbPath = filepath.Join(root, "cards.cdb")
	a.scriptPath = filepath.Join(root, "script")
	a.singleDir = filepath.Join(root, "single")
	return a
}

// eventRecorder 截获 EmitWailsEvent：记录事件流并按提示代答（等价前端
// PromptHost + WailsBridge.respond*，走的就是 routeResponse* 响应管线）。
type eventRecorder struct {
	mu     sync.Mutex
	names  []string
	byName map[string][]interface{}
	ended  bool
	end    map[string]interface{}
}

func newEventRecorder() *eventRecorder {
	return &eventRecorder{byName: map[string][]interface{}{}}
}

func (r *eventRecorder) record(name string, data interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
	r.byName[name] = append(r.byName[name], data)
}

func (r *eventRecorder) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byName[name])
}

func (r *eventRecorder) last(name string) interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.byName[name]
	if len(list) == 0 {
		return nil
	}
	return list[len(list)-1]
}

func (r *eventRecorder) all(name string) []interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]interface{}(nil), r.byName[name]...)
}

// waitFor 轮询直到条件满足或超时。
func (r *eventRecorder) waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t.Fatalf("timed out waiting for %s (events so far: %v)", what, r.names)
}

func TestStartSingleDrivesPuzzleToWin(t *testing.T) {
	a := testApp(t)

	singles := a.ListSingles()
	if len(singles) < 2 {
		t.Fatalf("ListSingles = %v, want >= 2 puzzles", singles)
	}
	var target *SingleInfo
	for i := range singles {
		if singles[i].Name == "青眼一击.lua" {
			target = &singles[i]
		}
	}
	if target == nil {
		t.Fatalf("青眼一击.lua missing from ListSingles: %v", singles)
	}
	if !strings.Contains(target.Message, "残局") {
		t.Fatalf("message not parsed for %s: %q", target.Name, target.Message)
	}

	rec := newEventRecorder()
	orig := EmitWailsEvent
	EmitWailsEvent = func(_ context.Context, name string, data interface{}) {
		rec.record(name, data)
		// 按提示代答：进战斗阶段 → 青眼攻击 → 选攻击目标。
		switch name {
		case "duel:select_idlecmd":
			a.RespondIdleCmd(0, 6)
		case "duel:select_battlecmd":
			a.RespondBattleCmd(0, 1)
		case "duel:select_card":
			a.RespondSelectCard([]int32{0})
		case "duel:select_chain":
			a.SendResponseI(-1)
		}
	}
	defer func() { EmitWailsEvent = orig }()

	res := a.StartSingle("青眼一击.lua", false)
	if res["success"] != true {
		if strings.Contains(res["error"].(string), "init error") {
			t.Skipf("ocgcore library not available: %v", res["error"])
		}
		t.Fatalf("StartSingle: %v", res)
	}
	rec.waitFor(t, func() bool { return rec.count("single:ended") > 0 }, "single:ended")

	if rec.count("duel:win") == 0 {
		t.Fatal("puzzle did not reach duel:win")
	}
	end := rec.last("single:ended").(map[string]interface{})
	if end["completed"] != true {
		t.Fatalf("single:ended = %v, want completed=true", end)
	}
	if rec.count("duel:ai_name") == 0 {
		t.Fatal("MSG_AI_NAME was not forwarded as duel:ai_name")
	}
	if got := rec.last("duel:ai_name").(map[string]interface{})["name"]; got != "残局AI" {
		t.Fatalf("ai_name = %v, want 残局AI", got)
	}
	if rec.count("duel:reload_field") == 0 {
		t.Fatal("MSG_RELOAD_FIELD was not forwarded as duel:reload_field")
	}
	// 开场 duel:start 的 LP/卡组计数来自 query_field_info 快照（谜题脚本
	// 把对手打到 500，卡组 0）
	start := rec.last("duel:start").(map[string]interface{})
	if start["lp1"].(int32) != 500 {
		t.Fatalf("duel:start lp1 = %v, want 500", start["lp1"])
	}
	// 场地同步：对手 mzone 的恶魔的召唤（seq 2）要作为 update_data 到达
	foundOpponentMonster := false
	for _, ev := range rec.all("duel:update_data") {
		m := ev.(map[string]interface{})
		// map 值是 Go 原生类型（uint8），与未类型化常量比较会因动态类型不同而恒不等
		if m["player"] != uint8(1) || m["location"] != uint8(0x04) {
			continue
		}
		for _, c := range m["cards"].([]map[string]interface{}) {
			if c["code"] == uint32(70781052) {
				foundOpponentMonster = true
			}
		}
	}
	if !foundOpponentMonster {
		t.Fatal("opponent mzone was not synced via duel:update_data")
	}

	// 会话结束后的守卫：a.single 已被 goroutine 清理，新的谜题可以启动
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		a.singleMu.Lock()
		busy := a.single != nil
		a.singleMu.Unlock()
		if !busy {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if busy := func() bool { a.singleMu.Lock(); defer a.singleMu.Unlock(); return a.single != nil }(); busy {
		t.Fatal("a.single not cleared after single:ended")
	}
}

// StopPlay 语义：谜题等待玩家作答时退出 → single:ended completed=false，
// 且响应通道的后续投递被安全丢弃。
func TestStartSingleStopWhileWaiting(t *testing.T) {
	a := testApp(t)

	rec := newEventRecorder()
	orig := EmitWailsEvent
	EmitWailsEvent = func(_ context.Context, name string, data interface{}) {
		rec.record(name, data) // 不代答：模拟玩家挂机
	}
	defer func() { EmitWailsEvent = orig }()

	res := a.StartSingle("青眼一击.lua", false)
	if res["success"] != true {
		if strings.Contains(res["error"].(string), "init error") {
			t.Skipf("ocgcore library not available: %v", res["error"])
		}
		t.Fatalf("StartSingle: %v", res)
	}
	rec.waitFor(t, func() bool {
		return rec.count("duel:select_idlecmd") > 0
	}, "first prompt") // 确认引擎真的在等待玩家作答

	a.StopSingle()
	rec.waitFor(t, func() bool { return rec.count("single:ended") > 0 }, "single:ended")
	end := rec.last("single:ended").(map[string]interface{})
	if end["completed"] != false {
		t.Fatalf("single:ended = %v, want completed=false", end)
	}
}

// parseSingleMessage 的单行/多行两种 --[[message 形式（menu_handler.cpp:562-603）。
func TestParseSingleMessageForms(t *testing.T) {
	dir := t.TempDir()

	single := filepath.Join(dir, "single-line.lua")
	if err := os.WriteFile(single, []byte("--[[message 入门残局：一击制胜！]]\n\nDebug.ReloadFieldBegin(1)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := parseSingleMessage(single); got != "入门残局：一击制胜！" {
		t.Fatalf("single-line message = %q", got)
	}

	multi := filepath.Join(dir, "multi-line.lua")
	if err := os.WriteFile(multi, []byte("--[[message\n第一行说明\n第二行说明\n]]\nDebug.AddCard(1)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := parseSingleMessage(multi); got != "第一行说明\n第二行说明" {
		t.Fatalf("multi-line message = %q", got)
	}
}
