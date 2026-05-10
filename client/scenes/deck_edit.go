package scenes

import (
	"fmt"

	"github.com/sjm1327605995/tenon"
	"github.com/sjm1327605995/tenon/pkg/engine"
	"github.com/sjm1327605995/goygopro/client"
)

// DeckEditScene is a placeholder deck editor.
type DeckEditScene struct{}

var deckEditScene = &DeckEditScene{}

func DeckEditRoute(ctx engine.BuildContext, params engine.RouteParams) engine.Widget {
	return deckEditScene.build(ctx)
}

func (s *DeckEditScene) build(ctx engine.BuildContext) engine.Widget {
	nav := tenon.GetNavigator(ctx)
	deck := &client.MainGame.DeckMgr.CurrentDeck

	return tenon.Stack(
		 tenon.Positioned(
			 tenon.VStack(
				 tenon.Text("Deck Edit (Placeholder)").FontSize(24).Color(white),
				 tenon.Container(tenon.Spacer()).H(20),
				 tenon.Text(fmt.Sprintf("Main: %d cards", len(deck.Main))).FontSize(14).Color(white),
				 tenon.Text(fmt.Sprintf("Extra: %d cards", len(deck.Extra))).FontSize(14).Color(white),
				 tenon.Text(fmt.Sprintf("Side: %d cards", len(deck.Side))).FontSize(14).Color(white),
				 tenon.Container(tenon.Spacer()).H(20),
				 tenon.Button("Load Default Deck").Style(tenon.ButtonOutline).OnClick(func() {
					 if d, err := client.DeckMgr.LoadDeck("deck/default.ydk"); err == nil {
						 client.MainGame.DeckMgr.CurrentDeck = *d
						 fmt.Println("Deck loaded")
					 } else {
						 fmt.Println("Failed to load deck:", err)
					 }
				 }),
				 tenon.Container(tenon.Spacer()).H(12),
				 tenon.Button("Back").Style(tenon.ButtonOutline).OnClick(func() {
					 if nav != nil {
						 nav.Pop()
					 }
				 }),
			 ).Gap(12).Padding(24).AlignItems(tenon.AlignCenter).Justify(tenon.JustifyCenter),
		 ).L(0).T(0).R(0).B(0),
	).Background(black).Width(client.GameWindowWidth).Height(client.GameWindowHeight)
}
