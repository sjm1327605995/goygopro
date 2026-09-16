package main

import (
	"os"
	"strings"
	"testing"
)

// 配置测试必须真的驱动 system.conf 的读写变化：先写一份原版风格的
// system.conf，再验证 LoadConfig 解析、SaveConfig 补丁合并、落盘回读。
func withConfigDir(t *testing.T, confContent string) {
	t.Helper()
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if confContent != "" {
		if err := os.WriteFile("system.conf", []byte(confContent), 0644); err != nil {
			t.Fatalf("write system.conf: %v", err)
		}
	}
}

// 默认值与原版 game.h:47-104 字段初始化一致（抽样核对原版语义关键的键）。
func TestLoadConfigDefaults(t *testing.T) {
	withConfigDir(t, "")
	app := NewApp()
	cfg := app.loadConfig()
	if cfg.AutoSpellPos != 1 || cfg.DrawFieldSpell != 1 || cfg.SeparateClearButton != 1 {
		t.Fatalf("原版默认开启项错误: %+v", cfg)
	}
	if cfg.AutoSearchLimit != -1 {
		t.Fatalf("auto_search_limit 默认应为 -1（关闭），got %d", cfg.AutoSearchLimit)
	}
	if !cfg.EnableSound || !cfg.EnableMusic {
		t.Fatalf("音频默认应开启")
	}
	if cfg.SoundVolume != 50 || cfg.MusicVolume != 50 {
		t.Fatalf("音量默认 50, got %d/%d", cfg.SoundVolume, cfg.MusicVolume)
	}
	if cfg.ServerPort != 7911 || cfg.DefaultOT != 1 || cfg.UseLFList != 1 {
		t.Fatalf("大厅/禁限默认值错误: %+v", cfg)
	}
}

// 用原版 system.conf 的真实键名（game.cpp:1395-1535 的读表）解析。
func TestLoadConfigParsesOriginalKeys(t *testing.T) {
	conf := strings.Join([]string{
		"use_d3d = 0",
		"antialias = 0",
		"errorlog = 3",
		"serverport = 7911",
		"lasthost = 192.168.1.7",
		"lastport = 7911",
		"nickname = 海马瀬人",
		"gamename = 青眼房间",
		"lastcategory = 环境",
		"lastdeck = 青眼卡组",
		"automonsterpos = 1",
		"autospellpos = 1",
		"randompos = 0",
		"autochain = 1",
		"waitchain = 0",
		"mute_opponent = 1",
		"mute_spectators = 0",
		"hide_player_name = 1",
		"draw_field_spell = 0",
		"quick_animation = 1",
		"auto_save_replay = 1",
		"enable_sound = false",
		"sound_volume = 80",
		"music_volume = 120", // 越界应被夹到 100
		"default_ot = 2",     // 原版键名；defaultOT 是本项旧拼写，也应可读
		"default_lflist = 3",
	}, "\n")
	withConfigDir(t, conf)
	app := NewApp()
	cfg := app.loadConfig()

	if cfg.Nickname != "海马瀬人" || cfg.LastDeck != "青眼卡组" || cfg.LastCategory != "环境" {
		t.Fatalf("字符串键解析错误: %+v", cfg)
	}
	if cfg.LastHost != "192.168.1.7" || cfg.LastPort != "7911" {
		t.Fatalf("上次联机地址解析错误: %+v", cfg)
	}
	if cfg.AutoChain != 1 || cfg.MuteOpponent != 1 || cfg.HidePlayerName != 1 {
		t.Fatalf("开关键解析错误: %+v", cfg)
	}
	if cfg.DrawFieldSpell != 0 || cfg.QuickAnimation != 1 || cfg.AutoSaveReplay != 1 {
		t.Fatalf("功能开关解析错误: %+v", cfg)
	}
	if cfg.EnableSound {
		t.Fatalf("enable_sound=false 应解析为关")
	}
	if cfg.SoundVolume != 80 || cfg.MusicVolume != 100 {
		t.Fatalf("音量解析/夹取错误: %d/%d", cfg.SoundVolume, cfg.MusicVolume)
	}
	if cfg.DefaultOT != 2 || cfg.DefaultLFList != 3 {
		t.Fatalf("规则键解析错误: %+v", cfg)
	}
}

// 旧拼写的 defaultOT 键（本项目早期误写）仍可读，不破坏既有 conf 文件。
func TestLoadConfigLegacyDefaultOTKey(t *testing.T) {
	withConfigDir(t, "defaultOT = 4\n")
	app := NewApp()
	if cfg := app.loadConfig(); cfg.DefaultOT != 4 {
		t.Fatalf("defaultOT 兼容键应可读，DefaultOT=%d", cfg.DefaultOT)
	}
}

// SaveConfig：补丁合并（未提及键保持不变）+ 落盘 + 回读。
func TestSaveConfigMergesAndPersists(t *testing.T) {
	withConfigDir(t, "nickname = 旧昵称\nsound_volume = 30\n")
	app := NewApp()
	app.SaveConfig(map[string]interface{}{
		"nickname":     "新昵称",
		"sound_volume": 75,
		"hide_setname": 1,
	})

	// 回读：未提及的键（volume=30 之外的其它默认值）必须保留
	cfg := app.loadConfig()
	if cfg.Nickname != "新昵称" || cfg.SoundVolume != 75 || cfg.HideSetName != 1 {
		t.Fatalf("落盘回读不一致: %+v", cfg)
	}
	if cfg.MusicVolume != 50 {
		t.Fatalf("未修改键不应变化: music_volume=%d", cfg.MusicVolume)
	}

	// 文件格式与原版一致（key = value 行）
	data, err := os.ReadFile("system.conf")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "nickname = 新昵称\n") {
		t.Fatalf("落盘缺少 nickname 行:\n%s", text)
	}
	// 键必须按字典序稳定落盘
	lines := strings.Split(strings.TrimSpace(text), "\n")
	sorted := append([]string(nil), lines...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if strings.Join(lines, "\n") != strings.Join(sorted, "\n") {
		t.Fatalf("落盘顺序不稳定")
	}
	if strings.Contains(text, "roompass") {
		t.Fatalf("roompass 只读不写（原版语义），不应出现在落盘文件")
	}
	// 落盘键名与原版一致：default_ot（不是旧拼写的 defaultOT）
	if !strings.Contains(text, "default_ot = ") || strings.Contains(text, "defaultOT") {
		t.Fatalf("落盘应使用原版键名 default_ot:\n%s", text)
	}
}
