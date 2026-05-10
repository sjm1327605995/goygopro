package scenes

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/sjm1327605995/tenon"
	"github.com/sjm1327605995/tenon/pkg/engine"

	"github.com/sjm1327605995/goygopro/client"
)

const appVersion = "1.036.2"

var mainMenuScene = &mainMenuSceneState{}

type mainMenuSceneState struct {
	bg *ebiten.Image
}

func (s *mainMenuSceneState) loadBackground() {
	if s.bg != nil {
		return
	}
	f, err := os.Open("textures/bg_menu.jpg")
	if err != nil {
		fmt.Println("main_menu: failed to open background:", err)
		return
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Println("main_menu: failed to decode background:", err)
		return
	}
	s.bg = ebiten.NewImageFromImage(img)
}

// MainMenuRoute is the tenon RouteBuilder for the main menu.
func MainMenuRoute(ctx engine.BuildContext, params engine.RouteParams) engine.Widget {
	mainMenuScene.loadBackground()

	nav := tenon.GetNavigator(ctx)
	btnWidth, btnHeight := float32(280), float32(30)

	// Menu panel mimics C++ wMainMenu window
	menuPanel := tenon.VStack(
		// Title bar (dark blue, small text)
		 tenon.Container(
			 tenon.Text("YGOPro Version:"+appVersion).FontSize(13).Color(white),
		 ).Height(24).Background(titleBarBlue).Padding(0).Width(320),
		// Button area with even padding on all sides
		 tenon.Container(
			 tenon.VStack(
				 makeMenuButton("联机模式", btnWidth, btnHeight, func() {
					 if nav != nil { nav.Push("lanWindow") }
				 }),
				 makeMenuButton("单人模式", btnWidth, btnHeight, func() {
					 fmt.Println("Single mode not yet implemented")
				 }),
				 makeMenuButton("观看录像", btnWidth, btnHeight, func() {
					 fmt.Println("Replay mode not yet implemented")
				 }),
				 makeMenuButton("编辑卡组", btnWidth, btnHeight, func() {
					 if nav != nil { nav.Push("deckEdit") }
				 }),
				 makeMenuButton("退出", btnWidth, btnHeight, func() {
					 os.Exit(0)
				 }),
			 ).Gap(5).AlignItems(tenon.AlignCenter).Justify(tenon.JustifyCenter),
		 ).Background(color.RGBA{R: 200, G: 200, B: 200, A: 255}).
			 Padding(12).
			 Width(320).
			 Height(210),
	).Gap(0).AlignItems(tenon.AlignStretch)

	return tenon.Stack(
		// Background layer
			 tenon.Positioned(
					 tenon.Image(mainMenuScene.bg).Fit(tenon.ObjectFitCover),
			).L(0).T(0).R(0).B(0),
		// Centered menu panel
			 tenon.Positioned(menuPanel).Center(),
	).Width(client.GameWindowWidth).Height(client.GameWindowHeight)
}

func makeMenuButton(label string, w, h float32, onClick func()) tenon.Widget {
	return tenon.Container(
		 tenon.Button(label).
			 Style(tenon.ButtonDefault).
			 OnClick(func() { onClick() }),
	).W(w).H(h)
}
