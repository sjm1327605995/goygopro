package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/core/duel"
)

// 波 G-3：禁限卡表下拉（gframe 建房窗 cbLFlist）。HostInfo.LFList 存哈希，
// LoadLFListSingle 按 deck_manager.cpp 的异或折叠逐条累积。
func TestListLFLists(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "lflist.conf")
	// 一条注释 + 一个空内容卡表（哈希 = 种子 0x7dfcee6a）
	if err := os.WriteFile(conf, []byte("# comment\n!Test List\n"), 0o644); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	duel.DeckManger.LoadLFListSingle(conf)
	// 显式补上「N/A」（LoadLFList 尾部追加，哈希 0）
	duel.DeckManger.LoadLFList()

	a := NewApp()
	entries := a.ListLFLists()

	var testList, nolimit *duel.LFList
	for i := range duel.DeckManger.LFList {
		l := &duel.DeckManger.LFList[i]
		switch l.ListName {
		case "Test List":
			testList = l
		case "N/A":
			nolimit = l
		}
	}
	if testList == nil {
		t.Fatalf("Test List 未加载，现有 %d 个卡表", len(entries))
	}
	if testList.Hash != 0x7dfcee6a {
		t.Errorf("空卡表哈希应为种子 0x7dfcee6a，实际 %#x", testList.Hash)
	}
	if nolimit == nil || nolimit.Hash != 0 {
		t.Errorf("N/A 卡表（哈希 0）应被追加")
	}

	found := false
	for _, e := range entries {
		if e.Name == "Test List" && e.Hash == testList.Hash {
			found = true
		}
	}
	if !found {
		t.Fatalf("ListLFLists 应包含 Test List，实际 %+v", entries)
	}
}
