package tenon

import (
	"testing"

	appservice "github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

func TestHostSelectionsBuildActualRequest(t *testing.T) {
	desktop := NewDesktop(appservice.NewApp())
	desktop.hostCardRule.Set("5")
	desktop.hostMode.Set("2")
	desktop.hostDuelRule.Set("4")
	request := desktop.hostInfoRequest()
	if request.Rule != 5 || request.Mode != 2 || request.DuelRule != 4 || request.StartLp != 8000 || request.StartHand != 5 || request.DrawCount != 1 || request.TimeLimit != 180 {
		t.Fatalf("host request = %+v", request)
	}
}

func TestDeckSelectionsBuildCardFilter(t *testing.T) {
	desktop := NewDesktop(appservice.NewApp())
	desktop.filterKind.Set("1")
	desktop.filterSubtype.Set("32")
	desktop.filterAttribute.Set("16")
	desktop.filterRace.Set("8192")
	desktop.filterAttack.Set(">=2000")
	filter := desktop.currentCardFilter()
	if filter.Type != 33 || filter.Attribute != 16 || filter.Race != 8192 || filter.AtkFilter != ">=2000" {
		t.Fatalf("card filter = %+v", filter)
	}
}

func TestLobbyHostControlsUpdateRequest(t *testing.T) {
	desktop := NewDesktop(appservice.NewApp())
	desktop.page.Set(pageLobby)
	desktop.createOpen.Set(true)
	h := ui.Mount(desktop.Root(), 1024, 640)
	defer h.Close()
	if !h.Root().ByText("OCG").Click() || !h.Overlays()[0].ByText("TCG").Click() {
		t.Fatal("card-rule selection did not fire")
	}
	if !h.Root().ByText("单局模式").Click() || !h.Overlays()[0].ByText("TAG").Click() {
		t.Fatal("duel-mode selection did not fire")
	}
	request := desktop.hostInfoRequest()
	if request.Rule != 1 || request.Mode != 2 {
		t.Fatalf("request after UI selection = %+v", request)
	}
}
