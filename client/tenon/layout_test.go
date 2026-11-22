package tenon

import (
	"math"
	"strings"
	"testing"

	"github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

func TestMenuTextUsesOriginalDesignCoordinates(t *testing.T) {
	desktop := NewDesktop(app.NewApp())
	root := ui.Use(func(_ struct{}) *ui.Node {
		return ui.Div(ui.Style(ui.Fill, ui.Bg(colorBackground)), desktop.menuPage())
	}, struct{}{})
	harness := ui.Mount(root, 1280, 720)
	defer harness.Close()
	for text, wantX := range map[string]float32{
		"每一张卡，": 176,
		"D":     179,
		"联机模式":  195,
	} {
		bounds := harness.Root().ByText(text).Bounds()
		if bounds.X != wantX {
			t.Errorf("%q x = %v, want %v", text, bounds.X, wantX)
		}
	}
}

func TestOriginalWindowContentCoordinates(t *testing.T) {
	tests := []struct {
		name  string
		page  pageID
		text  string
		wantX float32
	}{
		{name: "lobby", page: pageLobby, text: "昵称：", wantX: 343},
		{name: "single", page: pageSingle, text: "主要信息", wantX: 741},
		{name: "replay", page: pageReplay, text: "录像信息：", wantX: 709},
		{name: "duel", page: pageDuel, text: "DUEL FIELD", wantX: 248},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			desktop := NewDesktop(app.NewApp())
			desktop.page.Set(test.page)
			harness := ui.Mount(desktop.Root(), 1280, 720)
			defer harness.Close()
			if got := harness.Root().ByText(test.text).Bounds().X; got != test.wantX {
				t.Fatalf("%q x = %v, want %v", test.text, got, test.wantX)
			}
		})
	}
}

func TestNavigationKeepsPageHookStateSeparate(t *testing.T) {
	desktop := NewDesktop(app.NewApp())
	harness := ui.Mount(desktop.Root(), 1280, 720)
	defer harness.Close()
	steps := []struct {
		click string
		want  string
	}{
		{click: "单人模式", want: "主要信息"},
		{click: "退出", want: "观看录像"},
		{click: "观看录像", want: "录像信息："},
		{click: "退出", want: "联机模式"},
		{click: "联机模式", want: "昵称："},
		{click: "取消", want: "编辑卡组"},
	}
	for _, step := range steps {
		var target *ui.Query
		for _, clickable := range harness.Root().Clickables() {
			if strings.Contains(clickable.AllText(), step.click) {
				target = clickable
				break
			}
		}
		if target == nil {
			t.Fatalf("navigation target %q does not exist", step.click)
		}
		bounds := target.Bounds()
		const scaleX = float32(1.25)
		const scaleY = float32(1.125)
		x := 640 + (bounds.X+bounds.W/2-640)*scaleX
		y := 360 + (bounds.Y+bounds.H/2-360)*scaleY
		if !harness.ClickAt(x, y) {
			t.Fatalf("navigation target %q did not fire", step.click)
		}
		if !harness.Root().ByText(step.want).Exists() {
			t.Fatalf("after clicking %q at (%v,%v), %q does not exist (page=%s)", step.click, x, y, step.want, desktop.page.Get())
		}
	}
}

func TestDeckUsesOriginal1024Coordinates(t *testing.T) {
	desktop := NewDesktop(app.NewApp())
	desktop.page.Set(pageDeck)
	harness := ui.Mount(desktop.Root(), 1280, 720)
	defer harness.Close()
	for text, wantX := range map[string]float32{
		"退出编辑": 352,
		"卡片信息": 140,
		"卡组分类": 442,
		"主卡组：": 448,
	} {
		if got := harness.Root().ByText(text).Bounds().X; got != wantX {
			t.Errorf("%q x = %v, want %v", text, got, wantX)
		}
	}
}

func TestDeckSelectWorksAtNativeWindowSize(t *testing.T) {
	desktop := NewDesktop(app.NewApp())
	desktop.page.Set(pageDeck)
	h := ui.Mount(desktop.Root(), 1280, 720)
	defer h.Close()
	var trigger *ui.Query
	for _, candidate := range h.Root().Clickables() {
		bounds := candidate.Bounds()
		if bounds.X == 789 && bounds.Y == 50 {
			trigger = candidate
			break
		}
	}
	if trigger == nil {
		t.Fatal("card-type select trigger not found")
	}
	bounds := trigger.Bounds()
	x := 640 + (bounds.X+bounds.W/2-640)*1.25
	y := 360 + (bounds.Y+bounds.H/2-360)*1.125
	if !h.ClickAt(x, y) || len(h.Overlays()) != 1 {
		t.Fatal("native-size card-type select did not open")
	}
	if !h.Overlays()[0].ByText("怪兽").Click() || !h.Root().ByText("怪兽").Exists() {
		t.Fatal("selected option was not reflected by the deck select")
	}
	if desktop.currentCardFilter().Type != 1 {
		t.Fatal("card-type selection did not update the actual search filter")
	}
}

func TestArenaViewportMatchesOriginalCanvas(t *testing.T) {
	got := arenaViewport(1280, 720)
	want := ui.Rect{X: 300, Y: 99, W: 980, H: 535}
	for name, pair := range map[string][2]float32{
		"x": {got.X, want.X}, "y": {got.Y, want.Y}, "w": {got.W, want.W}, "h": {got.H, want.H},
	} {
		if math.Abs(float64(pair[0]-pair[1])) > 1 {
			t.Errorf("%s = %v, want %v", name, pair[0], pair[1])
		}
	}
}
