package tenon

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/g3n/engine/core"
	"github.com/g3n/engine/experimental/collision"
	"github.com/g3n/engine/math32"
	"github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/backend/g3n"
	"github.com/sjm1327605995/tenon/pkg/ui"
	"github.com/sjm1327605995/tenon/pkg/value"
)

type CardComponent struct {
	Code           uint32
	Seat, Location int
	Sequence       int
	mesh           *g3n.MeshComponent
}

type FieldHit struct {
	Code           uint32
	Seat, Location int
	Sequence       int
	X, Y           float32
}

func NewCardComponent(service *app.App, code uint32, position g3n.Vec3) *CardComponent {
	return newCardComponent(service, code, position, g3n.V3(0, 0, 0), false)
}

func newCardComponent(service *app.App, code uint32, position, rotation g3n.Vec3, faceDown bool) *CardComponent {
	return newCardComponentSeat(service, code, position, rotation, faceDown, 0)
}

func newCardComponentSeat(service *app.App, code uint32, position, rotation g3n.Vec3, faceDown bool, seat int) *CardComponent {
	material := ui.Material3DProps{Color: value.Static(ui.Color{R: 41, G: 158, B: 235, A: 255})}
	originalCode := code
	if faceDown {
		code = 0
	}
	if img := cardImageForSeat(service, code, seat); img != nil {
		material = ui.Material3DProps{Texture: img, TextureKey: fmtCardTextureKey(code, seat), Unlit: true, CacheKey: fmtCardTextureKey(code, seat)}
	}
	return &CardComponent{
		Code: originalCode, Seat: seat,
		mesh: g3n.Mesh3D(ui.Mesh3D(ui.Mesh3DProps{
			Shape:    ui.Cube3D(1),
			Material: material,
			Transform: ui.Transform3DProps{
				Position: value.Static(ui.V3(position.X, position.Y, position.Z)),
				Rotation: value.Static(ui.V3(rotation.X, rotation.Y, rotation.Z)),
				Scale:    value.Static(ui.V3(1.4, 0.03, 2)),
			},
		})),
	}
}

func fmtCardTextureKey(code uint32, seat int) string {
	if code == 0 {
		return "ygopro-card-back-" + strconv.Itoa(seat)
	}
	return "ygopro-card-" + strconv.FormatUint(uint64(code), 10)
}

func (c *CardComponent) Mount(ctx *g3n.SceneContext) error { return c.mesh.Mount(ctx) }
func (c *CardComponent) Update(dt time.Duration)           { c.mesh.Update(dt) }
func (c *CardComponent) Close()                            { c.mesh.Close() }

type boardZone struct {
	hit  FieldHit
	mesh *g3n.MeshComponent
}

type BoardComponent struct {
	service           *app.App
	model             *DuelModel
	scenery           []*g3n.MeshComponent
	zones             []boardZone
	selectionMarks    []*g3n.MeshComponent
	cards             []*CardComponent
	ctx               *g3n.SceneContext
	revision          int
	selectionRevision int
	mounted           bool
	hovered           *CardComponent
	OnHover           func(*FieldHit)
	OnSelect          func(*FieldHit)
	OnMiss            func()
	OnCancel          func()
}

func NewBoardComponent(service *app.App, model *DuelModel) *BoardComponent {
	scenery, zones := newArenaScenery()
	return &BoardComponent{service: service, model: model, scenery: scenery, zones: zones}
}

func (b *BoardComponent) Mount(ctx *g3n.SceneContext) error {
	b.ctx = ctx
	for _, zone := range b.scenery {
		if err := zone.Mount(ctx); err != nil {
			return err
		}
	}
	b.mounted = true
	b.syncCards()
	return nil
}

func (b *BoardComponent) Update(dt time.Duration) {
	for _, zone := range b.scenery {
		zone.Update(dt)
	}
	if b.mounted && b.revision != b.model.BoardRevision {
		b.syncCards()
	} else if b.mounted && b.selectionRevision != b.model.Revision {
		b.syncSelectionMarks()
		b.selectionRevision = b.model.Revision
	}
	for _, card := range b.cards {
		card.Update(dt)
	}
}

func (b *BoardComponent) HandlePointer(event g3n.PointerEvent) {
	if b.ctx == nil || b.ctx.Game.Camera == nil {
		return
	}
	var cardHit *CardComponent
	var zoneHit *FieldHit
	if event.Inside {
		b.ctx.Game.Camera.UpdateMatrixWorld()
		raycaster := collision.NewRaycaster(new(math32.Vector3), &math32.Vector3{Z: 1})
		if err := raycaster.SetFromCamera(b.ctx.Game.Camera, event.X, event.Y); err == nil {
			nodes := make([]core.INode, 0, len(b.cards)+len(b.zones))
			if b.model.Placement == nil {
				for _, card := range b.cards {
					if card.mesh == nil || card.mesh.Native == nil || (b.model.CardSelection != nil && !b.cardChoiceOffered(card)) {
						continue
					}
					card.mesh.Native.UpdateMatrixWorld()
					nodes = append(nodes, card.mesh.Native)
				}
			}
			for _, zone := range b.zones {
				if zone.mesh == nil || zone.mesh.Native == nil || b.model.CardSelection != nil || (b.model.Placement != nil && !b.placeZoneOffered(zone.hit)) {
					continue
				}
				zone.mesh.Native.UpdateMatrixWorld()
				nodes = append(nodes, zone.mesh.Native)
			}
			intersections := raycaster.IntersectObjects(nodes, false)
			if len(intersections) > 0 {
				object := intersections[0].Object
				for _, card := range b.cards {
					if card.mesh != nil && card.mesh.Native == object {
						cardHit = card
						break
					}
				}
				if cardHit == nil {
					for _, zone := range b.zones {
						if zone.mesh != nil && zone.mesh.Native == object {
							hit := zone.hit
							zoneHit = &hit
							break
						}
					}
				}
			}
		}
	}
	if b.hovered != cardHit {
		b.hovered = cardHit
		if b.OnHover != nil {
			b.OnHover(fieldHitForCard(cardHit))
		}
	}
	if event.RightClick {
		if b.OnCancel != nil {
			b.OnCancel()
		}
		return
	}
	if !event.LeftClick {
		return
	}
	selection := fieldHitForCard(cardHit)
	if selection == nil || b.model.Placement != nil {
		selection = zoneHit
	}
	if selection != nil {
		if b.OnSelect != nil {
			viewport := arenaViewport(1024, 640)
			selection.X = viewport.X + (event.X+1)/2*viewport.W
			selection.Y = viewport.Y + (1-event.Y)/2*viewport.H
			b.OnSelect(selection)
		}
	} else if b.OnMiss != nil {
		b.OnMiss()
	}
}

func (b *BoardComponent) placeZoneOffered(hit FieldHit) bool {
	placement := b.model.Placement
	if placement == nil {
		return false
	}
	for _, zone := range placement.Zones {
		if zone.Seat == hit.Seat && zone.Location == hit.Location && zone.Sequence == hit.Sequence {
			return true
		}
	}
	return false
}

func (b *BoardComponent) cardChoiceOffered(card *CardComponent) bool {
	selection := b.model.CardSelection
	if selection == nil {
		return false
	}
	for _, choice := range selection.Choices {
		if choice.Code == card.Code && choice.Seat == card.Seat && choice.Location == card.Location && choice.Sequence == card.Sequence {
			return true
		}
	}
	return false
}

func fieldHitForCard(card *CardComponent) *FieldHit {
	if card == nil {
		return nil
	}
	return &FieldHit{Code: card.Code, Seat: card.Seat, Location: card.Location, Sequence: card.Sequence}
}

func (b *BoardComponent) syncSelectionMarks() {
	for _, mark := range b.selectionMarks {
		mark.Close()
	}
	b.selectionMarks = b.selectionMarks[:0]
	addMark := func(position g3n.Vec3, key string, active, selected bool) {
		y := float32(0.16)
		points := []math32.Vector3{
			{X: position.X - 0.76, Y: y, Z: position.Z - 1.02}, {X: position.X + 0.76, Y: y, Z: position.Z - 1.02},
			{X: position.X + 0.76, Y: y, Z: position.Z - 1.02}, {X: position.X + 0.76, Y: y, Z: position.Z + 1.02},
			{X: position.X + 0.76, Y: y, Z: position.Z + 1.02}, {X: position.X - 0.76, Y: y, Z: position.Z + 1.02},
			{X: position.X - 0.76, Y: y, Z: position.Z + 1.02}, {X: position.X - 0.76, Y: y, Z: position.Z - 1.02},
		}
		color := rgb8(250, 190, 60)
		if !active {
			color = rgb8(92, 176, 255)
		} else if !selected {
			color = rgb8(240, 210, 70)
		}
		mark := g3n.Mesh(g3n.MeshProps{
			Shape: arenaLinesShape(points),
			Material: g3n.MaterialProps{
				Color: value.Static(color), Opacity: value.Static(float32(0.9)), Unlit: true,
				CacheKey: fmt.Sprintf("selection-mark-%s-%t", key, selected),
			},
		})
		if b.mounted {
			if err := mark.Mount(b.ctx); err != nil {
				panic(err)
			}
		}
		b.selectionMarks = append(b.selectionMarks, mark)
	}
	if placement := b.model.Placement; placement != nil {
		selected := make(map[int]bool, len(placement.Selected))
		for _, index := range placement.Selected {
			selected[index] = true
		}
		for index, zone := range placement.Zones {
			addMark(zonePosition(zone.Seat, uint8(zone.Location), zone.Sequence), fmt.Sprintf("place-%d", index), false, selected[index])
		}
		return
	}
	selection := b.model.CardSelection
	if selection == nil {
		return
	}
	selected := make(map[int]bool, len(selection.Selected))
	for _, index := range selection.Selected {
		selected[index] = true
	}
	for index, choice := range selection.Choices {
		if choice.Location != locationMZone && choice.Location != locationSZone {
			continue
		}
		addMark(zonePosition(choice.Seat, uint8(choice.Location), choice.Sequence), fmt.Sprintf("card-%d", index), true, selected[index])
	}
}

func (b *BoardComponent) Close() {
	for _, mark := range b.selectionMarks {
		mark.Close()
	}
	b.selectionMarks = nil
	for _, card := range b.cards {
		card.Close()
	}
	for _, zone := range b.scenery {
		zone.Close()
	}
	b.mounted = false
	b.ctx = nil
}

func (b *BoardComponent) syncCards() {
	b.hovered = nil
	for _, card := range b.cards {
		card.Close()
	}
	b.cards = b.cards[:0]
	for seat := 0; seat < 2; seat++ {
		side := b.model.Board[seat]
		for location, count := range map[uint8]int{locationMZone: 7, locationSZone: 8} {
			for sequence := 0; sequence < count; sequence++ {
				var card *DuelCard
				if location == locationMZone {
					card = side.MZone[sequence]
				} else {
					card = side.SZone[sequence]
				}
				if card != nil {
					b.addCard(seat, location, sequence, *card)
				}
			}
		}
		for location, stack := range map[uint8][]DuelCard{locationGrave: side.Grave, locationBanish: side.Banish} {
			for sequence, card := range stack {
				b.addCard(seat, location, sequence, card)
			}
		}
		b.addPile(seat, locationDeck, b.model.Piles[seat].Deck)
		b.addPile(seat, locationExtra, b.model.Piles[seat].Extra)
	}
	if b.mounted {
		for _, card := range b.cards {
			if err := card.Mount(b.ctx); err != nil {
				panic(err)
			}
		}
	}
	b.revision = b.model.BoardRevision
	b.selectionRevision = b.model.Revision
	b.syncSelectionMarks()
}

func (b *BoardComponent) addCard(seat int, location uint8, sequence int, card DuelCard) {
	position := zonePosition(seat, location, sequence)
	position.Y += 0.07
	if location == locationGrave || location == locationBanish {
		position.Y += float32(min(sequence, 20)) * 0.02
	}
	component := newCardComponentSeat(b.service, card.Code, position, cardRotation(seat, card.Position), card.Position&0x0a != 0, seat)
	component.Location, component.Sequence = int(location), sequence
	b.cards = append(b.cards, component)
}

func (b *BoardComponent) addPile(seat int, location uint8, count int) {
	if count <= 0 {
		return
	}
	position := zonePosition(seat, location, 0)
	position.Y += 0.04 + float32(min(count, 60))*0.003
	material := ui.Material3DProps{Color: value.Static(ui.Color{R: 158, G: 148, B: 128, A: 255})}
	if img := cardImageForSeat(b.service, 0, seat); img != nil {
		material = ui.Material3DProps{Texture: img, TextureKey: fmtCardTextureKey(0, seat), Unlit: true, CacheKey: fmtCardTextureKey(0, seat)}
	}
	b.cards = append(b.cards, &CardComponent{Seat: seat, Location: int(location), mesh: g3n.Mesh3D(ui.Mesh3D(ui.Mesh3DProps{
		Shape: ui.Cube3D(1), Material: material,
		Transform: ui.Transform3DProps{
			Position: value.Static(ui.V3(position.X, position.Y, position.Z)), Rotation: value.Static(ui.V3(cardRotation(seat, 1).X, cardRotation(seat, 1).Y, cardRotation(seat, 1).Z)),
			Scale: value.Static(ui.V3(1.4, max(float32(0.3), float32(min(count, 60))/15), 2)),
		},
	}))})
}

func cardRotation(seat int, position uint8) g3n.Vec3 {
	rotation := float32(0)
	if position&0x0c != 0 {
		rotation = -math.Pi / 2
	}
	if seat == 1 {
		rotation += math.Pi
	}
	return g3n.V3(0, rotation, 0)
}

func zonePosition(seat int, location uint8, sequence int) g3n.Vec3 {
	mzone := [7]g3n.Vec3{g3n.V3(-4.5, 0.02, 2.8), g3n.V3(-2.3, 0.02, 2.8), g3n.V3(-0.1, 0.02, 2.8), g3n.V3(2.1, 0.02, 2.8), g3n.V3(4.3, 0.02, 2.8), g3n.V3(-2.3, 0.02, 0), g3n.V3(2.1, 0.02, 0)}
	szone := [8]g3n.Vec3{g3n.V3(-4.5, 0.02, 5.2), g3n.V3(-2.3, 0.02, 5.2), g3n.V3(-0.1, 0.02, 5.2), g3n.V3(2.1, 0.02, 5.2), g3n.V3(4.3, 0.02, 5.2), g3n.V3(-6.8, 0.02, 4), g3n.V3(-6.8, 0.02, 1.4), g3n.V3(8.6, 0.02, 1.4)}
	if seat == 1 {
		mzone = [7]g3n.Vec3{g3n.V3(4.3, 0.02, -2.8), g3n.V3(2.1, 0.02, -2.8), g3n.V3(-0.1, 0.02, -2.8), g3n.V3(-2.3, 0.02, -2.8), g3n.V3(-4.5, 0.02, -2.8), g3n.V3(2.1, 0.02, 0), g3n.V3(-2.3, 0.02, 0)}
		szone = [8]g3n.Vec3{g3n.V3(4.3, 0.02, -5.2), g3n.V3(2.1, 0.02, -5.2), g3n.V3(-0.1, 0.02, -5.2), g3n.V3(-2.3, 0.02, -5.2), g3n.V3(-4.5, 0.02, -5.2), g3n.V3(6.6, 0.02, -4), g3n.V3(6.6, 0.02, -1.4), g3n.V3(-8.8, 0.02, -1.4)}
	}
	if location == locationMZone && sequence >= 0 && sequence < 7 {
		return mzone[sequence]
	}
	if location == locationSZone && sequence >= 0 && sequence < 8 {
		return szone[sequence]
	}
	if seat == 1 {
		switch location {
		case locationGrave:
			return g3n.V3(-6.8, 0.02, -4)
		case locationBanish:
			return g3n.V3(-6.8, 0.02, -1.4)
		case locationDeck:
			return g3n.V3(-6.8, 0.02, -6.6)
		case locationExtra:
			return g3n.V3(6.6, 0.02, -6.6)
		}
	}
	switch location {
	case locationGrave:
		return g3n.V3(6.6, 0.02, 4)
	case locationBanish:
		return g3n.V3(6.6, 0.02, 1.4)
	case locationDeck:
		return g3n.V3(6.6, 0.02, 6.6)
	case locationExtra:
		return g3n.V3(-6.8, 0.02, 6.6)
	}
	return g3n.V3(0, 0.02, 0)
}

var decodedCardImages sync.Map
var cardImages sync.Map

func decodeCardImage(service *app.App, code uint32) image.Image {
	if cached, ok := decodedCardImages.Load(code); ok {
		return cached.(image.Image)
	}
	if code == 0 {
		file, err := os.Open("frontend/textures/cover.jpg")
		if err != nil {
			return nil
		}
		defer file.Close()
		img, _, err := image.Decode(file)
		if err != nil {
			return nil
		}
		decodedCardImages.Store(code, img)
		return img
	}
	data, _ := service.GetCardImageData(code)
	if len(data) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	decodedCardImages.Store(code, img)
	return img
}

func cardImageForSeat(service *app.App, code uint32, seat int) image.Image {
	key := fmtCardTextureKey(code, seat)
	if cached, ok := cardImages.Load(key); ok {
		return cached.(image.Image)
	}
	var img image.Image
	if code == 0 {
		path := "frontend/textures/cover.jpg"
		if seat == 1 {
			path = "frontend/textures/cover2.jpg"
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		img, _, err = image.Decode(file)
		if err != nil {
			return nil
		}
	} else {
		img = decodeCardImage(service, code)
	}
	if img == nil {
		return nil
	}
	cardImages.Store(key, img)
	return img
}

func cardImage(service *app.App, code uint32) image.Image {
	return cardImageForSeat(service, code, 0)
}

func (d *Desktop) selectFieldTarget(hit *FieldHit) {
	if hit == nil {
		d.actionPopup.Set(nil)
		return
	}
	if d.duelModel.Placement != nil {
		d.duelController.ChoosePlace(hit.Seat, hit.Location, hit.Sequence)
		d.actionPopup.Set(nil)
		return
	}
	if d.duelModel.CardSelection != nil {
		d.duelController.ChooseCard(hit.Code, hit.Seat, hit.Location, hit.Sequence)
		d.actionPopup.Set(nil)
		return
	}
	if hit.Code != 0 {
		d.selected.Set(hit.Code)
	}
	options := d.duelController.FieldOptions(hit.Code, hit.Seat, hit.Location, hit.Sequence)
	if len(options) == 0 {
		d.actionPopup.Set(nil)
		return
	}
	d.actionPopup.Set(&ActionPopupProps{X: hit.X, Y: hit.Y, Options: options})
}

func newDemoScene(desktop *Desktop) *ui.NativeScene3D {
	board := NewBoardComponent(desktop.service, desktop.duelModel)
	board.OnHover = func(hit *FieldHit) {
		if hit != nil && hit.Code != 0 {
			desktop.selected.Set(hit.Code)
		}
	}
	board.OnMiss = func() { desktop.actionPopup.Set(nil) }
	board.OnCancel = func() {
		if desktop.duelModel.CardSelection != nil {
			desktop.duelController.CancelOrUndoCardSelection()
			desktop.actionPopup.Set(nil)
			return
		}
		if !desktop.duelController.CancelOrUndoPlace() && desktop.duelModel.Placement == nil {
			desktop.actionPopup.Set(nil)
		}
	}
	board.OnSelect = desktop.selectFieldTarget
	return ui.GameScene3D(
		ui.PerspectiveCamera3D(ui.Camera3DProps{
			FOV: 45,
			Transform: ui.Transform3DProps{
				Position: value.Static(ui.V3(0.4, 24, 11)),
				Rotation: value.Static(ui.V3(-1.14, 0, 0)),
			},
		}),
		ui.AmbientLight3D(ui.Light3DProps{Intensity: value.Static(float32(0.8))}),
		ui.DirectionalLight3D(ui.Light3DProps{Color: value.Static(ui.Hex("#ffe4b1")), Intensity: value.Static(float32(0.65)), Transform: ui.Transform3DProps{Position: value.Static(ui.V3(-8, 18, -7))}}),
		ui.DirectionalLight3D(ui.Light3DProps{Color: value.Static(ui.Hex("#95c5dc")), Intensity: value.Static(float32(0.25)), Transform: ui.Transform3DProps{Position: value.Static(ui.V3(10, 8, 6))}}),
		ui.Native3D(board),
	)
}
