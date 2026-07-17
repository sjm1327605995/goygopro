package scenes

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// 转向的卡（守备表示 90°、对手的卡 180°）必须仍待在自己的格子里，并且仍是平躺的。
//
// 这条曾经是坏的，而且很显眼：用 tenon 的 ui.Rotate 时，旋转绕的是**场景中心**、且施加在
// 投影**之后**（pkg/ui/project3d.go 末尾），于是卡被「公转」出自己的区、前缩也被转没了 ——
// 画面上就是一堆立着的浮空卡，对手的每张卡都跑到了我方场地上。
// 现在改成在 ImageManager 里预转位图 + 节点宽高对调（见 duel_card.go 的 quarter），
// 天然是平面内旋转，这个测试就是钉住它别退回去。tenon 侧的 bug 记在 docs/tenon-needs.md 第 4 节。
//
// 量的是像素，不是我的眼睛 —— 我在这个问题上目视误判过三次，最后是靠这段代码定死的。
func TestRotatedCardStaysInItsZone(t *testing.T) {
	for _, tc := range []struct {
		name string
		pos  uint8
	}{
		{"表侧攻击（不旋转，对照组）", 0x01},
		{"表侧守备（转 90°）", 0x04},
	} {
		t.Run(tc.name, func(t *testing.T) {
			box, want := renderOneCard(t, 0, tc.pos)
			if box.Empty() {
				t.Fatal("整屏都找不到这张卡")
			}
			gotX := float32(box.Min.X+box.Max.X) / 2
			gotY := float32(box.Min.Y+box.Max.Y) / 2
			// 允许透视放大带来的几像素误差；这里要抓的是「飞出几百像素」。
			if absf(gotX-want.X) > 12 || absf(gotY-want.Y) > 12 {
				t.Errorf("卡画在 (%.0f,%.0f)，应当在它的格子里 (%.0f,%.0f) 附近 —— 偏了 (%.0f,%.0f)",
					gotX, gotY, want.X, want.Y, gotX-want.X, gotY-want.Y)
			}
			if box.Dx() <= box.Dy() {
				t.Errorf("卡渲染成 %dx%d（高≥宽）—— 平躺在 %d° 斜面上应当被前缩成扁的",
					box.Dx(), box.Dy(), tableTilt)
			}
		})
	}
}

// renderOneCard 在空场上摆一张卡，返回它在屏幕上的实际包围盒，以及它所在格子的屏幕中心。
// 卡图换成纯品红，方便从画面里精确框出来。
func renderOneCard(t *testing.T, seq uint8, pos uint8) (image.Rectangle, ui.Rect) {
	t.Helper()

	client.MainGame.DField = client.NewClientField()
	client.MainGame.DInfo = client.DuelInfo{DuelRule: 5}
	client.MainGame.Dialog = client.NewDialogState()

	magenta := image.NewRGBA(image.Rect(0, 0, 177, 254))
	for i := range magenta.Pix {
		if i%4 == 1 { // G=0，其余 255 -> 品红不透明
			continue
		}
		magenta.Pix[i] = 255
	}
	client.ImageMgr.TUnknown = magenta
	client.ImageMgr.TCover[0] = magenta
	client.ImageMgr.TMap[0] = map[int]image.Image{}

	df := client.MainGame.DField
	df.MZone[0] = make([]*client.ClientCard, 7)
	card := client.NewClientCard()
	// 卡号必须是本测试专用的：tenon 的位图按 SrcImage 的 key（"card:<code>"）缓存，
	// 同一个 key 只建一次图。跟别的测试共用卡号，拿到的就是它留下的那张灰占位图，
	// 这里的品红根本不会出现在画面上（正是本测试第一版「整屏找不到卡」的原因）。
	card.Code, card.Controler, card.Location, card.Sequence = 987654, 0, 0x04, seq
	card.Position, card.Type = pos, 0x1
	df.MZone[0][seq] = card

	img, err := ui.Screenshot(ui.Use(DuelFieldScene, struct{}{}),
		client.GameWindowWidth, client.GameWindowHeight)
	if err != nil {
		t.Skipf("需要可用的 GPU: %v", err)
	}

	// 该格子投影后的屏幕中心：拿不旋转时的渲染结果作为基准（对照组自证）。
	cx, cy := client.ZoneCenter(0, 0x04, int(seq), client.FieldRule())
	l, tp, w, h := tableRect(cx, cy)
	sceneLeft := (float32(client.GameWindowWidth) - w2()) / 2
	want := projectOnTable(l+w/2+sceneLeft, tp+h/2+tableTop)

	return magentaBBox(img), want
}

func w2() float32 { w, _ := tableSize(); return w }

// projectOnTable 把桌面上一点（窗口绝对坐标、未投影）投到屏幕，与 Scene3D 同一套数学：
// 绕场景中心倾斜 tableTilt，再做透视除法。
func projectOnTable(x, y float32) ui.Rect {
	w, h := tableSize()
	ocx := (float32(client.GameWindowWidth)-w)/2 + w/2
	ocy := tableTop + h/2
	dx, dy := x-ocx, y-ocy

	const deg = math.Pi / 180
	cosT := float32(math.Cos(tableTilt * deg))
	sinT := float32(math.Sin(tableTilt * deg))
	y2 := dy * cosT
	z2 := dy * sinT
	denom := 1 - z2/(tablePerspective*tableScale)
	if denom < 0.05 {
		denom = 0.05
	}
	return ui.Rect{X: ocx + dx/denom, Y: ocy + y2/denom}
}

func magentaBBox(img *image.RGBA) image.Rectangle {
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if c.R < 200 || c.G > 60 || c.B < 200 {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < 0 {
		return image.Rectangle{}
	}
	return image.Rectangle{Min: image.Point{minX, minY}, Max: image.Point{maxX + 1, maxY + 1}}
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
