package duel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/sjm1327605995/goygopro/protocol"
	"github.com/ulikunitz/xz/lzma"
)

// Replay flags
const (
	REPLAY_COMPRESSED  = 0x1
	REPLAY_TAG         = 0x2
	REPLAY_DECODED     = 0x4
	REPLAY_SINGLE_MODE = 0x8
	REPLAY_UNIFORM     = 0x10
)

const (
	REPLAY_ID_YRP1 = 0x31707279
	REPLAY_ID_YRP2 = 0x32707279
)

const (
	MAX_REPLAY_SIZE = 0x80000
	MAX_COMP_SIZE   = 0x10000
	SEED_COUNT      = 8
)

type ReplayHeader struct {
	ID         uint32
	Version    uint32
	Flag       uint32
	Seed       uint32
	DataSize   uint32
	StartTime  uint32
	Props      [8]uint8
}

type ExtendedReplayHeader struct {
	Base         ReplayHeader
	SeedSequence [SEED_COUNT]uint32
	HeaderVersion uint32
	Value1       uint32
	Value2       uint32
	Value3       uint32
}

type DuelParameters struct {
	StartLP    int32
	StartHand  int32
	DrawCount  int32
	DuelFlag   uint32
}

type DeckArray struct {
	Main  []uint32
	Extra []uint32
}

type Replay struct {
	fp           *os.File
	pheader      ExtendedReplayHeader
	compData     []byte
	compSize     int
	players      []string
	params       DuelParameters
	decks        []DeckArray
	scriptName   string
	
	replayData   []byte
	replaySize   int
	dataPosition int
	infoOffset   int
	isRecording  bool
	isReplaying  bool
	canRead      bool
}

func NewReplay() *Replay {
	return &Replay{
		replayData: make([]byte, MAX_REPLAY_SIZE),
		compData:   make([]byte, MAX_COMP_SIZE),
	}
}

func (r *Replay) BeginRecord() {
	if _, err := os.Stat("./replay"); os.IsNotExist(err) {
		if err := os.Mkdir("./replay", 0755); err != nil {
			return
		}
	}
	if r.isRecording && r.fp != nil {
		r.fp.Close()
	}
	fp, err := os.Create("./replay/_LastReplay.yrp")
	if err != nil {
		return
	}
	r.fp = fp
	r.Reset()
	r.isRecording = true
}

func (r *Replay) WriteHeader(header ExtendedReplayHeader) {
	r.pheader = header
	binary.Write(r.fp, binary.LittleEndian, header)
	r.fp.Sync()
}

func (r *Replay) WriteData(data []byte, flush bool) {
	if !r.isRecording {
		return
	}
	if r.replaySize+len(data) > MAX_REPLAY_SIZE {
		return
	}
	copy(r.replayData[r.replaySize:], data)
	r.replaySize += len(data)
	r.fp.Write(data)
	if flush {
		r.fp.Sync()
	}
}

func (r *Replay) WriteInt32(data int32, flush bool) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(data))
	r.WriteData(b, flush)
}

func (r *Replay) Flush() {
	if !r.isRecording {
		return
	}
	r.fp.Sync()
}

func (r *Replay) EndRecord() {
	if !r.isRecording {
		return
	}
	r.fp.Close()
	r.pheader.Base.DataSize = uint32(r.replaySize)

	// LZMA compression using ulikunitz/xz/lzma
	// C++: LzmaCompress(compData, &comp_size, replayData, replay_size, pheader.Base.Props, &propsize, 5, 0x1U << 24, 3, 0, 2, 32, 1)
	var buf bytes.Buffer
	cfg := lzma.WriterConfig{
		Properties:   &lzma.Properties{LC: 3, LP: 0, PB: 2},
		DictCap:      1 << 24,
		SizeInHeader: false,
	}
	w, err := cfg.NewWriter(&buf)
	if err != nil {
		// fallback to uncompressed
		r.pheader.Base.Flag &^= REPLAY_COMPRESSED
		copy(r.compData, r.replayData[:r.replaySize])
		r.compSize = r.replaySize
		r.isRecording = false
		return
	}
	if _, err = w.Write(r.replayData[:r.replaySize]); err != nil {
		w.Close()
		r.pheader.Base.Flag &^= REPLAY_COMPRESSED
		copy(r.compData, r.replayData[:r.replaySize])
		r.compSize = r.replaySize
		r.isRecording = false
		return
	}
	if err = w.Close(); err != nil {
		r.pheader.Base.Flag &^= REPLAY_COMPRESSED
		copy(r.compData, r.replayData[:r.replaySize])
		r.compSize = r.replaySize
		r.isRecording = false
		return
	}

	output := buf.Bytes()
	if len(output) < 13 || len(output)-8 > MAX_COMP_SIZE {
		// compressed data too large or error, fallback to uncompressed
		r.pheader.Base.Flag &^= REPLAY_COMPRESSED
		copy(r.compData, r.replayData[:r.replaySize])
		r.compSize = r.replaySize
		r.isRecording = false
		return
	}

	// ulikunitz 产出的是「13 字节 LZMA 文件头 + 压缩数据」，其中前 5 字节正是
	// props（1 字节编码参数 + 4 字节字典大小），后 8 字节是原始长度。
	//
	// yrp 的布局是 header + **纯压缩数据**：props 只存在于 header.Props 里，
	// 原始长度存在 header.DataSize 里（见 C++ replay.cpp 的 EndRecord/SaveReplay，
	// LzmaCompress 把 props 单独输出到 pheader.base.props，comp_data 只有压缩数据）。
	// 曾经这里把 props 又写进了压缩数据段开头，于是文件比原版多 5 字节 ——
	// 存出来的录像原版 ygopro 打不开，原版的录像这边也读不了。
	r.pheader.Base.Flag |= REPLAY_COMPRESSED
	copy(r.pheader.Base.Props[:], output[0:5])
	compLen := len(output) - 13
	copy(r.compData, output[13:])
	r.compSize = compLen
	r.isRecording = false
}

func (r *Replay) SaveReplay(baseName string) bool {
	if _, err := os.Stat("./replay"); os.IsNotExist(err) {
		if err := os.Mkdir("./replay", 0755); err != nil {
			return false
		}
	}
	filename := strings.ReplaceAll(baseName, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	path := filepath.Join("./replay", filename+".yrp")
	
	rfp, err := os.Create(path)
	if err != nil {
		return false
	}
	defer rfp.Close()
	
	// Write header: YRP1 writes only Base, YRP2 writes full ExtendedReplayHeader
	if r.pheader.Base.ID == REPLAY_ID_YRP2 {
		binary.Write(rfp, binary.LittleEndian, r.pheader)
	} else {
		binary.Write(rfp, binary.LittleEndian, r.pheader.Base)
	}
	rfp.Write(r.compData[:r.compSize])
	return true
}

func (r *Replay) OpenReplay(name string) bool {
	path := name
	rfp, err := os.Open(path)
	if err != nil {
		path = filepath.Join("./replay", name)
		rfp, err = os.Open(path)
		if err != nil {
			return false
		}
	}
	defer rfp.Close()
	
	r.Reset()
	
	var correctHeader bool
	_ = correctHeader
	if err := binary.Read(rfp, binary.LittleEndian, &r.pheader.Base); err != nil {
		return false
	}
	
	if r.pheader.Base.ID != REPLAY_ID_YRP1 && r.pheader.Base.ID != REPLAY_ID_YRP2 {
		return false
	}
	if r.pheader.Base.Version < 0x12d0 {
		return false
	}
	if r.pheader.Base.Version >= 0x1353 && (r.pheader.Base.Flag&REPLAY_UNIFORM) == 0 {
		return false
	}
	
	if r.pheader.Base.ID == REPLAY_ID_YRP2 {
		// Read only the extended fields (after Base)
		var extra struct {
			SeedSequence  [SEED_COUNT]uint32
			HeaderVersion uint32
			Value1        uint32
			Value2        uint32
			Value3        uint32
		}
		if err := binary.Read(rfp, binary.LittleEndian, &extra); err != nil {
			return false
		}
		r.pheader.SeedSequence = extra.SeedSequence
		r.pheader.HeaderVersion = extra.HeaderVersion
		r.pheader.Value1 = extra.Value1
		r.pheader.Value2 = extra.Value2
		r.pheader.Value3 = extra.Value3
	}
	
	if r.pheader.Base.Flag&REPLAY_COMPRESSED != 0 {
		r.compSize, _ = rfp.Read(r.compData)
		r.replaySize = int(r.pheader.Base.DataSize)

		// 文件里的压缩数据段是纯压缩数据，props 在 header 里（见 EndRecord 的说明）。
		// ulikunitz 的 Reader 要的是「13 字节文件头 + 压缩数据」，所以这里拿 header 的
		// props 拼出那 13 字节：前 5 字节是 props，后 8 字节是原始长度 ——
		// 填 0xFF 表示「长度未知」，让它一直解到流结束。
		var fakeHeader [13]byte
		copy(fakeHeader[0:5], r.pheader.Base.Props[:5])
		for i := 5; i < 13; i++ {
			fakeHeader[i] = 0xFF
		}
		fullData := append(fakeHeader[:], r.compData[:r.compSize]...)
		reader, err := lzma.NewReader(bytes.NewReader(fullData))
		if err != nil {
			r.Reset()
			return false
		}
		var decompressed bytes.Buffer
		if _, err := decompressed.ReadFrom(reader); err != nil {
			r.Reset()
			return false
		}
		if decompressed.Len() != r.replaySize {
			r.Reset()
			return false
		}
		copy(r.replayData, decompressed.Bytes())
	} else {
		r.replaySize, _ = rfp.Read(r.replayData)
		r.compSize = 0
	}
	
	r.isReplaying = true
	r.canRead = true
	if !r.ReadInfo() {
		r.Reset()
		return false
	}
	r.infoOffset = r.dataPosition
	r.dataPosition = 0
	return true
}

func (r *Replay) DeleteReplay(name string) bool {
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	path := filepath.Join("./replay", name)
	return os.Remove(path) == nil
}

func (r *Replay) RenameReplay(oldName, newName string) bool {
	if strings.Contains(oldName, "/") || strings.Contains(oldName, "\\") {
		return false
	}
	if strings.Contains(newName, "/") || strings.Contains(newName, "\\") {
		return false
	}
	oldPath := filepath.Join("./replay", oldName)
	newPath := filepath.Join("./replay", newName)
	return os.Rename(oldPath, newPath) == nil
}

func (r *Replay) ReadNextResponse(resp []byte) bool {
	var length uint8
	if !r.ReadData([]byte{length}, 1) {
		return false
	}
	length = r.replayData[r.dataPosition-1]
	if !r.ReadData(resp, int(length)) {
		return false
	}
	return true
}

func (r *Replay) ReadName() string {
	var buffer [20]uint16
	if !r.ReadData(buffer[:], 40) {
		return ""
	}
	// Convert UTF-16 LE to string
	runes := utf16.Decode(buffer[:])
	// Trim null terminators
	end := len(runes)
	for end > 0 && runes[end-1] == 0 {
		end--
	}
	return string(runes[:end])
}

func (r *Replay) ReadHeader() ExtendedReplayHeader {
	return r.pheader
}

func (r *Replay) ReadData(data interface{}, length int) bool {
	if !r.isReplaying || !r.canRead {
		return false
	}
	if r.dataPosition+length > r.replaySize {
		r.canRead = false
		return false
	}
	if length > 0 {
		src := r.replayData[r.dataPosition : r.dataPosition+length]
		switch d := data.(type) {
		case []byte:
			copy(d, src)
		case []uint16:
			for i := 0; i < length/2 && i < len(d); i++ {
				d[i] = binary.LittleEndian.Uint16(src[i*2:])
			}
		case []uint32:
			for i := 0; i < length/4 && i < len(d); i++ {
				d[i] = binary.LittleEndian.Uint32(src[i*4:])
			}
		default:
			// 结构体指针（DuelParameters 等）走通用路径。
			//
			// 这里原本只有前两个 case，其余类型一个都不匹配 —— 什么都不写，
			// 却照样推进位置并返回 true。于是录像里的决斗参数和卡组读出来全是 0，
			// 而调用方完全看不出失败。默认分支宁可报错，也不能再静默吞掉。
			if err := binary.Read(bytes.NewReader(src), binary.LittleEndian, data); err != nil {
				r.canRead = false
				return false
			}
		}
	}
	r.dataPosition += length
	return true
}

func (r *Replay) ReadInt32() int32 {
	var b [4]byte
	r.ReadData(b[:], 4)
	return int32(binary.LittleEndian.Uint32(b[:]))
}

// ReadUint16 读两个字节。录像里的脚本名长度用的就是这个宽度。
func (r *Replay) ReadUint16() uint16 {
	var b [2]byte
	r.ReadData(b[:], 2)
	return binary.LittleEndian.Uint16(b[:])
}

func (r *Replay) Rewind() {
	r.dataPosition = 0
	r.canRead = true
}

func (r *Replay) Reset() {
	r.isRecording = false
	r.isReplaying = false
	r.canRead = false
	r.replaySize = 0
	r.compSize = 0
	r.dataPosition = 0
	r.infoOffset = 0
	r.players = nil
	r.params = DuelParameters{}
	r.decks = nil
	r.scriptName = ""
}

func (r *Replay) SkipInfo() {
	if r.dataPosition == 0 {
		r.dataPosition = r.infoOffset
	}
}

func (r *Replay) IsReplaying() bool {
	return r.isReplaying
}

func (r *Replay) SaveDeck(index int, filename string) bool {
	if index >= len(r.decks) {
		return false
	}
	deck := r.decks[index]
	// C++ reverses main/extra before saving — follow the same behavior
	// so the exported .ydk matches the original deck order.
	main := make([]uint32, len(deck.Main))
	copy(main, deck.Main)
	for i, j := 0, len(main)-1; i < j; i, j = i+1, j-1 {
		main[i], main[j] = main[j], main[i]
	}
	extra := make([]uint32, len(deck.Extra))
	copy(extra, deck.Extra)
	for i, j := 0, len(extra)-1; i < j; i, j = i+1, j-1 {
		extra[i], extra[j] = extra[j], extra[i]
	}

	dir := filepath.Dir(filename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return false
		}
	}
	fp, err := os.Create(filename)
	if err != nil {
		return false
	}
	defer fp.Close()

	if _, err := fp.WriteString("#created by ...\n#main\n"); err != nil {
		return false
	}
	for _, code := range main {
		if _, err := fp.WriteString(fmt.Sprintf("%d\n", code)); err != nil {
			return false
		}
	}
	if _, err := fp.WriteString("#extra\n"); err != nil {
		return false
	}
	for _, code := range extra {
		if _, err := fp.WriteString(fmt.Sprintf("%d\n", code)); err != nil {
			return false
		}
	}
	if _, err := fp.WriteString("!side\n"); err != nil {
		return false
	}
	return true
}

func (r *Replay) ReadInfo() bool {
	playerCount := 2
	if r.pheader.Base.Flag&REPLAY_TAG != 0 {
		playerCount = 4
	}
	
	for i := 0; i < playerCount; i++ {
		name := r.ReadName()
		if name == "" {
			return false
		}
		r.players = append(r.players, name)
	}
	
	if !r.ReadData(&r.params, 16) {
		return false
	}
	
	isTag1 := r.pheader.Base.Flag&REPLAY_TAG != 0
	isTag2 := r.params.DuelFlag&0x20 != 0 // DUEL_TAG_MODE
	if isTag1 != isTag2 {
		return false
	}
	
	if r.pheader.Base.Flag&REPLAY_SINGLE_MODE != 0 {
		// C++ 这里读的是 uint16_t（replay.cpp 的 ReadInfo），读成 4 字节会让其后全部错位
		slen := int32(r.ReadUint16())
		if slen == 0 || slen > 255 {
			return false
		}
		var filename [256]byte
		if !r.ReadData(filename[:], int(slen)) {
			return false
		}
		filename[slen] = 0
		nameStr := string(filename[:slen])
		if !strings.HasPrefix(nameStr, "./single/") {
			return false
		}
		r.scriptName = nameStr[9:]
		if strings.Contains(r.scriptName, "/") || strings.Contains(r.scriptName, "\\") {
			return false
		}
	} else {
		for p := 0; p < playerCount; p++ {
			var deck DeckArray
			main := r.ReadInt32()
			if main > protocol.MAINC_MAX {
				return false
			}
			if main > 0 {
				deck.Main = make([]uint32, main)
				if !r.ReadData(deck.Main, int(main)*4) {
					return false
				}
			}
			extra := r.ReadInt32()
			if extra > protocol.MAINC_MAX {
				return false
			}
			if extra > 0 {
				deck.Extra = make([]uint32, extra)
				if !r.ReadData(deck.Extra, int(extra)*4) {
					return false
				}
			}
			r.decks = append(r.decks, deck)
		}
	}
	return true
}

func GetDeckPlayer(deckIndex int) int {
	switch deckIndex {
	case 2:
		return 3
	case 3:
		return 2
	default:
		return deckIndex
	}
}
