package client

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/ulikunitz/xz/lzma"
)

// 客户端只读一小段（头 + 玩家名）用于列表展示，完整实现在 core/duel/replay.go。
// 两处解析同一种格式就会漂，所以这里既按原版布局手工造文件来读，
// 也在 TestClientReadsServerWrittenReplay 里拿服务端存出来的文件交叉验证。

// makeReplayFile 按原版布局造一个 .yrp：header + 纯压缩数据，props 只在 header 里。
func makeReplayFile(t *testing.T, dir string, players []string, isTag bool, start time.Time) string {
	t.Helper()

	var body bytes.Buffer
	for _, name := range players {
		var buf [20]uint16
		copy(buf[:], utf16.Encode([]rune(name)))
		binary.Write(&body, binary.LittleEndian, buf)
	}
	// 玩家名之后是决斗参数，列表用不到，塞点东西占位
	binary.Write(&body, binary.LittleEndian, [4]int32{8000, 5, 1, 0})
	raw := body.Bytes()

	var packed bytes.Buffer
	w, err := lzma.WriterConfig{
		Properties: &lzma.Properties{LC: 3, LP: 0, PB: 2},
		DictCap:    1 << 24,
	}.NewWriter(&packed)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(raw)
	w.Close()
	out := packed.Bytes()

	flag := uint32(replayCompressed | replayUniform)
	if isTag {
		flag |= replayTag
	}

	var hdr bytes.Buffer
	binary.Write(&hdr, binary.LittleEndian, uint32(replayIDYRP2))
	binary.Write(&hdr, binary.LittleEndian, uint32(0x1360))       // version
	binary.Write(&hdr, binary.LittleEndian, flag)                 // flag
	binary.Write(&hdr, binary.LittleEndian, uint32(0))            // seed
	binary.Write(&hdr, binary.LittleEndian, uint32(len(raw)))     // datasize
	binary.Write(&hdr, binary.LittleEndian, uint32(start.Unix())) // start_time
	hdr.Write(out[0:5])                                           // props
	hdr.Write(make([]byte, 3))                                    // props 补满 8 字节
	hdr.Write(make([]byte, replayFullHeaderSize-replayBaseHeaderSize))

	path := filepath.Join(dir, start.Format("2006-01-02 15-04-05")+".yrp")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.Write(hdr.Bytes())
	f.Write(out[13:]) // 纯压缩数据
	return path
}

func TestReadReplayInfo(t *testing.T) {
	dir := t.TempDir()
	start := time.Unix(1700000000, 0)
	path := makeReplayFile(t, dir, []string{"红方", "蓝方"}, false, start)

	info, err := ReadReplayInfo(path)
	if err != nil {
		t.Fatalf("读不了按原版布局写的录像: %v", err)
	}

	if len(info.Players) != 2 || info.Players[0] != "红方" || info.Players[1] != "蓝方" {
		t.Errorf("玩家 = %v, want [红方 蓝方]", info.Players)
	}
	if !info.StartTime.Equal(start) {
		t.Errorf("开始时间 = %v, want %v", info.StartTime, start)
	}
	if info.IsTag {
		t.Error("这不是双打录像")
	}
	if got := info.Title(); got != "红方 vs 蓝方" {
		t.Errorf("标题 = %q, want 红方 vs 蓝方", got)
	}
}

// 双打是 4 个玩家名，读少了后面的字段就全错位。
func TestReadReplayInfoTag(t *testing.T) {
	dir := t.TempDir()
	path := makeReplayFile(t, dir,
		[]string{"甲", "乙", "丙", "丁"}, true, time.Unix(1700000000, 0))

	info, err := ReadReplayInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Players) != 4 {
		t.Fatalf("玩家数 = %d, want 4", len(info.Players))
	}
	if !info.IsTag {
		t.Error("应当识别为双打")
	}
	if got := info.Title(); got != "甲+乙 vs 丙+丁" {
		t.Errorf("标题 = %q, want 甲+乙 vs 丙+丁", got)
	}
}

// 列表按时间倒序，最近的在前；坏文件跳过而不是整个失败。
func TestListReplaysSortsAndSkipsBadFiles(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	if err := os.MkdirAll(replayDir, 0755); err != nil {
		t.Fatal(err)
	}
	early := time.Unix(1600000000, 0)
	late := time.Unix(1700000000, 0)
	makeReplayFile(t, replayDir, []string{"老", "对局"}, false, early)
	makeReplayFile(t, replayDir, []string{"新", "对局"}, false, late)
	// 混进一个半截文件和一个非录像
	os.WriteFile(filepath.Join(replayDir, "truncated.yrp"), []byte("坏"), 0644)
	os.WriteFile(filepath.Join(replayDir, "note.txt"), []byte("不是录像"), 0644)

	got := ListReplays()

	if len(got) != 2 {
		t.Fatalf("列出 %d 个录像, want 2（坏文件应当跳过）", len(got))
	}
	if !got[0].StartTime.Equal(late) {
		t.Errorf("首项时间 = %v, want %v —— 应当最近的在前", got[0].StartTime, late)
	}
}

// 保存要落在 replay 目录、文件名用录像自带的开始时间。
// 此前存到当前目录、名字是进程时间戳，列表根本找不到。
func TestSaveReplayFileUsesReplayDirAndStartTime(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	start := time.Unix(1700000000, 0)
	src := makeReplayFile(t, t.TempDir(), []string{"红方", "蓝方"}, false, start)
	payload, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	path, err := SaveReplayFile(payload)
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	if got := filepath.Dir(path); got != replayDir {
		t.Errorf("保存到 %q, want %q 目录", got, replayDir)
	}
	wantName := start.Format("2006-01-02 15-04-05") + ".yrp"
	if got := filepath.Base(path); got != wantName {
		t.Errorf("文件名 = %q, want %q", got, wantName)
	}
	// 存下来的必须还能读回来
	if _, err := ReadReplayInfo(path); err != nil {
		t.Errorf("存下来的录像读不回来: %v", err)
	}
}

func TestSaveReplayFileRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	if _, err := SaveReplayFile([]byte("太短")); err == nil {
		t.Error("过短的数据不该被当成录像存下来")
	}
}
