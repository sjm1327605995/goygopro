package tenon

import (
	"testing"

	appservice "github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

func TestDeckEditorButtonsAddAndRemove(t *testing.T) {
	desktop := NewDesktop(appservice.NewApp())
	desktop.page.Set(pageDeck)
	h := ui.Mount(desktop.Root(), 1024, 640)
	defer h.Close()
	if !h.Root().ByText("+主").Click() || len(desktop.deck.Get().Main) != 1 {
		t.Fatal("add-main button did not update the deck")
	}
	if !h.Root().ByText("+副").Click() || len(desktop.deck.Get().Side) != 1 {
		t.Fatal("add-side button did not update the deck")
	}
	if !h.Root().ByText("移除").Click() || len(desktop.deck.Get().Main) != 0 || len(desktop.deck.Get().Side) != 1 {
		t.Fatalf("remove button produced %+v", desktop.deck.Get())
	}
}

func TestDeckEditAddRoutesExtraAndLimitsCopies(t *testing.T) {
	deck := appservice.DeckData{}
	updated, message := addDeckCard(deck, 100, 0x40, deckTargetMain)
	if len(updated.Extra) != 1 || len(updated.Main) != 0 || message != "已加入额外卡组" {
		t.Fatalf("extra redirect = %+v, %q", updated, message)
	}
	if _, message := addDeckCard(deck, 101, 0x1, deckTargetExtra); message != "这张卡不属于额外卡组" {
		t.Fatalf("non-extra validation = %q", message)
	}
	deck.Main = []uint32{100, 100}
	deck.Side = []uint32{100}
	before := append([]uint32(nil), deck.Main...)
	updated, message = addDeckCard(deck, 100, 0x1, deckTargetMain)
	if message != "同名卡最多加入 3 张" || len(updated.Main) != 2 {
		t.Fatalf("copy limit = %+v, %q", updated, message)
	}
	if len(before) != 2 || len(deck.Main) != 2 {
		t.Fatal("failed edit mutated the source deck")
	}
}

func TestDeckEditCapacityAndRemove(t *testing.T) {
	deck := appservice.DeckData{Main: make([]uint32, 60)}
	if _, message := addDeckCard(deck, 100, 0x1, deckTargetMain); message != "主卡组最多 60 张" {
		t.Fatalf("main capacity = %q", message)
	}
	deck = appservice.DeckData{Main: []uint32{100}, Extra: []uint32{200}, Side: []uint32{300}}
	updated, message := removeDeckCard(deck, 200, deckTargetExtra)
	if message != "已移除 1 张卡片" || len(updated.Extra) != 0 || len(deck.Extra) != 1 {
		t.Fatalf("extra remove = %+v, %q", updated, message)
	}
	updated, _ = removeDeckCard(deck, 300, deckTargetAny)
	if len(updated.Side) != 0 {
		t.Fatalf("any-zone remove = %+v", updated)
	}
}
