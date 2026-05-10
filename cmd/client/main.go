package main

import (
	"os"

	"github.com/sjm1327605995/tenon"
	"github.com/sjm1327605995/tenon/pkg/engine"
	"github.com/sjm1327605995/tenon/pkg/font"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/client/scenes"
)

func main() {
	if err := initFonts(); err != nil {
		panic(err)
	}

	if !client.MainGame.Initialize() {
		panic("Failed to initialize game")
	}

	// Navigator key allows non-UI code (network handlers) to push/pop routes.
	navKey := engine.NewGlobalKey()
	client.MainGame.SetNavigatorKey(navKey)

	// Route table: each route is a builder function invoked by the framework.
	routes := map[string]engine.RouteBuilder{
		"mainMenu":  scenes.MainMenuRoute,
		"lanWindow": scenes.LanWindowRoute,
		"lobby":     scenes.LobbyRoute,
		"duelField": scenes.DuelFieldRoute,
		"deckEdit":  scenes.DeckEditRoute,
	}

	// Root widget: the Navigator manages the page stack and transitions.
	// tenon.Run invokes this builder on every frame; the framework diffs
	// and schedules rendering automatically — no explicit Build() calls.
	nav := engine.Navigator(routes, "mainMenu").
		WithTransition(engine.TransitionNone)
	nav.SetKey(navKey)

	tenon.Run(func() engine.Widget {
		return nav
	}, client.GameWindowWidth, client.GameWindowHeight)
}

func initFonts() error {
	cjkPaths := []string{
		"font/OPPOSans-Medium.ttf",
		"./font/OPPOSans-Medium.ttf",
		"C:/Windows/Fonts/msyh.ttc",
		"C:/Windows/Fonts/simsun.ttc",
		"C:/Windows/Fonts/simhei.ttf",
	}
	for _, path := range cjkPaths {
		if _, err := os.Stat(path); err == nil {
			if err := font.ReloadFontFromFile(font.FontFamilyDefault, path); err == nil {
				return nil
			}
		}
	}
	return font.InitDefaultFont()
}
