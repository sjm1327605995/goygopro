package main

// 联机对战集成测试（两个真实客户端经 TCP 互相对战）：
// 进程内起决斗服务器（等价客户端 A「启动服务器」），客户端 A 建房、客户端 B
// 凭密码加入，双方更新卡组并准备后房主开始决斗；猜拳、先后手由收到提示的一
// 侧自动应答，直到双方都收到 duel:start；最后房主投降，双方都应收到
// duel:win / stoc:duel_end / stoc:replay，验证全链路事件往返。

import (
	"testing"
)

func TestTwoClientsDuelEachOther(t *testing.T) {
	const port = 17911
	const roomPass = "smoke123"

	// 客户端 A：同时承担「启动服务器」的角色（与大厅「启动服务器」按钮同路径）。
	a := testApp(t)
	if res := a.StartLocalServer(port); res["success"] != true {
		if errStr, ok := res["error"].(string); ok {
			t.Skipf("server could not start (ocgcore missing?): %s", errStr)
		}
		t.Fatalf("StartLocalServer: %v", res)
	}

	recA := newEventRecorder()
	a.client = NewWailsDuelClient(func(name string, data ...interface{}) {
		var d interface{}
		if len(data) > 0 {
			d = data[0]
		}
		recA.record(name, d)
		// 猜拳 / 先后手代答：谁收到提示谁应答（与前端行为一致）。
		// 猜拳合法值是 1/2/3（0 会被服务端当成「未作答」哨兵丢弃）。
		switch name {
		case "stoc:error_msg":
			t.Logf("A got error_msg: %v", d)
		case "stoc:select_hand":
			a.SendHandResult(1) // 石头
		case "stoc:select_tp":
			a.SendTPResult(0) // 先手
		}
	})
	t.Cleanup(func() { a.client.Disconnect() })

	// 客户端 B：独立 App，只连服务器
	b := testApp(t)
	recB := newEventRecorder()
	b.client = NewWailsDuelClient(func(name string, data ...interface{}) {
		var d interface{}
		if len(data) > 0 {
			d = data[0]
		}
		recB.record(name, d)
		switch name {
		case "stoc:error_msg":
			t.Logf("B got error_msg: %v", d)
		case "stoc:select_hand":
			b.SendHandResult(2) // 剪刀 —— 与 A 不同值即可决出先后手
		case "stoc:select_tp":
			b.SendTPResult(0)
		}
	})
	t.Cleanup(func() { b.client.Disconnect() })

	// A 连接并建房
	if res := a.ConnectServer("127.0.0.1:17911", "HostPlayer", ""); res["success"] != true {
		t.Fatalf("A ConnectServer: %v", res)
	}
	if res := a.CreateGame(HostInfoReq{
		Rule: 0, Mode: 0, DuelRule: 5, StartLp: 8000,
		StartHand: 5, DrawCount: 1, TimeLimit: 180, NoCheckDeck: true,
	}, "Smoke Room", roomPass); res["success"] != true {
		t.Fatalf("A CreateGame: %v", res)
	}

	// B 连接并凭密码加入
	if res := b.ConnectServer("127.0.0.1:17911", "GuestPlayer", ""); res["success"] != true {
		t.Fatalf("B ConnectServer: %v", res)
	}
	if res := b.JoinGame(roomPass); res["success"] != true {
		t.Fatalf("B JoinGame: %v", res)
	}

	// 双方都应在房间里：B 收到 join_game + type_change，A 看到 B 进入
	recB.waitFor(t, func() bool { return recB.count("stoc:join_game") > 0 }, "B stoc:join_game")
	recB.waitFor(t, func() bool { return recB.count("stoc:type_change") > 0 }, "B stoc:type_change")
	recA.waitFor(t, func() bool { return recA.count("stoc:player_enter") > 0 }, "A stoc:player_enter (B entered)")

	// 聊天连通性冒烟：A 发言，B 应收到
	a.SendChat("hello from A")
	recB.waitFor(t, func() bool { return recB.count("stoc:chat") > 0 }, "B stoc:chat")

	// 双方上卡组（40 张主卡：4 种真实卡号 ×10，nocheck 房）并准备
	main := make([]uint32, 0, 40)
	for i := 0; i < 10; i++ {
		main = append(main, 89631139, 46986414, 38033121, 70781052)
	}
	if err := a.client.UpdateDeck(main, nil); err != nil {
		t.Fatalf("A UpdateDeck: %v", err)
	}
	if err := b.client.UpdateDeck(main, nil); err != nil {
		t.Fatalf("B UpdateDeck: %v", err)
	}
	a.SetReady(true)
	b.SetReady(true)
	// 跨连接无顺序保证：等双方 ready 的广播都到达 A（说明两人的准备都已被
	// 服务器处理）再开始决斗，否则 HS_START 可能先于 B 的准备到达而空转。
	recA.waitFor(t, func() bool { return recA.count("stoc:player_change") >= 2 }, "both ready broadcast")

	// 房主开始决斗 → 双方都应走到 duel:start（中间猜拳/先后手由回调代答）
	a.StartDuel()
	recA.waitFor(t, func() bool { return recA.count("duel:start") > 0 }, "A duel:start")
	recB.waitFor(t, func() bool { return recB.count("duel:start") > 0 }, "B duel:start")

	// 房主投降 → 双方都应收到胜负与对局结束事件
	a.Surrender()
	recA.waitFor(t, func() bool { return recA.count("duel:win") > 0 }, "A duel:win")
	recB.waitFor(t, func() bool { return recB.count("duel:win") > 0 }, "B duel:win")
	recA.waitFor(t, func() bool { return recA.count("stoc:duel_end") > 0 }, "A stoc:duel_end")
	recB.waitFor(t, func() bool { return recB.count("stoc:duel_end") > 0 }, "B stoc:duel_end")

	// 服务器在决斗结束时推送录像（前端确认后落盘）
	recA.waitFor(t, func() bool { return recA.count("stoc:replay") > 0 }, "A stoc:replay")
	recB.waitFor(t, func() bool { return recB.count("stoc:replay") > 0 }, "B stoc:replay")
}
