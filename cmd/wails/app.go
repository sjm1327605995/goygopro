package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/protocol"
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
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{
		dbPath:     "cards.cdb",
		scriptPath: "script",
		deckDir:    "deck",
		replayDir:  "replay",
		cardDB:     NewCardDBManager(),
	}
	return app
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
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
	a.client.SetContext(ctx)

	// Initialize card database
	if _, err := os.Stat(a.dbPath); err == nil {
		_ = a.cardDB.OpenDB(a.dbPath)
	}

	// Initialize deck & replay dirs
	_ = os.MkdirAll(a.deckDir, 0755)
	_ = os.MkdirAll(a.replayDir, 0755)
}

// Shutdown is called when the app terminates
func (a *App) Shutdown(ctx context.Context) {
	if a.client != nil {
		a.client.Disconnect()
	}
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

func (a *App) DisconnectServer() {
	if a.client != nil {
		a.client.Disconnect()
	}
}

func (a *App) StartLocalServer(port int) map[string]interface{} {
	if port <= 0 {
		port = 7911
	}

	// Initialize server data if needed
	err := duel.InitServerData(a.dbPath, a.scriptPath, ".")
	if err != nil {
		return map[string]interface{}{"success": false, "error": fmt.Sprintf("init error: %v", err)}
	}

	go func() {
		_ = duel.StartDuelServer(port, false)
	}()

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
	_ = a.client.SendResponseI(val)
}

func (a *App) SendResponseB(data []byte) {
	_ = a.client.SendResponseB(data)
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

func (a *App) SearchCards(filter CardFilter) []CardInfo {
	return a.cardDB.SearchCards(filter)
}

func (a *App) ListDecks() []string {
	return a.cardDB.ListDecks(a.deckDir)
}

func (a *App) LoadDeck(name string) (*DeckData, error) {
	deckPath := filepath.Join(a.deckDir, name+".ydk")
	return a.cardDB.LoadDeck(deckPath)
}

func (a *App) SaveDeck(deck DeckData) error {
	deckPath := filepath.Join(a.deckDir, deck.Name+".ydk")
	return a.cardDB.SaveDeck(deckPath, deck)
}

func (a *App) DeleteDeck(name string) error {
	deckPath := filepath.Join(a.deckDir, name+".ydk")
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

// EmitWailsEvent dispatches event to Wails frontend or logs
var EmitWailsEvent = func(ctx context.Context, name string, data interface{}) {
	// Standard Wails event dispatch placeholder
}
