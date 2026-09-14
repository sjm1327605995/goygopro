package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sjm1327605995/goygopro/core/duel"
)

// buildReplayPayload 按服务器实际发送的字节布局构造 STOC_REPLAY 包体：
// ReplayHeader(32B) [+ ExtendedReplayHeader 扩展段] + compData。
func buildReplayPayload(id uint32, flag uint32, seed uint32, startTime uint32, comp []byte) []byte {
	buf := make([]byte, 32, 32+len(comp))
	binary.LittleEndian.PutUint32(buf[0:4], id)     // ID
	binary.LittleEndian.PutUint32(buf[4:8], 0x1353) // Version
	binary.LittleEndian.PutUint32(buf[8:12], flag)  // Flag
	binary.LittleEndian.PutUint32(buf[12:16], seed) // Seed
	binary.LittleEndian.PutUint32(buf[16:20], 0)    // DataSize
	binary.LittleEndian.PutUint32(buf[20:24], startTime)
	copy(buf[32:], comp)
	return buf
}

// TestSTOCReplayCachedAndNamed：收到 STOC_REPLAY 后缓存包体、按
// duelclient.cpp:739-744 的语义推导建议文件名（UNIFORM→StartTime，否则
// Seed 当 time_t）并 emit stoc:replay。
func TestSTOCReplayCachedAndNamed(t *testing.T) {
	var emitted map[string]interface{}
	c := NewWailsDuelClient(func(eventName string, optionalData ...interface{}) {
		if eventName == "stoc:replay" {
			emitted = optionalData[0].(map[string]interface{})
		}
	})

	// 2020-01-02 03:04:05 UTC（文件名按本地时区格式化，与 gframe 一致）
	ts := uint32(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC).Unix())
	wantName := time.Unix(int64(ts), 0).Format("2006-01-02 15-04-05")
	payload := buildReplayPayload(duel.REPLAY_ID_YRP2, duel.REPLAY_UNIFORM, 42, ts, []byte{0xde, 0xad})

	c.handleSTOCPacket(byte(0x17) /*STOC_REPLAY*/, payload)

	if emitted == nil {
		t.Fatal("stoc:replay not emitted")
	}
	if emitted["name"] != wantName {
		t.Errorf("suggested name = %v, want %s", emitted["name"], wantName)
	}
	if emitted["size"] != len(payload) {
		t.Errorf("size = %v, want %d", emitted["size"], len(payload))
	}

	// 非 UNIFORM 的旧录像：Seed 即开始时间
	emitted = nil
	payload2 := buildReplayPayload(duel.REPLAY_ID_YRP1, 0, ts, 0, []byte{0x01})
	c.handleSTOCPacket(0x17, payload2)
	if emitted == nil || emitted["name"] != wantName {
		t.Errorf("non-uniform replay should derive name from seed, got %v", emitted)
	}

	// 包体完整缓存（最后一个 STOC_REPLAY 胜出）
	c.mu.Lock()
	cached := c.lastReplay
	c.mu.Unlock()
	if string(cached) != string(payload2) {
		t.Fatalf("lastReplay mismatch: got %d bytes, want %d", len(cached), len(payload2))
	}
}

// TestSaveReplayWritesFile：SaveReplay 把缓存包体原样落盘；空名兜底
// _LastReplay；路径分隔符被清洗。
func TestSaveReplayWritesFile(t *testing.T) {
	c := NewWailsDuelClient(func(string, ...interface{}) {})
	dir := t.TempDir()
	c.replayDir = dir

	if _, err := c.SaveReplay("whatever"); err == nil {
		t.Fatal("saving without a received replay must fail")
	}

	payload := buildReplayPayload(duel.REPLAY_ID_YRP2, duel.REPLAY_UNIFORM, 1, 2, []byte{0xaa, 0xbb, 0xcc})
	c.handleSTOCPacket(0x17, payload)

	saved, err := c.SaveReplay("my duel")
	if err != nil {
		t.Fatalf("SaveReplay: %v", err)
	}
	if saved != "my duel" {
		t.Errorf("saved = %q, want my duel", saved)
	}
	got, err := os.ReadFile(filepath.Join(dir, "my duel.yrp"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatal("file content != STOC_REPLAY payload")
	}

	// 空名 → _LastReplay.yrp
	saved, err = c.SaveReplay("")
	if err != nil || saved != "_LastReplay" {
		t.Fatalf("empty name fallback: saved=%q err=%v", saved, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "_LastReplay.yrp")); err != nil {
		t.Fatalf("_LastReplay.yrp missing: %v", err)
	}

	// 路径分隔符清洗：不允许越出 replay 目录
	saved, err = c.SaveReplay("a/b\\c")
	if err != nil {
		t.Fatalf("SaveReplay sanitized: %v", err)
	}
	if saved != "a_b_c" {
		t.Errorf("sanitized name = %q, want a_b_c", saved)
	}
}
