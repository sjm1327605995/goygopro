package tenon

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"
)

type arenaRandom struct{ state uint32 }

func (r *arenaRandom) next() float64 {
	r.state = r.state*1664525 + 1013904223
	return float64(r.state) / 4294967296
}

func rect(img *image.NRGBA, x, y, w, h int, c color.NRGBA) {
	mode := draw.Over
	if c.A == 255 {
		mode = draw.Src
	}
	draw.Draw(img, image.Rect(x, y, x+w, y+h), image.NewUniform(c), image.Point{}, mode)
}

func line(img *image.NRGBA, x0, y0, x1, y1 float64, c color.NRGBA) {
	dx, dy := x1-x0, y1-y0
	steps := int(max(math.Abs(dx), math.Abs(dy)))
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(max(1, steps))
		x, y := int(math.Round(x0+dx*t)), int(math.Round(y0+dy*t))
		if image.Pt(x, y).In(img.Bounds()) {
			draw.Draw(img, image.Rect(x, y, x+1, y+1), image.NewUniform(c), image.Point{}, draw.Over)
		}
	}
}

func strokeRect(img *image.NRGBA, x, y, w, h int, c color.NRGBA) {
	for i := 0; i < 2; i++ {
		line(img, float64(x), float64(y+i), float64(x+w), float64(y+i), c)
		line(img, float64(x), float64(y+h-i), float64(x+w), float64(y+h-i), c)
		line(img, float64(x+i), float64(y), float64(x+i), float64(y+h), c)
		line(img, float64(x+w-i), float64(y), float64(x+w-i), float64(y+h), c)
	}
}

func ellipse(img *image.NRGBA, cx, cy, rx, ry, rotation float64, c color.NRGBA) {
	cos, sin := math.Cos(rotation), math.Sin(rotation)
	for y := int(cy - ry); y <= int(cy+ry); y++ {
		for x := int(cx - rx); x <= int(cx+rx); x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			px, py := dx*cos+dy*sin, -dx*sin+dy*cos
			if px*px/(rx*rx)+py*py/(ry*ry) <= 1 && image.Pt(x, y).In(img.Bounds()) {
				draw.Draw(img, image.Rect(x, y, x+1, y+1), image.NewUniform(c), image.Point{}, draw.Over)
			}
		}
	}
}

var arenaStoneImage = sync.OnceValue(func() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 820))
	random := &arenaRandom{state: 73}
	rect(img, 0, 0, 1024, 820, color.NRGBA{32, 45, 45, 255})
	for row := 0; row < 14; row++ {
		for col := -1; col < 18; col++ {
			x, y := col*64+(row%2)*32, row*64
			tone := uint8(random.next() * 12)
			base := color.NRGBA{47 + tone, 64 + tone, 65 + tone, 255}
			if y < 410 {
				base = color.NRGBA{58 + tone, 66 + tone, 59 + tone, 255}
			}
			rect(img, x+2, y+2, 60, 60, base)
			strokeRect(img, x+3, y+3, 58, 58, color.NRGBA{193, 198, 165, 27})
			if random.next() > 0.58 {
				line(img, float64(x+8), float64(y+5), float64(x+18)+random.next()*16, float64(y+24), color.NRGBA{13, 28, 36, 136})
				line(img, float64(x+18)+random.next()*16, float64(y+24), float64(x+13)+random.next()*30, float64(y+45), color.NRGBA{13, 28, 36, 136})
				line(img, float64(x+13)+random.next()*30, float64(y+45), float64(x+40), float64(y+61), color.NRGBA{13, 28, 36, 136})
			}
		}
	}
	for i := 0; i < 42000; i++ {
		c := color.NRGBA{2, 12, 23, 23}
		if random.next() > 0.5 {
			c = color.NRGBA{226, 222, 195, 10}
		}
		rect(img, int(random.next()*1024), int(random.next()*820), int(random.next()*3+1), int(random.next()*2+1), c)
	}
	for i := 0; i < 450; i++ {
		x, y := random.next()*1024, random.next()*820
		if x > 100 && x < 924 && y > 75 && y < 745 {
			continue
		}
		ellipse(img, x, y, random.next()*21+3, random.next()*12+2, random.next()*3, color.NRGBA{74, 94, 54, uint8(20 + random.next()*40)})
	}
	strokeRect(img, 18, 18, 988, 784, color.NRGBA{181, 164, 113, 101})
	strokeRect(img, 28, 28, 968, 764, color.NRGBA{181, 164, 113, 101})
	line(img, 36, 410, 988, 410, color.NRGBA{181, 164, 113, 101})
	for _, inset := range []float64{69, 57} {
		points := [][2]float64{{0, -inset}, {inset, 0}, {0, inset}, {-inset, 0}, {0, -inset}}
		for i := 0; i+1 < len(points); i++ {
			line(img, 512+points[i][0], 410+points[i][1], 512+points[i+1][0], 410+points[i+1][1], color.NRGBA{188, 167, 120, 80})
		}
	}
	for y := 0; y < 820; y++ {
		for x := 0; x < 1024; x++ {
			distance := math.Hypot(float64(x-512), float64(y-410))
			alpha := max(0, min(1, (distance-130)/510)) * 160
			factor := float32(1 - alpha/255)
			i := y*img.Stride + x*4
			img.Pix[i] = uint8(float32(img.Pix[i]) * factor)
			img.Pix[i+1] = uint8(float32(img.Pix[i+1]) * factor)
			img.Pix[i+2] = uint8(float32(img.Pix[i+2]) * factor)
		}
	}
	return img
})

var arenaGroundImage = sync.OnceValue(func() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			distance := math.Hypot(float64(x-256), float64(y-256))
			t := max(0, min(1, (distance-70)/185))
			alpha := uint8(255 * (1 - t))
			i := y*img.Stride + x*4
			img.Pix[i] = uint8(36 + (20-36)*t)
			img.Pix[i+1] = uint8(56 + (32-56)*t)
			img.Pix[i+2] = uint8(49 + (31-49)*t)
			img.Pix[i+3] = alpha
		}
	}
	return img
})
