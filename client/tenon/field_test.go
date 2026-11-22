package tenon

import (
	"reflect"
	"testing"

	"github.com/g3n/engine/camera"
	"github.com/g3n/engine/core"
	engine_material "github.com/g3n/engine/material"
	"github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/backend/g3n"
	"github.com/sjm1327605995/tenon/pkg/value"
)

func TestCardImageLoadsLocalArt(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	img := cardImage(app.NewApp(), 89631139)
	if img == nil {
		t.Fatal("expected Blue-Eyes artwork to decode")
	}
	if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
		t.Fatal("decoded image has invalid dimensions")
	}
}

func TestBoardRaycastSelectsRealFieldCard(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	model := NewDuelModel(nil)
	model.Apply("duel:summoning", map[string]any{"code": 89631139, "cc": 0, "cl": locationMZone, "cs": 2, "cp": 1})
	board := NewBoardComponent(app.NewApp(), model)
	game := &g3n.Game{Scene: core.NewNode(), Camera: camera.New(980.0 / 535.0)}
	scene := g3n.GameScene(g3n.PerspectiveCamera(g3n.CameraProps{
		FOV: 45,
		Transform: g3n.TransformProps{
			Position: value.Static(g3n.V3(0.4, 24, 11)),
			Rotation: value.Static(g3n.V3(-1.14, 0, 0)),
		},
	}), board)
	if err := scene.Mount(game); err != nil {
		t.Fatal(err)
	}
	defer scene.Close()

	var hovered, selected *FieldHit
	missed := 0
	board.OnHover = func(hit *FieldHit) { hovered = hit }
	board.OnSelect = func(hit *FieldHit) { selected = hit }
	board.OnMiss = func() { missed++ }
	point := zonePosition(0, locationMZone, 2)
	point.Y += 0.07
	game.Camera.Project(&point)
	board.HandlePointer(g3n.PointerEvent{X: point.X, Y: point.Y, Inside: true, LeftClick: true})
	if hovered == nil || hovered.Sequence != 2 || hovered.Location != locationMZone {
		t.Fatalf("hovered field card = %+v", hovered)
	}
	if selected == nil || selected.Code != 89631139 || selected.Seat != 0 || selected.Sequence != 2 {
		t.Fatalf("selected field card = %+v", selected)
	}

	board.HandlePointer(g3n.PointerEvent{X: -0.99, Y: -0.99, Inside: true, LeftClick: true})
	if missed != 1 || hovered != nil {
		t.Fatalf("empty-field click: missed=%d hovered=%+v", missed, hovered)
	}
}

func TestBoardPlacementPrefersOfferedZoneOverCard(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	model := NewDuelModel(nil)
	model.Apply("duel:summoning", map[string]any{"code": 89631139, "cc": 0, "cl": locationMZone, "cs": 2, "cp": 1})
	model.Apply("duel:select_place", map[string]any{
		"player": 0, "count": 1,
		"zones": []any{map[string]any{"player": 0, "loc": locationMZone, "seq": 2}},
	})
	board := NewBoardComponent(app.NewApp(), model)
	game := &g3n.Game{Scene: core.NewNode(), Camera: camera.New(980.0 / 535.0)}
	scene := g3n.GameScene(g3n.PerspectiveCamera(g3n.CameraProps{
		FOV: 45,
		Transform: g3n.TransformProps{
			Position: value.Static(g3n.V3(0.4, 24, 11)),
			Rotation: value.Static(g3n.V3(-1.14, 0, 0)),
		},
	}), board)
	if err := scene.Mount(game); err != nil {
		t.Fatal(err)
	}
	defer scene.Close()
	if len(board.selectionMarks) != 1 {
		t.Fatalf("place marks = %d, want 1", len(board.selectionMarks))
	}

	var selected *FieldHit
	cancelled := 0
	board.OnSelect = func(hit *FieldHit) { selected = hit }
	board.OnCancel = func() { cancelled++ }
	point := zonePosition(0, locationMZone, 2)
	game.Camera.Project(&point)
	board.HandlePointer(g3n.PointerEvent{X: point.X, Y: point.Y, Inside: true, LeftClick: true})
	if selected == nil || selected.Code != 0 || selected.Location != locationMZone || selected.Sequence != 2 {
		t.Fatalf("placement hit = %+v", selected)
	}
	board.HandlePointer(g3n.PointerEvent{X: point.X, Y: point.Y, Inside: true, RightClick: true})
	if cancelled != 1 {
		t.Fatalf("right-click cancellations = %d, want 1", cancelled)
	}
}

func TestDuelSceneSelectsAttackTarget(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	desktop := NewDesktop(app.NewApp())
	responses := &responseRecorder{}
	desktop.duelController = NewDuelController(responses, desktop.duelModel)
	desktop.duelModel.Apply("duel:summoning", map[string]any{"code": 89631139, "cc": 1, "cl": locationMZone, "cs": 1, "cp": 1})
	desktop.duelModel.Apply("duel:summoning", map[string]any{"code": 46986414, "cc": 1, "cl": locationMZone, "cs": 2, "cp": 1})
	desktop.duelModel.Apply("duel:select_card", map[string]any{
		"player": 0, "min": 1, "max": 1,
		"cards": []any{map[string]any{"code": 46986414, "c": 1, "l": locationMZone, "s": 2}},
	})
	game := &g3n.Game{Scene: core.NewNode(), Camera: camera.New(980.0 / 535.0)}
	scene := g3n.Scene3D(newDemoScene(desktop))
	if err := scene.Mount(game); err != nil {
		t.Fatal(err)
	}
	defer scene.Close()

	point := zonePosition(1, locationMZone, 1)
	game.Camera.Project(&point)
	game.HandlePointer(g3n.PointerEvent{X: point.X, Y: point.Y, Inside: true, LeftClick: true})
	if len(responses.cardCalls) != 0 {
		t.Fatal("non-candidate card triggered a selection response")
	}
	point = zonePosition(1, locationMZone, 2)
	point.Y += 0.07
	game.Camera.Project(&point)
	game.HandlePointer(g3n.PointerEvent{X: point.X, Y: point.Y, Inside: true, LeftClick: true})
	if !reflect.DeepEqual(responses.cardCalls, [][]int32{{0}}) {
		t.Fatalf("attack target response = %+v", responses.cardCalls)
	}
}

func TestBoardPromptDoesNotRebuildCardMeshes(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	model := NewDuelModel(nil)
	model.Apply("duel:summoning", map[string]any{"code": 89631139, "cc": 0, "cl": locationMZone, "cs": 2, "cp": 1})
	board := NewBoardComponent(app.NewApp(), model)
	game := &g3n.Game{Scene: core.NewNode(), Camera: camera.New(1)}
	scene := g3n.GameScene(board)
	if err := scene.Mount(game); err != nil {
		t.Fatal(err)
	}
	defer scene.Close()
	native := board.cards[0].mesh.Native
	boardRevision := model.BoardRevision
	model.Apply("duel:select_idlecmd", map[string]any{"toBP": true})
	board.Update(0)
	if board.cards[0].mesh.Native != native || model.BoardRevision != boardRevision {
		t.Fatal("prompt update rebuilt field meshes")
	}
	if board.selectionRevision != model.Revision {
		t.Fatal("prompt update did not refresh selection state")
	}
}

func TestCardMeshMountsTextureMaterial(t *testing.T) {
	t.Setenv("YGOPRO_PICS_DIR", "../..")
	card := NewCardComponent(app.NewApp(), 89631139, g3n.V3(0, 0, 0))
	scene := g3n.GameScene(card)
	game := &g3n.Game{Scene: core.NewNode(), Camera: camera.New(1)}
	if err := scene.Mount(game); err != nil {
		t.Fatal(err)
	}
	defer scene.Close()
	materials := card.mesh.Native.GetGraphic().Materials()
	if len(materials) != 1 {
		t.Fatalf("material count = %d, want 1", len(materials))
	}
	mat := materials[0].IMaterial().GetMaterial()
	if got := mat.TextureCount(); got != 1 {
		t.Fatalf("texture count = %d, want 1", got)
	}
	if got := mat.Shader(); got != "tenon_unlit" {
		t.Fatalf("shader = %q, want tenon_unlit", got)
	}
	if got := mat.UseLights(); got != engine_material.UseLightNone {
		t.Fatalf("card lights = %v, want none", got)
	}
}
