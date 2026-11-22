package tenon

import (
	_ "golang.org/x/image/webp"
	"image"
	"image/draw"
	"math"
	"os"
	"sync"
)

var menuArtOnce = sync.OnceValues(func() (image.Image, error) {
	file, err := os.Open("frontend/textures/blue-eyes-art.webp")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoded, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(decoded.Bounds())
	draw.Draw(img, img.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	for y := 0; y < h; y++ {
		ny := ((float64(y)+0.5)/float64(h) - 0.47) / 0.54
		for x := 0; x < w; x++ {
			nx := ((float64(x)+0.5)/float64(w) - 0.5) / 0.52
			distance := math.Hypot(nx, ny)
			alpha := 0.0
			switch {
			case distance <= 0.28:
				alpha = 1
			case distance <= 0.52:
				alpha = 1 - 0.27*(distance-0.28)/0.24
			case distance < 0.91:
				alpha = 0.73 * (0.91 - distance) / 0.39
			}
			index := y*img.Stride + x*4
			img.Pix[index+3] = uint8(float64(img.Pix[index+3])*alpha + 0.5)
		}
	}
	return img, nil
})

func blueEyesMenuArt() image.Image {
	img, _ := menuArtOnce()
	return img
}
