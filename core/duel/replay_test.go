package duel

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
	"unicode/utf16"

	"github.com/ulikunitz/xz/lzma"
)

func TestReplayLZMACompression(t *testing.T) {
	r := NewReplay()
	r.BeginRecord()

	// Write some test data
	testData := []byte("Hello, World! This is a test of LZMA compression for YGOPro replay format.")
	r.WriteData(testData, true)

	// End recording (this triggers LZMA compression)
	r.EndRecord()

	// Verify compressed flag is set
	if r.pheader.Base.Flag&REPLAY_COMPRESSED == 0 {
		t.Fatal("REPLAY_COMPRESSED flag not set")
	}

	// Verify props are set
	if r.pheader.Base.Props[0] == 0 {
		t.Fatal("LZMA props not set")
	}

	// compData holds only the raw LZMA1 stream; the 5-byte props live in the
	// header (Props[0:4]). Reconstruct the 13-byte .lzma header to decompress.
	var fakeHeader [13]byte
	copy(fakeHeader[0:5], r.pheader.Base.Props[:5])
	// uncompressed size = unknown (0xFFFFFFFFFFFFFFFF)
	for i := 5; i < 13; i++ {
		fakeHeader[i] = 0xFF
	}

	fullData := append(fakeHeader[:], r.compData[:r.compSize]...)

	reader, err := lzma.NewReader(bytes.NewReader(fullData))
	if err != nil {
		t.Fatalf("Failed to create LZMA reader: %v", err)
	}

	var decompressed bytes.Buffer
	if _, err := decompressed.ReadFrom(reader); err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if !bytes.Equal(decompressed.Bytes(), testData) {
		t.Fatalf("Decompressed data mismatch. Got %q, want %q", decompressed.Bytes(), testData)
	}

	t.Logf("Original size: %d, Compressed size: %d", len(testData), r.compSize)
}

func TestReplaySaveAndOpen(t *testing.T) {
	// Clean up test replay files
	defer os.RemoveAll("./replay")

	r := NewReplay()
	r.BeginRecord()

	// Write valid replay data: 2 player names (40 bytes each) + DuelParameters (16 bytes) + deck data
	// Player names are UTF-16 LE, 20 uint16s = 40 bytes each
	player1Name := make([]byte, 40)
	player2Name := make([]byte, 40)
	// Write "Player1" and "Player2" as UTF-16 LE
	for i, c := range utf16.Encode([]rune("Player1")) {
		binary.LittleEndian.PutUint16(player1Name[i*2:], c)
	}
	for i, c := range utf16.Encode([]rune("Player2")) {
		binary.LittleEndian.PutUint16(player2Name[i*2:], c)
	}
	r.WriteData(player1Name, false)
	r.WriteData(player2Name, false)
	// DuelParameters: StartLP, StartHand, DrawCount, DuelFlag
	params := make([]byte, 16)
	binary.LittleEndian.PutUint32(params[0:], 8000)  // StartLP
	binary.LittleEndian.PutUint32(params[4:], 5)     // StartHand
	binary.LittleEndian.PutUint32(params[8:], 1)     // DrawCount
	binary.LittleEndian.PutUint32(params[12:], 0)    // DuelFlag
	r.WriteData(params, false)
	// Deck data: main count (4) + extra count (4) for each player
	// For simplicity, write 0-count decks
	deckData := make([]byte, 16)
	r.WriteData(deckData, false)
	// Some payload data
	payload := []byte("Test replay data for save/open roundtrip with LZMA compression.")
	r.WriteData(payload, true)

	// Set up a minimal valid header for YRP2
	totalDataSize := 40 + 40 + 16 + 16 + len(payload)
	r.pheader.Base.ID = REPLAY_ID_YRP2
	r.pheader.Base.Version = 0x1353
	r.pheader.Base.Flag = REPLAY_UNIFORM
	r.pheader.Base.DataSize = uint32(totalDataSize)

	// End recording
	r.EndRecord()

	// Save replay
	if !r.SaveReplay("test_roundtrip") {
		t.Fatal("SaveReplay failed")
	}

	// Verify file content manually
	data, err := os.ReadFile("./replay/test_roundtrip.yrp")
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	// Base header is 40 bytes, extended header is 48 bytes, compSize is r.compSize
	expectedSize := binary.Size(ExtendedReplayHeader{}) + r.compSize
	if len(data) != expectedSize {
		t.Fatalf("File size mismatch: got %d, want %d", len(data), expectedSize)
	}

	// Verify header ID
	if binary.LittleEndian.Uint32(data[0:4]) != REPLAY_ID_YRP2 {
		t.Fatalf("Header ID mismatch")
	}

	// Verify compressed flag
	flag := binary.LittleEndian.Uint32(data[8:12])
	if flag&REPLAY_COMPRESSED == 0 {
		t.Fatal("REPLAY_COMPRESSED not set in saved file")
	}

	// Manually decompress and verify
	compOffset := binary.Size(ExtendedReplayHeader{})
	var fakeHeader [13]byte
	copy(fakeHeader[0:5], data[24:29]) // Base.Props lives at offset 24 in the header
	for i := 5; i < 13; i++ {
		fakeHeader[i] = 0xFF
	}
	fullData := append(fakeHeader[:], data[compOffset:compOffset+r.compSize]...)
	reader, err := lzma.NewReader(bytes.NewReader(fullData))
	if err != nil {
		t.Fatalf("Failed to create LZMA reader: %v", err)
	}
	var decompressed bytes.Buffer
	if _, err := decompressed.ReadFrom(reader); err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}
	if !bytes.Equal(decompressed.Bytes(), r.replayData[:r.replaySize]) {
		t.Fatalf("Data mismatch after manual decompression")
	}

	// Test OpenReplay
	r2 := NewReplay()
	if !r2.OpenReplay("test_roundtrip.yrp") {
		t.Fatalf("OpenReplay failed")
	}

	// Verify decompressed data
	if r2.replaySize != totalDataSize {
		t.Fatalf("replaySize mismatch: got %d, want %d", r2.replaySize, totalDataSize)
	}
	// Verify payload is at the end of decompressed data
	if !bytes.Equal(r2.replayData[r2.replaySize-len(payload):r2.replaySize], payload) {
		t.Fatalf("Payload mismatch after roundtrip")
	}

	t.Logf("Roundtrip successful: original=%d, compressed=%d", totalDataSize, r2.compSize)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
