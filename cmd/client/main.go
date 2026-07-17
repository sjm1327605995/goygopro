package main

import (
	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/client/scenes"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

func main() {
	if !client.MainGame.Initialize() {
		panic("Failed to initialize game")
	}

	ui.WindowTitle("YGOPro")
	ui.WindowSize(client.GameWindowWidth, client.GameWindowHeight)
	ui.Run(ui.Use(scenes.App, struct{}{}))
}
