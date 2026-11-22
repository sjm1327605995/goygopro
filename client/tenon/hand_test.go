package tenon

import (
	"math"
	"testing"

	"github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

func TestArenaHandLayoutMatchesOriginalFormula(t *testing.T) {
	pose := arenaHandLayout(0, 5, 784, 462, true, 0, 0)
	height := float32(462)
	width := float32(784)
	cardHeight := min(float32(160), height*0.205)
	cardWidth := cardHeight * 0.7
	spread := min(cardWidth*0.74, (width*0.76-cardWidth)/4)
	if pose.x != float32(392)-2*spread || pose.y != float32(462)-cardHeight*0.6-12 {
		t.Fatalf("first card position = (%v,%v)", pose.x, pose.y)
	}
	if pose.width != float32(cardWidth) || pose.height != float32(cardHeight) || pose.angle != 0.13 {
		t.Fatalf("first card pose = %+v", pose)
	}
}

func TestHandFanHoverAndSelectUseSequence(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	var selected HandSelection
	var hovered uint32
	fan := HandFanProps{
		Service: app.NewApp(), Cards: []uint32{89631139, 46986414}, Width: 784, Height: 462, Near: true,
		OnSelect: func(selection HandSelection) { selected = selection },
		OnHover:  func(code uint32) { hovered = code },
	}
	h := ui.Mount(ui.Div(ui.Style(ui.Fill), ui.Use(HandFan, fan)), 800, 600)
	defer h.Close()
	cards := h.Root().Clickables()
	if len(cards) != 2 {
		t.Fatalf("clickable hand cards = %d, want 2", len(cards))
	}
	cards[1].Hover(true)
	if hovered != 46986414 {
		t.Fatalf("hovered code = %d", hovered)
	}
	if !cards[1].Click() || selected.Sequence != 1 || selected.Code != 46986414 {
		t.Fatalf("selected = %+v", selected)
	}
	if math.Abs(float64(arenaHandLayout(1, 2, 784, 462, true, 1, 0).angle)) > 0.0001 {
		t.Fatal("hovered card did not straighten")
	}
}

func TestHandCardPointerLeaveRestoresUnselectedCard(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	h := ui.Mount(ui.Div(ui.Style(ui.Fill), ui.Use(HandCard, handCardProps{
		service: app.NewApp(), code: 89631139, count: 2, width: 784, height: 462, near: true,
	})), 800, 600)
	defer h.Close()

	initial := h.Root().Clickables()[0].Bounds()
	h.MoveTo(initial.X+initial.W/2, initial.Y+initial.H/2)
	h.Step(110)
	h.MoveTo(10, 10)
	h.Step(110)
	if restored := h.Root().Clickables()[0].Bounds(); restored != initial {
		t.Fatalf("pointer leave did not restore card: initial=%+v restored=%+v", initial, restored)
	}
}

func TestHandFanPointerLeaveRestoresUnselectedCard(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	h := ui.Mount(ui.Div(ui.Style(ui.Fill), ui.Use(HandFan, HandFanProps{
		Service: app.NewApp(), Cards: []uint32{89631139, 46986414}, Width: 784, Height: 462, Near: true,
	})), 800, 600)
	defer h.Close()

	initial := h.Root().Clickables()[0].Bounds()
	h.MoveTo(initial.X+initial.W/2, initial.Y+initial.H/2)
	h.Step(110)
	lifted := h.Root().Clickables()[0].Bounds()
	if lifted.W <= initial.W || lifted.Y >= initial.Y {
		t.Fatalf("hover did not enlarge and lift card: initial=%+v lifted=%+v", initial, lifted)
	}

	h.MoveTo(10, 10)
	h.Step(110)
	restored := h.Root().Clickables()[0].Bounds()
	if restored != initial {
		t.Fatalf("pointer leave did not restore card: initial=%+v restored=%+v", initial, restored)
	}
}

func TestHandFanPointerSwitchesBetweenCards(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	h := ui.Mount(ui.Div(ui.Style(ui.Fill), ui.Use(HandFan, HandFanProps{
		Service: app.NewApp(), Cards: []uint32{89631139, 46986414, 24094653}, Width: 784, Height: 462, Near: true,
	})), 800, 600)
	defer h.Close()

	cards := h.Root().Clickables()
	initial := []ui.Rect{cards[0].Bounds(), cards[1].Bounds(), cards[2].Bounds()}
	h.MoveTo(initial[0].X+initial[0].W/2, initial[0].Y+initial[0].H/2)
	h.Step(110)
	h.MoveTo(initial[1].X+initial[1].W/2, initial[1].Y+initial[1].H/2)
	h.Step(110)

	cards = h.Root().Clickables()
	if cards[0].Bounds() != initial[0] {
		t.Fatalf("previous card remained hovered: initial=%+v actual=%+v", initial[0], cards[0].Bounds())
	}
	if cards[1].Bounds().W <= initial[1].W || cards[1].Bounds().Y >= initial[1].Y {
		t.Fatalf("next card did not become hovered: initial=%+v actual=%+v", initial[1], cards[1].Bounds())
	}
	if cards[2].Bounds() != initial[2] {
		t.Fatalf("unrelated card moved: initial=%+v actual=%+v", initial[2], cards[2].Bounds())
	}
}

func TestHandFanClickedCardRestoresAfterPointerLeave(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	h := ui.Mount(ui.Div(ui.Style(ui.Fill), ui.Use(HandFan, HandFanProps{
		Service: app.NewApp(), Cards: []uint32{89631139, 46986414}, Width: 784, Height: 462, Near: true,
	})), 800, 600)
	defer h.Close()

	card := h.Root().Clickables()[0]
	initial := card.Bounds()
	if !card.Click() {
		t.Fatal("card click did not fire")
	}
	h.MoveTo(10, 10)
	h.Step(110)
	restored := h.Root().Clickables()[0].Bounds()
	if restored != initial {
		t.Fatalf("clicked card remained raised after pointer leave: initial=%+v restored=%+v", initial, restored)
	}
}

func TestDuelHUDHandHoverRestoresAtNativeWindowSize(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	desktop := NewDesktop(app.NewApp())
	desktop.page.Set(pageDuel)
	desktop.duelModel.Apply("duel:update_data", map[string]any{
		"player": 0, "location": locationHand, "cards": []any{
			map[string]any{"code": 89631139}, map[string]any{"code": 46986414},
		},
	})
	h := ui.Mount(desktop.Root(), 1280, 720)
	defer h.Close()
	clickables := h.Root().Clickables()
	card := clickables[len(clickables)-1]
	initial := card.Bounds()
	toScreen := func(bounds ui.Rect) (float32, float32) {
		return 640 + (bounds.X+bounds.W/2-640)*1.25, 360 + (bounds.Y+bounds.H/2-360)*1.125
	}

	x, y := toScreen(initial)
	h.MoveTo(x, y)
	h.Step(110)
	lifted := h.Root().Clickables()[len(clickables)-1].Bounds()
	if lifted.W <= initial.W || lifted.Y >= initial.Y {
		t.Fatalf("native-window hover did not lift card: initial=%+v lifted=%+v", initial, lifted)
	}
	h.MoveTo(5, 5)
	h.Step(110)
	restored := h.Root().Clickables()[len(clickables)-1].Bounds()
	if restored != initial {
		t.Fatalf("native-window pointer leave did not restore card: initial=%+v restored=%+v", initial, restored)
	}
}

func TestHandFanDisablesFarHoverAndUnknownSelection(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	hovered := false
	far := HandFanProps{Service: app.NewApp(), Cards: []uint32{89631139}, Width: 784, Height: 462, OnHover: func(uint32) { hovered = true }}
	h := ui.Mount(ui.Div(ui.Style(ui.Fill), ui.Use(HandFan, far)), 800, 600)
	h.Root().Clickables()[0].Hover(true)
	if hovered {
		t.Fatal("far hand responded to hover")
	}
	h.Close()

	selected := false
	unknown := HandFanProps{Service: app.NewApp(), Cards: []uint32{0}, Width: 784, Height: 462, Near: true, OnSelect: func(HandSelection) { selected = true }}
	h = ui.Mount(ui.Div(ui.Style(ui.Fill), ui.Use(HandFan, unknown)), 800, 600)
	defer h.Close()
	h.Root().Clickables()[0].Click()
	if selected {
		t.Fatal("unknown card triggered selection")
	}
}
