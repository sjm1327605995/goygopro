package client

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// system.conf 的读写。
//
// 原版 ygopro 用的是 `key = value` 的纯文本（game.cpp 的 Game::LoadConfig 逐行 sscanf），
// 而这里一度写成了 JSON —— 文件名却还是 system.conf。后果是双向的：
// 老用户的配置读不进来（JSON 解析失败就静默用默认值，玩家不知道为什么设置没生效），
// 而这边一保存又会把人家原来的配置覆盖成 JSON。
//
// 所以格式必须回到 key = value。字段名与 C++ 完全一致，两边可以互相读写。

// configBinding 把一个配置项与它在 Config 里的读写方式绑在一起。
// 用显式的 get/set 而不是反射：键名与字段名对不上的地方不少
// （errorlog→EnableLog、automonsterpos→ChkMAutoPos……），反射反而更容易出错。
type configBinding struct {
	get func(c *Config) string
	set func(c *Config, v string)
}

func boolBinding(ptr func(*Config) *bool) configBinding {
	return configBinding{
		get: func(c *Config) string {
			if *ptr(c) {
				return "1"
			}
			return "0"
		},
		set: func(c *Config, v string) { *ptr(c) = atoiDefault(v, 0) > 0 },
	}
}

func intBinding[T ~int | ~uint8 | ~uint16 | ~uint32](ptr func(*Config) *T) configBinding {
	return configBinding{
		get: func(c *Config) string { return strconv.Itoa(int(*ptr(c))) },
		set: func(c *Config, v string) { *ptr(c) = T(atoiDefault(v, 0)) },
	}
}

func strBinding(ptr func(*Config) *string) configBinding {
	return configBinding{
		get: func(c *Config) string { return *ptr(c) },
		set: func(c *Config, v string) { *ptr(c) = v },
	}
}

// configKeys 是键名到字段的映射，键名照抄 C++ game.cpp 的 LoadConfig。
var configKeys = map[string]configBinding{
	"antialias":                        intBinding(func(c *Config) *uint16 { return &c.Antialias }),
	"use_d3d":                          boolBinding(func(c *Config) *bool { return &c.UseD3D }),
	"use_image_scale_multi_thread":     boolBinding(func(c *Config) *bool { return &c.UseImageScaleMultiThread }),
	"use_image_load_background_thread": boolBinding(func(c *Config) *bool { return &c.UseImageLoadBackgroundThread }),
	"errorlog":                         intBinding(func(c *Config) *uint32 { return &c.EnableLog }),
	"serverport":                       intBinding(func(c *Config) *uint16 { return &c.ServerPort }),
	"lasthost":                         strBinding(func(c *Config) *string { return &c.LastHost }),
	"lastport":                         strBinding(func(c *Config) *string { return &c.LastPort }),
	"automonsterpos":                   intBinding(func(c *Config) *int { return &c.ChkMAutoPos }),
	"autospellpos":                     intBinding(func(c *Config) *int { return &c.ChkSTAutoPos }),
	"randompos":                        intBinding(func(c *Config) *int { return &c.ChkRandomPos }),
	"autochain":                        intBinding(func(c *Config) *int { return &c.ChkAutoChain }),
	"waitchain":                        intBinding(func(c *Config) *int { return &c.ChkWaitChain }),
	"showchain":                        intBinding(func(c *Config) *int { return &c.ChkDefaultShowChain }),
	"mute_opponent":                    intBinding(func(c *Config) *int { return &c.ChkIgnore1 }),
	"mute_spectators":                  intBinding(func(c *Config) *int { return &c.ChkIgnore2 }),
	"use_lflist":                       intBinding(func(c *Config) *int { return &c.UseLFList }),
	"default_lflist":                   intBinding(func(c *Config) *int { return &c.DefaultLFList }),
	"default_rule":                     intBinding(func(c *Config) *int { return &c.DefaultRule }),
	"hide_setname":                     intBinding(func(c *Config) *int { return &c.HideSetName }),
	"hide_hint_button":                 intBinding(func(c *Config) *int { return &c.HideHintButton }),
	"control_mode":                     intBinding(func(c *Config) *int { return &c.ControlMode }),
	"draw_field_spell":                 intBinding(func(c *Config) *int { return &c.DrawFieldSpell }),
	"separate_clear_button":            intBinding(func(c *Config) *int { return &c.SeparateClearButton }),
	"auto_search_limit":                intBinding(func(c *Config) *int { return &c.AutoSearchLimit }),
	"search_multiple_keywords":         intBinding(func(c *Config) *int { return &c.SearchMultipleKeywords }),
	"ignore_deck_changes":              intBinding(func(c *Config) *int { return &c.ChkIgnoreDeckChanges }),
	"default_ot":                       intBinding(func(c *Config) *int { return &c.DefaultOT }),
	"enable_bot_mode":                  intBinding(func(c *Config) *int { return &c.EnableBotMode }),
	"quick_animation":                  intBinding(func(c *Config) *int { return &c.QuickAnimation }),
	"auto_save_replay":                 intBinding(func(c *Config) *int { return &c.AutoSaveReplay }),
	"draw_single_chain":                intBinding(func(c *Config) *int { return &c.DrawSingleChain }),
	"hide_player_name":                 intBinding(func(c *Config) *int { return &c.HidePlayerName }),
	"prefer_expansion_script":          intBinding(func(c *Config) *int { return &c.PreferExpansionScript }),
	"swap_yes_no_button":               boolBinding(func(c *Config) *bool { return &c.SwapYesNoButton }),
	"window_maximized":                 boolBinding(func(c *Config) *bool { return &c.WindowMaximized }),
	"window_width":                     intBinding(func(c *Config) *int { return &c.WindowWidth }),
	"window_height":                    intBinding(func(c *Config) *int { return &c.WindowHeight }),
	"resize_select_window":             boolBinding(func(c *Config) *bool { return &c.ResizeSelectWindow }),
	"resize_popup_menu":                intBinding(func(c *Config) *int { return &c.ResizePopupMenu }),
	"enable_sound":                     boolBinding(func(c *Config) *bool { return &c.EnableSound }),
	"sound_volume":                     intBinding(func(c *Config) *int { return &c.SoundVolume }),
	"enable_music":                     boolBinding(func(c *Config) *bool { return &c.EnableMusic }),
	"music_volume":                     intBinding(func(c *Config) *int { return &c.MusicVolume }),
	"music_mode":                       intBinding(func(c *Config) *int { return &c.MusicMode }),
	"textfont":                         strBinding(func(c *Config) *string { return &c.TextFont }),
	"numfont":                          strBinding(func(c *Config) *string { return &c.NumFont }),
	"nickname":                         strBinding(func(c *Config) *string { return &c.Nickname }),
	"gamename":                         strBinding(func(c *Config) *string { return &c.GameName }),
	"roompass":                         strBinding(func(c *Config) *string { return &c.RoomPass }),
	"lastcategory":                     strBinding(func(c *Config) *string { return &c.LastCategory }),
	"lastdeck":                         strBinding(func(c *Config) *string { return &c.LastDeck }),
	"bot_deck_path":                    strBinding(func(c *Config) *string { return &c.BotDeckPath }),
}

// ParseConfig 按 `key = value` 逐行读入。认不出的键跳过 —— 配置文件可能来自更新的版本，
// 遇到不认识的项就整个放弃太脆了。
func ParseConfig(r *bufio.Scanner, cfg *Config) {
	for r.Scan() {
		line := strings.TrimSpace(r.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		// textfont 的值是「路径 字号」两段（如 fonts/simhei.ttf 14），
		// C++ 那边靠 %s 只取第一段、字号另行 sscanf。这里把整行值留给绑定自己处理。
		val = strings.TrimSpace(val)

		b, known := configKeys[key]
		if !known {
			continue
		}
		if key == "textfont" {
			// 字号跟在路径后面，用空格分开
			if path, size, hasSize := strings.Cut(val, " "); hasSize {
				cfg.TextFont = path
				cfg.TextFontSize = uint8(atoiDefault(strings.TrimSpace(size), 14))
				continue
			}
		}
		b.set(cfg, val)
	}
}

// FormatConfig 按 `key = value` 写出，键名排序好让 diff 稳定。
func FormatConfig(cfg *Config) string {
	keys := make([]string, 0, len(configKeys))
	for k := range configKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		v := configKeys[k].get(cfg)
		if k == "textfont" {
			// 与 C++ 一致：路径后面跟字号
			fmt.Fprintf(&sb, "%s = %s %d\n", k, v, cfg.TextFontSize)
			continue
		}
		fmt.Fprintf(&sb, "%s = %s\n", k, v)
	}
	return sb.String()
}

// LoadConfigFile 从 system.conf 读配置。文件不存在就保持默认值。
func LoadConfigFile(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	ParseConfig(bufio.NewScanner(f), cfg)
	return nil
}

// SaveConfigFile 把配置写回 system.conf。
func SaveConfigFile(path string, cfg *Config) error {
	return os.WriteFile(path, []byte(FormatConfig(cfg)), 0644)
}

func atoiDefault(s string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}
