package tenon

import (
	"fmt"

	"github.com/sjm1327605995/tenon/pkg/ui"
)

type ActionPopupProps struct {
	X, Y     float32
	Options  []ActionOption
	OnSelect func(ActionOption)
	OnClose  func()
}

type ActionButtonProps struct {
	Option   ActionOption
	OnSelect func(ActionOption)
}

func actionPopupLayout(x, y float32, count int, viewportW, viewportH float32) (float32, float32, float32, float32) {
	const margin, gap, width, rowHeight, rowGap, padding = float32(8), float32(12), float32(220), float32(34), float32(4), float32(8)
	height := padding*2 + float32(count)*rowHeight + float32(max(0, count-1))*rowGap
	left, top := x+gap, y+gap
	if left+width > viewportW-margin {
		left = x - width - gap
	}
	if top+height > viewportH-margin {
		top = y - height - gap
	}
	left = max(margin, min(left, viewportW-width-margin))
	top = max(margin, min(top, viewportH-height-margin))
	return left, top, width, height
}

func ActionButton(p ActionButtonProps) *ui.Node {
	hovered, setHovered := ui.UseState(false)
	foreground := colorText
	background := ui.Hex("#ffffff0a")
	if hovered {
		foreground = ui.White
		background = ui.Hex("#789f4a")
	}
	return ui.Button(
		ui.Style(ui.Width(204), ui.Height(34), ui.Bg(background), ui.Border(1, ui.Hex("#a8bdca24")), ui.Radius(6)),
		ui.OnHover(setHovered),
		ui.OnClick(func() {
			if p.OnSelect != nil {
				p.OnSelect(p.Option)
			}
		}),
		textAt(p.Option.Label, 12, 8, 180, 18, 11, foreground),
	)
}

func ActionPopup(p ActionPopupProps) *ui.Node {
	if len(p.Options) == 0 {
		return nil
	}
	close := p.OnClose
	if close == nil {
		close = func() {}
	}
	ui.UseEscape(true, close)
	x, y, width, height := actionPopupLayout(p.X, p.Y, len(p.Options), 1024, 640)
	children := []*ui.Node{
		ui.Style(ui.Absolute, ui.Left(x), ui.Top(y), ui.Width(width), ui.Height(height),
			ui.LinearGradient(ui.Hex("#172630fa"), ui.Hex("#0b141efa"), 90), ui.Border(1, colorBorder), ui.Radius(10),
			ui.Shadow(ui.Color{R: 0, G: 0, B: 0, A: 150}, 0, 12, 30, 0), ui.ZIndex(100)),
		ui.OnClick(func() {}),
	}
	for index, option := range p.Options {
		children = append(children, pos(8, 8+float32(index)*38, 204, 34,
			ui.Fragment(ui.Use(ActionButton, ActionButtonProps{Option: option, OnSelect: p.OnSelect}), ui.Key(fmt.Sprintf("%s:%d", option.Kind, option.Index))),
		))
	}
	return ui.Div(children...)
}
