package main

// 波 J：单人模式（wSinglePlay，menu_handler.cpp:400-411 / single_mode.cpp）。
// ListSingles 扫描 single/*.lua；StartSingle 用 core/duel.SingleSession 在进程内
// 驱动谜题引擎，把每个消息批次经 collector.handleGameMessage 翻成前端事件，
// 玩家的响应经 respCh 投回引擎（等价原版 SingleMode::SetResponse / StopPlay）。

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// SingleInfo 是 wSinglePlay 残局列表（lstSinglePlayList）的一项：文件名 +
// 选中时 stSinglePlayInfo 显示的 message 文本。
type SingleInfo struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// singleRun 是一场进行中的单人谜题（App.single 指向它；nil = 没有进行中）。
type singleRun struct {
	session *duel.SingleSession
	respCh  chan []byte
	stopCh  chan struct{}
	done    chan struct{} // Run goroutine 退出后关闭
}

// ListSingles 列出 single/ 下的全部谜题（game.cpp RefreshSingleplay 的目录遍历
// 顺序），message 从每个脚本的 --[[message 块解析（menu_handler.cpp:562-603
// 的读取规则，支持单行与多行两种形式）。
func (a *App) ListSingles() []SingleInfo {
	entries, err := os.ReadDir(a.singleDir)
	if err != nil {
		return nil
	}
	var out []SingleInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".lua" {
			continue
		}
		out = append(out, SingleInfo{
			Name:    e.Name(),
			Message: parseSingleMessage(filepath.Join(a.singleDir, e.Name())),
		})
	}
	return out
}

// parseSingleMessage 提取谜题脚本的 --[[message 文本（UTF-8）。
func parseSingleMessage(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var message strings.Builder
	inMessage := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if strings.HasPrefix(line, "--[[message") {
			// C++ 的 fgets 行含 '\n'（len 比这里 +1）：len<=13 = 裸开场行，
			// message 从下一行开始；len>15 且有 ']' = 单行形式。
			if len(line) <= 12 {
				inMessage = true
				continue
			}
			if len(line) > 14 {
				if idx := strings.LastIndexByte(line, ']'); idx > 12 {
					// 去掉 "]]"（C++ 把倒数第二个字节置 0）
					message.WriteString(line[12 : idx-1])
				}
			}
			break
		}
		if strings.HasPrefix(line, "]]") {
			break
		}
		if inMessage {
			if message.Len() > 0 {
				message.WriteString("\n")
			}
			message.WriteString(line)
		}
	}
	return message.String()
}

// StartSingle 加载并运行一个谜题（原版 BUTTON_LOAD_SINGLEPLAY →
// SingleMode::StartPlay）。引擎在后台 goroutine 上驱动；返回后前端切到
// 决斗画面，事件流与在线对局同构（duel:start → reload_field/update_data →
// select_* 提示 → 玩家响应 → …… → single:ended）。
func (a *App) StartSingle(name string, returnDeckTop bool) map[string]interface{} {
	if a.client != nil && a.client.IsConnected() {
		return map[string]interface{}{"success": false, "error": "在线对局进行中，无法启动单人模式"}
	}
	a.singleMu.Lock()
	if a.single != nil {
		a.singleMu.Unlock()
		return map[string]interface{}{"success": false, "error": "单人模式已在进行中"}
	}
	a.singleMu.Unlock()

	// ocgcore 需要先初始化卡数据库/脚本根（与 PlayReplay/StartLocalServer 相同）
	if err := duel.InitServerData(a.dbPath, a.scriptPath, "."); err != nil {
		return map[string]interface{}{"success": false, "error": fmt.Sprintf("init error: %v", err)}
	}

	name = filepath.Base(name)
	if _, err := os.Stat(filepath.Join(a.singleDir, name)); err != nil {
		return map[string]interface{}{"success": false, "error": "找不到谜题脚本：" + name}
	}

	// 随机种子（SinglePlayThread 的 std::seed_seq）
	var seedBytes [32]byte
	if _, err := rand.Read(seedBytes[:]); err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}
	var seed [8]uint32
	for i := range seed {
		seed[i] = binary.LittleEndian.Uint32(seedBytes[i*4:])
	}

	ss := duel.NewSingleSession(seed)
	ss.HostName = a.loadConfig().Nickname
	if ss.HostName == "" {
		ss.HostName = "Duelist"
	}
	opt := int32(0)
	if returnDeckTop {
		opt = int32(ocgcore.DUEL_RETURN_DECK_TOP)
	}
	if err := ss.PrepareWithOpt(name, opt); err != nil {
		// Prepare 失败（如脚本加载失败）不会进入 Run 的 defer d.End()，
		// 引擎句柄已建，需在此显式释放，避免泄漏（end_duel + Dispose）。
		if ss.Duel != nil {
			ss.Duel.End()
		}
		return map[string]interface{}{"success": false, "error": err.Error()}
	}

	run := &singleRun{
		session: ss,
		respCh:  make(chan []byte, 8),
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}
	a.singleMu.Lock()
	if a.single != nil { // 双重检查：并发调用只放一个进来
		a.singleMu.Unlock()
		// 上一路径的会话仍在跑；这个已 Prepare 的引擎句柄不会进入 Run，同样释放。
		if ss.Duel != nil {
			ss.Duel.End()
		}
		return map[string]interface{}{"success": false, "error": "单人模式已在进行中"}
	}
	a.single = run
	a.singleMu.Unlock()

	emit := func(eventName string, data interface{}) {
		EmitWailsEvent(a.ctx, eventName, data)
	}
	collector := NewWailsDuelClient(func(eventName string, optionalData ...interface{}) {
		var data interface{}
		if len(optionalData) > 0 {
			data = optionalData[0]
		}
		emit(eventName, data)
	})

	go func() {
		defer close(run.done)
		defer func() {
			a.singleMu.Lock()
			if a.single == run {
				a.single = nil
			}
			a.singleMu.Unlock()
		}()

		// 谜题布场后的权威快照：query_field_info 直接产出一条完整的
		// MSG_RELOAD_FIELD 消息（ocgapi.cpp:319-338）；info[0] 是 opcode，
		// body 从 rule 字节开始。缓冲尾部是残留，按 consumed+1 截断。
		info := ss.Duel.QueryFieldInfo()
		fields, consumed, infoOK := parseReloadFieldBody(info[1:])

		// 开场事件（在线路径由服务器 MSG_START 合成；键名与 PlayReplay 一致）
		startPayload := map[string]interface{}{
			"playerType": uint8(0),
			"duelRule":   uint8(5), // CURRENT_RULE；解析成功时被真实值覆盖
			"lp0":        8000,
			"lp1":        8000,
			"deck0":      ss.DeckCount(0, ocgcore.LOCATION_DECK),
			"extra0":     ss.DeckCount(0, ocgcore.LOCATION_EXTRA),
			"deck1":      ss.DeckCount(1, ocgcore.LOCATION_DECK),
			"extra1":     ss.DeckCount(1, ocgcore.LOCATION_EXTRA),
		}
		if infoOK {
			players := fields["players"].([]map[string]interface{})
			startPayload["duelRule"] = fields["rule"]
			startPayload["lp0"] = players[0]["lp"]
			startPayload["lp1"] = players[1]["lp"]
			startPayload["deck0"] = players[0]["deck"]
			startPayload["extra0"] = players[0]["extra"]
			startPayload["deck1"] = players[1]["deck"]
			startPayload["extra1"] = players[1]["extra"]
		}
		emit("duel:start", startPayload)
		if infoOK {
			if err := collector.handleGameMessage(info[:1+consumed]); err != nil {
				log.Printf("[App] single reload_field parse error: %v", err)
			}
		}

		// 初始场地同步（SinglePlayReload 的 mzone/szone/hand 部分）
		refreshLocations(ss.Duel, collector)
		// 每个 select 提示后再刷一次（single_mode.cpp:788-843）
		ss.OnPrompt = func() {
			refreshLocations(ss.Duel, collector)
		}

		err := ss.Run(func(msg []byte) {
			if perr := collector.handleGameMessage(msg); perr != nil {
				log.Printf("[App] single mode message parse error: %v", perr)
			}
		}, run.respCh, run.stopCh)
		completed := err == nil
		emit("single:ended", map[string]interface{}{"completed": completed})
		if completed {
			a.finishSingleReplay(ss, emit)
		}
	}()

	return map[string]interface{}{"success": true, "name": name}
}

// StopSingle 结束进行中的谜题（原版 btnLeaveGame → SingleMode::StopPlay：
// 置停止标记唤醒等待中的信号量，这里由 Run 的 stopCh select 解除阻塞）。
func (a *App) StopSingle() {
	a.singleMu.Lock()
	run := a.single
	a.singleMu.Unlock()
	if run == nil {
		return
	}
	select {
	case <-run.stopCh:
	default:
		close(run.stopCh)
	}
}

// finishSingleReplay 单机谜题正常完结后的录像处理（原版 single_mode.cpp:141-162）：
// 建议名 = 当前时间 "%Y-%m-%d %H-%M-%S"；auto_save_replay=1 直接落盘并提示，
// 否则保留句柄并 emit single:replay 供前端确认（前端调 SaveSingleReplay 落盘）。
func (a *App) finishSingleReplay(ss *duel.SingleSession, emit func(string, interface{})) {
	name := time.Now().Format("2006-01-02 15-04-05")
	if a.loadConfig().AutoSaveReplay != 0 {
		if ss.SaveReplay(name) {
			emit("single:replay", map[string]interface{}{"name": name + ".yrp", "autoSaved": true})
		}
		return
	}
	a.singleMu.Lock()
	a.lastSingleReplay = ss.Replay()
	a.singleMu.Unlock()
	emit("single:replay", map[string]interface{}{"name": name, "autoSaved": false})
}

// SaveSingleReplay 落盘最近一次完结的单机录像（前端确认弹窗 / auto_save 兜底）。
func (a *App) SaveSingleReplay(name string) map[string]interface{} {
	a.singleMu.Lock()
	rp := a.lastSingleReplay
	a.singleMu.Unlock()
	if rp == nil {
		return map[string]interface{}{"success": false, "error": "没有可保存的单机录像"}
	}
	if strings.TrimSpace(name) == "" {
		name = time.Now().Format("2006-01-02 15-04-05")
	}
	if !rp.SaveReplay(name) {
		return map[string]interface{}{"success": false, "error": "保存录像失败"}
	}
	return map[string]interface{}{"success": true, "name": name + ".yrp"}
}

// refreshLocations 把双方 mzone/szone/hand 同步成 MSG_UPDATE_DATA 事件
// （single_mode.cpp SinglePlayRefresh：0,1 × mzone → 0,1 × szone → 0,1 × hand，
// flag 0xf81fff，single_mode.h:24 默认值——含 QUERY_LINK，场上连接怪的
// link_marker 随刷新到达前端供互连高亮）。与原版一致不做遮码 —— 单人/故事
// 决斗只有一条事件流，前端只按 playerSlot 取用；遮码需要像服务器那样按
// 座位分包发送。引擎查询与 Run 同 goroutine，无并发问题。
func refreshLocations(d *ocgcore.Duel, collector *WailsDuelClient) {
	for _, loc := range []uint8{ocgcore.LOCATION_MZONE, ocgcore.LOCATION_SZONE, ocgcore.LOCATION_HAND} {
		for player := 0; player < 2; player++ {
			refreshLocation(d, collector, player, loc)
		}
	}
}

func refreshLocation(d *ocgcore.Duel, collector *WailsDuelClient, player int, location uint8) {
	flag := uint32(0xf81fff) | ocgcore.QUERY_CODE | ocgcore.QUERY_POSITION
	buf := make([]byte, 3+ocgcore.SIZE_QUERY_BUFFER)
	buf[0] = ocgcore.MSG_UPDATE_DATA
	buf[1] = byte(player)
	buf[2] = byte(location)
	n := int(d.QueryFieldCard(uint8(player), location, flag, buf[3:], false))
	if n <= 0 {
		return
	}
	if err := collector.handleGameMessage(buf[:3+n]); err != nil {
		log.Printf("[App] refresh (player %d, loc 0x%02x): %v", player, location, err)
	}
}

// routeResponseB 把玩家的原始响应字节投给当前接收方：谜题进行中投给
// SingleSession 的响应通道（等价 SingleMode::SetResponse）；故事决斗中
// 投给 StorySession 的响应通道；否则走在线路径发给服务器。
func (a *App) routeResponseB(data []byte) error {
	a.singleMu.Lock()
	run := a.single
	a.singleMu.Unlock()
	if run != nil {
		select {
		case run.respCh <- append([]byte(nil), data...):
		case <-run.done:
			// 会话刚结束：响应无处可投，丢弃
		}
		return nil
	}
	return a.client.SendResponseB(data)
}

// routeResponseI 同 routeResponseB，int32 标量响应（select_effectyn/yesno/
// option/chain 等）在单机路径同样编成 4 字节 LE。
func (a *App) routeResponseI(val int32) error {
	a.singleMu.Lock()
	run := a.single
	a.singleMu.Unlock()
	if run != nil {
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(val))
		select {
		case run.respCh <- buf:
		case <-run.done:
		}
		return nil
	}
	return a.client.SendResponseI(val)
}
