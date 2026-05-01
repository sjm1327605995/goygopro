package duel

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjm1327605995/goygopro/protocol"
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
	r.pheader.Base.Flag |= REPLAY_COMPRESSED
	// TODO: LZMA compression
	// C++: LzmaCompress(compData, &comp_size, replayData, replay_size, pheader.Base.Props, &propsize, 5, 0x1U << 24, 3, 0, 2, 32, 1)
	// Go: Need github.com/ulikunitz/xz/lzma or similar
	// For now, leave uncompressed
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
	
	binary.Write(rfp, binary.LittleEndian, r.pheader)
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
		var extra ExtendedReplayHeader
		if err := binary.Read(rfp, binary.LittleEndian, &extra); err != nil {
			return false
		}
		// Copy extended fields
		r.pheader.SeedSequence = extra.SeedSequence
		r.pheader.HeaderVersion = extra.HeaderVersion
		r.pheader.Value1 = extra.Value1
		r.pheader.Value2 = extra.Value2
		r.pheader.Value3 = extra.Value3
	}
	
	if r.pheader.Base.Flag&REPLAY_COMPRESSED != 0 {
		// TODO: LZMA decompression
		// For now, just read raw data
		r.compSize, _ = rfp.Read(r.compData)
		r.replaySize = int(r.pheader.Base.DataSize)
		// copy(r.replayData, decompressedData)
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
	// Convert UTF-16 to string
	// TODO: proper UTF-16 conversion
	return ""
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
		switch d := data.(type) {
		case []byte:
			copy(d, r.replayData[r.dataPosition:r.dataPosition+length])
		case []uint16:
			for i := 0; i < length/2 && i < len(d); i++ {
				d[i] = binary.LittleEndian.Uint16(r.replayData[r.dataPosition+i*2:])
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
	// TODO: implement deck saving
	return false
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
		slen := r.ReadInt32()
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
