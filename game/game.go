package game

import (
	"bytes"
	"github.com/TotallyGamerJet/clay"
	"github.com/TotallyGamerJet/clay/renderers/ebitengine"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"os"
	"unsafe"
)

const (
	ScreenWidth  = 256
	ScreenHeight = 240
)

type Game struct {
	sceneManager *SceneManager
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return ScreenWidth, ScreenHeight
}

func (g *Game) Update() error {

	if err := g.sceneManager.Update(); err != nil {
		return err
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.sceneManager.Draw(screen)
}
func (g *Game) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	clay.SetLayoutDimensions(clay.Dimensions{
		Width:  float32(outsideWidth),
		Height: float32(outsideHeight),
	})

	s := ebiten.Monitor().DeviceScaleFactor()
	ScaleFactor = float32(s)
	outsideWidth *= s
	outsideHeight *= s
	Width = outsideWidth
	Height = outsideHeight

	return Width, Height
}

const (
	winWidth, winHeight = 800, 600
	fontSize            = 16
)

func handleClayError(errorData clay.ErrorData) {
	panic(errorData)
}

func NewGame() *Game {

	ebiten.SetWindowSize(winWidth, winHeight)
	f, _ := os.ReadFile("OPPOSans-Medium.ttf")
	source, err := text.NewGoTextFaceSource(bytes.NewReader(f))
	if err != nil {
		panic(err)
	}

	scaleFactor := ebiten.Monitor().DeviceScaleFactor()
	s := NewSceneManager(NewHome())
	Width, Height = winWidth, winHeight
	Fonts = []text.Face{&text.GoTextFace{
		Source: source,
		Size:   fontSize * scaleFactor,
	}}
	FontSource = source
	g := &Game{sceneManager: s}
	// Initialize Clay
	totalMemorySize := clay.MinMemorySize()
	memory := make([]byte, totalMemorySize)
	arena := clay.CreateArenaWithCapacityAndMemory(memory)
	clay.Initialize(arena, clay.Dimensions{Width: winWidth, Height: winHeight}, clay.ErrorHandler{ErrorHandlerFunction: handleClayError})
	clay.SetMeasureTextFunction(ebitengine.MeasureText, unsafe.Pointer(&Fonts))
	return g
}
