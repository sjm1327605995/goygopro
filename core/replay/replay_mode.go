package replay

import (
	"errors"
	"fmt"

	"github.com/sjm1327605995/goygopro/core/enginemsg"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

var (
	ErrReplayResponseUnderflow = errors.New("replay response stream underflow")
	ErrReplayDesynchronized    = errors.New("replay message stream desynchronized")
)

// ReplayMode contains the metadata and deterministic engine runner for one .yrp file.
type ReplayMode struct {
	Replay       *Replay
	Players      []string
	Decks        []DeckArray
	Params       DuelParameters
	IsTag        bool
	IsSingleMode bool
	ScriptName   string
}

func NewReplayMode() *ReplayMode { return &ReplayMode{} }

func (m *ReplayMode) Load(path string) error {
	r := NewReplay()
	if !r.OpenReplay(path) {
		return fmt.Errorf("open replay %q: invalid or unsupported replay file", path)
	}
	header := r.ReadHeader()
	m.Replay = r
	m.Players = append([]string(nil), r.Players()...)
	m.Decks = append([]DeckArray(nil), r.Decks()...)
	m.Params = r.Params()
	m.IsTag = header.Base.Flag&REPLAY_TAG != 0
	m.IsSingleMode = header.Base.Flag&REPLAY_SINGLE_MODE != 0
	m.ScriptName = r.ScriptName()
	return nil
}

func (m *ReplayMode) createDuel() (*ocgcore.Duel, error) {
	if m.Replay == nil {
		return nil, errors.New("replay is not loaded")
	}
	if ocgcore.API == nil {
		return nil, errors.New("ocgcore is not initialized")
	}
	if m.IsSingleMode {
		return nil, errors.New("single-mode replay playback is not supported by this runner")
	}
	header := m.Replay.ReadHeader()
	var duel *ocgcore.Duel
	if header.Base.ID == REPLAY_ID_YRP1 {
		duel = ocgcore.NewDuel(header.Base.Seed)
	} else {
		duel = ocgcore.NewDuelV2(header.SeedSequence)
	}
	if duel == nil {
		return nil, errors.New("create replay duel")
	}
	duel.InitPlayers(m.Params.StartLP, m.Params.StartHand, m.Params.DrawCount)
	m.Replay.SkipInfo()
	if err := m.loadDecks(duel); err != nil {
		duel.End()
		return nil, err
	}
	duel.Start(int32(m.Params.DuelFlag))
	return duel, nil
}

func (m *ReplayMode) loadDecks(duel *ocgcore.Duel) error {
	if !m.IsTag && len(m.Decks) < 2 {
		return errors.New("replay has fewer than two decks")
	}
	if m.IsTag && len(m.Decks) < 4 {
		return errors.New("tag replay has fewer than four decks")
	}
	for i, deck := range m.Decks {
		owner := uint8(0)
		if i >= 2 {
			owner = 1
		}
		for _, code := range deck.Main {
			if m.IsTag && i%2 == 1 {
				duel.AddTagCard(code, owner, ocgcore.LOCATION_DECK)
			} else {
				duel.AddCard(code, int(owner), ocgcore.LOCATION_DECK)
			}
		}
		for _, code := range deck.Extra {
			if m.IsTag && i%2 == 1 {
				duel.AddTagCard(code, owner, ocgcore.LOCATION_EXTRA)
			} else {
				duel.AddCard(code, int(owner), ocgcore.LOCATION_EXTRA)
			}
		}
	}
	return nil
}

// Run replays the engine stream and forwards each complete engine batch to emit.
// Responses are read from the recording rather than from UI callbacks.
func (m *ReplayMode) Run(emit func([]byte)) error {
	duel, err := m.createDuel()
	if err != nil {
		return err
	}
	defer duel.End()

	buf := make([]byte, ocgcore.SIZE_MESSAGE_BUFFER)
	for steps := 0; steps < 1000000; steps++ {
		result := duel.Process()
		engLen := int(result & ocgcore.PROCESSOR_BUFFER_LEN)
		engFlag := result & ocgcore.PROCESSOR_FLAG
		if engLen > len(buf) {
			buf = make([]byte, engLen)
		}
		responseOffset := -1
		walked := true
		if engLen > 0 {
			duel.GetMessage(buf)
			batch := buf[:engLen]
			if emit != nil {
				emit(batch)
			}
			responseOffset, walked = enginemsg.BatchResponseOffset(batch)
			if responseOffset == enginemsg.BatchWinOffset {
				return nil
			}
		}
		if responseOffset >= 0 || engFlag == ocgcore.PROCESSOR_WAITING || !walked && engLen > 0 {
			response := make([]byte, 64)
			if !m.Replay.ReadNextResponse(response) {
				return ErrReplayResponseUnderflow
			}
			if err := duel.SetResponseBytes(response); err != nil {
				return fmt.Errorf("set replay response: %w", err)
			}
		}
		if engFlag == ocgcore.PROCESSOR_END {
			return nil
		}
	}
	return ErrReplayDesynchronized
}
