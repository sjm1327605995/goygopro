package duel

import (
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

// TestReplayModeLoad drives ReplayMode.Load against real .yrp files and asserts
// the metadata extracted matches what OpenReplay already decoded.
func TestReplayModeLoad(t *testing.T) {
	dir := realReplayDir()
	if dir == "" {
		t.Skip("real replay directory not present")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.yrp"))
	if len(files) == 0 {
		t.Skip("no .yrp files found")
	}
	loaded := 0
	for _, f := range files {
		rm := NewReplayMode()
		if err := rm.Load(f); err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		if len(rm.Players) == 0 {
			t.Fatalf("%s: no players", filepath.Base(f))
		}
		if rm.Params.StartLP <= 0 || rm.Params.StartHand <= 0 {
			t.Fatalf("%s: bad params %+v", filepath.Base(f), rm.Params)
		}
		if !rm.IsSingleMode && rm.Replay.decks == nil {
			t.Fatalf("%s: no decks", filepath.Base(f))
		}
		loaded++
	}
	t.Logf("loaded %d real replays via ReplayMode.Load", loaded)
}

// TestReplayModeRun verifies the replay engine driver actually starts the core
// duel and replays it. The engine does not emit MSG_START (that is synthesized
// by the server layer); its first message is MSG_DRAW for the opening hands.
// Full effect evaluation requires the card scripts, which are not part of this
// repository, so the duel may diverge after the opening draw — the assertions
// here cover the deterministic pre-script phase: both players draw their
// recorded starting hand sizes from the recorded decks.
func TestReplayModeRun(t *testing.T) {
	dir := realReplayDir()
	if dir == "" {
		t.Skip("real replay directory not present")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := ocgcore.Init(ocgcore.WithRootPath(root)); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}

	rm := NewReplayMode()
	if err := rm.Load(filepath.Join(dir, "001.yrp")); err != nil {
		t.Fatalf("load: %v", err)
	}

	var draw0, draw1 uint8
	err = rm.Run(func(msg []byte) {
		p := 0
		for p < len(msg) {
			switch msg[p] {
			case ocgcore.MSG_DRAW:
				// player(1) count(1) count*4
				if p+2 > len(msg) {
					return
				}
				player, count := msg[p+1], msg[p+2]
				if player == 0 {
					draw0 = count
				} else if player == 1 {
					draw1 = count
				}
				p += 2 + int(count)*4
			default:
				// Stop at the first message we do not parse here: MSG_DRAW
				// for both players always leads the stream, before any other
				// message type can appear.
				return
			}
		}
	})
	if err != nil {
		// Without card scripts the engine cannot reproduce recorded effect
		// decisions, so the replay diverges; that is a data availability issue
		// rather than a driver bug. Any other error is a real failure.
		if err != ErrReplayResponseUnderflow && err != ErrReplayDesynchronized {
			t.Fatalf("unexpected run error: %v", err)
		}
		t.Logf("Run stopped early (card scripts unavailable): %v", err)
	}

	if int(draw0) != int(rm.Params.StartHand) || int(draw1) != int(rm.Params.StartHand) {
		t.Fatalf("opening draws (%d/%d) do not match recorded StartHand %d",
			draw0, draw1, rm.Params.StartHand)
	}
	t.Logf("opening draws: player0=%d player1=%d (StartHand=%d)", draw0, draw1, rm.Params.StartHand)
}
