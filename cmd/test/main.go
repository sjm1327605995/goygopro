package main

import (
	"bytes"
	"unsafe"

	"github.com/TotallyGamerJet/clay"
	"github.com/TotallyGamerJet/clay/examples/fonts"
	"github.com/TotallyGamerJet/clay/examples/videodemo"
	"github.com/TotallyGamerJet/clay/renderers/ebitengine"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func handleClayError(errorData clay.ErrorData) {
	panic(errorData)
}

const (
	winWidth, winHeight = 640, 480
	fontSize            = 16
)

type App struct {
	demoData    videodemo.Data
	cmds        clay.RenderCommandArray
	click       bool
	fonts       []text.Face
	fontSource  *text.GoTextFaceSource
	scaleFactor float32
	width       float64
	height      float64
}

func (a *App) Update() error {
	dx, dy := ebiten.Wheel()
	clay.UpdateScrollContainers(true, clay.Vector2{
		X: float32(dx),
		Y: float32(dy),
	}, 0.01)

	x, y := ebiten.CursorPosition()
	clay.SetPointerState(clay.Vector2{
		X: float32(x) / a.scaleFactor,
		Y: float32(y) / a.scaleFactor,
	}, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))

	clay.BeginLayout()
	clay.UI()(clay.ElementDeclaration{
		Layout: clay.LayoutConfig{
			Sizing: clay.Sizing{Width: clay.SizingGrow(100)},
		},
		BackgroundColor: clay.Color{
			R: 100,
			G: 100,
			B: 100,
			A: 255,
		},
	}, func() {
		//if a.click {
		//	fmt.Println("xxxx")
		//}
		clay.OnHover(func(elementId clay.ElementId, pointerInfo clay.PointerData, userData int64) {
			if pointerInfo.State == clay.POINTER_DATA_PRESSED {
				data := (*bool)(unsafe.Pointer(uintptr(userData)))
				*data = true
			}
		}, int64(uintptr(unsafe.Pointer(&a.click))))
		clay.Text("xxx", clay.TextConfig(clay.TextElementConfig{
			UserData:  nil,
			TextColor: clay.Color{R: 255, G: 255, B: 255, A: 255},
			FontId:    0,
			FontSize:  16,
		}))
	})
	a.cmds = clay.EndLayout()

	return nil
}

func (a *App) Draw(screen *ebiten.Image) {
	_ = ebitengine.ClayRender(screen, a.scaleFactor, a.cmds, a.fonts)
}

func (a *App) Layout(_, _ int) (int, int) {
	panic("use Ebitengine >=v2.5.0")
}

func (a *App) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	clay.SetLayoutDimensions(clay.Dimensions{
		Width:  float32(outsideWidth),
		Height: float32(outsideHeight),
	})

	s := ebiten.Monitor().DeviceScaleFactor()
	a.scaleFactor = float32(s)
	outsideWidth *= s
	outsideHeight *= s
	a.width = outsideWidth
	a.height = outsideHeight

	return a.width, a.height
}

func main() {
	ebiten.SetWindowTitle("Ebitengine")
	ebiten.SetWindowSize(winWidth, winHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	source, err := text.NewGoTextFaceSource(bytes.NewReader(fonts.RobotoRegularTTF))
	if err != nil {
		panic(err)
	}

	scaleFactor := ebiten.Monitor().DeviceScaleFactor()
	app := &App{
		fonts: []text.Face{
			&text.GoTextFace{
				Source: source,
				Size:   fontSize * scaleFactor,
			},
		},

		fontSource: source,
	}

	// Initialize Clay
	totalMemorySize := clay.MinMemorySize()
	memory := make([]byte, totalMemorySize)
	arena := clay.CreateArenaWithCapacityAndMemory(memory)
	clay.Initialize(arena, clay.Dimensions{Width: winWidth, Height: winHeight}, clay.ErrorHandler{ErrorHandlerFunction: handleClayError})
	clay.SetMeasureTextFunction(ebitengine.MeasureText, unsafe.Pointer(&app.fonts))

	if err := ebiten.RunGame(app); err != nil {
		panic(err)
	}
}
