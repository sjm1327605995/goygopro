package tenon

import (
	"math"

	appservice "github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

type HandSelection struct {
	Code     uint32
	Sequence int
	X, Y     float32
}

type HandFanProps struct {
	Service         *appservice.App
	Cards           []uint32
	X, Y            float32
	Width           float32
	Height          float32
	Near            bool
	Shuffle         float32
	ShuffleRevision int
	OnSelect        func(HandSelection)
	OnHover         func(uint32)
	Highlight       func(uint32, int) int
}

type handLayout struct {
	x, y, width, height, angle float32
}

func arenaHandLayout(sequence, count int, width, height float32, near bool, hover, shuffle float32) handLayout {
	cardHeight := min(70, height*0.083)
	if near {
		cardHeight = min(160, height*0.205)
	}
	cardWidth := cardHeight * 0.7
	spreadFactor := float32(0.59)
	available := width * 0.48
	arcSize := float32(4)
	if near {
		spreadFactor = 0.74
		available = width * 0.76
		arcSize = 11
	}
	spread := min(cardWidth*spreadFactor, max(0, (available-cardWidth)/float32(max(1, count-1))))
	offset := float32(sequence) - float32(count-1)/2
	normalized := float32(0)
	if count > 1 {
		normalized = offset / (float32(count-1) / 2)
	}
	arc := (1 - normalized*normalized) * arcSize
	y := cardHeight*0.6 + 12 + arc
	angle := normalized * 0.075
	if near {
		y = height - cardHeight*0.6 - 12 - arc - cardHeight*0.10*hover
		angle = -normalized * 0.13
	}
	return handLayout{
		x: width/2 + offset*spread*(1-shuffle), y: y,
		height: cardHeight * (1 + 0.24*hover), width: cardWidth * (1 + 0.24*hover),
		angle: angle * (1 - hover),
	}
}

type handCardProps struct {
	service   *appservice.App
	code      uint32
	sequence  int
	count     int
	width     float32
	height    float32
	near      bool
	shuffle   float32
	highlight int
	onSelect  func(HandSelection)
	onHover   func(uint32)
}

func HandCard(p handCardProps) *ui.Node {
	hovered, setHovered := ui.UseState(false)
	lift := ui.UseTween(boolFloat(hovered), 110, ui.EaseOutQuad)
	pose := arenaHandLayout(p.sequence, p.count, p.width, p.height, p.near, lift, p.shuffle)
	zIndex := p.sequence + 1
	if hovered {
		zIndex += 100
	}
	accent := ui.Transparent
	if p.highlight == 1 {
		accent = ui.Hex("#f0d246")
	} else if p.highlight == 2 {
		accent = ui.Hex("#fabe3c")
	}
	return pos(pose.x-pose.width/2, pose.y-pose.height/2, pose.width, pose.height,
		ui.Style(ui.Rotate(pose.angle*180/math.Pi), ui.ZIndex(zIndex),
			ui.Shadow(ui.Color{R: 0, G: 0, B: 0, A: uint8(150 + 80*lift)}, 0, 6+8*lift, 16+12*lift, 1)),
		ui.OnHover(func(value bool) {
			if !p.near {
				setHovered(false)
				return
			}
			setHovered(value)
			if value && p.code != 0 && p.onHover != nil {
				p.onHover(p.code)
			}
		}),
		ui.OnClick(func() {
			if p.onSelect != nil {
				p.onSelect(HandSelection{Code: p.code, Sequence: p.sequence, X: pose.x, Y: pose.y})
			}
		}),
		CardArt(CardArtProps{Service: p.service, Code: p.code, Frame: ui.Rect{W: pose.width, H: pose.height}, Selected: hovered || p.highlight > 0, Accent: accent}),
	)
}

func HandFan(p HandFanProps) *ui.Node {
	shuffleStart, setShuffleStart := ui.UseState(float32(-1))
	clock := ui.UseRef(float32(0))
	shuffleRunning := shuffleStart >= 0 && *clock-shuffleStart < 0.32
	elapsed := ui.UseElapsedActive(shuffleRunning)
	*clock = elapsed
	ui.UseEffect(func() ui.Cleanup {
		if p.ShuffleRevision > 0 {
			setShuffleStart(elapsed)
		}
		return nil
	}, p.ShuffleRevision)
	shuffle := p.Shuffle
	if shuffleStart >= 0 {
		t := elapsed - shuffleStart
		switch {
		case t < 0.09:
			shuffle = max(shuffle, t/0.09)
		case t < 0.19:
			shuffle = 1
		case t < 0.32:
			shuffle = 1 - (t-0.19)/0.13
		}
	}
	children := make([]*ui.Node, 0, len(p.Cards))
	for sequence, code := range p.Cards {
		sequence, code := sequence, code
		highlight := 0
		if p.Highlight != nil {
			highlight = p.Highlight(code, sequence)
		}
		children = append(children, ui.Use(HandCard, handCardProps{
			service: p.Service, code: code, sequence: sequence, count: len(p.Cards),
			width: p.Width, height: p.Height, near: p.Near, shuffle: shuffle, highlight: highlight,
			onSelect: func(selection HandSelection) {
				if selection.Code == 0 {
					return
				}
				if p.OnSelect != nil {
					selection.X += p.X
					selection.Y += p.Y
					p.OnSelect(selection)
				}
			},
			onHover: p.OnHover,
		}))
	}
	return pos(p.X, p.Y, p.Width, p.Height, children...)
}

func boolFloat(value bool) float32 {
	if value {
		return 1
	}
	return 0
}
