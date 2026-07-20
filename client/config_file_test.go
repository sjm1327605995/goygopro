package client

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// system.conf 的格式必须与原版 ygopro 一致（key = value 纯文本）。
// 这里一度写成了 JSON，文件名却没变，于是老用户的配置读不进来（静默用默认值），
// 保存时又把人家的配置覆盖成 JSON。
//
// 这段是照原版 system.conf 抄的片段 —— 测的是「能不能读别人写的文件」，
// 只测自我往返是发现不了格式错的。
const realYgoproConfig = `#config file
#nickname & gamename should be less than 20 characters
use_d3d = 0
antialias = 0
errorlog = 3
nickname = 测试玩家
gamename = 我的房间
lastdeck = 青眼白龙
lastcategory = 我的卡组
textfont = fonts/simhei.ttf 14
numfont = fonts/arial.ttf
serverport = 7911
lasthost = 192.168.1.100
lastport = 8911
automonsterpos = 0
autospellpos = 1
randompos = 0
autochain = 0
waitchain = 0
mute_opponent = 0
mute_spectators = 0
use_lflist = 1
default_lflist = 0
default_rule = 5
hide_setname = 0
hide_hint_button = 0
control_mode = 0
draw_field_spell = 1
separate_clear_button = 1
auto_search_limit = -1
search_multiple_keywords = 1
ignore_deck_changes = 0
default_ot = 1
enable_bot_mode = 0
quick_animation = 0
auto_save_replay = 1
draw_single_chain = 0
hide_player_name = 0
prefer_expansion_script = 1
window_maximized = 0
window_width = 1024
window_height = 640
resize_popup_menu = 0
enable_sound = 1
enable_music = 1
sound_volume = 50
music_volume = 50
music_mode = 1
`

func TestParseRealYgoproConfig(t *testing.T) {
	cfg := Config{}
	ParseConfig(bufio.NewScanner(strings.NewReader(realYgoproConfig)), &cfg)

	for _, tc := range []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"昵称", cfg.Nickname, "测试玩家"},
		{"房间名", cfg.GameName, "我的房间"},
		{"上次卡组", cfg.LastDeck, "青眼白龙"},
		{"上次分类", cfg.LastCategory, "我的卡组"},
		{"服务器端口", int(cfg.ServerPort), 7911},
		{"上次主机", cfg.LastHost, "192.168.1.100"},
		{"上次端口", cfg.LastPort, "8911"},
		{"决斗规则", cfg.DefaultRule, 5},
		{"窗口宽", cfg.WindowWidth, 1024},
		{"窗口高", cfg.WindowHeight, 640},
		{"音量", cfg.SoundVolume, 50},
		{"自动保存录像", cfg.AutoSaveReplay, 1},
		{"errorlog", int(cfg.EnableLog), 3},
		{"魔法陷阱自动位置", cfg.ChkSTAutoPos, 1},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}

	// 布尔项：C++ 用 0/1
	if !cfg.EnableSound || !cfg.EnableMusic {
		t.Error("enable_sound / enable_music = 1 应当解析为 true")
	}
	if cfg.UseD3D {
		t.Error("use_d3d = 0 应当解析为 false")
	}

	// textfont 的值是「路径 字号」两段
	if cfg.TextFont != "fonts/simhei.ttf" {
		t.Errorf("字体路径 = %q, want fonts/simhei.ttf", cfg.TextFont)
	}
	if cfg.TextFontSize != 14 {
		t.Errorf("字号 = %d, want 14", cfg.TextFontSize)
	}
}

// 注释与空行要跳过，认不出的键也要跳过而不是整个放弃 ——
// 配置文件可能来自更新的版本，多出几个键不该让整份配置失效。
func TestParseConfigSkipsCommentsAndUnknownKeys(t *testing.T) {
	src := `# 这是注释

nickname = 阿福
some_future_option = 42
serverport = 1234
`
	cfg := Config{}
	ParseConfig(bufio.NewScanner(strings.NewReader(src)), &cfg)

	if cfg.Nickname != "阿福" {
		t.Errorf("昵称 = %q, want 阿福", cfg.Nickname)
	}
	if cfg.ServerPort != 1234 {
		t.Errorf("端口 = %d —— 不认识的键让后面的项也丢了", cfg.ServerPort)
	}
}

// 写出去的必须还能读回来，而且格式是 key = value（不是 JSON）。
func TestConfigRoundTripAndFormat(t *testing.T) {
	orig := Config{
		Nickname: "决斗者", GameName: "房间A", LastHost: "10.0.0.1", LastPort: "7911",
		ServerPort: 7911, DefaultRule: 5, WindowWidth: 1280, WindowHeight: 720,
		SoundVolume: 80, EnableSound: true, EnableMusic: false,
		TextFont: "fonts/simhei.ttf", TextFontSize: 16,
	}

	text := FormatConfig(&orig)
	if strings.HasPrefix(strings.TrimSpace(text), "{") {
		t.Fatal("写出来是 JSON —— 原版 ygopro 读不了")
	}
	if !strings.Contains(text, "nickname = 决斗者") {
		t.Errorf("没有按 key = value 写昵称:\n%s", text)
	}

	var back Config
	ParseConfig(bufio.NewScanner(strings.NewReader(text)), &back)

	if back.Nickname != orig.Nickname || back.ServerPort != orig.ServerPort ||
		back.WindowWidth != orig.WindowWidth || back.SoundVolume != orig.SoundVolume ||
		back.TextFont != orig.TextFont || back.TextFontSize != orig.TextFontSize {
		t.Errorf("往返后不一致:\n原值 %+v\n读回 %+v", orig, back)
	}
	if back.EnableSound != orig.EnableSound || back.EnableMusic != orig.EnableMusic {
		t.Error("布尔项往返不一致")
	}
}

func TestLoadConfigFileMissingIsNotFatal(t *testing.T) {
	cfg := Config{ServerPort: 7911}
	err := LoadConfigFile(filepath.Join(t.TempDir(), "nope.conf"), &cfg)
	if err == nil {
		t.Error("文件不存在应当返回错误")
	}
	if cfg.ServerPort != 7911 {
		t.Error("读取失败不该改动已有配置")
	}
}

func TestSaveConfigFileWritesReadableText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "system.conf")
	cfg := Config{Nickname: "小明", ServerPort: 7911}

	if err := SaveConfigFile(path, &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "nickname = 小明") {
		t.Errorf("文件内容不是预期格式:\n%s", data)
	}
}
