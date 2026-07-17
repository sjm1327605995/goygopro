package scenes

import (
	"image/png"
	"os"
	"testing"

	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// TestShotDuelField 把决斗盘无头渲染成 PNG，用来肉眼核对伪 3D 的观感。
// 不是断言式测试：它只保证渲染不崩，图给人看。
// 运行： go test ./client/scenes -run TestShot -tags shot
func TestShotDuelField(t *testing.T) {
	if os.Getenv("SHOT") == "" {
		t.Skip("设置 SHOT=1 才生成截图")
	}
	seedDemoField()

	img, err := ui.Screenshot(ui.Use(DuelFieldScene, struct{}{}),
		client.GameWindowWidth, client.GameWindowHeight)
	if err != nil {
		t.Fatalf("截图失败（需要可用的 GPU/驱动）: %v", err)
	}
	out, err := os.Create(os.Getenv("SHOT_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		t.Fatal(err)
	}
}

// TestShotMainMenu 同上，看主菜单。注意背景图走的是 Src（异步解码），无头截图里不会出现；
// 真实运行时正常。
func TestShotMainMenu(t *testing.T) {
	if os.Getenv("SHOT") == "" {
		t.Skip("设置 SHOT=1 才生成截图")
	}
	img, err := ui.Screenshot(ui.Use(MainMenuScene, struct{}{}),
		client.GameWindowWidth, client.GameWindowHeight)
	if err != nil {
		t.Fatalf("截图失败: %v", err)
	}
	out, err := os.Create(os.Getenv("SHOT_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		t.Fatal(err)
	}
}

// seedDemoField 摆一副假的场面：双方各有怪兽、魔陷、手牌，用来看布局。
func seedDemoField() {
	df := client.MainGame.DField
	client.MainGame.DInfo.LP = [2]int{8000, 6400}
	client.MainGame.DInfo.Turn = 3
	client.MainGame.DInfo.Phase = 0x04
	client.MainGame.DInfo.DuelRule = 5

	mk := func(code uint32, controler uint8, loc uint8, seq uint8, pos uint8) *client.ClientCard {
		c := client.NewClientCard()
		c.Code, c.Controler, c.Location, c.Sequence, c.Position = code, controler, loc, seq, pos
		return c
	}
	for p := uint8(0); p < 2; p++ {
		df.MZone[p] = make([]*client.ClientCard, 7)
		df.SZone[p] = make([]*client.ClientCard, 8)
		for i := uint8(0); i < 3; i++ {
			df.MZone[p][i] = mk(10000+uint32(i), p, 0x04, i, 0x01)
		}
		df.MZone[p][3] = mk(20000, p, 0x04, 3, 0x08) // 里侧守备
		for i := uint8(0); i < 2; i++ {
			df.SZone[p][i] = mk(30000+uint32(i), p, 0x08, i, 0x01)
		}
		df.SZone[p][2] = mk(0, p, 0x08, 2, 0x08) // 盖伏
		for i := uint8(0); i < 5; i++ {
			df.Hand[p] = append(df.Hand[p], mk(40000+uint32(i), p, 0x02, i, 0x01))
		}
		for i := uint8(0); i < 4; i++ {
			df.Deck[p] = append(df.Deck[p], mk(0, p, 0x01, i, 0x02))
			df.Grave[p] = append(df.Grave[p], mk(50000+uint32(i), p, 0x10, i, 0x01))
			df.Extra[p] = append(df.Extra[p], mk(0, p, 0x40, i, 0x02))
		}
	}
}
