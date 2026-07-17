package scenes

import (
	"fmt"
	"image"
	"math"

	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

type fieldCardProps struct {
	Card *client.ClientCard
	// OnHoverCard 把悬停事件报给决斗盘，用来显示大图预览。
	OnHoverCard func(card *client.ClientCard, on bool)
}

// fieldCard 是桌上的一张卡。它是 Scene3D 的直接子元素，故自带透视：
// 摆位来自 C++ 的布局直译（GetCardLocation），朝向叠在相机角度之上。
func fieldCard(p fieldCardProps) *ui.Node {
	card := p.Card
	// 没用 UseInteraction：它内部就占了 OnHover，而这里还要把悬停转报给父级
	// （hostProps.onHover 只有一个槽，两边都挂会互相覆盖）。
	hovered, setHovered := ui.UseState(false)

	x, y, rotZ, faceUp := client.MainGame.DField.GetCardLocation(card)

	// 卡的朝向只有四种（0/±90/180），且是**桌面平面内**的旋转：守备表示的怪兽平躺着横过来，
	// 不是立起来。这里不用 tenon 的 ui.Rotate —— 它在 Scene3D 里绕场景中心、且在投影之后
	// 施加，卡会被甩出格子还丢掉前缩（docs/tenon-needs.md 第 4 节）。
	// 改为：把卡图预转好（ImageMgr.Rotated），节点自身只对调宽高。这天然就是平面内旋转，
	// 而且徽标、边框、命中区都保持正立正确。
	quarter := quarterTurns(rotZ)
	left, top, cw, ch := tableRect(x, y)
	if quarter%2 == 1 {
		cw, ch = ch, cw
		left, top = x*tableScale-cw/2, y*tableScale-ch/2
	}

	// 手牌立起来朝向观察者：桌面被 RotateX(tableTilt) 压倒了，子元素的角度是叠加上去的，
	// 减掉同样的角度就正好把手牌扳回正面 —— 原版里手牌也是竖在身前的。
	tiltBack := float32(0)
	if card.Location == 0x02 {
		tiltBack = -tableTilt
	}

	// 悬停/选中时朝观察者浮起。TranslateZ 不进布局，卡不会挤动邻居。
	lift := float32(0)
	switch {
	case card.IsSelected:
		lift = 24
	case hovered && (card.IsSelectable || card.CmdFlag != 0):
		lift = 16
	}
	z := ui.UseTween(lift, 160, ui.EaseOut)

	border, bw := ui.Hex("#ffffff4f"), float32(1)
	switch {
	case card.IsSelected:
		border, bw = selectedGold, 3
	case card.IsSelectable:
		border, bw = selectableCyan, 2
	case card.CmdFlag != 0:
		border, bw = cmdActivateColor, 2
	}

	style := []ui.StyleOpt{
		ui.Absolute, ui.Left(left), ui.Top(top), ui.Width(cw), ui.Height(ch),
		ui.RotateX(tiltBack), ui.TranslateZ(z),
		ui.Border(bw, border), ui.Radius(3), ui.Clip,
		ui.Bg(cardFaceColor(card, faceUp)),
		// FLIP：卡换区时 Left/Top 变了，Animated 让它滑过去而不是瞬移。
		// 节点身份靠 UID 保持（见 cardNodes），tenon 才认得出「还是这张卡」。
		ui.Animated,
	}

	kids := []*ui.Node{cardFace(card, faceUp, quarter)}
	if marks := cardMarks(card); marks != nil {
		kids = append(kids, marks)
	}
	kids = append(kids,
		ui.OnHover(func(on bool) {
			setHovered(on)
			if p.OnHoverCard != nil {
				p.OnHoverCard(card, on)
			}
		}),
		ui.OnClick(func() {
			onCardClick(card, int(card.Controler), card.Location, int(card.Sequence))
		}),
	)

	return ui.Button(append([]*ui.Node{ui.Style(style...)}, kids...)...)
}

// cardFace 画卡图。背面盖放时用卡背贴图，正面用卡图；卡图缺失时 ImageManager 会给灰底占位。
// quarter 是卡在桌面平面内的朝向（0..3 个 90°）：图预先转好，节点自身不做旋转。
func cardFace(card *client.ClientCard, faceUp bool, quarter int) *ui.Node {
	var img image.Image
	var key string
	if faceUp && card.Code != 0 {
		img, key = client.ImageMgr.GetTexture(int(card.Code)), fmt.Sprintf("card:%d", card.Code)
	} else {
		img, key = client.ImageMgr.TCover[0], "card:cover"
	}
	// key 必须带上角度：tenon 按 key 缓存位图，同一 key 只建一次图，
	// 不区分角度的话转过的图会和没转的互相顶掉。
	if quarter%4 != 0 {
		img = client.ImageMgr.Rotated(key, img, quarter)
		key = fmt.Sprintf("%s:q%d", key, ((quarter%4)+4)%4)
	}
	if img == nil {
		// 连卡背都没有：退化成一行卡号，至少能看出这里有张卡。
		return ui.Box([]ui.StyleOpt{ui.Fill, ui.ItemsCenter, ui.JustifyCenter},
			ui.Text(fmt.Sprint(card.Code), ui.FontSize(8), ui.TextColor(white)))
	}
	// SrcImage 而非 Src：卡图由 ImageManager 统一管（缓存、占位、将来从压缩包取），
	// 这里只是把已在内存里的那张交给 tenon，不再让它去读一次盘。
	return ui.Img(ui.SrcImage(key, img), ui.Fit(ui.FitCover),
		ui.Style(ui.Absolute, ui.Left(0), ui.Top(0), ui.Fill))
}

// cardMarks 是叠在卡上的标记：可做的操作、被指示/装备/连锁的目标、攻守数值。
func cardMarks(card *client.ClientCard) *ui.Node {
	var badges []*ui.Node
	add := func(text string, c ui.Color) {
		badges = append(badges, ui.Box(
			[]ui.StyleOpt{ui.Bg(c), ui.Radius(2), ui.PaddingXY(2, 0)},
			ui.Text(text, ui.FontSize(8), ui.TextColor(white)),
		))
	}
	if card.CmdFlag&client.CommandSummon != 0 {
		add("召", cmdSummonColor)
	}
	if card.CmdFlag&client.CommandActivate != 0 {
		add("发", cmdActivateColor)
	}
	if card.CmdFlag&client.CommandAttack != 0 {
		add("攻", cmdAttackColor)
	}
	if card.IsShowEquip {
		add("装", equipColor)
	}
	if card.IsShowTarget {
		add("标", targetColor)
	}
	if card.IsShowChainTarget {
		add("锁", chainColor)
	}
	if len(badges) == 0 {
		return nil
	}
	return ui.Box([]ui.StyleOpt{
		ui.Absolute, ui.Left(2), ui.Bottom(2), ui.Row, ui.Gap(1),
	}, badges...)
}

// cardFaceColor 是卡图没加载出来时的底色，按卡种区分（怪兽/魔法/陷阱/盖放）。
func cardFaceColor(card *client.ClientCard, faceUp bool) ui.Color {
	if !faceUp {
		return cardSetBg
	}
	switch {
	case card.Type&0x4 != 0: // TYPE_TRAP
		return cardTrapBg
	case card.Type&0x2 != 0: // TYPE_SPELL
		return cardSpellBg
	default:
		return cardMonsterBg
	}
}

// quarterTurns 把 GetCardLocation 给的弧度换成顺时针的 90° 次数（0..3）。
// 卡的朝向只可能是 0、±90、180，所以四舍五入到整数圈是精确的、不是近似。
func quarterTurns(rad float32) int {
	q := int(math.Round(float64(rad) / (math.Pi / 2)))
	return ((q % 4) + 4) % 4
}
