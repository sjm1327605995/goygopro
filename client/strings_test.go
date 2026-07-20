package client

import (
	"os"
	"path/filepath"
	"testing"
)

// 界面上给人看的文字全来自 strings.conf。此前只能靠猜，而猜出来的是错的：
// victoryReason 里 0x0 被写成「LP 归零」，实际是「投降」—— 26 条胜负原因整体错位一格，
// 玩家看到的败因根本对不上。
//
// 这些测试读**仓库里那份真的 strings.conf**（来自原版 ygopro），
// 而不是自己造一段来自测自答。

func loadRealStrings(t *testing.T) *StringTable {
	t.Helper()
	path := filepath.Join("..", "strings.conf")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("没有 strings.conf: %v", err)
	}
	tb := &StringTable{
		system:  map[int]string{},
		victory: map[int]string{},
		counter: map[int]string{},
		setName: map[int]string{},
	}
	if err := tb.LoadStrings(path); err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	return tb
}

// 胜负原因是十六进制编号，而且我猜错过 —— 这里逐条对着原版文件核。
func TestVictoryStringsMatchUpstream(t *testing.T) {
	tb := loadRealStrings(t)

	for _, tc := range []struct {
		code int
		want string
	}{
		{0x0, "投降"},
		{0x1, "基本分变成0"},
		{0x2, "没有卡可抽"},
		{0x3, "超时"},
		{0x4, "失去连接"},
	} {
		if got := tb.Victory(tc.code); got != tc.want {
			t.Errorf("Victory(%#x) = %q, want %q", tc.code, got, tc.want)
		}
	}

	// 0x10 起是各种卡的特殊胜利，条数远超我原先内置的六条
	if got := tb.Victory(0x10); got == "" || got == "胜利条件 16" {
		t.Errorf("Victory(0x10) = %q, 应当是特殊胜利的说明", got)
	}
}

// 系统提示是十进制编号，与 victory 的进制不同 —— 解析时搞混就会全表错乱。
func TestSystemStringsUseDecimal(t *testing.T) {
	tb := loadRealStrings(t)

	if got := tb.System(1390); got != "等待行动中..." {
		t.Errorf("System(1390) = %q, want 等待行动中...", got)
	}
	// 属性与种族按位排开，界面直接按 base+n 取
	if got := tb.System(1010); got != "地" {
		t.Errorf("System(1010) = %q, want 地（属性起始）", got)
	}
	if got := tb.System(1020); got != "战士" {
		t.Errorf("System(1020) = %q, want 战士（种族起始）", got)
	}
	// 若把 1390 当成十六进制读，就会落到 0x1390=5008 上，这里正好能抓住
	if tb.System(5008) != "系统提示 5008" {
		t.Error("5008 不该有内容 —— system 段被当成十六进制解析了")
	}
}

func TestCounterAndSetNameStrings(t *testing.T) {
	tb := loadRealStrings(t)

	if got := tb.Counter(0x1); got != "魔力指示物" {
		t.Errorf("Counter(0x1) = %q, want 魔力指示物", got)
	}
	// 系列名后面用制表符跟注释，必须截掉
	name := tb.SetName(0x1)
	if name == "" {
		t.Skip("这份 strings.conf 里没有 0x1 系列")
	}
	for _, r := range name {
		if r == '\t' {
			t.Errorf("系列名 %q 里混进了制表符注释", name)
		}
	}
}

// 查不到时回显编号，而不是空字符串 —— 空串会让界面莫名其妙少一块。
func TestMissingStringFallsBackToCode(t *testing.T) {
	tb := &StringTable{
		system:  map[int]string{},
		victory: map[int]string{},
		counter: map[int]string{},
		setName: map[int]string{},
	}
	if got := tb.System(9999); got != "系统提示 9999" {
		t.Errorf("缺失时 = %q, 应当回显编号", got)
	}
}

// 注释行、空行、格式不对的行都要跳过，不能让整份文件失效。
func TestLoadStringsSkipsJunk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "strings.conf")
	content := "#这是注释\n\n!system 100 好的\n!system 没有值\n乱七八糟\n!victory 0x5 反则负\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tb := &StringTable{
		system:  map[int]string{},
		victory: map[int]string{},
		counter: map[int]string{},
		setName: map[int]string{},
	}
	if err := tb.LoadStrings(path); err != nil {
		t.Fatal(err)
	}

	if got := tb.System(100); got != "好的" {
		t.Errorf("System(100) = %q, want 好的", got)
	}
	if got := tb.Victory(0x5); got != "反则负" {
		t.Errorf("Victory(0x5) = %q —— 坏行让后面的项也丢了", got)
	}
}
