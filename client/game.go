package client

import (
	"fmt"
	"sync"

	"github.com/sjm1327605995/goygopro/protocol"
)

// Game corresponds to C++ class Game in game.h
// In the Go rewrite, Game acts as the global state manager.
type Game struct {
	mu sync.RWMutex

	Config   Config
	DInfo    DuelInfo
	DField   *ClientField
	DeckMgr  *DeckManager
	ImageMgr *ImageManager

	// Scene navigation. UI 层订阅它切换场景。
	SceneSignal *Store[string]

	// Duel 状态（DField / DInfo / 聊天）的变更计数器：这些状态由网络线程整块改写，
	// 逐字段做成 Store 不划算，改完 Bump 一次让 UI 整体重读。
	FieldRev *Revision

	// Host info (for room creation)
	HostInfo protocol.HostInfo

	// Chat
	ChatMsg       [8]string
	ChatTiming    [8]int
	ChatType      [8]int
	HideChat      bool
	HideChatTimer int

	// Animation / overlay state
	WaitFrame    int
	SignalFrame  int
	ActionParam  int
	ShowingCode  int
	ShowingText  string
	ShowCard     int
	ShowCardCode int
	ShowCardDif  int
	ShowCardP    int
	IsAttacking  int
	AttackSv     int
	LpFrame      int
	LpD          int
	LpPlayer     int
	LpCString    string

	// Flags
	AlwaysChain    bool
	IgnoreChain    bool
	ChainWhenAvail bool
	IsBuilding     bool
	IsSiding       bool
	// 换副卡组前三部分的张数。换牌只允许在主/额外/副之间挪，三边数量必须保持不变。
	SidePreMain  int
	SidePreExtra int
	SidePreSide  int
	ExitOnReturn bool
	OpenFile     bool
	OpenFileName string
	BotMode      bool

	// Lobby state
	HostPrepNames [4]string
	HostPrepReady [4]bool
	ObserverCount int

	// Dialog state (for select popups)
	Dialog *DialogState

	// Scale
	XScale float32
	YScale float32

	// Window size
	WindowWidth  int
	WindowHeight int
}

// MainGame is the global Game instance (corresponds to C++ mainGame global pointer).
var MainGame = &Game{
	Config: Config{
		TextFontSize:           14,
		ServerPort:             7911,
		ChkSTAutoPos:           1,
		UseLFList:              1,
		DefaultRule:            DefaultDuelRule,
		DrawFieldSpell:         1,
		SeparateClearButton:    1,
		SearchMultipleKeywords: 1,
		DefaultOT:              1,
		EnableSound:            true,
		EnableMusic:            true,
		SoundVolume:            50,
		MusicVolume:            50,
		MusicMode:              1,
		WindowWidth:            GameWindowWidth,
		WindowHeight:           GameWindowHeight,
		ResizeSelectWindow:     true,
	},
	DField:       NewClientField(),
	DeckMgr:      DeckMgr,
	ImageMgr:     ImageMgr,
	Dialog:       NewDialogState(),
	SceneSignal:  NewStore("mainMenu"),
	FieldRev:     NewRevision(),
	XScale:       1.0,
	YScale:       1.0,
	WindowWidth:  GameWindowWidth,
	WindowHeight: GameWindowHeight,
}

func (g *Game) Initialize() bool {
	g.LoadConfig()
	// 界面上的中文提示都来自 strings.conf。读不到不算致命错误 ——
	// 各 Get 会回显编号，界面还能用，只是文字变成「系统提示 1390」这样。
	if err := Strings.LoadStrings("strings.conf"); err != nil {
		fmt.Printf("strings.conf 读取失败，界面文本将显示为编号: %v\n", err)
	}
	if !g.ImageMgr.Initial() {
		return false
	}
	return true
}

func (g *Game) LoadConfig() {
	// 在默认值基础上覆盖：配置文件里没写的项保持默认，与 C++ 行为一致。
	if err := LoadConfigFile("system.conf", &g.Config); err != nil {
		return
	}
	if g.Config.WindowWidth == 0 {
		g.Config.WindowWidth = GameWindowWidth
	}
	if g.Config.WindowHeight == 0 {
		g.Config.WindowHeight = GameWindowHeight
	}
}

func (g *Game) SaveConfig() {
	_ = SaveConfigFile("system.conf", &g.Config)
}

func (g *Game) LocalPlayer(player int) int {
	if g.DInfo.IsTag {
		if g.DInfo.Swapped {
			return (player + 1) % 2
		}
	}
	if g.DInfo.Swapped {
		return 1 - player
	}
	return player
}

func (g *Game) OppositePlayer(player int) int {
	return 1 - player
}

func (g *Game) ChatLocalPlayer(player int) int {
	if player > 3 {
		return player
	}
	return g.LocalPlayer(player)
}

func (g *Game) AddChatMsg(msg string, player int, playSound bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := 7; i > 0; i-- {
		g.ChatMsg[i] = g.ChatMsg[i-1]
		g.ChatTiming[i] = g.ChatTiming[i-1]
		g.ChatType[i] = g.ChatType[i-1]
	}
	g.ChatMsg[0] = msg
	g.ChatTiming[0] = 0
	g.ChatType[0] = player
	g.HideChat = false
	g.HideChatTimer = 0
}

func (g *Game) ClearChatMsg() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := 0; i < 8; i++ {
		g.ChatMsg[i] = ""
	}
}

func (g *Game) SetHostPrepName(pos int, name string) {
	g.mu.Lock()
	if pos >= 0 && pos < 4 {
		g.HostPrepNames[pos] = name
	}
	g.mu.Unlock()
}

func (g *Game) SetHostPrepReady(pos int, ready bool) {
	g.mu.Lock()
	if pos >= 0 && pos < 4 {
		g.HostPrepReady[pos] = ready
	}
	g.mu.Unlock()
}

func (g *Game) GetHostPrepName(pos int) string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if pos >= 0 && pos < 4 {
		return g.HostPrepNames[pos]
	}
	return ""
}

func (g *Game) GetHostPrepReady(pos int) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if pos >= 0 && pos < 4 {
		return g.HostPrepReady[pos]
	}
	return false
}

// PushScene changes to the named scene via the global scene signal.
func PushScene(name string) {
	MainGame.SceneSignal.Set(name)
}

// PopScene returns to the previous scene (simplified: goes to mainMenu).
func PopScene() {
	MainGame.SceneSignal.Set("mainMenu")
}
