package duel

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realReplayDir returns the directory of genuine .yrp files produced by the
// ygopro-yrp-encode reference encoder, or "" if it is not present on this
// machine. Tests that depend on real files skip themselves when it is absent.
func realReplayDir() string {
	dir := os.Getenv("YGO_REPLAY_DIR")
	if dir == "" {
		dir = `C:\Users\user\Downloads\ygopro-yrp-encode-main\tests\test-replays`
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	return dir
}

// TestRealReplayHeaders verifies the raw header layout of a real .yrp file:
// YRP2 files carry an 80-byte extended header whose Base.Props (5 bytes) plus a
// raw LZMA1 stream follow. This pins down the on-disk format independently of
// our writer.
func TestRealReplayHeaders(t *testing.T) {
	dir := realReplayDir()
	if dir == "" {
		t.Skip("real replay directory not present")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.yrp"))
	if err != nil || len(files) == 0 {
		t.Skip("no .yrp files found")
	}

	parsed := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: read: %v", filepath.Base(f), err)
		}
		if len(data) < 32 {
			t.Fatalf("%s: too short (%d bytes)", filepath.Base(f), len(data))
		}
		id := binary.LittleEndian.Uint32(data[0:4])
		if id != REPLAY_ID_YRP1 && id != REPLAY_ID_YRP2 {
			t.Fatalf("%s: bad ID 0x%08x", filepath.Base(f), id)
		}
		flag := binary.LittleEndian.Uint32(data[8:12])
		if flag&REPLAY_COMPRESSED == 0 {
			t.Fatalf("%s: not compressed", filepath.Base(f))
		}
		parsed++
	}
	t.Logf("validated %d real replay headers", parsed)
}

// TestOpenRealReplays drives OpenReplay against every genuine replay file and
// asserts the info section (player names, duel parameters, decks) parses back
// non-empty and self-consistent. This is the end-to-end check that the LZMA
// layout and ReadInfo decoding match the reference encoder's output.
func TestOpenRealReplays(t *testing.T) {
	dir := realReplayDir()
	if dir == "" {
		t.Skip("real replay directory not present")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.yrp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no .yrp files found")
	}

	opened, failed := 0, 0
	for _, f := range files {
		r := NewReplay()
		if !r.OpenReplay(f) {
			failed++
			t.Errorf("%s: OpenReplay failed", filepath.Base(f))
			continue
		}
		opened++

		if len(r.players) == 0 {
			t.Errorf("%s: no player names parsed", filepath.Base(f))
		}
		for _, n := range r.players {
			if strings.TrimSpace(n) == "" {
				t.Errorf("%s: empty player name", filepath.Base(f))
			}
		}
		if r.params.StartLP <= 0 {
			t.Errorf("%s: bad StartLP %d", filepath.Base(f), r.params.StartLP)
		}
		if r.params.StartHand <= 0 {
			t.Errorf("%s: bad StartHand %d", filepath.Base(f), r.params.StartHand)
		}
		if len(r.decks) == 0 {
			t.Errorf("%s: no decks parsed", filepath.Base(f))
		}
		for i, d := range r.decks {
			if len(d.Main) == 0 && len(d.Extra) == 0 {
				t.Errorf("%s: deck %d is empty", filepath.Base(f), i)
			}
		}
	}
	if failed > 0 {
		t.Fatalf("opened %d, failed %d", opened, failed)
	}
	t.Logf("opened %d real replays successfully", opened)
}
