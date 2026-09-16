package main

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
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
	DefaultOT     int `conf:"default_ot"` // 1=OCG+TCG 2=OCG 4=TCG 8=自定（原版键名 game.cpp:1463）

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

// loadConfig 读取 system.conf；文件不存在或个别行解析失败都按默认值兜底。
// 解析语义照抄原版：先按 "key = value" 严格匹配，键名小写比对。
func (a *App) loadConfig() AppConfig {
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

// applyConfigLine 单行应用（loadConfig 与 SaveConfig 的合并补丁共用）。
// 键 → setter 的映射由 configSetters 表提供（见下），未知键忽略。
func applyConfigLine(cfg *AppConfig, key, val string) {
	if set, ok := configSetters[key]; ok {
		set(cfg, val)
	}
}

// configSetters 是 conf 键 → 字段 setter 的总表，由 AppConfig 的 conf tag
// 反射生成（tag 即原版 system.conf 键名，config.go 结构体声明处注释）。
// 生成时机保留原版每键的转换语义：
//   - int 键：atoiOr 失败回退当前值（非法值不覆盖已有配置）；
//   - bool 键：仅 "true"/"1" 为真，其余一律为假（与原版一致，不做回退）；
//   - string 键：原样赋值。
//
// 两类键在反射生成后追加特例：
//   - sound_volume/music_volume：赋值后再夹取到 0-100；
//   - defaultot：本项目早期误拼键名，作为 default_ot 的别名兼容旧 conf。
var configSetters = buildConfigSetters()

func buildConfigSetters() map[string]func(*AppConfig, string) {
	setters := map[string]func(*AppConfig, string){}
	t := reflect.TypeOf(AppConfig{})
	for i := 0; i < t.NumField(); i++ {
		idx, field := i, t.Field(i)
		key := field.Tag.Get("conf")
		if key == "" {
			continue
		}
		switch field.Type.Kind() {
		case reflect.Int:
			setters[key] = func(cfg *AppConfig, val string) {
				fv := reflect.ValueOf(cfg).Elem().Field(idx)
				fv.SetInt(int64(atoiOr(val, int(fv.Int()))))
			}
		case reflect.String:
			setters[key] = func(cfg *AppConfig, val string) {
				reflect.ValueOf(cfg).Elem().Field(idx).SetString(val)
			}
		case reflect.Bool:
			setters[key] = func(cfg *AppConfig, val string) {
				reflect.ValueOf(cfg).Elem().Field(idx).SetBool(val == "true" || val == "1")
			}
		}
	}
	for _, k := range []string{"sound_volume", "music_volume"} {
		inner, fieldName := setters[k], clampedFields[k]
		setters[k] = func(cfg *AppConfig, val string) {
			inner(cfg, val)
			fv := reflect.ValueOf(cfg).Elem().FieldByName(fieldName)
			fv.SetInt(int64(clampInt(int(fv.Int()), 0, 100)))
		}
	}
	setters["defaultot"] = setters["default_ot"]
	return setters
}

// clampedFields：音量键 → AppConfig 字段名（夹取特例用）。
var clampedFields = map[string]string{
	"sound_volume": "SoundVolume",
	"music_volume": "MusicVolume",
}

// SaveConfig 用补丁合并当前配置并写回 system.conf（键名即 conf 键）。
// 返回合并后的完整配置。未知键忽略；roompass 等原版只读不写的键不在本表内。
func (a *App) SaveConfig(patch map[string]interface{}) AppConfig {
	cfg := a.loadConfig()
	for k, v := range patch {
		s := fmt.Sprintf("%v", v)
		applyConfigLine(&cfg, strings.ToLower(strings.TrimSpace(k)), s)
	}
	_ = a.writeConfig(cfg)
	return cfg
}

// GetConfig 返回完整配置快照（JSON-friendly map，键=conf 键）。
func (a *App) GetConfig() map[string]interface{} {
	cfg := a.loadConfig()
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
