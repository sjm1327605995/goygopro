package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListDecksRecursiveWithCategories(t *testing.T) {
	// gframe 分类语义：./deck/<分类>/<名>.ydk；ListDecks 应递归返回
	// 相对路径名（'/' 分隔），根目录的即「未分类」。
	dir := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#created by test\n#main\n\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("BlueEyes.ydk")
	write("Meta/sub/deckA.ydk")
	write("Meta/deep/deckB.ydk")
	// 非 .ydk 文件与空目录不应出现
	write("ignore.txt")
	if err := os.MkdirAll(filepath.Join(dir, "EmptyCat"), 0755); err != nil {
		t.Fatal(err)
	}

	m := &CardDBManager{}
	got := m.ListDecks(dir)
	want := map[string]bool{"BlueEyes": true, "Meta/sub/deckA": true, "Meta/deep/deckB": true}
	if len(got) != len(want) {
		t.Fatalf("ListDecks = %v, want exactly %v", got, want)
	}
	for _, n := range got {
		if !want[n] {
			t.Fatalf("ListDecks 意外条目 %q（全量 %v）", n, got)
		}
	}
}

func TestListDecksCreatesMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deck")
	m := &CardDBManager{}
	if got := m.ListDecks(dir); got != nil {
		t.Fatalf("空目录应返回 nil，got %v", got)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("缺失的 deck 目录应被创建，err=%v", err)
	}
}

func TestSafeDeckPath(t *testing.T) {
	deckDir := filepath.Join("work", "deck") // 相对基准即可，测形状不测存在性
	ok := []struct{ name, wantSuffix string }{
		{"BlueEyes", filepath.Join("BlueEyes") + ".ydk"},
		{"Meta/sub/deckA", filepath.Join("Meta", "sub", "deckA") + ".ydk"},
		{"  spaced  ", filepath.Join("spaced") + ".ydk"},          // 去空白
		{"cate/name.ydk", filepath.Join("cate", "name") + ".ydk"}, // .ydk 后缀剥掉
	}
	for _, c := range ok {
		got, err := safeDeckPath(deckDir, c.name)
		if err != nil {
			t.Fatalf("safeDeckPath(%q) 意外报错 %v", c.name, err)
		}
		if want := filepath.Join(deckDir, c.wantSuffix); got != want {
			t.Fatalf("safeDeckPath(%q) = %q, want %q", c.name, got, want)
		}
	}
	bad := []string{
		"",             // 空名
		"   ",          // 全空白
		".ydk",         // 剥后缀后为空
		"../evil",      // 目录穿越
		"a/../../evil", // 中段穿越
		"/abs/path",    // 绝对路径
		"C:evil",       // Windows 盘符
		"a//b",         // 空段
		"a/./b",        // 点段
		`a\..\evil`,    // 反斜杠穿越
	}
	for _, name := range bad {
		if got, err := safeDeckPath(deckDir, name); err == nil {
			t.Fatalf("safeDeckPath(%q) 应被拒绝，却返回 %q", name, got)
		}
	}
}

func TestLoadDeckSaveDeckWithCategory(t *testing.T) {
	dir := t.TempDir()
	m := &CardDBManager{}
	deck := DeckData{Name: "Meta/deckA", Main: []uint32{89631139}, Extra: []uint32{}, Side: []uint32{}}
	p, err := safeDeckPath(dir, deck.Name)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SaveDeck(p, deck); err != nil {
		t.Fatal(err)
	}
	// 落在子目录里
	if _, err := os.Stat(filepath.Join(dir, "Meta", "deckA.ydk")); err != nil {
		t.Fatalf("分类卡组应落盘到子目录：%v", err)
	}
	// 递归列表能看到它
	if got := m.ListDecks(dir); len(got) != 1 || got[0] != "Meta/deckA" {
		t.Fatalf("ListDecks = %v, want [Meta/deckA]", got)
	}
	// LoadDeck 读回卡号
	loaded, err := m.LoadDeck(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Main) != 1 || loaded.Main[0] != 89631139 {
		t.Fatalf("LoadDeck 读回错误：%+v", loaded)
	}
}
