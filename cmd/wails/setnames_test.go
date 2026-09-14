package main

import (
	"os"
	"path/filepath"
	"testing"
)

func loadTestSetNames(t *testing.T) *SetNameTable {
	t.Helper()
	content := "!setname 0x1 正义盟军\tA・O・J\n" +
		"!setname 0x1002 真次世代\tレアル・ジェネクス\n" +
		"!setname 0x18 魔术师\n" +
		"!system 1403 房间人数已满\n" + // 非 setname 行必须被忽略
		"!setname 0xabc 无效行留空\n" + // 名称非空，有效
		"!setname garbage\n" // 解析失败的行必须被跳过
	path := filepath.Join(t.TempDir(), "strings.conf")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	table := NewSetNameTable()
	if err := table.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return table
}

// TestSetNameLoad locks down the !setname line format: hex key, value ends at
// the tab (tab = comment separator, data_manager.cpp:203-211), non-setname
// lines and malformed lines are ignored.
func TestSetNameLoad(t *testing.T) {
	table := loadTestSetNames(t)
	if got := table.Get(0x1); got != "正义盟军" {
		t.Fatalf("Get(0x1) = %q, want 正义盟军", got)
	}
	if got := table.Get(0x18); got != "魔术师" {
		t.Fatalf("Get(0x18) = %q, want 魔术师", got)
	}
	if got := table.Get(0x1403); got != "" {
		t.Fatalf("!system line leaked into setname table: %q", got)
	}
	if got := table.Get(0xabc); got != "无效行留空" {
		t.Fatalf("Get(0xabc) = %q", got)
	}
}

// TestSetNameCodesByName mirrors GetSetCodes: <2 chars = exact token match,
// otherwise substring (data_manager.cpp:289-322).
func TestSetNameCodesByName(t *testing.T) {
	table := loadTestSetNames(t)
	if codes := table.CodesByName("正义盟军"); len(codes) != 1 || codes[0] != 0x1 {
		t.Fatalf("CodesByName(正义盟军) = %v", codes)
	}
	// 长关键字子串：真次世代 contains 次世代
	if codes := table.CodesByName("次世代"); len(codes) != 1 || codes[0] != 0x1002 {
		t.Fatalf("CodesByName(次世代) = %v", codes)
	}
	// 单字符必须精确，不能命中"正义盟军"
	if codes := table.CodesByName("正"); len(codes) != 0 {
		t.Fatalf("short keyword must match exactly, got %v", codes)
	}
}

// TestFormatSetNames unpacks the 4×16-bit packed setcode and joins the
// known series names (unknown sub-codes are silently skipped).
func TestFormatSetNames(t *testing.T) {
	table := loadTestSetNames(t)
	setcode := uint64(0x1002)<<16 | 0x1
	got := table.FormatSetNames(setcode)
	if len(got) != 2 || got[0] != "正义盟军" || got[1] != "真次世代" {
		t.Fatalf("FormatSetNames = %v", got)
	}
	if got := table.FormatSetNames(0xffff); len(got) != 0 {
		t.Fatalf("unknown-only setcode should yield no names, got %v", got)
	}
}

// TestSetNameLoadMissingFile: missing strings.conf is not an error.
func TestSetNameLoadMissingFile(t *testing.T) {
	table := NewSetNameTable()
	if err := table.Load(filepath.Join(t.TempDir(), "nope.conf")); err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
}

// findRepoRoot walks up from the working directory looking for cards.cdb
// （Windows 下 Go 测试进程对 `../cards.cdb` 这类相对路径的 stat 不可靠，
// 直接向上搜绝对路径）。
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Skip("no working dir")
	}
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "cards.cdb")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("vendored cards.cdb not present")
	return ""
}

// TestSearchBySetnameIntegration runs against the vendored cards.cdb and
// strings.conf: @青眼 must surface Blue-Eyes White Dragon (89631139).
func TestSearchBySetnameIntegration(t *testing.T) {
	root := findRepoRoot(t)
	m := NewCardDBManager()
	if err := m.OpenDB(filepath.Join(root, "cards.cdb")); err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	if err := m.LoadSetNames(filepath.Join(root, "strings.conf")); err != nil {
		t.Fatalf("LoadSetNames: %v", err)
	}
	// 青眼系列卡很多，前 20 条按 id 序全是变体；放宽到 100 才能包含原版白龙。
	res := m.SearchCards(CardFilter{Keyword: "@青眼", Limit: 100})
	found := false
	for _, c := range res {
		if c.Code == 89631139 {
			found = true
			if len(c.SetNames) == 0 {
				t.Fatalf("Blue-Eyes returned without series names")
			}
		}
	}
	if !found {
		t.Fatalf("@青眼 search did not return 89631139 (got %d results)", len(res))
	}
}
