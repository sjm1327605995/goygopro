package scenes

import "image/color"

var (
	white           = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	gray            = color.RGBA{R: 128, G: 128, B: 128, A: 255}
	black           = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	blackTransparent = color.RGBA{R: 0, G: 0, B: 0, A: 180}
	cardGray        = color.RGBA{R: 200, G: 200, B: 200, A: 255}

	// Card colors — functional, not shadcn-styled
	cardMonsterBg   = color.RGBA{R: 50, G: 80, B: 140, A: 255}
	cardSpellBg     = color.RGBA{R: 60, G: 110, B: 70, A: 255}
	cardTrapBg      = color.RGBA{R: 120, G: 60, B: 110, A: 255}
	cardSetBg       = color.RGBA{R: 35, G: 35, B: 35, A: 255}
	cardEmptyBg     = color.RGBA{R: 0, G: 0, B: 0, A: 60}

	// Selection / interaction colors
	selectedGold    = color.RGBA{R: 255, G: 200, B: 50, A: 255}
	selectableCyan  = color.RGBA{R: 50, G: 220, B: 255, A: 255}
	placePurple     = color.RGBA{R: 150, G: 100, B: 255, A: 220}
	cmdSummonColor  = color.RGBA{R: 50, G: 200, B: 100, A: 255}
	cmdAttackColor  = color.RGBA{R: 200, G: 80, B: 50, A: 255}
	cmdActivateColor = color.RGBA{R: 50, G: 150, B: 255, A: 255}

	// Status indicator colors
	equipColor      = color.RGBA{R: 200, G: 150, B: 50, A: 255}
	targetColor     = color.RGBA{R: 200, G: 50, B: 50, A: 255}
	chainColor      = color.RGBA{R: 150, G: 50, B: 200, A: 255}

	// UI accents
	infoBarBg       = color.RGBA{R: 0, G: 0, B: 0, A: 160}
	modalOverlay    = color.RGBA{R: 0, G: 0, B: 0, A: 200}
)
