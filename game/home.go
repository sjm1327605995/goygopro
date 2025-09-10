package game

import (
	"fmt"
	"github.com/TotallyGamerJet/clay"
	"github.com/TotallyGamerJet/clay/examples/videodemo"
	"github.com/TotallyGamerJet/clay/renderers/ebitengine"
	"github.com/hajimehoshi/ebiten/v2"
	"unsafe"
)

type Home struct {
	Bg unsafe.Pointer
}

func NewHome() Scene {
	return &Home{}
}
func (h *Home) Update() error {
	dx, dy := ebiten.Wheel()
	clay.UpdateScrollContainers(true, clay.Vector2{
		X: float32(dx),
		Y: float32(dy),
	}, 0.01)

	x, y := ebiten.CursorPosition()
	clay.SetPointerState(clay.Vector2{
		X: float32(x) / ScaleFactor,
		Y: float32(y) / ScaleFactor,
	}, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))
	clay.BeginLayout()
	clay.UI()(clay.ElementDeclaration{
		Layout: clay.LayoutConfig{
			Sizing: clay.Sizing{
				Width:  clay.SizingGrow(0),
				Height: clay.SizingGrow(0),
			},

			ChildAlignment: clay.ChildAlignment{
				X: clay.ALIGN_X_CENTER,
				Y: clay.ALIGN_Y_CENTER,
			},
			LayoutDirection: clay.TOP_TO_BOTTOM,
		},
		Image: func() clay.ImageElementConfig {
			if h.Bg != nil {
				return clay.ImageElementConfig{
					ImageData: h.Bg,
				}
			}
			return clay.ImageElementConfig{}
		}(),
	}, func() {
		clay.UI()(clay.ElementDeclaration{
			Layout: clay.LayoutConfig{
				LayoutDirection: clay.TOP_TO_BOTTOM,
				Sizing: clay.Sizing{
					Width: clay.SizingFixed(200),
				},
			},
			BackgroundColor: clay.Color{
				R: 255,
				G: 0,
				B: 100,
				A: 255,
			},
		}, func() {
			clay.UI()(clay.ElementDeclaration{
				Layout: clay.LayoutConfig{
					Sizing: clay.Sizing{
						Width: clay.SizingPercent(1),
					},
				},
				BackgroundColor: clay.Color{
					R: 100,
					G: 0,
					B: 100,
					A: 255,
				},
			}, func() {
				clay.Text("test", &clay.TextElementConfig{
					FontId:    0,
					TextColor: clay.Color{R: 255, G: 255, B: 255, A: 255},
					FontSize:  16,
				})
			})
			clay.UI()(clay.ElementDeclaration{
				Layout: clay.LayoutConfig{
					Padding:         clay.PaddingAll(5),
					LayoutDirection: clay.TOP_TO_BOTTOM,
					ChildGap:        5,
					Sizing:          clay.Sizing{Width: clay.SizingPercent(1)},
				},

				BackgroundColor: clay.Color{
					R: 100,
					G: 100,
					B: 100,
					A: 255,
				},
			}, func() {
				RenderHeaderButton("111")
				RenderHeaderButton("exit")
			})
		})

	})
	Cmds = clay.EndLayout()
	return nil
}
func alloc[T any](arena *arena) *T {
	prev := uintptr(arena.offset)
	arena.offset = int64(prev + unsafe.Sizeof(*new(T)))
	return (*T)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(arena.memory)), prev))
}

type arena struct {
	offset int64
	memory []byte
}
type Click struct {
	Click bool
}
var frameArena = arena{memory: make([]byte, 1024)},
func RenderHeaderButton(text string) {
	clay.UI()(clay.ElementDeclaration{
		Layout: clay.LayoutConfig{
			Sizing:  clay.Sizing{Width: clay.SizingPercent(1)},
			Padding: clay.Padding{Top: 8, Bottom: 8},
			ChildAlignment: clay.ChildAlignment{
				X: clay.ALIGN_X_CENTER,
				Y: clay.ALIGN_Y_CENTER,
			},
		},

		BackgroundColor: clay.Color{R: 50, G: 140, B: 140, A: 255},
		CornerRadius:    clay.CornerRadiusAll(2),
	}, func() {
		var click := alloc[sidebarClickData](&data.frameArena)

		clay.OnHover(func(elementId clay.ElementId, pointerInfo clay.PointerData, userData int64) {
			if pointerInfo.State == clay.POINTER_DATA_PRESSED {
				click = true
			}
		}, int64(uintptr(unsafe.Pointer(nil))))
		if click == true {
			fmt.Println(click)
		}
		clay.Text(text, clay.TextConfig(clay.TextElementConfig{
			FontId:    0,
			FontSize:  16,
			TextColor: clay.Color{R: 255, G: 255, B: 255, A: 255},
		}))
	})
}
func (h *Home) Draw(screen *ebiten.Image) {
	err := ebitengine.ClayRender(screen, ScaleFactor, Cmds, Fonts)
	if err != nil {
		fmt.Println(err)
	}
}

func (h *Home) OnEnter() {
	if h.Bg == nil {
		//ebitenutil.NewImageFromFile("")
		ebImg := ebiten.NewImageFromImage(videodemo.SquirrelImage)
		h.Bg = unsafe.Pointer(ebImg)
	}
}

func (h *Home) OnExit() {
}
func (h *Home) Layout(_, _ int) (int, int) {
	panic("use Ebitengine >=v2.5.0")
}
