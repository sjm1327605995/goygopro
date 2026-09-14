package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

// replayFixtureDir mirrors core/duel's realReplayDir convention: the directory
// of genuine .yrp files produced by the ygopro-yrp-encode reference encoder,
// overridable via YGO_REPLAY_DIR, skipped when absent.
func replayFixtureDir() string {
	dir := os.Getenv("YGO_REPLAY_DIR")
	if dir == "" {
		dir = `C:\Users\user\Downloads\ygopro-yrp-encode-main\tests\test-replays`
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	return dir
}

// joinLines renders failure strings one per line, indented.
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

// TestWailsClientParsesReplayMessageStreams feeds the complete engine message
// stream of every real replay through the production client parser
// (WailsDuelClient.handleGameMessage) — the exact code path a live duel uses
// for STOC_GAME_MSG. This is the bulk client-behavior check: every byte of
// every batch must be consumed to alignment and translated into frontend
// events, with no unknown opcode, misparsed body, or lost message.
//
// A parse error here means the client's message table (handleGameMessage +
// skipEngineMessageBody) does not match what ocgcore actually emits, which is
// exactly the defect class this test exists to catch.
func TestWailsClientParsesReplayMessageStreams(t *testing.T) {
	dir := replayFixtureDir()
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
	if err := duel.DefaultDataManager.LoadDB(filepath.Join(root, "cards.cdb")); err != nil {
		t.Fatalf("load cards.cdb: %v", err)
	}
	if err := ocgcore.Init(
		ocgcore.WithRootPath(root),
		ocgcore.WithScriptDirectory(filepath.Join(root, "script")),
		ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
			return duel.DefaultDataManager.GetData(cardId)
		}),
	); err != nil {
		t.Skipf("ocgcore library not available: %v", err)
	}

	var (
		parseFailures []string
		emptyBatches  []string
		totalBatches  int
		totalEvents   int
		winEvents     int
	)
	for _, f := range files {
		name := filepath.Base(f)

		rm := duel.NewReplayMode()
		if err := rm.Load(f); err != nil {
			t.Fatalf("%s: load: %v", name, err)
		}

		client := NewWailsDuelClient(func(eventName string, data ...interface{}) {
			totalEvents++
			if eventName == "duel:win" {
				winEvents++
			}
		})

		batches := 0
		var firstParseErr error
		err := rm.Run(func(msg []byte) {
			batches++
			totalBatches++
			if perr := client.handleGameMessage(msg); perr != nil && firstParseErr == nil {
				firstParseErr = perr
			}
		})
		if batches == 0 {
			emptyBatches = append(emptyBatches, fmt.Sprintf("%s: no engine batches (run err: %v)", name, err))
			continue
		}
		if firstParseErr != nil {
			parseFailures = append(parseFailures, fmt.Sprintf("%s: %v", name, firstParseErr))
		}
		t.Logf("%s: batches=%d outcome=%v", name, batches, err)
	}

	t.Logf("client parsed %d batches from %d replays into %d events (duel:win x%d)",
		totalBatches, len(files), totalEvents, winEvents)

	if len(emptyBatches) > 0 {
		t.Errorf("%d replays produced no engine batches:\n%s", len(emptyBatches), joinLines(emptyBatches))
	}
	if len(parseFailures) > 0 {
		t.Fatalf("client parser failed on %d/%d replays:\n%s",
			len(parseFailures), len(files), joinLines(parseFailures))
	}
}
