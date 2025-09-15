package game

import (
	"fmt"

	"github.com/TotallyGamerJet/clay"
	"github.com/TotallyGamerJet/clay/renderers/ebitengine"
	"github.com/hajimehoshi/ebiten/v2"

	"unsafe"

	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type OnlineMode struct {
	SceneManager *SceneManager
	Bg           unsafe.Pointer
	ButtonList   []*Button
}

func NewOnlineMode() Scene {
	return &OnlineMode{}
}
func (h *OnlineMode) Update() error {
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
					Width: clay.SizingFixed(500),
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
				clay.Text("联机模式", &clay.TextElementConfig{
					FontId:    0,
					TextColor: clay.Color{R: 255, G: 255, B: 255, A: 255},
					FontSize:  16,
				})

			})
			clay.UI()(clay.ElementDeclaration{
				Layout: clay.LayoutConfig{
					Padding:         clay.PaddingAll(10),
					LayoutDirection: clay.LEFT_TO_RIGHT,
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
				clay.Text("昵称", &clay.TextElementConfig{
					FontId:    0,
					TextColor: clay.Color{R: 255, G: 255, B: 255, A: 255},
					FontSize:  16,
				})
				// clay.UI()(clay.ElementDeclaration{}, func() {})
			})
		})

	})
	Cmds = clay.EndLayout()
	return nil
}

func (h *OnlineMode) Draw(screen *ebiten.Image) {
	err := ebitengine.ClayRender(screen, ScaleFactor, Cmds, Fonts)
	if err != nil {
		fmt.Println(err)
	}
}

func (h *OnlineMode) OnEnter(s *SceneManager) {
	h.SceneManager = s
	bg, _, err := ebitenutil.NewImageFromFile("textures/bg_menu.jpg")
	if err == nil {
		h.Bg = unsafe.Pointer(bg)
	}
}

func (h *OnlineMode) OnExit() {
}
func (h *OnlineMode) Layout(_, _ int) (int, int) {
	panic("use Ebitengine >=v2.5.0")
}
