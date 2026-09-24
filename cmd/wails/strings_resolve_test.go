package main

import (
	"testing"
)

// openTestCardDB opens the repo-root cards.cdb (path relative to the package
// dir cmd/wails). Skips the caller's test when the fixture is unavailable.
func openTestCardDB(t *testing.T) *CardDBManager {
	t.Helper()
	m := NewCardDBManager()
	if err := m.OpenDB("../../cards.cdb"); err != nil {
		t.Skipf("cards.cdb unavailable: %v", err)
	}
	return m
}

// TestResolveDescCardStrings locks down the GetDesc decoding from
// data_manager.cpp:262: code=(strCode>>4)&0x0fffffff, offset=strCode&0xf,
// offset 0 → card desc text, 1-15 → texts.str{offset}.
func TestResolveDescCardStrings(t *testing.T) {
	a := &App{cardDB: openTestCardDB(t)}

	// 青眼白龙(89631139)：desc=0x896311390 → 卡牌描述。
	const dragon = uint32(89631139)

	res := a.ResolveDesc(dragon << 4)
	if !res["success"].(bool) {
		t.Fatalf("success=false for card desc: %+v", res)
	}
	if res["code"].(uint32) != dragon {
		t.Fatalf("code = %v, want %d", res["code"], dragon)
	}
	if res["text"].(string) == "" {
		t.Fatalf("card desc resolved to empty text")
	}

	// 39015(任意 code≥4000 且带 str1 的卡)：desc id = (39015<<4)|1 → texts.str1。
	// 注意 code<4000 的卡 ((code<<4)|1) < 64000 会落进系统字符串分支（与原版
	// GetDesc 一致的已知歧义），所以测试必须用大卡号。
	const withStr = uint32(39015)
	res = a.ResolveDesc((withStr << 4) | 1)
	if !res["success"].(bool) || res["text"].(string) == "" {
		t.Fatalf("str1 failed to resolve: %+v", res)
	}
	if res["code"].(uint32) != withStr {
		t.Fatalf("code = %v, want %d", res["code"], withStr)
	}
}

// TestResolveDescSystemStringAndMisses covers the two non-card branches:
// system strings (< 64000) resolve against the embedded common subset
// (strings.conf excerpt shared with frontend domain/sys_strings.ts); ids
// outside the subset still come back empty so the frontend can fall back;
// plus unknown codes / bad offsets.
func TestResolveDescSystemStringAndMisses(t *testing.T) {
	a := &App{cardDB: openTestCardDB(t)}

	// 收录的 id → 子集文案
	res := a.ResolveDesc(1000)
	if !res["success"].(bool) || res["text"].(string) != "卡组" {
		t.Fatalf("system string 1000 must resolve to 卡组: %+v", res)
	}
	res = a.ResolveDesc(1409)
	if !res["success"].(bool) || res["text"].(string) != "等待更换副卡组中..." {
		t.Fatalf("system string 1409 must resolve: %+v", res)
	}

	// 未收录的 id → 空文本（前端兜底）
	res = a.ResolveDesc(999)
	if !res["success"].(bool) || res["text"].(string) != "" {
		t.Fatalf("unlisted system string must be success with empty text: %+v", res)
	}

	res = a.ResolveDesc((99999999 << 4) | 1) // 不存在的卡号
	if res["success"].(bool) && res["text"].(string) != "" {
		t.Fatalf("unknown card must not return text: %+v", res)
	}

	res = a.ResolveDesc(0) // 边界：恰好等于 64000 之下
	if !res["success"].(bool) || res["text"].(string) != "" {
		t.Fatalf("desc 0 must be treated as system string: %+v", res)
	}
}

// TestResolveDescNilDB guards the no-database path (db failed to open in dev).
func TestResolveDescNilDB(t *testing.T) {
	a := &App{}
	res := a.ResolveDesc((89631139 << 4) | 1)
	if res["success"].(bool) || res["text"].(string) != "" {
		t.Fatalf("nil cardDB must fail cleanly: %+v", res)
	}
}
