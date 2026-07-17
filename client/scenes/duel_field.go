package scenes

import (
	"fmt"
	"image"

	"github.com/sjm1327605995/goygopro/client"
	"github.com/sjm1327605995/goygopro/protocol/network"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// 决斗盘 —— 伪 3D。
//
// 原版是 Irrlicht 的真 3D 场景：一台相机、每张卡是一个贴了图的四边形。这里不需要 3D 引擎：
// 卡本来就是平的，把整块桌面交给 tenon 的 Scene3D 当一台共享相机，卡牌作为直接子元素按
// C++ 的世界坐标（client.ZoneCenter）绝对定位，投影和命中测试由 Scene3D 负责，
// 点击打在卡的视觉位置上。
//
// 摆位坐标全部来自 client/field_location.go（那是 C++ 布局的直译），这里只管怎么画。
const (
	// tableTilt 是桌面俯角。原版相机的俯角约 45.7°；这里取更缓的角度是为了取景 ——
	// 角度越陡，近端手牌在画面里越占地方，而窗口上下还得留给 HUD 条。
	tableTilt = 34
	// tablePerspective 是灭点距离（px）。越小透视越夸张；这个值下远端约收窄到七成，
	// 与原版观感接近。注意它和 tableScale 一起缩放，否则桌子缩小了透视强度却没变。
	tablePerspective = 1400

	// 桌面在窗口里的取景。
	//
	// C++ 用的是偏心视锥（game.cpp: BuildProjectionMatrix(-0.90, 0.45, -0.42, 0.42, ...)），
	// 画面整体偏移，近端手牌仍在画面内。Scene3D 只能绕场景中心做对称透视，学不了偏心，
	// 所以改用最朴素的办法：把整块桌面缩小、居中，给近端手牌和上下 HUD 条留出余量。
	// 不缩放的话 wy=4.0 的手牌正好压在窗口下边缘，被切掉一半。
	tableScale = 0.78
	tableTop   = 44
)

func DuelFieldScene(_ struct{}) *ui.Node {
	// 决斗状态由网络线程整块改写，靠版本号触发重渲染，渲染时直接读 MainGame 的最新值。
	_ = UseRevision(client.MainGame.FieldRev)

	// 悬停预览的状态提到这里：卡自己知道有没有被悬停，但预览面板画在场景外面。
	hovered, setHovered := ui.UseState[*client.ClientCard](nil)
	onHoverCard := ui.UseCallback(func(card *client.ClientCard, on bool) {
		if on {
			setHovered(card)
			return
		}
		// 只有「离开的正是当前预览的那张」才清空：鼠标从 A 滑到 B 时，
		// B 的进入可能先于 A 的离开到达，不判断就会把 B 的预览误清掉。
		if hovered == card {
			setHovered(nil)
		}
	}, hovered)

	return ui.Box([]ui.StyleOpt{ui.Fill, ui.Bg(black)},
		texture("bg", client.ImageMgr.TBackGround, ui.FitCover,
			ui.Absolute, ui.Top(0), ui.Left(0), ui.Fill),

		fieldScene(onHoverCard),

		ui.Box([]ui.StyleOpt{ui.Absolute, ui.Left(8), ui.Top(56)},
			cardPreview(hovered),
		),

		// —— 平面 HUD：不进 3D，始终正对观察者 ——
		ui.Box([]ui.StyleOpt{ui.Absolute, ui.Top(0), ui.Left(0), ui.WidthPct(100)},
			playerInfoBar(1),
		),
		ui.Box([]ui.StyleOpt{ui.Absolute, ui.Bottom(0), ui.Left(0), ui.WidthPct(100), ui.Column},
			playerInfoBar(0),
			cmdBar(),
		),
		ui.Box([]ui.StyleOpt{ui.Absolute, ui.Left(8), ui.Bottom(64), ui.MaxWidth(340), ui.Column, ui.Gap(4)},
			chatOverlay(),
			ui.Use(chatBar, chatBarProps{}),
		),
		duelDialog(),
	)
}

// fieldScene 是那张倾斜的桌子。Scene3D 只对**直接子元素**生效 —— 卡牌必须直接挂在这里，
// 中间多套一层容器，那层会被当作一个整体投影，尺寸一大就失真。
func fieldScene(onHoverCard func(*client.ClientCard, bool)) *ui.Node {
	kids := []*ui.Node{fieldTexture()}
	kids = append(kids, zoneSlots()...)
	kids = append(kids, cardNodes(onHoverCard)...)

	w, h := tableSize()
	return ui.Box([]ui.StyleOpt{
		ui.Absolute, ui.Left((float32(client.GameWindowWidth) - w) / 2), ui.Top(tableTop),
		ui.Width(w), ui.Height(h),
		ui.Scene3D, ui.Perspective(tablePerspective * tableScale), ui.RotateX(tableTilt),
	}, kids...)
}

// tableSize 是桌面盒子在窗口里的像素尺寸。
func tableSize() (w, h float32) {
	fw, fh := client.FieldSize()
	return fw * tableScale, fh * tableScale
}

// tableRect 把桌面平面坐标（client.ZoneCenter 那套，以中心点表示）换算成桌面盒子内
// 左上角对齐的绝对定位。缩放是整体的：坐标和卡牌尺寸必须同步缩，否则卡会挤出格子。
func tableRect(cx, cy float32) (left, top, w, h float32) {
	cw, ch := client.CardSize()
	cw, ch = cw*tableScale, ch*tableScale
	return cx*tableScale - cw/2, cy*tableScale - ch/2, cw, ch
}

// fieldTexture 铺原版那张整幅场地图。
//
// 必须用 PlaneImage 而不是普通 Img：地板是场上最大的元素，又是卡牌落位的参照系。
// Scene3D 画内容用的是仿射，偏差随元素尺寸增长，这么大的图会被画成斜切的平行四边形，
// 格线不朝灭点收敛，卡（小元素、投影准确）就明显对不上格子。PlaneImage 用精确单应把图
// CPU 预变形一次再正着贴，且变形所用投影与卡牌同源，二者严丝合缝。
func fieldTexture() *ui.Node {
	w, h := tableSize()
	rule := client.FieldRule()
	img := client.ImageMgr.TField[rule]
	if img == nil {
		return nil
	}
	return ui.Img(ui.PlaneImage(fmt.Sprintf("field:%d", rule), img),
		ui.Style(ui.Absolute, ui.Left(0), ui.Top(0), ui.Width(w), ui.Height(h)))
}

// texture 画一张由 ImageManager 管着的贴图。
//
// 用 SrcImage 而不是 Src(路径)：贴图在 ImageManager.Initial 时就已解码在内存里，
// 再让 tenon 去读一次盘只会让首帧闪空白（Src 是异步解码，要等一次 Post 才出图）。
func texture(key string, img image.Image, fit ui.ObjectFit, opts ...ui.StyleOpt) *ui.Node {
	if img == nil {
		return nil
	}
	return ui.Img(ui.SrcImage(key, img), ui.Fit(fit), ui.Style(opts...))
}

// zoneSlots 画出怪兽/魔陷区的空格子。它们同时是「选择位置」时的点击目标。
func zoneSlots() []*ui.Node {
	rule := client.FieldRule()
	var out []*ui.Node
	for player := 0; player < 2; player++ {
		for _, z := range []struct {
			loc   uint8
			slots int
		}{{0x04, 7}, {0x08, 8}} {
			for seq := 0; seq < z.slots; seq++ {
				// 有卡的格子也画：格线是桌面的一部分，卡是压在它上面的。
				// 卡在 kids 里排在后面，自然盖住格子并接管点击。
				x, y := client.ZoneCenter(player, z.loc, seq, rule)
				out = append(out, ui.Keyed(
					fmt.Sprintf("slot-%d-%d-%d", player, z.loc, seq),
					ui.Use(zoneSlot, zoneSlotProps{
						Player: player, Location: z.loc, Sequence: seq, X: x, Y: y,
						Selectable: isPlaceSelectable(player, z.loc, seq),
					}),
				))
			}
		}
	}
	return out
}

type zoneSlotProps struct {
	Player     int
	Location   uint8
	Sequence   int
	X, Y       float32
	Selectable bool
}

func zoneSlot(p zoneSlotProps) *ui.Node {
	hovered, _, ia := ui.UseInteraction()

	face, border, bw := cardEmptyBg, ui.Hex("#ffffff29"), float32(1)
	if p.Selectable {
		face, border, bw = placePurple, white, 2
		if hovered {
			face = ui.Mix(placePurple, white, 0.3)
		}
	}

	left, top, cw, ch := tableRect(p.X, p.Y)
	style := []ui.StyleOpt{
		ui.Absolute, ui.Left(left), ui.Top(top), ui.Width(cw), ui.Height(ch),
		ui.Bg(face), ui.Border(bw, border), ui.Radius(4),
	}
	if !p.Selectable {
		return ui.Box(style)
	}
	return ui.Button(ui.Style(style...), ia,
		ui.OnClick(func() { onEmptySlotClick(p.Player, p.Location, p.Sequence) }),
	)
}

// cardNodes 把双方所有区域里的卡摊平成场景的直接子元素。
func cardNodes(onHoverCard func(*client.ClientCard, bool)) []*ui.Node {
	df := client.MainGame.DField
	var out []*ui.Node

	for player := 0; player < 2; player++ {
		lists := []struct {
			loc   uint8
			cards []*client.ClientCard
		}{
			{0x01, df.Deck[player]},
			{0x40, df.Extra[player]},
			{0x10, df.Grave[player]},
			{0x20, df.Remove[player]},
			{0x04, df.MZone[player]},
			{0x08, df.SZone[player]},
			{0x02, df.Hand[player]}, // 手牌最后：叠在最上层
		}
		for _, l := range lists {
			for _, card := range l.cards {
				if card == nil {
					continue
				}
				// key 跟着卡走（UID），不跟着位置走 —— 否则卡一换区 key 就变，
				// tenon 会当成旧节点消失、新节点出现，移动动画无从谈起。
				out = append(out, ui.Keyed(
					fmt.Sprintf("card-%d", card.UID),
					ui.Use(fieldCard, fieldCardProps{Card: card, OnHoverCard: onHoverCard}),
				))
			}
		}
	}
	return out
}

// ---- HUD ----

func playerInfoBar(player int) *ui.Node {
	g := client.MainGame
	df := g.DField

	lp := g.DInfo.StrLP[player]
	if lp == "" {
		lp = fmt.Sprintf("%d", g.DInfo.LP[player])
	}
	counts := fmt.Sprintf("卡组:%d  手牌:%d  墓地:%d  额外:%d  除外:%d",
		len(df.Deck[player]), len(df.Hand[player]), len(df.Grave[player]),
		len(df.Extra[player]), len(df.Remove[player]))

	return ui.Box([]ui.StyleOpt{
		ui.Row, ui.Gap(12), ui.Padding(4), ui.ItemsCenter, ui.Bg(infoBarBg),
	},
		ui.Box([]ui.StyleOpt{ui.Width(110)},
			ui.Text("LP: "+lp, ui.FontSize(16), ui.TextColor(white)),
		),
		ui.Text(counts, ui.FontSize(11), ui.TextColor(white)),
		ui.Box([]ui.StyleOpt{ui.Grow(1)}),
		ui.Text(fmt.Sprintf("回合 %d", g.DInfo.Turn), ui.FontSize(12), ui.TextColor(white)),
		ui.Text(phaseName(g.DInfo.Phase), ui.FontSize(12), ui.TextColor(selectedGold)),
	)
}

func cmdBar() *ui.Node {
	df := client.MainGame.DField
	var btns []*ui.Node

	switch client.MainGame.DInfo.CurMsg {
	case network.MSG_SELECT_IDLECMD:
		if df.ShowBP {
			btns = append(btns, lobbyButton("进入战斗阶段", func() { sendCmd(6) }))
		}
		if df.ShowEP {
			btns = append(btns, lobbyButton("结束回合", func() { sendCmd(7) }))
		}
		if df.ShowShuffle {
			btns = append(btns, lobbyButton("洗切手牌", func() { sendCmd(8) }))
		}
	case network.MSG_SELECT_BATTLECMD:
		if df.ShowM2 {
			btns = append(btns, lobbyButton("进入主要阶段2", func() { sendCmd(2) }))
		}
		if df.ShowEP {
			btns = append(btns, lobbyButton("结束回合", func() { sendCmd(3) }))
		}
	}

	if len(btns) == 0 {
		return nil
	}
	return ui.Box([]ui.StyleOpt{
		ui.Row, ui.Gap(8), ui.Padding(4), ui.JustifyCenter, ui.ItemsCenter, ui.Bg(blackTransparent),
	}, btns...)
}

func chatOverlay() *ui.Node {
	if client.MainGame.HideChat {
		return nil
	}
	var msgs []*ui.Node
	for i := 7; i >= 0; i-- {
		msg := client.MainGame.ChatMsg[i]
		if msg == "" {
			continue
		}
		prefix := ""
		switch t := client.MainGame.ChatType[i]; {
		case t < 4:
			prefix = fmt.Sprintf("P%d: ", t)
		case t < 8:
			prefix = "观战: "
		}
		msgs = append(msgs, ui.Keyed(fmt.Sprint(i),
			ui.Text(prefix+msg, ui.FontSize(11), ui.TextColor(white))))
	}
	if len(msgs) == 0 {
		return nil
	}
	return ui.Box([]ui.StyleOpt{
		ui.Column, ui.Gap(2), ui.Padding(6), ui.Bg(blackTransparent), ui.Radius(6),
	}, msgs...)
}

func phaseName(phase uint16) string {
	switch phase {
	case 0x01:
		return "抽卡阶段"
	case 0x02:
		return "准备阶段"
	case 0x04:
		return "主要阶段1"
	case 0x08:
		return "战斗开始"
	case 0x10:
		return "步骤阶段"
	case 0x20:
		return "伤害阶段"
	case 0x40:
		return "伤害计算"
	case 0x80:
		return "战斗结束"
	case 0x100:
		return "主要阶段2"
	case 0x200:
		return "结束阶段"
	default:
		return ""
	}
}
