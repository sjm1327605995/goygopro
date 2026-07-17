package scenes

import (
	"fmt"

	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// 决斗盘左侧的大图预览与底部的聊天输入 —— 都是平面 HUD，不进 3D。

// cardPreview 是悬停某张卡时左侧显示的大图，对应原版鼠标停在卡上时的放大预览。
//
// 只显示图和数值：卡名和卡text 要靠 cards.cdb，仓库里没有这个文件，
// client 也还没有读它的通道（core/duel 的 DataManager 只读决斗引擎要的数值，不含文本）。
func cardPreview(card *client.ClientCard) *ui.Node {
	if card == nil {
		return nil
	}
	// 盖着的卡不该被预览泄底 —— 那等于开挂。自己的盖伏也一视同仁：
	// 服务器发下来的对手盖卡 Code 本就是 0，这里只挡自己这边看得见 Code 的情况。
	if card.IsFaceDown() && card.Controler != 0 {
		return nil
	}

	img := client.ImageMgr.GetTexture(int(card.Code))
	if img == nil || card.Code == 0 {
		return nil
	}

	var lines []*ui.Node
	add := func(s string) { lines = append(lines, ui.Text(s, ui.FontSize(11), ui.TextColor(white))) }
	add(fmt.Sprintf("卡密: %d", card.Code))
	if card.Type&0x1 != 0 { // TYPE_MONSTER
		add(fmt.Sprintf("攻 %d / 守 %d", card.Attack, card.Defense))
		if card.Link > 0 {
			add(fmt.Sprintf("LINK-%d", card.Link))
		} else if card.Rank > 0 {
			add(fmt.Sprintf("阶级 %d", card.Rank))
		} else {
			add(fmt.Sprintf("等级 %d", card.Level))
		}
	}

	return ui.Box([]ui.StyleOpt{
		ui.Column, ui.Gap(6), ui.Padding(8), ui.Width(184),
		ui.Bg(blackTransparent), ui.Radius(8),
	},
		ui.Box([]ui.StyleOpt{ui.Width(168), ui.Height(245), ui.Radius(4), ui.Clip},
			ui.Img(ui.SrcImage(fmt.Sprintf("card:%d", card.Code), img), ui.Fit(ui.FitCover),
				ui.Style(ui.Absolute, ui.Left(0), ui.Top(0), ui.Fill)),
		),
		ui.Box([]ui.StyleOpt{ui.Column, ui.Gap(2)}, lines...),
	)
}

type chatBarProps struct{}

// chatBar 是决斗中的聊天输入：回车或点「发送」都能发出去。
func chatBar(_ chatBarProps) *ui.Node {
	msg, setMsg := ui.UseState("")

	send := func(text string) {
		if text == "" {
			return
		}
		client.Client.SendChat(text)
		setMsg("")
	}

	return ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(6), ui.ItemsCenter},
		ui.Input(
			ui.Value(msg), ui.OnChange(setMsg), ui.OnSubmit(send), ui.Placeholder("发言…"),
			ui.Style(ui.Width(260), ui.Height(22), ui.PaddingXY(4, 0),
				ui.Bg(ui.Hex("#ffffffd9")), ui.Border(1, ui.Hex("#8c8c8c")),
				ui.FontSize(12), ui.TextColor(black)),
		),
		lobbyButton("发送", func() { send(msg) }),
	)
}
