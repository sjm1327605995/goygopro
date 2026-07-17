package scenes

import ui "github.com/sjm1327605995/tenon/pkg/ui"

// 配色取自 ygopro 原版界面（source/ygopro/gframe）。
var (
	white            = ui.Hex("#ffffff")
	gray             = ui.Hex("#808080")
	black            = ui.Hex("#000000")
	blackTransparent = ui.Hex("#000000b3")
	cardGray         = ui.Hex("#c7c7c7")

	// 卡片底色（无卡图时的占位）
	cardMonsterBg = ui.Hex("#334f8c")
	cardSpellBg   = ui.Hex("#3d6e45")
	cardTrapBg    = ui.Hex("#783d6e")
	cardSetBg     = ui.Hex("#242424")
	cardEmptyBg   = ui.Hex("#0000003d")

	// 选中 / 可选中
	selectedGold     = ui.Hex("#ffc733")
	selectableCyan   = ui.Hex("#33dbff")
	placePurple      = ui.Hex("#9663ffdb")
	cmdSummonColor   = ui.Hex("#33c763")
	cmdAttackColor   = ui.Hex("#c74f33")
	cmdActivateColor = ui.Hex("#3396ff")

	// 状态标记
	equipColor  = ui.Hex("#c7963d")
	targetColor = ui.Hex("#c73333")
	chainColor  = ui.Hex("#9633c7")

	// 界面
	infoBarBg    = ui.Hex("#000000a1")
	modalOverlay = ui.Hex("#000000c7")
	titleBarBlue = ui.Hex("#294a7a")
	windowBg     = ui.Hex("#d1d1d1")
)
