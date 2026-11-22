package tenon

import (
	"github.com/sjm1327605995/tenon/pkg/ui"
	"github.com/sjm1327605995/tenon/pkg/value"
)

var (
	colorBackground = ui.Hex("#060b12")
	colorPanel      = ui.Hex("#101b24")
	colorPanelAlt   = ui.Hex("#1e3039")
	colorLime       = ui.Hex("#c8ee4a")
	colorText       = ui.Hex("#edf3f6")
	colorMuted      = ui.Hex("#899ba9")
	colorBorder     = ui.Hex("#263c45")
)

func frame(children ...*ui.Node) *ui.Node {
	return designFrame(1024, 640, children...)
}

func designFrame(width, height float32, children ...*ui.Node) *ui.Node {
	viewport := ui.Viewport()
	scaleX := viewport.W / width
	scaleY := viewport.H / height
	x := (viewport.W - width) / 2
	y := (viewport.H - height) / 2
	return ui.Div(
		ui.Style(ui.Absolute, ui.Left(x), ui.Top(y), ui.Width(width), ui.Height(height), ui.ScaleXY(scaleX, scaleY)),
		ui.Div(children...),
	)
}

func pos(x, y, w, h float32, children ...*ui.Node) *ui.Node {
	return ui.Div(append([]*ui.Node{ui.Style(ui.Absolute, ui.Left(x), ui.Top(y), ui.Width(w), ui.Height(h))}, children...)...)
}

func textAt(text string, x, y, w, h, size float32, color ui.Color) *ui.Node {
	return pos(x, y, w, h, ui.Text(text).FontSize(size).TextColor(color))
}

func textCenterAt(text string, x, y, w, h, size float32, color ui.Color) *ui.Node {
	return pos(x, y, w, h, ui.HStack(0,
		ui.Style(ui.Fill, ui.JustifyCenter, ui.ItemsCenter),
		ui.Text(text).FontSize(size).TextColor(color),
	))
}

func screenHeading(title, subtitle string) *ui.Node {
	return ui.Div(
		pos(36, 25, 3, 27, ui.Style(ui.Bg(colorLime))),
		textAt(title, 52, 25, 360, 28, 19, ui.Hex("#e8eff2")),
		textAt(subtitle, 52, 53, 460, 12, 7, ui.Hex("#879eaa")),
		pos(36, 68, 848, 1, ui.Style(ui.Bg(ui.Hex("#a8bdca26")))),
	)
}

func windowPanel(title string, x, y, w, h float32, children ...*ui.Node) *ui.Node {
	body := pos(0, 24, w, h-24, children...)
	return pos(x, y, w, h,
		ui.Style(ui.LinearGradient(ui.Hex("#15232df5"), ui.Hex("#0a131cfb"), 45), ui.Border(1, ui.Hex("#b1c4d13d")), ui.Radius(2)),
		pos(0, 0, w, 24,
			ui.Style(ui.LinearGradient(ui.Hex("#293c42"), ui.Hex("#12222e"), 10), ui.Border(1, ui.Hex("#bfd58b42"))),
			textAt(title, 8, 4, w-16, 17, 12, ui.Hex("#e4ebe8")),
		),
		body,
	)
}

func darkTextField(source value.Bind[string]) *ui.Node {
	return ui.Input(
		ui.Style(ui.Fill, ui.PaddingXY(5, 0), ui.Radius(1), ui.Bg(ui.Hex("#08111b")), ui.Border(1, ui.Hex("#a8bdca2b")), ui.TextColor(colorText), ui.FontSize(12)),
		ui.BindInput(source),
	)
}

func windowButton(label string, x, y, w float32, onClick func()) *ui.Node {
	return pos(x, y, w, 26,
		ui.Style(ui.LinearGradient(ui.Hex("#253842"), ui.Hex("#182a34"), 90), ui.Border(1, ui.Hex("#9fb6bf50"))),
		ui.OnClick(onClick),
		textAt(label, 6, 5, w-12, 17, 12, ui.Hex("#e0e9eb")),
	)
}

func panelButton(label, meta string, x, y, w, h float32, primary bool, onClick func()) *ui.Node {
	foreground := colorText
	from := ui.Hex("#1e3039c9")
	to := ui.Hex("#101b2490")
	fontSize := float32(14)
	metaSize := float32(7)
	if primary {
		foreground = ui.Hex("#182315")
		from = ui.Hex("#d0ec78")
		to = ui.Hex("#a5cf46")
		fontSize = 24
		metaSize = 11
	}
	style := []ui.StyleOpt{ui.Absolute, ui.Left(x), ui.Top(y), ui.Width(w), ui.Height(h)}
	if label == "退出" {
		foreground = ui.Hex("#8796a2")
		fontSize = 10
	} else {
		style = append(style, ui.LinearGradient(from, to, 10), ui.Border(1, colorBorder))
		if primary {
			style = append(style, ui.CutCorner(12))
		}
	}
	children := []*ui.Node{
		ui.Style(style...),
		ui.OnClick(onClick),
		textAt(label, 18, h/2-fontSize/2, w-36, fontSize+4, fontSize, foreground),
	}
	if meta != "" {
		children = append(children, textAt(meta, w-125, h/2-metaSize/2, 110, metaSize+4, metaSize, foreground))
	}
	return ui.Div(children...)
}
