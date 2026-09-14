package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// AppConfig：原版 gframe system.conf 配置项的完整翻译。
//
// 键名与 vendored source/ygopro/gframe/game.cpp:1395-1607 的
// LoadConfig/SaveConfig 一一对应（原版是 Game::Config 结构体字段
// game.h:47-104），这样同一份 system.conf 可以在两个客户端间互换。
// 原版中仅 conf 无 UI 的键也一并提供默认值，保持语义完整。
type AppConfig struct {
	// ---- 决斗辅助（原版 tabHelper 复选框） ----
	AutoMonsterPos  int `conf:"automonsterpos"` // 怪兽自动放置（原版默认 0）
	AutoSpellPos    int `conf:"autospellpos"`   // 魔陷自动放置（原版默认 1）
	RandomPos       int `conf:"randompos"`      // 放置位置随机
	AutoChain       int `conf:"autochain"`      // 自动连锁（不询问）
	WaitChain       int `conf:"waitchain"`      // 有可连锁时等待
	ShowChain       int `conf:"showchain"`      // 默认显示连锁
	QuickAnimation  int `conf:"quick_animation"`
	AutoSaveReplay  int `conf:"auto_save_replay"`
	DrawSingleChain int `conf:"draw_single_chain"`

	// ---- 系统设置（原版 tabSystem 复选框） ----
	MuteOpponent           int `conf:"mute_opponent"` // 屏蔽对手聊天
	MuteSpectators         int `conf:"mute_spectators"`
	HidePlayerName         int `conf:"hide_player_name"`
	IgnoreDeckChanges      int `conf:"ignore_deck_changes"`
	AutoSearchLimit        int `conf:"auto_search_limit"`        // -1 关；≥0 输入 N 字自动搜索
	SearchMultipleKeywords int `conf:"search_multiple_keywords"` // 0 关 1 空格 2 +
	DrawFieldSpell         int `conf:"draw_field_spell"`         // 场地魔法背景（原版默认 1）
	SeparateClearButton    int `conf:"separate_clear_button"`
	HideSetName            int `conf:"hide_setname"`
	HideHintButton         int `conf:"hide_hint_button"`
	SwapYesNoButton        int `conf:"swap_yes_no_button"`
	ResizeSelectWindow     int `conf:"resize_select_window"`
	ResizePopupMenu        int `conf:"resize_popup_menu"` // 0-5 档
	ControlMode            int `conf:"control_mode"`      // 0=快捷键+右键菜单 1=鼠标模式
	PreferExpansionScript  int `conf:"prefer_expansion_script"`

	// ---- 音频（原版 chkEnableSound/scrSoundVolume 等） ----
	EnableSound bool `conf:"enable_sound"`
	EnableMusic bool `conf:"enable_music"`
	SoundVolume int  `conf:"sound_volume"` // 0-100
	MusicVolume int  `conf:"music_volume"` // 0-100
	MusicMode   int  `conf:"music_mode"`   // 1=分场景 0=混合

	// ---- 卡组/禁限/规则 ----
	UseLFList     int `conf:"use_lflist"`
	DefaultLFList int `conf:"default_lflist"`
	DefaultRule   int `conf:"default_rule"`
	DefaultOT     int `conf:"defaultOT"` // 1=OCG+TCG 2=OCG 4=TCG 8=自定

	// ---- 大厅记忆（原版各输入框回填） ----
	Nickname     string `conf:"nickname"`
	GameName     string `conf:"gamename"`
	LastHost     string `conf:"lasthost"`
	LastPort     string `conf:"lastport"`
	LastCategory string `conf:"lastcategory"`
	LastDeck     string `conf:"lastdeck"`
	ServerPort   int    `conf:"serverport"` // 本机建房监听端口（原版默认 7911）
}

// defaultConfig 返回带原版默认值的配置（game.h:47-104 的字段初始化）。
func defaultConfig() AppConfig {
	return AppConfig{
		AutoSpellPos:           1,
		DrawFieldSpell:         1,
		SeparateClearButton:    1,
		ResizeSelectWindow:     1,
		SearchMultipleKeywords: 1,
		AutoSearchLimit:        -1,
		EnableSound:            true,
		EnableMusic:            true,
		SoundVolume:            50,
		MusicVolume:            50,
		MusicMode:              1,
		UseLFList:              1,
		DefaultOT:              1,
		ServerPort:             7911,
	}
}

// configFilePath：工作目录下的 system.conf（与原版一致）。
func (a *App) configFilePath() string { return "system.conf" }

// LoadConfig 读取 system.conf；文件不存在或个别行解析失败都按默认值兜底。
// 解析语义照抄原版：先按 "key = value" 严格匹配，键名小写比对。
func (a *App) LoadConfig() AppConfig {
	cfg := defaultConfig()
	f, err := os.Open(a.configFilePath())
	if err != nil {
		return cfg
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:eq]))
		val := strings.TrimSpace(line[eq+1:])
		applyConfigLine(&cfg, key, val)
	}
	return cfg
}

// applyConfigLine 单行应用（LoadConfig 与 SaveConfig 的合并补丁共用）。
func applyConfigLine(cfg *AppConfig, key, val string) {
	switch key {
	case "automonsterpos":
		cfg.AutoMonsterPos = atoiOr(val, cfg.AutoMonsterPos)
	case "autospellpos":
		cfg.AutoSpellPos = atoiOr(val, cfg.AutoSpellPos)
	case "randompos":
		cfg.RandomPos = atoiOr(val, cfg.RandomPos)
	case "autochain":
		cfg.AutoChain = atoiOr(val, cfg.AutoChain)
	case "waitchain":
		cfg.WaitChain = atoiOr(val, cfg.WaitChain)
	case "showchain":
		cfg.ShowChain = atoiOr(val, cfg.ShowChain)
	case "quick_animation":
		cfg.QuickAnimation = atoiOr(val, cfg.QuickAnimation)
	case "auto_save_replay":
		cfg.AutoSaveReplay = atoiOr(val, cfg.AutoSaveReplay)
	case "draw_single_chain":
		cfg.DrawSingleChain = atoiOr(val, cfg.DrawSingleChain)
	case "mute_opponent":
		cfg.MuteOpponent = atoiOr(val, cfg.MuteOpponent)
	case "mute_spectators":
		cfg.MuteSpectators = atoiOr(val, cfg.MuteSpectators)
	case "hide_player_name":
		cfg.HidePlayerName = atoiOr(val, cfg.HidePlayerName)
	case "ignore_deck_changes":
		cfg.IgnoreDeckChanges = atoiOr(val, cfg.IgnoreDeckChanges)
	case "auto_search_limit":
		cfg.AutoSearchLimit = atoiOr(val, cfg.AutoSearchLimit)
	case "search_multiple_keywords":
		cfg.SearchMultipleKeywords = atoiOr(val, cfg.SearchMultipleKeywords)
	case "draw_field_spell":
		cfg.DrawFieldSpell = atoiOr(val, cfg.DrawFieldSpell)
	case "separate_clear_button":
		cfg.SeparateClearButton = atoiOr(val, cfg.SeparateClearButton)
	case "hide_setname":
		cfg.HideSetName = atoiOr(val, cfg.HideSetName)
	case "hide_hint_button":
		cfg.HideHintButton = atoiOr(val, cfg.HideHintButton)
	case "swap_yes_no_button":
		cfg.SwapYesNoButton = atoiOr(val, cfg.SwapYesNoButton)
	case "resize_select_window":
		cfg.ResizeSelectWindow = atoiOr(val, cfg.ResizeSelectWindow)
	case "resize_popup_menu":
		cfg.ResizePopupMenu = atoiOr(val, cfg.ResizePopupMenu)
	case "control_mode":
		cfg.ControlMode = atoiOr(val, cfg.ControlMode)
	case "prefer_expansion_script":
		cfg.PreferExpansionScript = atoiOr(val, cfg.PreferExpansionScript)
	case "enable_sound":
		cfg.EnableSound = val == "true" || val == "1"
	case "enable_music":
		cfg.EnableMusic = val == "true" || val == "1"
	case "sound_volume":
		cfg.SoundVolume = clampInt(atoiOr(val, cfg.SoundVolume), 0, 100)
	case "music_volume":
		cfg.MusicVolume = clampInt(atoiOr(val, cfg.MusicVolume), 0, 100)
	case "music_mode":
		cfg.MusicMode = atoiOr(val, cfg.MusicMode)
	case "use_lflist":
		cfg.UseLFList = atoiOr(val, cfg.UseLFList)
	case "default_lflist":
		cfg.DefaultLFList = atoiOr(val, cfg.DefaultLFList)
	case "default_rule":
		cfg.DefaultRule = atoiOr(val, cfg.DefaultRule)
	case "default_ot", "defaultot":
		// 原版读写键为 default_ot（game.cpp:1463/1586）；defaultot 是本
		// 项目早期误拼的键名，兼容旧 conf 文件继续可读
		cfg.DefaultOT = atoiOr(val, cfg.DefaultOT)
	case "nickname":
		cfg.Nickname = val
	case "gamename":
		cfg.GameName = val
	case "lasthost":
		cfg.LastHost = val
	case "lastport":
		cfg.LastPort = val
	case "lastcategory":
		cfg.LastCategory = val
	case "lastdeck":
		cfg.LastDeck = val
	case "serverport":
		cfg.ServerPort = atoiOr(val, cfg.ServerPort)
	}
}

// SaveConfig 用补丁合并当前配置并写回 system.conf（键名即 conf 键）。
// 返回合并后的完整配置。未知键忽略；roompass 等原版只读不写的键不在本表内。
func (a *App) SaveConfig(patch map[string]interface{}) AppConfig {
	cfg := a.LoadConfig()
	for k, v := range patch {
		s := fmt.Sprintf("%v", v)
		applyConfigLine(&cfg, strings.ToLower(strings.TrimSpace(k)), s)
	}
	_ = a.writeConfig(cfg)
	return cfg
}

// GetConfig 返回完整配置快照（JSON-friendly map，键=conf 键）。
func (a *App) GetConfig() map[string]interface{} {
	cfg := a.LoadConfig()
	return configToMap(cfg)
}

// writeConfig 落盘为原版格式的 "key = value" 行序。
func (a *App) writeConfig(cfg AppConfig) error {
	m := configToMap(cfg)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 稳定排序：Go map 遍历无序，写盘顺序抖动会污染 diff 与冒烟
	sort.Strings(keys)

	f, err := os.Create(a.configFilePath())
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, k := range keys {
		fmt.Fprintf(w, "%s = %v\n", k, m[k])
	}
	return w.Flush()
}

func configToMap(cfg AppConfig) map[string]interface{} {
	return map[string]interface{}{
		"automonsterpos":           cfg.AutoMonsterPos,
		"autospellpos":             cfg.AutoSpellPos,
		"randompos":                cfg.RandomPos,
		"autochain":                cfg.AutoChain,
		"waitchain":                cfg.WaitChain,
		"showchain":                cfg.ShowChain,
		"quick_animation":          cfg.QuickAnimation,
		"auto_save_replay":         cfg.AutoSaveReplay,
		"draw_single_chain":        cfg.DrawSingleChain,
		"mute_opponent":            cfg.MuteOpponent,
		"mute_spectators":          cfg.MuteSpectators,
		"hide_player_name":         cfg.HidePlayerName,
		"ignore_deck_changes":      cfg.IgnoreDeckChanges,
		"auto_search_limit":        cfg.AutoSearchLimit,
		"search_multiple_keywords": cfg.SearchMultipleKeywords,
		"draw_field_spell":         cfg.DrawFieldSpell,
		"separate_clear_button":    cfg.SeparateClearButton,
		"hide_setname":             cfg.HideSetName,
		"hide_hint_button":         cfg.HideHintButton,
		"swap_yes_no_button":       cfg.SwapYesNoButton,
		"resize_select_window":     cfg.ResizeSelectWindow,
		"resize_popup_menu":        cfg.ResizePopupMenu,
		"control_mode":             cfg.ControlMode,
		"prefer_expansion_script":  cfg.PreferExpansionScript,
		"enable_sound":             cfg.EnableSound,
		"enable_music":             cfg.EnableMusic,
		"sound_volume":             cfg.SoundVolume,
		"music_volume":             cfg.MusicVolume,
		"music_mode":               cfg.MusicMode,
		"use_lflist":               cfg.UseLFList,
		"default_lflist":           cfg.DefaultLFList,
		"default_rule":             cfg.DefaultRule,
		"default_ot":               cfg.DefaultOT, // 原版键名（game.cpp:1586）
		"nickname":                 cfg.Nickname,
		"gamename":                 cfg.GameName,
		"lasthost":                 cfg.LastHost,
		"lastport":                 cfg.LastPort,
		"lastcategory":             cfg.LastCategory,
		"lastdeck":                 cfg.LastDeck,
		"serverport":               cfg.ServerPort,
	}
}

func atoiOr(s string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return n
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
