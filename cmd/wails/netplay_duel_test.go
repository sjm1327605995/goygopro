package main

// 双客户端完整对战 e2e（自动应答打到分出胜负）：
// 与 netplay_smoke_test.go 的差别在于不在 duel:start 后投降，而是给两个
// 客户端各挂一个「自动驾驶」——按前端事件语义逐一回答引擎的交互提示
// （召唤/选区域/战斗/连锁/选卡等），让一局真实规则的对局在两个 TCP
// 客户端之间打完：通常召唤（2000 攻白板）→ 战斗阶段攻击 → 战斗伤害
// 扣 LP → 一方 LP 归零触发 MSG_WIN。断言双方都看到召唤、攻击、伤害与
// 一致的胜负结果，证明联机链路完整符合游戏王规则结算。
//
// 卡组用 40 张「基因狼人」(69247929，4 星白板 2000/100)：无效果、无祭品、
// 无连锁，交互面收敛到 summon/place/battlecmd 三类提示；NoCheckDeck 房
// 允许 40 张同名卡。App.CreateGame 会把 TimeLimit=0 兜底成 180 秒限时，
// 所以自动驾驶同时对 stoc:time_limit 回 TIME_CONFIRM（等价原版客户端的
// 限时心跳），否则服务器不收 CTOS_RESPONSE。

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/protocol"
)

// freeTCPPort 取一个当前空闲的回环端口（监听后立刻关闭，交给服务器绑定）。
func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// autoPilot 返回一个 emit 回调：记录事件并按提示类型自动应答。
// handRes 是猜拳出招（两名客户端须不同）；所有应答策略都是「合法的第一
// 选择」，只依赖事件 payload 里的列表字段，不维护任何局面状态。
func autoPilot(t *testing.T, tag string, a *App, rec *eventRecorder, handRes byte) func(string, ...interface{}) {
	t.Helper()
	return func(name string, data ...interface{}) {
		var d interface{}
		if len(data) > 0 {
			d = data[0]
		}
		rec.record(name, d)
		switch name {
		case "stoc:error_msg":
			t.Errorf("%s got error_msg: %v", tag, d)
		case "stoc:time_limit":
			// 限时开启时服务器把被等待者状态置为 CTOS_TIME_CONFIRM，客户端须先
			// 回 TIME_CONFIRM 才会进入 CTOS_RESPONSE（非被等待者应答是 no-op，
			// 见 core/duel/duel_common.go timeConfirm），所以无脑应答即可。
			a.SendTimeConfirm()
		case "stoc:select_hand":
			a.SendHandResult(handRes)
		case "stoc:select_tp":
			a.SendTPResult(1) // 先攻

		case "duel:select_idlecmd":
			cmd := d.(selectIdleCmdDTO)
			switch {
			case len(cmd.Summon) > 0:
				a.RespondIdleCmd(int32(cmd.Summon[0].Idx), 0) // 通常召唤
			case cmd.ToBP:
				a.RespondIdleCmd(0, 6)
			case cmd.ToEP:
				a.RespondIdleCmd(0, 7)
			default:
				// 无动作且不给换阶段（理论不发生）：应答 toEP 兜底
				t.Logf("%s idlecmd with no moves (toBP=%v toEP=%v), answering toEP", tag, cmd.ToBP, cmd.ToEP)
				a.RespondIdleCmd(0, 7)
			}

		case "duel:select_place":
			// 与前端 duel_manager 的自动落点同路径：取 zones（可用区域，Go 侧
			// 已对禁用位掩码取反）第一个条目，player 用区域自带归属。
			m := d.(map[string]interface{})
			zones, _ := m["zones"].([]map[string]interface{})
			if len(zones) == 0 {
				t.Errorf("%s select_place with empty zones (flag=%v)", tag, m["flag"])
				return
			}
			z := zones[0]
			player, _ := z["player"].(int)
			a.RespondSelectPlace(int32(player), int32(z["loc"].(int)), int32(z["seq"].(int)))

		case "duel:select_battlecmd":
			cmd := d.(selectBattleCmdDTO)
			if len(cmd.Attack) > 0 {
				// 优先直接攻击；否则攻击列表第一张
				idx := cmd.Attack[0].Idx
				for _, e := range cmd.Attack {
					if e.DirAtt {
						idx = e.Idx
						break
					}
				}
				a.RespondBattleCmd(int32(idx), 1)
			} else if cmd.ToM2 {
				a.RespondBattleCmd(0, 2)
			} else {
				a.RespondBattleCmd(0, 3)
			}

		case "duel:select_chain":
			chain := d.(selectChainDTO)
			if chain.Forced && len(chain.Chains) > 0 {
				a.SendResponseI(0) // 强制连锁：发第一个
			} else {
				a.SendResponseI(-1) // 不连锁
			}

		case "duel:select_card": // 含 MSG_SELECT_TRIBUTE
			sel := d.(selectCardDTO)
			n := int(sel.Min)
			if n < 1 {
				n = 1
			}
			if n > len(sel.Cards) {
				n = len(sel.Cards)
			}
			indices := make([]int32, n)
			for i := range indices {
				indices[i] = int32(i)
			}
			a.RespondSelectCard(indices)

		case "duel:select_unselect":
			a.RespondSelectUnselect(0)

		case "duel:select_yesno", "duel:select_effectyn":
			a.SendResponseI(0) // 一律选「否」（白板卡组不会出现，防御性应答）

		case "duel:select_option":
			a.SendResponseI(0)

		case "duel:select_position":
			m := d.(*protocol.SelectPositionMsg)
			pos := int32(1) // 优先表侧攻击表示
			if m.Positions&0x1 == 0 {
				for bit := uint8(0); bit < 4; bit++ {
					if m.Positions&(1<<bit) != 0 {
						pos = int32(1 << bit)
						break
					}
				}
			}
			a.SendResponseI(pos)

		case "duel:select_sum":
			m := d.(*protocol.SelectSumMsg)
			n := int(m.Min)
			if n > len(m.Select) {
				n = len(m.Select)
			}
			indices := make([]int32, n)
			for i := range indices {
				indices[i] = int32(i)
			}
			a.RespondSelectSum(int32(len(m.Must)+n), indices)

		case "duel:select_counter":
			m := d.(*protocol.SelectCounterMsg)
			counts := make([]int32, len(m.Entries))
			if len(counts) > 0 {
				counts[0] = int32(m.Count) // 全部从第一张卡移除
			}
			a.RespondCounter(counts)

		case "duel:sort_card":
			a.RespondSortCardCancel()

		case "duel:announce_card":
			m := d.(map[string]interface{})
			candidates, _ := m["candidates"].([]int32)
			if decodable, _ := m["decodable"].(bool); decodable && len(candidates) > 0 {
				a.SendResponseI(candidates[0])
			} else {
				a.SendResponseI(0)
			}

		case "duel:announce_race", "duel:announce_attrib":
			m := d.(*protocol.AnnounceRaceMsg)
			ans := int32(0)
			for bit := uint8(0); bit < 32; bit++ {
				if m.Available&(1<<bit) != 0 {
					ans = int32(1 << bit)
					break
				}
			}
			a.SendResponseI(ans)

		case "duel:announce_number":
			m := d.(*protocol.AnnounceCardMsg)
			if len(m.Values) > 0 {
				a.SendResponseI(m.Values[0])
			} else {
				a.SendResponseI(0)
			}
		}
	}
}

// waitEvent 在 deadline 内等待事件出现（rec.waitFor 固定 30s，完整对局
// 给更宽的余量）。
func waitEvent(t *testing.T, rec *eventRecorder, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if rec.count(name) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s (event counts: %v)", name, rec.summary())
}

// summary 返回各事件的计数，用于超时诊断。
func (r *eventRecorder) summary() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.byName))
	for k, v := range r.byName {
		out[k] = len(v)
	}
	return out
}

func TestTwoClientsAutoPlayFullDuel(t *testing.T) {
	const roomPass = "full123"

	// 每次运行用空闲端口并在收尾停掉服务器，保证 -count=N 连跑可重复。
	// 同时重置服务器的全局房间状态（上一轮对局会把 AcceptingConnections 落下）。
	port := freeTCPPort(t)
	duel.DefaultManager = duel.NewManager()
	duel.AcceptingConnections.Store(true)

	// 客户端 A：同时承担「启动服务器」的角色（与大厅「启动服务器」按钮同路径）。
	a := testApp(t)
	if res := a.StartLocalServer(port); res["success"] != true {
		if errStr, ok := res["error"].(string); ok {
			t.Skipf("server could not start (ocgcore missing?): %s", errStr)
		}
		t.Fatalf("StartLocalServer: %v", res)
	}
	t.Cleanup(func() {
		// 还原服务器全局状态，避免影响同包其他联机测试（对齐
		// core/duel/two_client_test.go 的收尾语义）。
		duel.AcceptingConnections.Store(true)
		duel.DefaultManager = duel.NewManager()
		if duel.NetServerEngine != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = duel.NetServerEngine.Stop(ctx)
		}
	})

	recA := newEventRecorder()
	a.client = NewWailsDuelClient(autoPilot(t, "A", a, recA, 1)) // 石头
	t.Cleanup(func() { a.client.Disconnect() })

	recB := newEventRecorder()
	b := testApp(t)
	b.client = NewWailsDuelClient(autoPilot(t, "B", b, recB, 2)) // 剪刀
	t.Cleanup(func() { b.client.Disconnect() })

	// A 连接并建房（白板卡组房：不查卡表）
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if res := a.ConnectServer(addr, "AutoA", ""); res["success"] != true {
		t.Fatalf("A ConnectServer: %v", res)
	}
	if res := a.CreateGame(HostInfoReq{
		Rule: 0, Mode: 0, DuelRule: 5, StartLp: 8000,
		StartHand: 5, DrawCount: 1, TimeLimit: 0, NoCheckDeck: true,
	}, "Full Duel Room", roomPass); res["success"] != true {
		t.Fatalf("A CreateGame: %v", res)
	}

	// B 连接并凭密码加入
	if res := b.ConnectServer(addr, "AutoB", ""); res["success"] != true {
		t.Fatalf("B ConnectServer: %v", res)
	}
	if res := b.JoinGame(roomPass); res["success"] != true {
		t.Fatalf("B JoinGame: %v", res)
	}
	recB.waitFor(t, func() bool { return recB.count("stoc:join_game") > 0 }, "B stoc:join_game")
	recA.waitFor(t, func() bool { return recA.count("stoc:player_enter") > 0 }, "A stoc:player_enter (B entered)")

	// 双方上卡组（40 张基因狼人）并准备
	deck := make([]uint32, 40)
	for i := range deck {
		deck[i] = 69247929
	}
	if err := a.client.UpdateDeck(deck, nil); err != nil {
		t.Fatalf("A UpdateDeck: %v", err)
	}
	if err := b.client.UpdateDeck(deck, nil); err != nil {
		t.Fatalf("B UpdateDeck: %v", err)
	}
	a.SetReady(true)
	b.SetReady(true)
	recA.waitFor(t, func() bool { return recA.count("stoc:player_change") >= 2 }, "both ready broadcast")

	// 房主开始决斗；此后全部由自动驾驶代答，直到分出胜负。
	a.StartDuel()
	waitEvent(t, recA, "duel:start", 10*time.Second)
	waitEvent(t, recB, "duel:start", 10*time.Second)

	// 完整对局：回合、召唤、攻击、战斗伤害都应真实发生（游戏王规则结算）。
	{
		deadline := time.Now().Add(120 * time.Second)
		for time.Now().Before(deadline) && recA.count("duel:win") == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		if recA.count("duel:win") == 0 {
			t.Fatalf("timed out waiting for duel:win\nA events: %v\nB events: %v", recA.summary(), recB.summary())
		}
	}
	waitEvent(t, recB, "duel:win", 10*time.Second)

	if got := recA.count("duel:new_turn"); got < 2 {
		t.Fatalf("A saw %d turns, want >= 2 (duel ended without real play)", got)
	}
	for tag, rec := range map[string]*eventRecorder{"A": recA, "B": recB} {
		if rec.count("duel:summoning") == 0 {
			t.Fatalf("%s never saw duel:summoning", tag)
		}
		if rec.count("duel:attack") == 0 {
			t.Fatalf("%s never saw duel:attack", tag)
		}
	}
	if recA.count("duel:damage") == 0 && recA.count("duel:lp_update") == 0 {
		t.Fatal("no damage/lp_update events: LP never changed")
	}

	// 胜负应由 LP 归零决定（战斗伤害），而不是超时或认输：
	// 战斗伤害走 MSG_DAMAGE（不带余额），本地按 8000 起始累计扣减，
	// 验证有一方被战斗伤害打到 0。
	lp := map[uint8]int32{0: 8000, 1: 8000}
	for _, e := range recA.all("duel:damage") {
		m := e.(*protocol.DamageMsg)
		lp[m.Player] -= m.Value
	}
	if lp[0] > 0 && lp[1] > 0 {
		t.Fatalf("no player reached 0 LP (lp=%v): win was not by battle damage", lp)
	}

	// 双方看到的胜者一致（MSG_WIN 的 player 是引擎侧玩家号，两边应相同）。
	winA, okA := recA.last("duel:win").(*protocol.WinMsg)
	winB, okB := recB.last("duel:win").(*protocol.WinMsg)
	if !okA || !okB {
		t.Fatalf("win payload type mismatch: A=%T B=%T", recA.last("duel:win"), recB.last("duel:win"))
	}
	if winA.Player != winB.Player {
		t.Fatalf("winner mismatch: A saw player %d, B saw player %d", winA.Player, winB.Player)
	}
	t.Logf("winner=player%d winType=%d turns=%d", winA.Player, winB.Type, recA.count("duel:new_turn"))

	// 对局结束收尾：duel_end + 服务器推送录像
	waitEvent(t, recA, "stoc:duel_end", 10*time.Second)
	waitEvent(t, recB, "stoc:duel_end", 10*time.Second)
	waitEvent(t, recA, "stoc:replay", 10*time.Second)
	waitEvent(t, recB, "stoc:replay", 10*time.Second)
}
