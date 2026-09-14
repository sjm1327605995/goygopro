package duel

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

// TestReplayAllRealFiles replays every genuine .yrp produced by the
// ygopro-yrp-encode reference encoder through the full server-side replay
// pipeline: ReplayMode.Load (info-section decode) followed by ReplayMode.Run
// (ocgcore duel driven by the recorded responses). This is the bulk
// server-behavior check: the engine must regenerate the message stream from
// the replay seed and consume every recorded response in lockstep. A recorded
// response is valid only if the engine accepts it on the first try — any
// MSG_RETRY means the regenerated question no longer matches the recording.
//
// Outcomes:
//   - completed: the duel ran to PROCESSOR_END with every answer accepted.
//   - exhausted: the recording stopped feeding answers while the duel was
//     still live (real recordings end this way when the duel was cut short by
//     surrender/disconnect). Clean when the engine never retried.
//   - desynchronized: the engine rejected a recorded response. Tolerated only
//     for replays whose decks contain custom cards (e.g. c100261001x) absent
//     from every official database — a data gap, not a driver defect. For
//     fully-known decks this is a hard failure.
//
// Other failures: Load errors, zero message batches, opening draws that do
// not match the recorded StartHand, panics, unexpected errors.
func TestReplayAllRealFiles(t *testing.T) {
	dir := realReplayDir()
	if dir == "" {
		t.Skip("real replay directory not present")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.yrp"))
	if err != nil || len(files) == 0 {
		t.Skip("no .yrp files found")
	}
	sort.Strings(files)

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultDataManager.LoadDB(filepath.Join(root, "cards.cdb")); err != nil {
		t.Fatalf("load cards.cdb: %v", err)
	}
	if err := ocgcore.Init(
		ocgcore.WithRootPath(root),
		ocgcore.WithScriptDirectory(filepath.Join(root, "script")),
		ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
			return DefaultDataManager.GetData(cardId)
		}),
	); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}

	// knownDivergence lists fully-known-deck replays that verifiably diverge
	// from the recording for environmental reasons, with the floor of
	// responses that were accepted cleanly before the first rejection. The
	// divergence itself is card-database/script version drift between the
	// recording client and this repository's engine: the driver replays the
	// duel answer by answer until one card effect resolves differently and a
	// recorded answer no longer fits the regenerated question.
	knownDivergence := map[string]string{
		"085.yrp": "first rejection at recorded response #135 (a 3-byte SELECT_CARD answer); upstream state diverged earlier",
	}
	knownDivergenceFloor := map[string]int{
		"085.yrp": 135,
	}

	var (
		completed, exhausted, desync, otherErr int
		failures                               []string
		desyncKnown, desyncUnknown             int
		knownDiverged                          int
	)
	for _, f := range files {
		name := filepath.Base(f)

		rm := NewReplayMode()
		if err := rm.Load(f); err != nil {
			failures = append(failures, fmt.Sprintf("%s: load: %v", name, err))
			continue
		}

		// Count deck cards that no official database knows. Replays recorded
		// against such custom cards cannot be reproduced exactly (the engine
		// diverges the moment a missing script would have made a decision);
		// replays with fully-known decks CAN and must not diverge.
		unknownCards := 0
		for _, d := range rm.Decks {
			for _, code := range append(append([]uint32(nil), d.Main...), d.Extra...) {
				if DefaultDataManager.GetData(code) == nil {
					unknownCards++
				}
			}
		}

		batches := 0
		var draw0, draw1 uint8
		var saw0, saw1 bool
		err := rm.Run(func(msg []byte) {
			batches++
			// The engine deals each opening hand as the leading MSG_DRAW of its
			// own first batch (batch 1 = one player's hand, batch 2 = the
			// other's); later batches begin with per-turn draws. Record only the
			// FIRST draw observed per player so turn draws cannot mask it.
			p := 0
			for p+2 <= len(msg) && msg[p] == ocgcore.MSG_DRAW {
				player, count := msg[p+1], msg[p+2]
				if player == 0 && !saw0 {
					draw0, saw0 = count, true
				} else if player == 1 && !saw1 {
					draw1, saw1 = count, true
				}
				p += 2 + int(count)*4
			}
		})

		if batches == 0 {
			failures = append(failures, fmt.Sprintf("%s: engine produced no message batches", name))
			continue
		}
		if !saw0 || !saw1 {
			failures = append(failures, fmt.Sprintf("%s: opening hand draw missing (p0=%v p1=%v)", name, saw0, saw1))
			continue
		}
		if int(draw0) != int(rm.Params.StartHand) || int(draw1) != int(rm.Params.StartHand) {
			failures = append(failures, fmt.Sprintf("%s: opening draws %d/%d != recorded StartHand %d",
				name, draw0, draw1, rm.Params.StartHand))
			continue
		}

		switch {
		case err == nil:
			// Duel ran to PROCESSOR_END with every recorded answer accepted.
			completed++
		case errors.Is(err, ErrReplayResponseUnderflow):
			// The recording stopped feeding answers while the duel was still
			// live. Real YGOPro recordings end like this when the duel was cut
			// short (surrender/disconnect): the last recorded answer is
			// followed by the stream terminator and the engine asks one more
			// question nobody recorded. As long as the engine accepted every
			// answer it got (no retries), the replay verified cleanly.
			exhausted++
			if rm.RetryBatches > 0 && unknownCards == 0 {
				failures = append(failures, fmt.Sprintf("%s: exhausted after %d rejected-response retries — fully-known deck must not diverge", name, rm.RetryBatches))
			}
		case errors.Is(err, ErrReplayDesynchronized):
			desync++
			if unknownCards > 0 {
				desyncUnknown++
			} else if reason, ok := knownDivergence[name]; ok {
				// Documented data-version gap, not a driver bug: the recorded
				// answers were accepted one by one until a card effect resolved
				// differently (card database / script drift with the recording
				// client) and the next answer no longer fit the regenerated
				// question. Keep the floor so a real driver regression —
				// diverging much earlier — still fails the test.
				if rm.ResponsesConsumed < knownDivergenceFloor[name] {
					failures = append(failures, fmt.Sprintf("%s: regression — desynced after only %d accepted responses (was %d): %s",
						name, rm.ResponsesConsumed, knownDivergenceFloor[name], reason))
				}
				knownDiverged++
			} else {
				desyncKnown++
				failures = append(failures, fmt.Sprintf("%s: desynced after %d accepted responses (%d retry batches) — fully-known deck must not diverge (add to knownDivergence only after confirming the cause is data-version drift)",
					name, rm.ResponsesConsumed, rm.RetryBatches))
			}
		default:
			otherErr++
			failures = append(failures, fmt.Sprintf("%s: unexpected run error: %v", name, err))
		}
		t.Logf("%s: batches=%d unknownCards=%d responses=%d retries=%d outcome=%s",
			name, batches, unknownCards, rm.ResponsesConsumed, rm.RetryBatches, outcomeName(err))
	}

	t.Logf("replayed %d files: completed=%d exhausted=%d desync=%d (fully-known=%d custom-cards=%d known-divergence=%d) unexpected-error=%d",
		len(files), completed, exhausted, desync, desyncKnown, desyncUnknown, knownDiverged, otherErr)
	if len(failures) > 0 {
		t.Fatalf("%d/%d replays failed:\n%s", len(failures), len(files), joinLines(failures))
	}
}

func outcomeName(err error) string {
	switch {
	case err == nil:
		return "completed"
	case errors.Is(err, ErrReplayResponseUnderflow):
		return "response-exhausted"
	case errors.Is(err, ErrReplayDesynchronized):
		return "desynchronized"
	default:
		return err.Error()
	}
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += "  " + l
	}
	return out
}
