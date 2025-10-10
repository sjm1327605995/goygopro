package game

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"github.com/millken/yoga"
)

type Node struct {
	Yoga     *yoga.Node
	index    uint32
	widget   layout.Widget
	Children []*Node
}

func NewNode(widget layout.Widget) *Node {
	return &Node{
		Yoga:   yoga.NewNode(),
		widget: widget,
	}
}
func (n *Node) Layout(gtx layout.Context) layout.Dimensions {
	m := op.Record(gtx.Ops)
	w, h := int(n.Yoga.StyleGetWidth()), int(n.Yoga.StyleGetHeight())
	x1, y1 := int(n.Yoga.LayoutLeft()), int(n.Yoga.LayoutTop())
	gtx.Constraints.Min.X = w
	gtx.Constraints.Min.Y = h
	gtx.Constraints.Max.X = w
	gtx.Constraints.Max.Y = h
	cal := m.Stop()
	trans := op.Offset(image.Pt(x1, y1)).Push(gtx.Ops)
	cal.Add(gtx.Ops)
	n.widget(gtx)
	for i := range n.Children {
		n.Children[i].Layout(gtx)
	}
	trans.Pop()
	return layout.Dimensions{Size: image.Pt(w, h)}
}
func (n *Node) Elements(nodes ...*Node) *Node {
	for _, v := range nodes {
		n.Yoga.InsertChild(v.Yoga, n.index)
		n.index++
	}
	n.Children = append(n.Children, nodes...)
	return n
}
func (n *Node) Style(f func(node *yoga.Node)) *Node {
	f(n.Yoga)
	return n
}

type Builder struct {
	root *Node
}

func NewBuilder() *Builder {
	return &Builder{root: NewNode(func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{
			Size: gtx.Constraints.Min,
		}
	})}
}
func (b *Builder) Elements(nodes ...*Node) *Builder {
	b.root.Elements(nodes...)
	return b
}
func (b *Builder) Layout(gtx layout.Context) layout.Dimensions {
	w, h := float32(gtx.Constraints.Min.X), float32(gtx.Constraints.Min.Y)
	b.root.Yoga.StyleSetWidth(w)
	b.root.Yoga.StyleSetHeight(h)
	yoga.CalculateLayout(b.root.Yoga, w, h, yoga.DirectionInherit)

	for i := range b.root.Children {
		b.root.Children[i].Layout(gtx)
	}
	return layout.Dimensions{Size: gtx.Constraints.Min}
}
