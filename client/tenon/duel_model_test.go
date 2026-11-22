package tenon

import (
	"testing"

	"github.com/sjm1327605995/goygopro/protocol"
)

func TestDuelModelBuildsAndMovesRealCards(t *testing.T) {
	model := NewDuelModel(nil)
	model.Apply("duel:start", &protocol.StartMsg{PlayerType: 0, LP0: 8000, LP1: 8000, Deck0: 40, Deck1: 40})
	model.Apply("duel:reload_field", map[string]any{"players": []any{
		map[string]any{"lp": 8000, "deck": 40, "grave": 0, "removed": 0, "extra": 0},
		map[string]any{"lp": 500, "deck": 40, "grave": 0, "removed": 0, "extra": 0},
	}})
	model.Apply("duel:update_data", map[string]any{"player": 0, "location": locationHand, "cards": []any{
		map[string]any{"code": 89631139},
	}})
	model.Apply("duel:summoning", &protocol.SummoningMsg{Code: 89631139, CC: 0, CL: locationMZone, CS: 2, CP: 1})
	if card := model.Board[0].MZone[2]; card == nil || card.Code != 89631139 {
		t.Fatalf("summoned card = %+v", card)
	}
	model.Apply("duel:move", &protocol.MoveMsg{Code: 89631139, PC: 0, PL: locationMZone, PS: 2, PP: 1, CC: 0, CL: locationGrave, CS: 0, CP: 1})
	if model.Board[0].MZone[2] != nil || len(model.Board[0].Grave) != 1 {
		t.Fatal("move did not transfer the monster to graveyard")
	}
	model.Apply("duel:damage", &protocol.DamageMsg{Player: 1, Value: 3000})
	if model.LP[1] != 0 {
		t.Fatalf("opponent LP = %d, want 0", model.LP[1])
	}
	model.Apply("duel:new_turn", &protocol.NewTurnMsg{Player: 0})
	if model.Turn != 1 || model.TurnPlayer != 0 {
		t.Fatalf("turn = %d/%d", model.Turn, model.TurnPlayer)
	}
	if len(model.Hands[0]) != 1 || model.Hands[0][0] != 89631139 {
		t.Fatalf("hand = %v", model.Hands[0])
	}
}

func TestZonePositionsMatchOriginalGeometry(t *testing.T) {
	tests := []struct {
		seat     int
		location uint8
		sequence int
		x, z     float32
	}{
		{0, locationMZone, 0, -4.5, 2.8},
		{1, locationMZone, 0, 4.3, -2.8},
		{0, locationSZone, 5, -6.8, 4},
		{1, locationSZone, 7, -8.8, -1.4},
		{0, locationDeck, 0, 6.6, 6.6},
		{1, locationExtra, 0, 6.6, -6.6},
	}
	for _, test := range tests {
		point := zonePosition(test.seat, test.location, test.sequence)
		if point.X != test.x || point.Z != test.z {
			t.Errorf("zone(%d,%d,%d) = (%v,%v), want (%v,%v)", test.seat, test.location, test.sequence, point.X, point.Z, test.x, test.z)
		}
	}
}

func TestDuelModelRecognizesTypedNilEmptySlots(t *testing.T) {
	type query struct{ Code uint32 }
	var empty *query
	model := NewDuelModel(nil)
	model.Apply("duel:update_data", map[string]any{"player": 0, "location": locationMZone, "cards": []any{empty}})
	if model.Board[0].MZone[0] != nil {
		t.Fatal("typed nil empty slot created a card-back mesh")
	}
}

func TestDuelModelReadsPointerQueryCodes(t *testing.T) {
	code := uint32(89631139)
	model := NewDuelModel(nil)
	model.Apply("duel:update_data", map[string]any{"player": 0, "location": locationMZone, "cards": []any{
		map[string]any{"code": &code, "position": map[string]any{"p": 1}},
	}})
	if card := model.Board[0].MZone[0]; card == nil || card.Code != code {
		t.Fatalf("pointer query card = %+v", card)
	}
}
