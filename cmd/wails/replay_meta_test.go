package main

// 波 I：ReplayInfo / ExportReplayDeck 绑定测试。
// 手工合成 .yrp（YRP1 头 + LZMA1 裸流，格式对齐 core/duel/replay.go）：
// 标准对局含双方卡组（非 UNIFORM → 录像时间取 seed 字段）、单机录像各一盘，
// 验证元信息读取与卡组导出（menu_handler.cpp:293-319 / 519-559 的 Go 对应面）。

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/ulikunitz/xz/lzma"
)

func utf16Name(name string) []byte {
	buf := make([]byte, 40)
	for i, c := range utf16.Encode([]rune(name)) {
		binary.LittleEndian.PutUint16(buf[i*2:], c)
	}
	return buf
}

// writeYRP1File 按 core/duel 的存储格式写出 .yrp：32 字节 Base 头 + LZMA 裸流
// （5 字节 props 在头内偏移 24，replayData 经 LZMA 压缩，头里只存裸流）。
func writeYRP1File(t *testing.T, path string, version, flag, seed uint32, replayData []byte) {
	t.Helper()
	var buf bytes.Buffer
	cfg := lzma.WriterConfig{
		Properties:   &lzma.Properties{LC: 3, LP: 0, PB: 2},
		DictCap:      1 << 24,
		SizeInHeader: false,
	}
	w, err := cfg.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(replayData); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out := buf.Bytes()
	if len(out) < 13 {
		t.Fatal("lzma output too short")
	}
	hdr := make([]byte, 32)
	binary.LittleEndian.PutUint32(hdr[0:], duel.REPLAY_ID_YRP1)
	binary.LittleEndian.PutUint32(hdr[4:], version)
	binary.LittleEndian.PutUint32(hdr[8:], flag|duel.REPLAY_COMPRESSED)
	binary.LittleEndian.PutUint32(hdr[12:], seed)
	binary.LittleEndian.PutUint32(hdr[16:], uint32(len(replayData)))
	copy(hdr[24:29], out[0:5])
	file := append(hdr, out[13:]...)
	if err := os.WriteFile(path, file, 0644); err != nil {
		t.Fatal(err)
	}
}

// buildDuelReplayData：Player1/Player2、标准参数、
// p0 主卡组 [89631139, 5318639]、p1 主卡组 [70902743]。
func buildDuelReplayData() []byte {
	var d []byte
	d = append(d, utf16Name("Player1")...)
	d = append(d, utf16Name("Player2")...)
	params := make([]byte, 16)
	binary.LittleEndian.PutUint32(params[0:], 8000)
	binary.LittleEndian.PutUint32(params[4:], 5)
	binary.LittleEndian.PutUint32(params[8:], 1)
	binary.LittleEndian.PutUint32(params[12:], 0)
	d = append(d, params...)
	cnt := make([]byte, 4)
	code := make([]byte, 4)
	// 布局与 replay.cpp:264-278 一致：主数量、主卡、副数量、副卡 交替
	binary.LittleEndian.PutUint32(cnt, 2)
	d = append(d, cnt...)
	for _, c := range []uint32{89631139, 5318639} {
		binary.LittleEndian.PutUint32(code, c)
		d = append(d, code...)
	}
	binary.LittleEndian.PutUint32(cnt, 0)
	d = append(d, cnt...)
	binary.LittleEndian.PutUint32(cnt, 1)
	d = append(d, cnt...)
	for _, c := range []uint32{70902743} {
		binary.LittleEndian.PutUint32(code, c)
		d = append(d, code...)
	}
	binary.LittleEndian.PutUint32(cnt, 0)
	d = append(d, cnt...)
	return d
}

func TestReplayInfoBinding(t *testing.T) {
	dir := t.TempDir()
	writeYRP1File(t, filepath.Join(dir, "export_meta_test.yrp"), 0x12d0, 0, 1750000000, buildDuelReplayData())
	a := &App{replayDir: dir, deckDir: t.TempDir()}

	info := a.ReplayInfo("export_meta_test.yrp")
	if info["success"] != true {
		t.Fatalf("ReplayInfo failed: %v", info["error"])
	}
	if got := info["players"].([]string); len(got) != 2 || got[0] != "Player1" || got[1] != "Player2" {
		t.Fatalf("players = %v", got)
	}
	if info["isTag"] != false || info["isSingle"] != false {
		t.Fatalf("flags = %v %v", info["isTag"], info["isSingle"])
	}
	wantDate := time.Unix(1750000000, 0).Format("2006/01/02 15:04:05")
	if info["date"] != wantDate {
		t.Fatalf("date = %v, want %v", info["date"], wantDate)
	}
	if info["version"].(uint32) != 0x12d0 {
		t.Fatalf("version = %v", info["version"])
	}

	// 坏文件 → success=false 而不是 panic
	if bad := a.ReplayInfo("nonexistent.yrp"); bad["success"] != false {
		t.Fatalf("ReplayInfo on missing file should fail: %v", bad)
	}
}

func TestExportReplayDeckBinding(t *testing.T) {
	dir := t.TempDir()
	writeYRP1File(t, filepath.Join(dir, "export_meta_test.yrp"), 0x12d0, 0, 1750000000, buildDuelReplayData())
	deckDir := t.TempDir()
	a := &App{replayDir: dir, deckDir: deckDir}

	res := a.ExportReplayDeck("export_meta_test.yrp")
	if res["success"] != true {
		t.Fatalf("ExportReplayDeck failed: %v", res["error"])
	}
	files := res["files"].([]string)
	want := []string{"export_meta_test.yrp-1 Player1.ydk", "export_meta_test.yrp-2 Player2.ydk"}
	if len(files) != 2 || files[0] != want[0] || files[1] != want[1] {
		t.Fatalf("files = %v, want %v", files, want)
	}

	// p0 卡组：录像里存的是召唤栈序，SaveDeck 按原版反转后写盘
	f0, err := os.Open(filepath.Join(deckDir, want[0]))
	if err != nil {
		t.Fatal(err)
	}
	defer f0.Close()
	var codes []uint32
	scanner := bufio.NewScanner(f0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		var code uint32
		fmt.Sscanf(line, "%d", &code)
		codes = append(codes, code)
	}
	if len(codes) != 2 || codes[0] != 5318639 || codes[1] != 89631139 {
		t.Fatalf("deck codes = %v, want reversed [5318639 89631139]", codes)
	}

	// 单机录像：无卡组信息 → 拒绝导出（C++ 单机段：uint16 长度 + ./single/ 脚本名）
	var d []byte
	d = append(d, utf16Name("SoloPlayer")...)
	d = append(d, utf16Name("Puzzle")...)
	params := make([]byte, 16)
	binary.LittleEndian.PutUint32(params[0:], 8000)
	d = append(d, params...)
	script := "./single/test.lua"
	lbuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(lbuf, uint16(len(script)))
	d = append(d, lbuf...)
	d = append(d, script...)
	smDir := t.TempDir()
	writeYRP1File(t, filepath.Join(smDir, "single_test.yrp"), 0x12d0, duel.REPLAY_SINGLE_MODE, 1, d)
	a2 := &App{replayDir: smDir, deckDir: t.TempDir()}
	if res := a2.ExportReplayDeck("single_test.yrp"); res["success"] != false {
		t.Fatalf("single-mode export should fail: %v", res)
	}
}
