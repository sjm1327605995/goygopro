package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/core/duel"
)

// TestLFListContent：卡组编辑器的禁限数据接口。写入带 禁/限/准限 三档的
// lflist.conf 后，LFListContent 应按哈希返回 卡码→0/1/2 映射；N/A（哈希 0）
// 与未知哈希返回空表（前端按「无限制 3」处理）。
func TestLFListContent(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "lflist.conf")
	body := "!Test List\n46986414 0\n89631139 1\n38033121 2\n"
	if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	duel.DeckManager.LoadLFListSingle(conf)
	duel.DeckManager.LoadLFList()

	var testList *duel.LFList
	for i := range duel.DeckManager.LFList {
		if duel.DeckManager.LFList[i].ListName == "Test List" {
			testList = &duel.DeckManager.LFList[i]
		}
	}
	if testList == nil {
		t.Fatalf("Test List 未加载")
	}

	a := NewApp()
	content := a.LFListContent(testList.Hash)
	if len(content) != 3 {
		t.Fatalf("内容条目 = %d, want 3: %+v", len(content), content)
	}
	if content[46986414] != 0 || content[89631139] != 1 || content[38033121] != 2 {
		t.Fatalf("禁限档位错: %+v", content)
	}

	// N/A（哈希 0）→ 空表；未知哈希 → 空表
	if got := a.LFListContent(0); len(got) != 0 {
		t.Fatalf("N/A 应返回空表, got %+v", got)
	}
	if got := a.LFListContent(0xdeadbeef); len(got) != 0 {
		t.Fatalf("未知哈希应返回空表, got %+v", got)
	}
}
