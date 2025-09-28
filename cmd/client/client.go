package main

import (
	"github.com/sjm1327605995/goygopro/cmd/game"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/joelschutz/stagehand"
)

const (
	screenWidth  = 640
	screenHeight = 480
)

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("My Game")

	state := game.State(10)

	s := &game.FirstScene{}
	sm := stagehand.NewSceneManager[game.State](s, state)

	if err := ebiten.RunGame(sm); err != nil {
		log.Fatal(err)
	}
}
