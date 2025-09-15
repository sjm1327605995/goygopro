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
	Bg     unsafe.Pointer
	ClickA bool
	ClickB bool
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
				var a bool
				RenderHeaderButton("111", &h.ClickA)
				RenderHeaderButton("exit", &h.ClickB)
				if a == true {
					fmt.Println(a)
				}
			})
		})

	})
	Cmds = clay.EndLayout()
	return nil
}

func RenderHeaderButton(text string, t *bool) {
	clay.UI()(clay.ElementDeclaration{
		Layout: clay.LayoutConfig{
			Sizing:  clay.Sizing{Width: clay.SizingPercent(1)},
			Padding: clay.Padding{Top: 8, Bottom: 8},
			ChildAlignment: clay.ChildAlignment{
				X: clay.ALIGN_X_CENTER,
				Y: clay.ALIGN_Y_CENTER,
			},
		},

		BackgroundColor: func() clay.Color {
			if *t == true {
				*t = false
				return clay.Color{R: 100, G: 100, B: 50, A: 255}
			}
			return clay.Color{R: 200, G: 200, B: 200, A: 255}

		}(),
		CornerRadius: clay.CornerRadiusAll(2),
	}, func() {

		clay.OnHover(func(elementId clay.ElementId, pointerInfo clay.PointerData, userData int64) {
			if pointerInfo.State == clay.POINTER_DATA_PRESSED {
				data := (*bool)(unsafe.Pointer(uintptr(userData)))
				*data = true
			}
		}, int64(uintptr(unsafe.Pointer(t))))
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
