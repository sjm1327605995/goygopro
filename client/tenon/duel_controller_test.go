package tenon

import (
	"reflect"
	"testing"
)

type responseCall struct {
	battle  bool
	index   int32
	command int32
}

type placeCall struct {
	multi  bool
	values []int32
}

type responseRecorder struct {
	calls         []responseCall
	placeCalls    []placeCall
	cardCalls     [][]int32
	unselectCalls []int32
	scalarCalls   []int32
	desc          map[uint32]string
}

func (r *responseRecorder) RespondIdleCmd(index, command int32) {
	r.calls = append(r.calls, responseCall{index: index, command: command})
}

func (r *responseRecorder) RespondBattleCmd(index, command int32) {
	r.calls = append(r.calls, responseCall{battle: true, index: index, command: command})
}

func (r *responseRecorder) ResolveDesc(desc uint32) map[string]any {
	return map[string]any{"text": r.desc[desc]}
}

func (r *responseRecorder) RespondSelectPlace(player, location, sequence int32) {
	r.placeCalls = append(r.placeCalls, placeCall{values: []int32{player, location, sequence}})
}

func (r *responseRecorder) RespondSelectPlaces(places []int32) {
	r.placeCalls = append(r.placeCalls, placeCall{multi: true, values: append([]int32(nil), places...)})
}

func (r *responseRecorder) RespondSelectCard(indices []int32) {
	r.cardCalls = append(r.cardCalls, append([]int32(nil), indices...))
}

func (r *responseRecorder) RespondSelectUnselect(index int32) {
	r.unselectCalls = append(r.unselectCalls, index)
}

func (r *responseRecorder) SendResponseI(value int32) {
	r.scalarCalls = append(r.scalarCalls, value)
}

func TestDuelControllerHandOptionsUseEngineIndices(t *testing.T) {
	responses := &responseRecorder{desc: map[uint32]string{81: "抽一张卡", 82: "回复生命"}}
	model := NewDuelModel(nil)
	model.Apply("duel:select_idlecmd", map[string]any{
		"summon": []any{map[string]any{"code": 89631139, "l": locationHand, "s": 1, "idx": 4}},
		"activate": []any{
			map[string]any{"code": 46986414, "l": locationHand, "s": 0, "idx": 7, "desc": 81},
			map[string]any{"code": 46986414, "l": locationHand, "s": 0, "idx": 9, "desc": 82},
			map[string]any{"code": 89631139, "l": locationMZone, "s": 1, "idx": 11},
		},
		"toBP": true, "toEP": true, "shuffle": true,
	})
	controller := NewDuelController(responses, model)

	options := controller.HandOptions(46986414, 0)
	if len(options) != 2 || options[0].Index != 7 || options[1].Index != 9 {
		t.Fatalf("activation options = %+v", options)
	}
	if options[0].Label != "发动：抽一张卡" || options[1].Label != "发动：回复生命" {
		t.Fatalf("activation labels = %+v", options)
	}
	if options := controller.HandOptions(89631139, 1); len(options) != 1 || options[0].Kind != "summon" || options[0].Index != 4 {
		t.Fatalf("summon options = %+v", options)
	}
	if options := controller.HandOptions(89631139, 0); len(options) != 0 {
		t.Fatalf("unoffered sequence received actions = %+v", options)
	}

	if !controller.Execute(optionsForTest(options, 9)) {
		t.Fatal("valid activation was rejected")
	}
	assertResponseCall(t, responses.calls, responseCall{index: 9, command: 5})
	if controller.Execute(ActionOption{Kind: "summon"}) {
		t.Fatal("responded twice to an already consumed prompt")
	}
}

func optionsForTest(options []ActionOption, index int) ActionOption {
	for _, option := range options {
		if option.Index == index {
			return option
		}
	}
	return ActionOption{}
}

func TestDuelControllerBattleCommands(t *testing.T) {
	responses := &responseRecorder{}
	model := NewDuelModel(nil)
	controller := NewDuelController(responses, model)
	model.Apply("duel:select_battlecmd", map[string]any{
		"activate": []any{map[string]any{"code": 46986414, "c": 0, "l": locationHand, "s": 0, "idx": 3}},
		"attack":   []any{map[string]any{"code": 89631139, "c": 0, "l": locationMZone, "s": 2, "idx": 6}},
		"toM2":     true, "toEP": true,
	})

	activations := controller.HandOptions(46986414, 0)
	if len(activations) != 1 || !controller.Execute(activations[0]) {
		t.Fatalf("battle activation options = %+v", activations)
	}
	assertResponseCall(t, responses.calls, responseCall{battle: true, index: 3, command: 0})

	model.Apply("duel:select_battlecmd", map[string]any{
		"attack": []any{map[string]any{"code": 89631139, "c": 0, "l": locationMZone, "s": 2, "idx": 6}},
		"toM2":   true, "toEP": true,
	})
	attacks := controller.FieldOptions(89631139, 0, locationMZone, 2)
	if len(attacks) != 1 || attacks[0].Kind != "attack" || !controller.Execute(attacks[0]) {
		t.Fatalf("attack options = %+v", attacks)
	}
	assertResponseCall(t, responses.calls[1:], responseCall{battle: true, index: 6, command: 1})
	if options := controller.FieldOptions(89631139, 1, locationMZone, 2); len(options) != 0 {
		t.Fatalf("opponent card received local commands = %+v", options)
	}
}

func TestDuelControllerPhaseGuards(t *testing.T) {
	responses := &responseRecorder{}
	model := NewDuelModel(nil)
	controller := NewDuelController(responses, model)
	if controller.ToBattle() || controller.ToMain2() || controller.ToEnd() || controller.ShuffleHand() {
		t.Fatal("phase action ran without a prompt")
	}
	model.Apply("duel:select_idlecmd", map[string]any{"toBP": true, "toEP": true, "shuffle": true})
	if !controller.ToBattle() {
		t.Fatal("available battle phase was rejected")
	}
	assertResponseCall(t, responses.calls, responseCall{command: 6})

	model.Apply("duel:select_battlecmd", map[string]any{"toM2": true, "toEP": true})
	if !controller.ToMain2() {
		t.Fatal("available main phase 2 was rejected")
	}
	assertResponseCall(t, responses.calls[1:], responseCall{battle: true, command: 2})
}

func TestDuelControllerChoosePlace(t *testing.T) {
	responses := &responseRecorder{}
	model := NewDuelModel(nil)
	controller := NewDuelController(responses, model)
	model.Apply("duel:select_place", map[string]any{
		"player": 0, "count": 1,
		"zones": []any{map[string]any{"player": 0, "loc": locationMZone, "seq": 2}},
	})
	if model.Placement == nil || model.Placement.Cancelable {
		t.Fatalf("place prompt = %+v", model.Placement)
	}
	if controller.ChoosePlace(1, locationMZone, 2) || len(responses.placeCalls) != 0 {
		t.Fatal("unoffered zone triggered a response")
	}
	if !controller.ChoosePlace(0, locationMZone, 2) {
		t.Fatal("offered zone was rejected")
	}
	want := placeCall{values: []int32{0, locationMZone, 2}}
	if len(responses.placeCalls) != 1 || !reflect.DeepEqual(responses.placeCalls[0], want) || model.Placement != nil {
		t.Fatalf("place response = %+v, prompt=%+v", responses.placeCalls, model.Placement)
	}
	if controller.ChoosePlace(0, locationMZone, 2) {
		t.Fatal("completed place prompt accepted another response")
	}
}

func TestDuelControllerMultiPlaceOrderingAndUndo(t *testing.T) {
	responses := &responseRecorder{}
	model := NewDuelModel(nil)
	controller := NewDuelController(responses, model)
	model.Apply("duel:select_place", map[string]any{
		"player": 0, "count": 3,
		"zones": []any{
			map[string]any{"player": 1, "loc": locationMZone, "seq": 3},
			map[string]any{"player": 0, "loc": locationSZone, "seq": 2},
			map[string]any{"player": 0, "loc": locationMZone, "seq": 1},
		},
	})
	if !controller.ChoosePlace(1, locationMZone, 3) || !controller.CancelOrUndoPlace() {
		t.Fatal("select/undo did not run")
	}
	if len(model.Placement.Selected) != 0 || len(responses.placeCalls) != 0 {
		t.Fatal("undo left a selection or sent an early response")
	}
	controller.ChoosePlace(1, locationMZone, 3)
	controller.ChoosePlace(0, locationSZone, 2)
	controller.ChoosePlace(0, locationMZone, 1)
	want := placeCall{multi: true, values: []int32{0, locationMZone, 1, 0, locationSZone, 2, 1, locationMZone, 3}}
	if len(responses.placeCalls) != 1 || !reflect.DeepEqual(responses.placeCalls[0], want) {
		t.Fatalf("multi-place response = %+v, want %+v", responses.placeCalls, want)
	}
}

func TestDuelControllerCancelablePlace(t *testing.T) {
	responses := &responseRecorder{}
	model := NewDuelModel(nil)
	controller := NewDuelController(responses, model)
	model.Apply("duel:select_place", map[string]any{
		"player": 0, "count": 0,
		"zones": []any{map[string]any{"player": 0, "loc": locationMZone, "seq": 0}},
	})
	if !controller.CancelOrUndoPlace() {
		t.Fatal("cancelable place prompt was not canceled")
	}
	want := placeCall{values: []int32{0, 0, 0}}
	if len(responses.placeCalls) != 1 || !reflect.DeepEqual(responses.placeCalls[0], want) {
		t.Fatalf("cancel response = %+v", responses.placeCalls)
	}
}

func TestDuelControllerCardSelectionFinishAndToggle(t *testing.T) {
	responses := &responseRecorder{}
	model := NewDuelModel(nil)
	controller := NewDuelController(responses, model)
	model.Apply("duel:select_card", map[string]any{
		"player": 0, "min": 1, "max": 2,
		"cards": []any{
			map[string]any{"code": 89631139, "c": 1, "l": locationMZone, "s": 2},
			map[string]any{"code": 46986414, "c": 0, "l": locationHand, "s": 0},
			map[string]any{"code": 24094653, "c": 0, "l": locationMZone, "s": 1},
		},
	})
	if !controller.ChooseCard(24094653, 0, locationMZone, 1) || len(responses.cardCalls) != 0 {
		t.Fatal("multi-card selection submitted before reaching max")
	}
	if !controller.ChooseCard(24094653, 0, locationMZone, 1) || len(model.CardSelection.Selected) != 0 {
		t.Fatal("clicking a selected card did not toggle it off")
	}
	controller.ChooseCard(24094653, 0, locationMZone, 1)
	controller.ChooseCard(89631139, 1, locationMZone, 2)
	if !reflect.DeepEqual(responses.cardCalls, [][]int32{{2, 0}}) || model.CardSelection != nil {
		t.Fatalf("card response = %+v", responses.cardCalls)
	}
}

func TestDuelControllerCardSelectionConfirmAndCancel(t *testing.T) {
	responses := &responseRecorder{}
	model := NewDuelModel(nil)
	controller := NewDuelController(responses, model)
	model.Apply("duel:select_card", map[string]any{
		"player": 0, "min": 1, "max": 2, "cancelable": true,
		"cards": []any{map[string]any{"code": 89631139, "c": 1, "l": locationMZone, "s": 2}},
	})
	controller.ChooseCard(89631139, 1, locationMZone, 2)
	if !controller.CancelOrUndoCardSelection() || !reflect.DeepEqual(responses.cardCalls, [][]int32{{0}}) {
		t.Fatal("right-click on a ready selection did not finish it")
	}
	model.Apply("duel:select_card", map[string]any{
		"player": 0, "min": 1, "max": 1, "cancelable": true,
		"cards": []any{map[string]any{"code": 89631139, "c": 1, "l": locationMZone, "s": 2}},
	})
	if !controller.CancelOrUndoCardSelection() || !reflect.DeepEqual(responses.scalarCalls, []int32{-1}) {
		t.Fatalf("cancel response = %+v", responses.scalarCalls)
	}
}

func TestDuelControllerSelectUnselectUsesCombinedIndex(t *testing.T) {
	responses := &responseRecorder{}
	model := NewDuelModel(nil)
	controller := NewDuelController(responses, model)
	model.Apply("duel:select_unselect", map[string]any{
		"player": 0,
		"cards": []any{
			map[string]any{"code": 89631139, "c": 0, "l": locationMZone, "s": 0},
			map[string]any{"code": 46986414, "c": 0, "l": locationMZone, "s": 1},
		},
		"unselectList": []any{map[string]any{"code": 24094653, "c": 1, "l": locationMZone, "s": 2}},
	})
	if model.CardSelection == nil || model.CardSelection.Min != 1 || model.CardSelection.Max != 1 {
		t.Fatalf("unselect prompt = %+v", model.CardSelection)
	}
	if !controller.ChooseCard(24094653, 1, locationMZone, 2) || !reflect.DeepEqual(responses.unselectCalls, []int32{2}) {
		t.Fatalf("unselect response = %+v", responses.unselectCalls)
	}
}

func assertResponseCall(t *testing.T, calls []responseCall, want responseCall) {
	t.Helper()
	if len(calls) != 1 || calls[0] != want {
		t.Fatalf("response calls = %+v, want %+v", calls, want)
	}
}
