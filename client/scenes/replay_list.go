package scenes

import (
	"fmt"

	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// ReplayListScene 列出 replay 目录里的录像：谁跟谁、什么时候打的。
//
// 回放本身还做不了 —— 重放要在本地跑一遍决斗引擎（C++ replay_mode.cpp 的
// StartDuel + process 循环），而引擎必须有卡片数据库才能建卡、跑效果脚本，
// 仓库里没有 cards.cdb。所以这里先把「有哪些录像」看得见，选中后给出明确说明，
// 而不是做一个点了没反应的按钮。
func ReplayListScene(_ struct{}) *ui.Node {
	replays := ui.UseMemo(func() []client.ReplayInfo {
		return client.ListReplays()
	}, 0)
	selected, setSelected := ui.UseState(-1)

	return bg("textures/bg_deck.jpg", []ui.StyleOpt{
		ui.Column, ui.Padding(16), ui.Gap(10), ui.Bg(ui.Hex("#0f1420e6")),
	},
		ui.Box([]ui.StyleOpt{ui.Row, ui.ItemsCenter, ui.Gap(12)},
			ui.Text("观看录像", ui.FontSize(18), ui.Bold, ui.TextColor(white)),
			ui.Text(fmt.Sprintf("共 %d 局", len(replays)),
				ui.FontSize(12), ui.TextColor(ui.Hex("#9aa7bd"))),
		),

		replayRows(replays, selected, setSelected),
		replayDetail(replays, selected),

		ui.Box([]ui.StyleOpt{ui.Row, ui.Gap(12), ui.JustifyCenter},
			lobbyButton("返回", func() { client.PopScene() }),
		),
	)
}

func replayRows(replays []client.ReplayInfo, selected int, setSelected func(int)) *ui.Node {
	if len(replays) == 0 {
		return ui.Box([]ui.StyleOpt{
			ui.Height(240), ui.ItemsCenter, ui.JustifyCenter,
			ui.Bg(blackTransparent), ui.Radius(4),
		},
			ui.Text("replay 目录里还没有录像", ui.FontSize(12), ui.TextColor(gray)),
		)
	}

	rows := make([]*ui.Node, 0, len(replays))
	for i, r := range replays {
		idx := i
		rows = append(rows, ui.Keyed(r.Path, ui.Use(replayRow, replayRowProps{
			Info:     r,
			Selected: idx == selected,
			OnClick:  func() { setSelected(idx) },
		})))
	}
	return ui.ScrollView(
		ui.Style(ui.Height(240), ui.Column, ui.Gap(2), ui.Padding(4),
			ui.Bg(blackTransparent), ui.Radius(4)),
		ui.Fragment(rows...),
	)
}

type replayRowProps struct {
	Info     client.ReplayInfo
	Selected bool
	OnClick  func()
}

func replayRow(p replayRowProps) *ui.Node {
	hovered, _, ia := ui.UseInteraction()
	face := ui.Hex("#00000000")
	switch {
	case p.Selected:
		face = ui.Hex("#2a4a7a")
	case hovered:
		face = ui.Hex("#1e2a3a")
	}

	return ui.Button(
		ui.Style(ui.Row, ui.Gap(10), ui.ItemsCenter, ui.PaddingXY(8, 5),
			ui.Bg(face), ui.Radius(3)),
		ui.OnClick(p.OnClick), ia,
		ui.Box([]ui.StyleOpt{ui.Width(150)},
			ui.Text(p.Info.StartTime.Format("2006-01-02 15:04"),
				ui.FontSize(11), ui.TextColor(ui.Hex("#9aa7bd"))),
		),
		ui.Text(p.Info.Title(), ui.FontSize(12), ui.TextColor(white)),
		ui.If(p.Info.IsTag, ui.Text("双打", ui.FontSize(10), ui.TextColor(selectedGold))),
		ui.If(p.Info.Single, ui.Text("单人", ui.FontSize(10), ui.TextColor(selectedGold))),
	)
}

func replayDetail(replays []client.ReplayInfo, selected int) *ui.Node {
	if selected < 0 || selected >= len(replays) {
		return ui.Box([]ui.StyleOpt{ui.Height(52)},
			ui.Text("选一局查看详情", ui.FontSize(11), ui.TextColor(gray)),
		)
	}
	r := replays[selected]
	return ui.Box([]ui.StyleOpt{
		ui.Column, ui.Gap(3), ui.Padding(8), ui.Height(52),
		ui.Bg(blackTransparent), ui.Radius(4),
	},
		ui.Text(r.Title()+"    "+r.StartTime.Format("2006-01-02 15:04:05"),
			ui.FontSize(12), ui.TextColor(white)),
		// 讲清楚为什么不能播，而不是给个点了没反应的按钮
		ui.Text("回放需要卡片数据库（cards.cdb）才能在本地重跑这局决斗，当前还缺这份数据",
			ui.FontSize(10), ui.TextColor(ui.Hex("#c9a227"))),
	)
}
