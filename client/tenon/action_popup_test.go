package tenon

import (
	"reflect"
	"testing"

	"github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

func TestActionPopupLayoutFlipsAtViewportEdges(t *testing.T) {
	x, y, width, height := actionPopupLayout(1000, 620, 2, 1024, 640)
	if x < 8 || y < 8 || x+width > 1016 || y+height > 632 {
		t.Fatalf("popup escaped viewport: x=%v y=%v w=%v h=%v", x, y, width, height)
	}
	if x >= 1000 || y >= 620 {
		t.Fatalf("popup did not flip to the available side: x=%v y=%v", x, y)
	}
}

func TestActionPopupSelectAndEscape(t *testing.T) {
	selected := -1
	closed := false
	options := []ActionOption{{Label: "召唤", Kind: "summon", Index: 4}, {Label: "盖放", Kind: "mset", Index: 7}}
	h := ui.Mount(ui.Use(ActionPopup, ActionPopupProps{
		X: 300, Y: 300, Options: options,
		OnSelect: func(option ActionOption) { selected = option.Index },
		OnClose:  func() { closed = true },
	}), 1024, 640)
	defer h.Close()

	if !h.Root().ByText("召唤").Click() || selected != 4 {
		t.Fatalf("selected action index = %d, want 4", selected)
	}
	h.Escape()
	if !closed {
		t.Fatal("Escape did not close the action popup")
	}
}

func TestDuelHUDHandActionSendsProtocolResponse(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	desktop := NewDesktop(app.NewApp())
	responses := &responseRecorder{}
	desktop.duelController = NewDuelController(responses, desktop.duelModel)
	desktop.page.Set(pageDuel)
	desktop.duelModel.Apply("duel:update_data", map[string]any{
		"player": 0, "location": locationHand, "cards": []any{map[string]any{"code": 89631139}},
	})
	desktop.duelModel.Apply("duel:select_idlecmd", map[string]any{
		"summon": []any{map[string]any{"code": 89631139, "l": locationHand, "s": 0, "idx": 4}},
	})
	h := ui.Mount(desktop.Root(), 1024, 640)
	defer h.Close()

	clickables := h.Root().Clickables()
	if len(clickables) == 0 || !clickables[len(clickables)-1].Click() {
		t.Fatal("hand card click did not fire")
	}
	if !h.Root().ByText("召唤").Exists() {
		t.Fatal("hand card did not open its offered action")
	}
	if !h.Root().ByText("召唤").Click() {
		t.Fatal("action popup item did not fire")
	}
	assertResponseCall(t, responses.calls, responseCall{index: 4, command: 0})
	if h.Root().ByText("召唤").Exists() {
		t.Fatal("action popup remained open after executing")
	}
}

func TestDuelHUDFieldActionSendsProtocolResponse(t *testing.T) {
	desktop := NewDesktop(app.NewApp())
	responses := &responseRecorder{}
	desktop.duelController = NewDuelController(responses, desktop.duelModel)
	desktop.page.Set(pageDuel)
	desktop.duelModel.Apply("duel:summoning", map[string]any{"code": 89631139, "cc": 0, "cl": locationMZone, "cs": 2, "cp": 1})
	desktop.duelModel.Apply("duel:select_battlecmd", map[string]any{
		"attack": []any{map[string]any{"code": 89631139, "c": 0, "l": locationMZone, "s": 2, "idx": 6}},
	})
	h := ui.Mount(desktop.Root(), 1024, 640)
	defer h.Close()
	desktop.selectFieldTarget(&FieldHit{Code: 89631139, Seat: 0, Location: locationMZone, Sequence: 2, X: 500, Y: 300})
	h.Step(0)

	if !h.Root().ByText("攻击").Click() {
		t.Fatal("field attack action did not fire")
	}
	assertResponseCall(t, responses.calls, responseCall{battle: true, index: 6, command: 1})
}

func TestDuelHUDPlaceSelectionSendsProtocolResponse(t *testing.T) {
	desktop := NewDesktop(app.NewApp())
	responses := &responseRecorder{}
	desktop.duelController = NewDuelController(responses, desktop.duelModel)
	desktop.page.Set(pageDuel)
	desktop.duelModel.Apply("duel:select_place", map[string]any{
		"player": 0, "count": 1,
		"zones": []any{map[string]any{"player": 0, "loc": locationMZone, "seq": 2}},
	})
	h := ui.Mount(desktop.Root(), 1024, 640)
	defer h.Close()
	desktop.selectFieldTarget(&FieldHit{Seat: 0, Location: locationMZone, Sequence: 2})
	h.Step(0)
	want := placeCall{values: []int32{0, locationMZone, 2}}
	if len(responses.placeCalls) != 1 || !reflect.DeepEqual(responses.placeCalls[0], want) || desktop.duelModel.Placement != nil {
		t.Fatalf("place response = %+v, prompt=%+v", responses.placeCalls, desktop.duelModel.Placement)
	}
}

func TestDuelHUDHandCandidateSendsSelectionResponse(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	desktop := NewDesktop(app.NewApp())
	responses := &responseRecorder{}
	desktop.duelController = NewDuelController(responses, desktop.duelModel)
	desktop.page.Set(pageDuel)
	desktop.duelModel.Apply("duel:update_data", map[string]any{
		"player": 0, "location": locationHand, "cards": []any{map[string]any{"code": 46986414}},
	})
	desktop.duelModel.Apply("duel:select_card", map[string]any{
		"player": 0, "min": 1, "max": 1,
		"cards": []any{map[string]any{"code": 46986414, "c": 0, "l": locationHand, "s": 0}},
	})
	h := ui.Mount(desktop.Root(), 1024, 640)
	defer h.Close()
	clickables := h.Root().Clickables()
	if len(clickables) == 0 || !clickables[len(clickables)-1].Click() {
		t.Fatal("hand candidate click did not fire")
	}
	if !reflect.DeepEqual(responses.cardCalls, [][]int32{{0}}) || desktop.duelModel.CardSelection != nil {
		t.Fatalf("hand selection response = %+v", responses.cardCalls)
	}
}

func TestDuelHUDPhaseButtonSendsProtocolResponse(t *testing.T) {
	desktop := NewDesktop(app.NewApp())
	responses := &responseRecorder{}
	desktop.duelController = NewDuelController(responses, desktop.duelModel)
	desktop.page.Set(pageDuel)
	desktop.duelModel.Apply("duel:select_idlecmd", map[string]any{"toBP": true})
	h := ui.Mount(desktop.Root(), 1024, 640)
	defer h.Close()

	if !h.Root().ByText("战斗阶段").Click() {
		t.Fatal("battle phase button did not fire")
	}
	assertResponseCall(t, responses.calls, responseCall{command: 6})
}
