package tenon

import "testing"

func TestArenaStoneSurfaceHasNoTransparentHoles(t *testing.T) {
	img := arenaStoneImage()
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if got := img.Pix[y*img.Stride+x*4+3]; got != 255 {
				t.Fatalf("pixel (%d,%d) alpha = %d, want 255", x, y, got)
			}
		}
	}
}
