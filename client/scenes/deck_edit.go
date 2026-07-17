package scenes

import (
	"fmt"

	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// DeckEditScene 目前只是占位：显示当前卡组张数、能载入默认卡组。
// 真正的卡组编辑（搜索/筛选/拖放）待决斗盘完成后再做。
func DeckEditScene(_ struct{}) *ui.Node {
	deck, setDeck := ui.UseState(client.MainGame.DeckMgr.CurrentDeck)
	status, setStatus := ui.UseState("")

	return bg("textures/bg_deck.jpg", []ui.StyleOpt{ui.ItemsCenter, ui.JustifyCenter, ui.Gap(12)},
		ui.Text("卡组编辑（占位）", ui.FontSize(24), ui.TextColor(white)),
		ui.Text(fmt.Sprintf("主卡组: %d 张", len(deck.Main)), ui.FontSize(14), ui.TextColor(white)),
		ui.Text(fmt.Sprintf("额外卡组: %d 张", len(deck.Extra)), ui.FontSize(14), ui.TextColor(white)),
		ui.Text(fmt.Sprintf("副卡组: %d 张", len(deck.Side)), ui.FontSize(14), ui.TextColor(white)),
		ui.If(status != "", ui.Text(status, ui.FontSize(12), ui.TextColor(selectedGold))),
		menuButton("载入默认卡组", func() {
			d, err := client.DeckMgr.LoadDeck("deck/default.ydk")
			if err != nil {
				setStatus("载入失败: " + err.Error())
				return
			}
			client.MainGame.DeckMgr.CurrentDeck = *d
			setDeck(*d)
			setStatus("已载入 deck/default.ydk")
		}),
		menuButton("返回", func() { client.PopScene() }),
	)
}
