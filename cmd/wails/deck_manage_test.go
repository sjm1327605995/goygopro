package main

import (
	"os"
	"path/filepath"
	"testing"
)

// wDeckManage 绑定（deck_manage.go）：分类新建/重命名/删除 + 卡组
// 重命名/复制/移动，目录语义对照 deck_manager.cpp CreateCategory 等。

func writeTestDeck(t *testing.T, a *App, name string) {
	t.Helper()
	path, err := safeDeckPath(a.deckDir, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.cardDB.SaveDeck(path, DeckData{Name: name, Main: []uint32{89631139}}); err != nil {
		t.Fatal(err)
	}
}

func TestDeckCategoryOps(t *testing.T) {
	a := &App{deckDir: t.TempDir(), cardDB: &CardDBManager{}}

	if err := a.CreateDeckCategory("环境"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "环境")); err != nil {
		t.Fatalf("分类目录未创建: %v", err)
	}
	// 重名拒绝
	if err := a.CreateDeckCategory("环境"); err == nil {
		t.Fatal("重复新建分类应报错")
	}
	// 非法名拒绝
	for _, bad := range []string{"", "..", "a/b", `a\b`, "a:b"} {
		if err := a.CreateDeckCategory(bad); err == nil {
			t.Fatalf("非法分类名 %q 应报错", bad)
		}
	}
	// 重命名（带内容一起搬）
	writeTestDeck(t, a, "环境/青眼")
	if err := a.RenameDeckCategory("环境", "比赛"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "比赛", "青眼.ydk")); err != nil {
		t.Fatalf("重命名后卡组未跟随: %v", err)
	}
	if err := a.CreateDeckCategory("环境"); err != nil {
		t.Fatal(err)
	}
	if err := a.RenameDeckCategory("比赛", "环境"); err == nil {
		t.Fatal("重命名到已存在分类应报错")
	}
	_ = a.DeleteDeckCategory("环境")
	// 删除
	if err := a.DeleteDeckCategory("比赛"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "比赛")); !os.IsNotExist(err) {
		t.Fatal("分类目录应已删除")
	}
	if err := a.DeleteDeckCategory("不存在"); err == nil {
		t.Fatal("删除不存在分类应报错")
	}
}

func TestDeckRenameCopyMove(t *testing.T) {
	a := &App{deckDir: t.TempDir(), cardDB: &CardDBManager{}}
	writeTestDeck(t, a, "环境/青眼")
	writeTestDeck(t, a, "白龙")

	// 分类内改名（新名不带 '/' 时沿用旧分类）
	if err := a.RenameDeck("环境/青眼", "青眼MAX"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "环境", "青眼MAX.ydk")); err != nil {
		t.Fatalf("改名未生效: %v", err)
	}
	// 跨分类改名（新名带 '/'）
	if err := a.RenameDeck("白龙", "环境/白龙"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "环境", "白龙.ydk")); err != nil {
		t.Fatalf("跨分类改名未生效: %v", err)
	}
	// 覆盖拒绝
	if err := a.RenameDeck("环境/白龙", "青眼MAX"); err == nil {
		t.Fatal("重命名覆盖已存在卡组应报错")
	}
	// 复制到其他分类（同名另存，BUTTON_COPY_DECK 语义）
	if err := a.CopyDeck("环境/青眼MAX", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "青眼MAX.ydk")); err != nil {
		t.Fatalf("复制未生效: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "环境", "青眼MAX.ydk")); err != nil {
		t.Fatal("复制后原文件应保留")
	}
	if err := a.CopyDeck("环境/青眼MAX", ""); err == nil {
		t.Fatal("复制到已存在同名卡组应报错")
	}
	// 清理根目录的副本，供后续移动用例
	if err := a.DeleteDeck("青眼MAX"); err != nil {
		t.Fatal(err)
	}
	// 移动到根（未分类）
	if err := a.MoveDeck("环境/青眼MAX", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "青眼MAX.ydk")); err != nil {
		t.Fatalf("移动到根未生效: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "环境", "青眼MAX.ydk")); !os.IsNotExist(err) {
		t.Fatal("移动后原文件应消失")
	}
	// 移动到新分类（目录自动创建）
	if err := a.MoveDeck("青眼MAX", "新分类"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.deckDir, "新分类", "青眼MAX.ydk")); err != nil {
		t.Fatalf("移动到新分类未生效: %v", err)
	}
	// 移动覆盖拒绝
	writeTestDeck(t, a, "新分类/冲突")
	writeTestDeck(t, a, "冲突")
	if err := a.MoveDeck("冲突", "新分类"); err == nil {
		t.Fatal("移动覆盖已存在卡组应报错")
	}
	// 非法分类名拒绝
	if err := a.MoveDeck("冲突", "a/b"); err == nil {
		t.Fatal("非法目标分类应报错")
	}
}
