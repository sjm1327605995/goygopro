package client

import (
	"os"
	"path/filepath"
	"testing"
)

// 卡组枚举此前完全没有：客户端只认死路径 deck/default.ydk，玩家没法用自己的牌联机，
// 而「准备」发给服务器的正是当前卡组。
//
// 这些测试造真实的目录和文件来验证 —— 路径拼接和后缀匹配靠脑补最容易错。

// withDeckDir 在临时目录里搭一套卡组目录，并把工作目录切过去。
func withDeckDir(t *testing.T, layout map[string][]string) {
	t.Helper()
	root := t.TempDir()
	for cat, names := range layout {
		dir := filepath.Join(root, deckRoot, cat)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, n := range names {
			if err := os.WriteFile(filepath.Join(dir, n), []byte("#created\n#main\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestDeckNamesListsYdkFiles(t *testing.T) {
	withDeckDir(t, map[string][]string{
		"": {"青眼白龙.ydk", "黑魔导.ydk", "readme.txt", "笔记.md"},
	})
	dm := &DeckManager{}

	got := dm.DeckNames("")

	if len(got) != 2 {
		t.Fatalf("枚举到 %v, 期望只有两个 .ydk", got)
	}
	// 排序后是「黑魔导」在前还是「青眼白龙」在前取决于字节序，这里只看集合
	found := map[string]bool{}
	for _, n := range got {
		found[n] = true
	}
	if !found["青眼白龙"] || !found["黑魔导"] {
		t.Errorf("枚举结果 = %v, 应当含青眼白龙与黑魔导（且不带 .ydk 后缀）", got)
	}
	for _, n := range got {
		if filepath.Ext(n) != "" {
			t.Errorf("%q 还带着后缀 —— 卡组名不该包含 .ydk", n)
		}
	}
}

// Windows 上后缀可能是大写，不能漏掉。
func TestDeckNamesIsCaseInsensitiveOnExtension(t *testing.T) {
	withDeckDir(t, map[string][]string{
		"": {"a.ydk", "b.YDK", "c.Ydk"},
	})
	dm := &DeckManager{}

	if got := dm.DeckNames(""); len(got) != 3 {
		t.Errorf("枚举到 %v, 大小写不同的 .ydk 都应当算卡组", got)
	}
}

// 分类就是 ./deck 下的子目录，默认分类（空名）指 ./deck 本身。
func TestDeckCategoriesAndPaths(t *testing.T) {
	withDeckDir(t, map[string][]string{
		"":   {"默认卡组.ydk"},
		"比赛": {"决赛用.ydk"},
		"娱乐": {"欢乐场.ydk"},
	})
	dm := &DeckManager{}

	cats := dm.DeckCategories()
	if len(cats) != 3 || cats[0] != "" {
		t.Fatalf("分类 = %v, 首项应为默认分类（空名），其后是子目录", cats)
	}

	if got := CategoryPath(""); got != deckRoot {
		t.Errorf("默认分类路径 = %q, want %q", got, deckRoot)
	}
	if got, want := CategoryPath("比赛"), filepath.Join(deckRoot, "比赛"); got != want {
		t.Errorf("分类路径 = %q, want %q", got, want)
	}
	if got, want := DeckPath("比赛", "决赛用"), filepath.Join(deckRoot, "比赛", "决赛用.ydk"); got != want {
		t.Errorf("卡组路径 = %q, want %q", got, want)
	}

	// 子分类里的卡组要能单独枚举出来
	if got := dm.DeckNames("比赛"); len(got) != 1 || got[0] != "决赛用" {
		t.Errorf("比赛分类下 = %v, want [决赛用]", got)
	}
}

// 载入卡组要把选择记进配置，下次启动才能直接用（对应 C++ 的 lastcategory/lastdeck）。
func TestLoadCurrentDeckRemembersChoice(t *testing.T) {
	withDeckDir(t, map[string][]string{
		"比赛": {"决赛用.ydk"},
	})
	dm := &DeckManager{}
	MainGame.Config.LastCategory, MainGame.Config.LastDeck = "", ""

	if err := dm.LoadCurrentDeck("比赛", "决赛用"); err != nil {
		t.Fatalf("载入失败: %v", err)
	}
	if MainGame.Config.LastCategory != "比赛" || MainGame.Config.LastDeck != "决赛用" {
		t.Errorf("配置里记的是 %q/%q, want 比赛/决赛用",
			MainGame.Config.LastCategory, MainGame.Config.LastDeck)
	}
}

// 目录不存在时返回空而不是崩掉 —— 全新安装就没有 deck 目录。
func TestDeckNamesMissingDirIsEmpty(t *testing.T) {
	withDeckDir(t, map[string][]string{"": {}})
	dm := &DeckManager{}

	if got := dm.DeckNames("根本不存在的分类"); got != nil {
		t.Errorf("不存在的分类应当返回空, 得到 %v", got)
	}
	if cats := dm.DeckCategories(); len(cats) == 0 || cats[0] != "" {
		t.Error("即使没有子目录，也应当有默认分类")
	}
}
