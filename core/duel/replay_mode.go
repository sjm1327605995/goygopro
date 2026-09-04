package duel

import (
	"errors"
	"fmt"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

// Errors returned by ReplayMode.Run. Without the card scripts installed the
// engine cannot reproduce the recorded effect decisions, so the duel state
// diverges from the replay and one of these errors is reported instead of a
// silent hang or a corrupted result.
var (
	ErrReplayResponseUnderflow = errors.New("replay response underflow: replay is corrupt or truncated")
	ErrReplayDesynchronized    = errors.New("replay desynchronized: engine waiting with no message")
)

// ReplayDeck mirrors the deck lists decoded from a replay's info section.
type ReplayDeck struct {
	Main  []uint32
	Extra []uint32
}

// ReplayMode drives a recorded duel to completion against the ocgcore engine,
// feeding back the responses stored in the .yrp file. The engine regenerates
// every game message deterministically from the replay's seed; only the
// player responses are read back from disk on demand.
//
// This is the Go port of source/ygopro/gframe/replay_mode.cpp
// (ReplayThread / StartDuel / ReplayAnalyze / ReadReplayResponse).
type ReplayMode struct {
	Replay       *Replay
	Duel         *ocgcore.Duel
	Params       DuelParameters
	Players      []string
	Decks        []ReplayDeck
	ScriptName   string
	IsSingleMode bool
	IsTag        bool
}

func NewReplayMode() *ReplayMode {
	return &ReplayMode{}
}

// Load opens the replay and captures its metadata without running the duel.
func (rm *ReplayMode) Load(path string) error {
	rm.Replay = NewReplay()
	if !rm.Replay.OpenReplay(path) {
		return fmt.Errorf("open replay %s: invalid or unsupported replay file", path)
	}
	hdr := rm.Replay.ReadHeader()
	rm.Params = rm.Replay.params
	rm.Players = append([]string(nil), rm.Replay.players...)
	for _, d := range rm.Replay.decks {
		rm.Decks = append(rm.Decks, ReplayDeck{Main: append([]uint32(nil), d.Main...), Extra: append([]uint32(nil), d.Extra...)})
	}
	rm.ScriptName = rm.Replay.scriptName
	rm.IsSingleMode = hdr.Base.Flag&REPLAY_SINGLE_MODE != 0
	rm.IsTag = hdr.Base.Flag&REPLAY_TAG != 0
	return nil
}

// Run executes the duel, invoking handler for every engine message batch.
// The handler must be able to parse the raw MSG_* byte stream (e.g. the
// frontend message parser); it is called once per get_message batch.
func (rm *ReplayMode) Run(handler func(msg []byte)) error {
	if rm.Replay == nil {
		return fmt.Errorf("replay not loaded")
	}
	r := rm.Replay
	hdr := r.ReadHeader()

	// The replay body begins after the info section; responses are read
	// from there in lockstep with the engine's select messages.
	r.SkipInfo()

	var d *ocgcore.Duel
	if hdr.Base.ID == REPLAY_ID_YRP1 {
		d = ocgcore.NewDuel(hdr.Base.Seed)
	} else {
		d = ocgcore.NewDuelV2(hdr.SeedSequence)
	}
	if d == nil {
		return fmt.Errorf("create duel: ocgcore returned nil")
	}
	rm.Duel = d
	defer func() {
		d.End()
		rm.Duel = nil
	}()

	d.InitPlayers(r.params.StartLP, r.params.StartHand, r.params.DrawCount)

	if rm.IsSingleMode {
		filename := "./single/" + r.scriptName
		if ocgcore.API.PreloadScript(d.GetNativePtr(), filename, int32(len(filename))) == 0 {
			return fmt.Errorf("preload script %s: failed", filename)
		}
	} else {
		rm.loadDecks(d)
	}

	d.Start(int32(r.params.DuelFlag))

	buf := make([]byte, ocgcore.SIZE_MESSAGE_BUFFER)
	var engFlag uint32
	for engFlag != ocgcore.PROCESSOR_END {
		result := d.Process()
		engLen := int(result & ocgcore.PROCESSOR_BUFFER_LEN)
		engFlag = result & ocgcore.PROCESSOR_FLAG
		if engLen > 0 {
			if engLen > len(buf) {
				buf = make([]byte, engLen)
			}
			n := int(d.GetMessage(buf))
			if handler != nil {
				handler(buf[:n])
			}
			if engFlag == ocgcore.PROCESSOR_WAITING {
				if !rm.readResponse(d) {
					return ErrReplayResponseUnderflow
				}
			}
		} else if engFlag == ocgcore.PROCESSOR_WAITING {
			// Engine is waiting for a response but produced no message;
			// the replay no longer matches the engine state.
			return ErrReplayDesynchronized
		}
	}
	return nil
}

// loadDecks reconstructs the core duel's deck from the replay's recorded
// deck lists, mirroring the deck layout logic of the C++ StartDuel.
func (rm *ReplayMode) loadDecks(d *ocgcore.Duel) {
	r := rm.Replay
	if !rm.IsTag {
		for p := 0; p < 2; p++ {
			for _, code := range r.decks[p].Main {
				d.AddCard(code, p, ocgcore.LOCATION_DECK)
			}
			for _, code := range r.decks[p].Extra {
				d.AddCard(code, p, ocgcore.LOCATION_EXTRA)
			}
		}
		return
	}
	// Tag duel: decks[0] + decks[2] belong to the leading players,
	// decks[1] + decks[3] are their tag partners.
	for _, code := range r.decks[0].Main {
		d.AddCard(code, 0, ocgcore.LOCATION_DECK)
	}
	for _, code := range r.decks[0].Extra {
		d.AddCard(code, 0, ocgcore.LOCATION_EXTRA)
	}
	for _, code := range r.decks[1].Main {
		d.AddTagCard(code, 0, ocgcore.LOCATION_DECK)
	}
	for _, code := range r.decks[1].Extra {
		d.AddTagCard(code, 0, ocgcore.LOCATION_EXTRA)
	}
	for _, code := range r.decks[2].Main {
		d.AddCard(code, 1, ocgcore.LOCATION_DECK)
	}
	for _, code := range r.decks[2].Extra {
		d.AddCard(code, 1, ocgcore.LOCATION_EXTRA)
	}
	for _, code := range r.decks[3].Main {
		d.AddTagCard(code, 1, ocgcore.LOCATION_DECK)
	}
	for _, code := range r.decks[3].Extra {
		d.AddTagCard(code, 1, ocgcore.LOCATION_EXTRA)
	}
}

// readResponse reads the next length-prefixed recorded response and feeds it
// back to the engine, exactly as ReplayMode::ReadReplayResponse does.
func (rm *ReplayMode) readResponse(d *ocgcore.Duel) bool {
	var resp [ocgcore.SIZE_RETURN_VALUE]byte
	if !rm.Replay.ReadNextResponse(resp[:]) {
		return false
	}
	_ = d.SetResponseBytes(resp[:])
	return true
}
