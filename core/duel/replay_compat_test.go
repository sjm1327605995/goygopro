package duel

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/ulikunitz/xz/lzma"
)

// .yrp 的布局必须与原版 ygopro 一致：ExtendedReplayHeader 之后直接跟**纯压缩数据**，
// LZMA 的 props 只存在 header.Props 里（见 C++ replay.cpp：LzmaCompress 把 props 单独
// 输出到 pheader.base.props，SaveReplay 写的是 pheader + comp_data）。
//
// 这里一度把 props 又重复写进了压缩段开头，文件比原版多 5 字节 ——
// 存出来的录像原版打不开，原版的录像这边也读不了，而自洽往返测试完全发现不了。
//
// 所以这个测试**手工按 C++ 布局造一个 .yrp**，再让 OpenReplay 去读：
// 测的是「能不能读别人写的文件」。

// buildCppStyleReplay 按原版布局造一个录像文件，返回路径。
func buildCppStyleReplay(t *testing.T, players []string, params DuelParameters, decks []DeckArray) string {
	t.Helper()

	// ---- 先拼出未压缩的数据段 ----
	var body bytes.Buffer
	for _, name := range players {
		// 每个玩家名固定 20 个 uint16（UTF-16LE），不足补零
		var buf [20]uint16
		copy(buf[:], utf16.Encode([]rune(name)))
		binary.Write(&body, binary.LittleEndian, buf)
	}
	binary.Write(&body, binary.LittleEndian, params)
	for _, d := range decks {
		binary.Write(&body, binary.LittleEndian, uint32(len(d.Main)))
		for _, c := range d.Main {
			binary.Write(&body, binary.LittleEndian, c)
		}
		binary.Write(&body, binary.LittleEndian, uint32(len(d.Extra)))
		for _, c := range d.Extra {
			binary.Write(&body, binary.LittleEndian, c)
		}
	}
	raw := body.Bytes()

	// ---- 压缩，并把 props 与压缩数据分开（这正是原版的做法）----
	var packed bytes.Buffer
	w, err := lzma.WriterConfig{
		Properties: &lzma.Properties{LC: 3, LP: 0, PB: 2},
		DictCap:    1 << 24,
	}.NewWriter(&packed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out := packed.Bytes()
	if len(out) < 13 {
		t.Fatal("压缩输出太短")
	}
	props := out[0:5]      // 1 字节编码参数 + 4 字节字典大小
	compressed := out[13:] // 跳过 13 字节 LZMA 文件头，只留压缩数据

	// ---- 写文件：header + 纯压缩数据 ----
	var hdr ExtendedReplayHeader
	hdr.Base.ID = REPLAY_ID_YRP2
	hdr.Base.Version = 0x1360
	hdr.Base.Flag = REPLAY_COMPRESSED | REPLAY_UNIFORM
	hdr.Base.DataSize = uint32(len(raw))
	hdr.Base.StartTime = 1700000000
	copy(hdr.Base.Props[:], props)

	path := filepath.Join(t.TempDir(), "compat.yrp")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := binary.Write(f, binary.LittleEndian, hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(compressed); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenReplayReadsCppLayout(t *testing.T) {
	params := DuelParameters{StartLP: 8000, StartHand: 5, DrawCount: 1, DuelFlag: 0}
	decks := []DeckArray{
		{Main: []uint32{89631139, 46986414, 89631139}, Extra: []uint32{12345678}},
		{Main: []uint32{38033121}, Extra: nil},
	}
	path := buildCppStyleReplay(t, []string{"红方", "蓝方"}, params, decks)

	r := NewReplay()
	if !r.OpenReplay(path) {
		t.Fatal("读不了按原版布局写的录像 —— 格式与 ygopro 不兼容")
	}

	if got := r.players; len(got) != 2 || got[0] != "红方" || got[1] != "蓝方" {
		t.Errorf("玩家 = %v, want [红方 蓝方]", got)
	}
	if r.params.StartLP != 8000 || r.params.StartHand != 5 || r.params.DrawCount != 1 {
		t.Errorf("决斗参数 = %+v, want LP8000/手牌5/抽卡1", r.params)
	}
	if len(r.decks) != 2 {
		t.Fatalf("卡组数 = %d, want 2", len(r.decks))
	}
	if len(r.decks[0].Main) != 3 || r.decks[0].Main[0] != 89631139 {
		t.Errorf("红方主卡组 = %v, want [89631139 46986414 89631139]", r.decks[0].Main)
	}
	if len(r.decks[0].Extra) != 1 || r.decks[0].Extra[0] != 12345678 {
		t.Errorf("红方额外卡组 = %v, want [12345678]", r.decks[0].Extra)
	}
	if len(r.decks[1].Main) != 1 || r.decks[1].Main[0] != 38033121 {
		t.Errorf("蓝方主卡组 = %v", r.decks[1].Main)
	}
}

// 反过来：自己存的录像，布局也必须是 header + 纯压缩数据。
// 多出 5 字节的话原版就打不开了。
func TestSavedReplayHasNoDuplicatedProps(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)

	r := NewReplay()
	r.pheader.Base.ID = REPLAY_ID_YRP2
	r.pheader.Base.Version = 0x1360
	r.pheader.Base.Flag = REPLAY_UNIFORM
	r.BeginRecord()
	payload := bytes.Repeat([]byte("决斗数据"), 32)
	r.WriteData(payload, true)
	r.EndRecord()
	if !r.SaveReplay("t") {
		t.Fatal("保存失败")
	}

	data, err := os.ReadFile(filepath.Join(dir, "replay", "t.yrp"))
	if err != nil {
		t.Fatal(err)
	}
	hdrSize := binary.Size(ExtendedReplayHeader{})
	compSeg := data[hdrSize:]

	// header 里的 props 不该在压缩段开头再出现一次
	props := data[24:29]
	if len(compSeg) >= 5 && bytes.Equal(compSeg[:5], props) {
		t.Error("压缩段开头重复了 header 里的 props —— 文件比原版多 5 字节，原版打不开")
	}

	// 压缩段本身要能解回原样。这里不走 OpenReplay —— 那条路会接着解析玩家名/卡组，
	// 而 payload 是随便造的、必然解析失败并 Reset，尺寸就查不到了。
	var fake [13]byte
	copy(fake[0:5], props)
	for i := 5; i < 13; i++ {
		fake[i] = 0xFF
	}
	reader, err := lzma.NewReader(bytes.NewReader(append(fake[:], compSeg...)))
	if err != nil {
		t.Fatalf("按原版布局解压失败: %v", err)
	}
	var got bytes.Buffer
	if _, err := got.ReadFrom(reader); err != nil {
		t.Fatalf("解压出错: %v", err)
	}
	if !bytes.Equal(got.Bytes(), payload) {
		t.Errorf("解压后 %d 字节, want %d —— 压缩段布局不对", got.Len(), len(payload))
	}
}
