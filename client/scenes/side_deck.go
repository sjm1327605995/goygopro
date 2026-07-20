package scenes

import (
	"fmt"

	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// SideDeckScene 是 match 第二、三局之间的换副卡组界面。
//
// 玩法很直接：点主卡组/额外卡组里的卡把它换下去（挪进副卡组），点副卡组里的卡把它换上来。
// 三边张数必须与换牌前一致才能提交 —— 这既是规则，也是服务端 LoadSide 的校验条件。
func SideDeckScene(_ struct{}) *ui.Node {
	rev := UseRevision(client.MainGame.FieldRev)
	_ = rev

	g := client.MainGame
	d := &g.DeckMgr.CurrentDeck
	hint := g.SideCountHint()
	canSubmit := hint == ""

	// 整块压一层深色底：白字的对比度不能指望背景图，背景一亮就糊成一片。
	return bg("textures/bg_deck.jpg", []ui.StyleOpt{ui.Column, ui.Padding(12), ui.Gap(8),
		ui.Bg(ui.Hex("#0f1420e6"))},
		ui.Box([]ui.StyleOpt{ui.Row, ui.ItemsCenter, ui.Gap(12)},
			ui.Text("更换副卡组", ui.FontSize(18), ui.Bold, ui.TextColor(white)),
			ui.Text("点上方卡片换下，点副卡组卡片换上；三边张数须与换牌前一致",
				ui.FontSize(11), ui.TextColor(ui.Hex("#9aa7bd"))),
		),

		sideRow("主卡组", d.Main, g.SidePreMain, 0, 2),
		sideRow("额外卡组", d.Extra, g.SidePreExtra, 1, 2),
		sideRow("副卡组", d.Side, g.SidePreSide, 2, 0),

		ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(12), ui.ItemsCenter, ui.JustifyCenter, ui.Padding(6)},
			ui.If(!canSubmit, ui.Text(hint, ui.FontSize(12), ui.TextColor(cmdAttackColor))),
			lobbyButtonDisabled("确定", !canSubmit, func() {
				client.Client.SendUpdateDeck(&client.MainGame.DeckMgr.CurrentDeck)
				client.MainGame.IsSiding = false
			}),
		),
	)
}

// sideRow 画卡组的一部分。moveTo 是点击这里的卡时把它挪去哪一区。
func sideRow(title string, codes []uint32, want, from, moveTo int) *ui.Node {
	countColor := ui.Hex("#7fe0a0") // 张数对得上：绿色
	if len(codes) != want {
		countColor = ui.Hex("#ff6b5b") // 不对：红色，且下方「确定」会禁用
	}

	cards := make([]*ui.Node, 0, len(codes))
	for i, code := range codes {
		idx := i
		cards = append(cards, ui.Keyed(fmt.Sprintf("%d-%d-%d", from, i, code),
			ui.Use(sideCard, sideCardProps{
				Code: code,
				OnClick: func() {
					if client.MainGame.DeckMgr.SideMove(from, moveTo, idx) {
						client.MainGame.FieldRev.Bump()
					}
				},
			})))
	}

	return ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(4)},
		ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(8), ui.ItemsCenter},
			ui.Text(title, ui.FontSize(13), ui.TextColor(white)),
			ui.Text(fmt.Sprintf("%d / %d", len(codes), want),
				ui.FontSize(12), ui.TextColor(countColor)),
		),
		ui.ScrollView(
			ui.Style(ui.Height(104), ui.Row, ui.Gap(3), ui.Wrap, ui.Padding(4),
				ui.Bg(blackTransparent), ui.Radius(4)),
			ui.Fragment(cards...),
		),
	)
}

type sideCardProps struct {
	Code    uint32
	OnClick func()
}

func sideCard(p sideCardProps) *ui.Node {
	hovered, _, ia := ui.UseInteraction()
	border := ui.Hex("#00000000")
	if hovered {
		border = selectableCyan
	}

	kids := []*ui.Node{
		ui.Style(ui.Width(48), ui.Height(68), ui.Bg(cardGray),
			ui.Border(2, border), ui.Radius(3), ui.Clip),
		ia, ui.OnClick(p.OnClick),
	}
	if img := client.ImageMgr.GetTexture(int(p.Code)); img != nil {
		kids = append(kids, ui.Img(ui.SrcImage(fmt.Sprintf("card:%d", p.Code), img),
			ui.Fit(ui.FitCover), ui.Style(ui.Absolute, ui.Left(0), ui.Top(0), ui.Fill)))
	} else {
		kids = append(kids, ui.Text(fmt.Sprint(p.Code), ui.FontSize(9), ui.TextColor(black)))
	}
	return ui.Button(kids...)
}
