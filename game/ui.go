package game

import (
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/millken/yoga"
)

// ColorBox creates a widget with the specified dimensions and color.
func ColorBox(gtx layout.Context, size image.Point, color color.NRGBA) layout.Dimensions {
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: color}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

type Home struct {
	Background  image.Image
	builder     *Builder
	Theme       *material.Theme
	OnlineMode  *widget.Clickable
	SingleMode  *widget.Clickable
	WatchReplay *widget.Clickable
	DeckEditor  *widget.Clickable
	Exit        *widget.Clickable
}

func NewHome(theme *material.Theme) *Home {
	home := &Home{
		Theme:       theme,
		OnlineMode:  new(widget.Clickable),
		SingleMode:  new(widget.Clickable),
		WatchReplay: new(widget.Clickable),
		DeckEditor:  new(widget.Clickable),
		Exit:        new(widget.Clickable),
		builder:     NewBuilder(),
	}
	home.loadBackground()
	home.builder.Elements(NewNode(widget.Image{
		Src: paint.NewImageOp(home.Background),
		Fit: widget.Fill,
	}.Layout).Style(func(node *yoga.Node) {
		node.StyleSetWidthPercent(100)
		node.StyleSetHeightPercent(100)
		node.StyleSetJustifyContent(yoga.JustifyCenter)
		node.StyleSetAlignItems(yoga.AlignCenter)
	}).Elements(
		NewNode(func(gtx layout.Context) layout.Dimensions {
			return ColorBox(gtx, image.Pt(300, 300), color.NRGBA{R: 186, G: 185, B: 188, A: 200})
		}).Elements(
			NewNode(func(gtx layout.Context) layout.Dimensions {
				return ColorBox(gtx, image.Pt(300, 20), color.NRGBA{R: 37, G: 36, B: 110, A: 245})
			}),
			NewNode(material.Button(home.Theme, home.OnlineMode, "online mode").Layout).
				Style(func(node *yoga.Node) {
					node.StyleSetHeight(50)
					node.StyleSetWidth(300)
				}))))

	return home
}
func (h *Home) loadBackground() {

	f, err := os.Open("textures/bg_menu.jpg")
	if err != nil {
		return
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return
	}

	h.Background = img
}
func (h *Home) Layout(ctx layout.Context) layout.Dimensions {

	return h.builder.Layout(ctx)
}
