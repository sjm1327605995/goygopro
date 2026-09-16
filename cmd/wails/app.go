package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// App struct manages Wails application state
type App struct {
	ctx        context.Context
	client     *WailsDuelClient
	cardDB     *CardDBManager
	dbPath     string
	scriptPath string
	deckDir    string
	replayDir  string
	singleDir  string

	// 单人谜题（single_mode.go）：nil = 没有进行中的谜题
	singleMu sync.Mutex
	single   *singleRun
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{
		dbPath:     "cards.cdb",
		scriptPath: "script",
		deckDir:    "deck",
		replayDir:  "replay",
		singleDir:  "single",
		cardDB:     NewCardDBManager(),
	}
	return app
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx
	a.client = NewWailsDuelClient(func(eventName string, optionalData ...interface{}) {
		// In Wails runtime: runtime.EventsEmit(a.ctx, eventName, optionalData...)
		// We encapsulate emit logic here
		if len(optionalData) > 0 {
			EmitWailsEvent(a.ctx, eventName, optionalData[0])
		} else {
			EmitWailsEvent(a.ctx, eventName, nil)
		}
	})
	// Initialize card database
	if _, err := os.Stat(a.dbPath); err == nil {
		_ = a.cardDB.OpenDB(a.dbPath)
	}
	// 系列名表（strings.conf !setname 行；可选数据，缺文件静默降级）
	_ = a.cardDB.LoadSetNames("strings.conf")

	// Initialize deck & replay dirs
	_ = os.MkdirAll(a.deckDir, 0755)
	_ = os.MkdirAll(a.replayDir, 0755)
	return nil
}

// ServiceShutdown is called when the app terminates
func (a *App) ServiceShutdown() error {
	a.StopSingle()
	if a.client != nil {
		a.client.Disconnect()
	}
	return nil
}

// ------------------------------------------------------------------
// Network / Server Connections
// ------------------------------------------------------------------

func (a *App) ConnectServer(addr string, username string, pass string) map[string]interface{} {
	if username == "" {
		username = "Duelist"
	}
	err := a.client.Connect(addr, username, pass)
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}
	return map[string]interface{}{"success": true}
}

func (a *App) StartLocalServer(port int) map[string]interface{} {
	// 端口优先用前端传入值；缺省时回退到 system.conf 的 serverport（原版
	// 大厅菜单「建立主机」的监听端口），再兜底 7911。
	if port <= 0 {
		port = a.loadConfig().ServerPort
	}
	if port <= 0 {
		port = 7911
	}

	// Initialize server data if needed
	err := duel.InitServerData(a.dbPath, a.scriptPath, ".")
	if err != nil {
		return map[string]interface{}{"success": false, "error": fmt.Sprintf("init error: %v", err)}
	}

	// gnet.Run 绑定端口失败会立刻返回错误，成功则阻塞运行到 Stop；这里后台
	// 起 goroutine，用短超时探测启动错误，避免把 err 静默丢弃。
	errCh := make(chan error, 1)
	go func() {
		errCh <- duel.StartDuelServer(port, false)
	}()
	select {
	case startErr := <-errCh:
		return map[string]interface{}{"success": false, "error": fmt.Sprintf("server failed to start: %v", startErr)}
	case <-time.After(500 * time.Millisecond):
	}

	return map[string]interface{}{"success": true, "port": port}
}

// ------------------------------------------------------------------
// Room / Game Management
// ------------------------------------------------------------------

type HostInfoReq struct {
	LFList        uint32 `json:"lflist"`
	Rule          uint8  `json:"rule"`
	Mode          uint8  `json:"mode"`
	DuelRule      uint8  `json:"duelRule"`
	NoCheckDeck   bool   `json:"noCheckDeck"`
	NoShuffleDeck bool   `json:"noShuffleDeck"`
	StartLp       int32  `json:"startLp"`
	StartHand     uint8  `json:"startHand"`
	DrawCount     uint8  `json:"drawCount"`
	TimeLimit     uint16 `json:"timeLimit"`
}

// LFListEntry 是禁限卡表下拉的一项。gframe 的 HostInfo.LFList 存的是
// 卡表哈希（deck_manager.cpp 逐条异或折叠），不是序号。
type LFListEntry struct {
	Hash uint32 `json:"hash"`
	Name string `json:"name"`
}

// Quit 对应主菜单「退出」按钮（gframe BUTTON_MODE_EXIT → device->closeDevice()）。
func (a *App) Quit() {
	if app := application.Get(); app != nil {
		app.Quit()
	}
}

// ListLFLists 列出已加载的禁限卡表（duelclient 建房窗 cbLFlist 的数据源）。
// LoadLFList 会把「N/A」（哈希 0）追加在末尾；仓库目前没有 lflist.conf，
// 因此通常只有 N/A，缺数据时下拉照常工作。
func (a *App) ListLFLists() []LFListEntry {
	if len(duel.DeckManger.LFList) == 0 {
		duel.DeckManger.LoadLFList()
	}
	entries := make([]LFListEntry, 0, len(duel.DeckManger.LFList))
	for _, l := range duel.DeckManger.LFList {
		entries = append(entries, LFListEntry{Hash: l.Hash, Name: l.ListName})
	}
	return entries
}

func (a *App) CreateGame(req HostInfoReq, roomName string, pass string) map[string]interface{} {
	var hostInfo protocol.HostInfo
	hostInfo.LFList = req.LFList
	hostInfo.Rule = req.Rule
	hostInfo.Mode = req.Mode
	hostInfo.DuelRule = req.DuelRule
	if req.NoCheckDeck {
		hostInfo.NoCheckDeck = 1
	}
	if req.NoShuffleDeck {
		hostInfo.NoShuffleDeck = 1
	}
	hostInfo.StartLp = req.StartLp
	if hostInfo.StartLp <= 0 {
		hostInfo.StartLp = 8000
	}
	hostInfo.StartHand = req.StartHand
	if hostInfo.StartHand == 0 {
		hostInfo.StartHand = 5
	}
	hostInfo.DrawCount = req.DrawCount
	if hostInfo.DrawCount == 0 {
		hostInfo.DrawCount = 1
	}
	hostInfo.TimeLimit = req.TimeLimit
	if hostInfo.TimeLimit == 0 {
		hostInfo.TimeLimit = 180
	}

	err := a.client.CreateGame(hostInfo, roomName, pass)
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}
	return map[string]interface{}{"success": true}
}

func (a *App) JoinGame(pass string) map[string]interface{} {
	err := a.client.JoinGame(0x1361, 0, pass)
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}
	return map[string]interface{}{"success": true}
}

func (a *App) LeaveGame() {
	_ = a.client.LeaveGame()
}

func (a *App) Surrender() {
	_ = a.client.Surrender()
}

func (a *App) SetReady(isReady bool) {
	_ = a.client.SetReady(isReady)
}

func (a *App) StartDuel() {
	_ = a.client.StartDuel()
}

func (a *App) ToObserver() {
	_ = a.client.ToObserver()
}

func (a *App) ToDuelist() {
	_ = a.client.ToDuelist()
}

func (a *App) KickPlayer(pos byte) {
	_ = a.client.SendKick(pos)
}

func (a *App) SendChat(msg string) {
	_ = a.client.SendChat(msg)
}

func (a *App) SendHandResult(res byte) {
	_ = a.client.SendHandResult(res)
}

func (a *App) SendTPResult(res byte) {
	_ = a.client.SendTPResult(res)
}

func (a *App) SendResponseI(val int32) {
	_ = a.routeResponseI(val)
}

func (a *App) SendTimeConfirm() {
	_ = a.client.SendTimeConfirm()
}

func (a *App) UpdateDeck(mainCards []uint32, sideCards []uint32) {
	_ = a.client.UpdateDeck(mainCards, sideCards)
}

// ------------------------------------------------------------------
// Card Database & Deck Editor API
// ------------------------------------------------------------------

func (a *App) GetCard(code uint32) *CardInfo {
	return a.cardDB.GetCard(code)
}

// GetCardImage returns card art as a data URL for the frontend. It follows the
// YGOPro layout: pics/<code>.jpg / .png / .webp (plus expansions/pics and the
// %d-1, %d-2 alternate-art suffixes). Search roots are the executable's
// directory and the working directory, with YGOPRO_PICS_DIR overriding both —
// so the app finds art whether it was launched from the install dir or a
// shortcut with a different working directory. Returns "" when no art exists,
// letting the frontend fall back to its procedural/CDN picture.
func (a *App) GetCardImage(code uint32) string {
	var roots []string
	if dir := strings.TrimSpace(os.Getenv("YGOPRO_PICS_DIR")); dir != "" {
		roots = append(roots, dir)
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}

	type base struct {
		root string
		rel  string
	}
	var bases []base
	for _, root := range roots {
		bases = append(bases,
			base{root, "pics"},
			base{root, filepath.Join("expansions", "pics")},
			// YGOPro2 layout keeps pictures in a versioned subdirectory.
			base{root, filepath.Join("YGOPro", "pics")},
		)
	}

	codeStr := fmt.Sprintf("%d", code)
	// Alias/alternate art convention: %d-1, %d-2, ...
	names := []string{codeStr, codeStr + "-1", codeStr + "-2"}
	exts := []string{".jpg", ".png", ".webp"}
	for _, b := range bases {
		for _, name := range names {
			for _, ext := range exts {
				data, err := os.ReadFile(filepath.Join(b.root, b.rel, name+ext))
				if err != nil {
					continue
				}
				mime := "image/jpeg"
				switch ext {
				case ".png":
					mime = "image/png"
				case ".webp":
					mime = "image/webp"
				}
				return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
			}
		}
	}
	return ""
}

func (a *App) SearchCards(filter CardFilter) []CardInfo {
	return a.cardDB.SearchCards(filter)
}

func (a *App) ListDecks() []string {
	return a.cardDB.ListDecks(a.deckDir)
}

func (a *App) LoadDeck(name string) (*DeckData, error) {
	deckPath, err := safeDeckPath(a.deckDir, name)
	if err != nil {
		return nil, err
	}
	return a.cardDB.LoadDeck(deckPath)
}

func (a *App) SaveDeck(deck DeckData) error {
	deckPath, err := safeDeckPath(a.deckDir, deck.Name)
	if err != nil {
		return err
	}
	return a.cardDB.SaveDeck(deckPath, deck)
}

func (a *App) DeleteDeck(name string) error {
	deckPath, err := safeDeckPath(a.deckDir, name)
	if err != nil {
		return err
	}
	return os.Remove(deckPath)
}

func (a *App) ListReplays() []string {
	files, err := os.ReadDir(a.replayDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".yrp" {
			names = append(names, f.Name())
		}
	}
	return names
}

// ReplayInfo 返回录像头部元信息，对应原版 LISTBOX_REPLAY_LIST 选中时填充
// stReplayInfo 的逻辑（menu_handler.cpp:519-559）：时间取 REPLAY_UNIFORM 的
// start_time，否则取 seed 兼作时间戳；再加玩家名 ===VS=== 布局所需的数据。
func (a *App) ReplayInfo(name string) map[string]interface{} {
	name = filepath.Base(name)
	rm := duel.NewReplayMode()
	if err := rm.Load(filepath.Join(a.replayDir, name)); err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}
	hdr := rm.Replay.ReadHeader()
	// 原版：UNIFORM 录像时间在 start_time，其余用 seed 字段兼作记录时间
	uniform := hdr.Base.Flag&duel.REPLAY_UNIFORM != 0
	rawTime := hdr.Base.Seed
	if uniform {
		rawTime = hdr.Base.StartTime
	}
	date := time.Unix(int64(rawTime), 0).Format("2006/01/02 15:04:05")
	return map[string]interface{}{
		"success":  true,
		"version":  hdr.Base.Version,
		"date":     date,
		"players":  rm.Players,
		"isTag":    rm.IsTag,
		"isSingle": rm.IsSingleMode,
		"script":   rm.ScriptName,
		"startLp":  rm.Params.StartLP,
		"duelRule": uint8(rm.Params.DuelFlag >> 16),
	}
}

// ExportReplayDeck 把录像里双方（组队赛为四人）的卡组导出成 .ydk 到 deckDir，
// 对应原版 BUTTON_EXPORT_DECK（menu_handler.cpp:293-319）：文件名
// <录像文件名>-<序号> <玩家名>.ydk（玩家名经 SafeFileName 清洗），单机录像
// 无卡组信息直接拒绝。卡组序号→玩家映射与原版 Replay::GetDeckPlayer 一致。
func (a *App) ExportReplayDeck(name string) map[string]interface{} {
	name = filepath.Base(name)
	rm := duel.NewReplayMode()
	if err := rm.Load(filepath.Join(a.replayDir, name)); err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}
	if rm.IsSingleMode {
		return map[string]interface{}{"success": false, "error": "单人模式录像没有卡组信息"}
	}
	safeName := func(s string) string {
		return strings.Map(func(r rune) rune {
			if strings.ContainsRune(`<>:"/\|?*`, r) {
				return '_'
			}
			return r
		}, s)
	}
	var saved []string
	for i := range rm.Decks {
		playerIdx := duel.GetDeckPlayer(i)
		player := ""
		if playerIdx < len(rm.Players) {
			player = rm.Players[playerIdx]
		}
		deckName := fmt.Sprintf("%s-%d %s", name, i+1, safeName(player))
		deckPath, err := safeDeckPath(a.deckDir, deckName)
		if err != nil {
			return map[string]interface{}{"success": false, "error": err.Error()}
		}
		if !rm.Replay.SaveDeck(i, deckPath) {
			return map[string]interface{}{"success": false, "error": "写卡组文件失败：" + deckName}
		}
		saved = append(saved, deckName+".ydk")
	}
	return map[string]interface{}{"success": true, "files": saved}
}

// SaveLastReplay 落盘最近一次对局录像（STOC_REPLAY → 缓存 → 前端确认后
// 调用；auto_save_replay=1 时前端直接调用）。返回实际保存的文件名。
func (a *App) SaveLastReplay(name string) map[string]interface{} {
	if a.client == nil {
		return map[string]interface{}{"success": false, "error": "not connected"}
	}
	a.client.replayDir = a.replayDir
	saved, err := a.client.SaveReplay(name)
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}
	return map[string]interface{}{"success": true, "name": saved + ".yrp"}
}

func (a *App) DeleteReplay(name string) error {
	name = filepath.Base(name)
	return os.Remove(filepath.Join(a.replayDir, name))
}

func (a *App) RenameReplay(oldName string, newName string) error {
	oldName = filepath.Base(oldName)
	if !strings.HasSuffix(oldName, ".yrp") {
		oldName += ".yrp"
	}
	newName = strings.TrimSpace(filepath.Base(newName))
	newName = strings.ReplaceAll(newName, "/", "_")
	newName = strings.ReplaceAll(newName, "\\", "_")
	if newName == "" {
		return errors.New("empty replay name")
	}
	if !strings.HasSuffix(newName, ".yrp") {
		newName += ".yrp"
	}
	return os.Rename(filepath.Join(a.replayDir, oldName), filepath.Join(a.replayDir, newName))
}

// PlayReplay loads a recorded .yrp duel and replays it against the ocgcore
// engine, returning every engine message as a {type, data} event for the
// step-based Replay Theater. The engine regenerates messages deterministically
// from the replay's seed; only player responses are read back from disk.
//
// The engine never emits MSG_START (the server layer synthesizes it), so the
// start event is built here from the replay metadata. Without the card scripts
// installed the engine may diverge mid-replay; the events collected up to that
// point are still returned so the viewer remains usable.
func (a *App) PlayReplay(name string) map[string]interface{} {
	// ocgcore needs the card database/script root initialized before a duel.
	if err := duel.InitServerData(a.dbPath, a.scriptPath, "."); err != nil {
		return map[string]interface{}{"success": false, "error": fmt.Sprintf("init error: %v", err)}
	}

	rm := duel.NewReplayMode()
	replayPath := filepath.Join(a.replayDir, name)
	if err := rm.Load(replayPath); err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}

	// Synthesize the opening duel:start event from the replay metadata.
	deckCount := func(i int) int {
		if i >= len(rm.Decks) {
			return 0
		}
		return len(rm.Decks[i].Main)
	}
	extraCount := func(i int) int {
		if i >= len(rm.Decks) {
			return 0
		}
		return len(rm.Decks[i].Extra)
	}

	events := []map[string]interface{}{
		{
			"type": "duel:start",
			"data": map[string]interface{}{
				"playerType": uint8(0),
				"duelRule":   uint8(rm.Params.DuelFlag >> 16),
				"lp0":        rm.Params.StartLP,
				"lp1":        rm.Params.StartLP,
				"deck0":      deckCount(0),
				"extra0":     extraCount(0),
				"deck1":      deckCount(1),
				"extra1":     extraCount(1),
			},
		},
	}

	// Collect engine events through the existing parser instead of emitting
	// them live; the Replay Theater steps through them on demand. Interaction
	// prompts (select_*/waiting) are questions the engine asks a duelist — in
	// replay the answers are fed back from disk, so they would only pop
	// modals. update_data/update_card are raw query refreshes with no visuals
	// the field renders from, so they are filtered out too.
	collector := NewWailsDuelClient(func(eventName string, optionalData ...interface{}) {
		if strings.HasPrefix(eventName, "duel:select_") ||
			eventName == "duel:waiting" ||
			eventName == "duel:update_data" ||
			eventName == "duel:update_card" {
			return
		}
		var data interface{}
		if len(optionalData) > 0 {
			data = optionalData[0]
		}
		events = append(events, map[string]interface{}{"type": eventName, "data": data})
	})

	err := rm.Run(func(msg []byte) {
		collector.handleGameMessage(msg)
	})
	// 中途截断的录像（underflow / desync）不视为致命：已收集的事件照常返回，
	// truncated 标记给前端。desync 错误经过 %w 包装，必须用 errors.Is 判断。
	if err != nil && !errors.Is(err, duel.ErrReplayResponseUnderflow) && !errors.Is(err, duel.ErrReplayDesynchronized) {
		return map[string]interface{}{"success": false, "error": err.Error()}
	}

	return map[string]interface{}{
		"success":   true,
		"name":      name,
		"players":   rm.Players,
		"startLp":   rm.Params.StartLP,
		"duelRule":  uint8(rm.Params.DuelFlag >> 16),
		"truncated": err != nil,
		"events":    events,
	}
}

// EmitWailsEvent dispatches a Wails v3 custom event to the frontend.
var EmitWailsEvent = func(ctx context.Context, name string, data interface{}) {
	if app := application.Get(); app != nil && app.Event != nil {
		app.Event.Emit(name, data)
	}
}
