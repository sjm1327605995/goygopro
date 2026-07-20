package client

// Config corresponds to C++ struct Config in game.h
// Game configuration loaded from/saved to system.conf
type Config struct {
	UseD3D                       bool
	UseImageScaleMultiThread     bool
	UseImageLoadBackgroundThread bool
	Antialias                    uint16
	EnableLog                    uint32
	ServerPort                   uint16
	TextFontSize                 uint8
	LastHost                     string
	LastPort                     string
	Nickname                     string
	GameName                     string
	RoomPass                     string
	LastCategory                 string
	LastDeck                     string
	TextFont                     string
	NumFont                      string
	BotDeckPath                  string
	// settings
	ChkMAutoPos            int
	ChkSTAutoPos           int
	ChkRandomPos           int
	ChkAutoChain           int
	ChkWaitChain           int
	ChkDefaultShowChain    int
	ChkIgnore1             int
	ChkIgnore2             int
	UseLFList              int
	DefaultLFList          int
	DefaultRule            int
	HideSetName            int
	HideHintButton         int
	ControlMode            int
	DrawFieldSpell         int
	SeparateClearButton    int
	AutoSearchLimit        int
	SearchMultipleKeywords int
	ChkIgnoreDeckChanges   int
	DefaultOT              int
	EnableBotMode          int
	QuickAnimation         int
	AutoSaveReplay         int
	DrawSingleChain        int
	HidePlayerName         int
	PreferExpansionScript  int
	EnableSound            bool
	EnableMusic            bool
	SoundVolume            int
	MusicVolume            int
	MusicMode              int
	WindowMaximized        bool
	WindowWidth            int
	WindowHeight           int
	ResizePopupMenu        int
	ResizeSelectWindow     bool
	SwapYesNoButton        bool
}

// DuelInfo corresponds to C++ struct DuelInfo in game.h
// Runtime duel state displayed in the UI
type DuelInfo struct {
	IsStarted        bool
	IsInDuel         bool
	IsFinished       bool
	IsReplay         bool
	IsReplaySkipping bool
	IsFirst          bool
	IsTag            bool
	IsSingleMode     bool
	IsShuffling      bool
	TagPlayer        [2]bool
	IsReplaySwapped  bool
	Swapped          bool
	LP               [2]int
	StartLP          int
	DuelRule         int
	Turn             int
	Phase            uint16
	CurMsg           int16
	HostName         string
	ClientName       string
	HostNameTag      string
	ClientNameTag    string
	StrLP            [2]string
	VicString        string
	// 胜负结果：WinPlayer 是获胜方（2=平局），WinType 是胜利原因（见 victoryReason）。
	WinPlayer  int
	WinType    int
	PlayerType uint8
	TimePlayer uint8
	TimeLimit  uint16
	TimeLeft   [2]uint16
}

func (d *DuelInfo) Clear() {
	*d = DuelInfo{}
}

// BotInfo corresponds to C++ struct BotInfo in game.h
type BotInfo struct {
	Name                  string
	Command               string
	Desc                  string
	SupportMasterRule3    bool
	SupportNewMasterRule  bool
	SupportMasterRule2020 bool
	SelectDeckFile        bool
}

// FadingUnit corresponds to C++ struct FadingUnit in game.h
// GUI fade-in/fade-out animation state
type FadingUnit struct {
	SignalAction     bool
	IsFadeIn         bool
	FadingFrame      int
	AutoFadeoutFrame int
}

// Game constants from C++ game.h
const (
	DefaultDuelRule         = 5
	ConfigLineSize          = 1024
	GameWindowWidth         = 1024
	GameWindowHeight        = 640
	CardImgWidth            = 177
	CardImgHeight           = 254
	CardThumbWidth          = 44
	CardThumbHeight         = 64
	SIZE_SETCODE            = 16
	DESC_COUNT              = 16
	MAX_STRING_ID           = 0x7ff
	MIN_CARD_ID             = (MAX_STRING_ID + 1) >> 4
	MAX_CARD_ID             = 0x0fffffff
	TEXT_LINE_SIZE          = 256
	DECK_MAX_SIZE           = 60
	DECK_MIN_SIZE           = 40
	EXTRA_MAX_SIZE          = 15
	SIDE_MAX_SIZE           = 15
	PACK_MAX_SIZE           = 1000
	MAINC_MAX               = 250
	SIDEC_MAX               = MAINC_MAX
	DECK_CATEGORY_PACK      = 0
	DECK_CATEGORY_BOT       = 1
	DECK_CATEGORY_NONE      = 2
	DECK_CATEGORY_SEPARATOR = 3
	DECK_CATEGORY_CUSTOM    = 4
)
