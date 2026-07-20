package client

import (
	"image"
	"os"
	"testing"
)

// 卡图缓存没有上限：一局决斗见过的卡越多涨得越厉害，一张 177x254 的 RGBA 约 180KB，
// 预转过的图还要再翻一倍。离开对局时必须清掉，否则连打几局就一直堆着。
func TestClearTextureFreesCardCaches(t *testing.T) {
	im := &ImageManager{
		TMap:           [2]map[int]image.Image{make(map[int]image.Image), make(map[int]image.Image)},
		TThumb:         make(map[int]image.Image),
		TFields:        make(map[int]image.Image),
		TButton:        make(map[int]image.Image),
		TButtonDefense: make(map[int]image.Image),
		TRotated:       make(map[string]image.Image),
	}
	dummy := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.TMap[0][1] = dummy
	im.TMap[1][2] = dummy
	im.TThumb[3] = dummy
	im.TRotated["card:1:q1"] = dummy
	// 系统贴图不该被清掉 —— 它们整个进程都要用
	im.TCover[0] = dummy
	im.TField[1] = dummy

	if im.CachedCount() != 4 {
		t.Fatalf("前提不成立：缓存 %d 张, want 4", im.CachedCount())
	}

	im.ClearTexture()

	if n := im.CachedCount(); n != 0 {
		t.Errorf("清理后仍缓存 %d 张", n)
	}
	if im.TCover[0] == nil || im.TField[1] == nil {
		t.Error("系统贴图（卡背/场地）被误清了 —— 它们不该随对局释放")
	}
	// 清完还要能继续用，不能留下 nil map
	im.TMap[0][9] = dummy
	im.TRotated["x"] = dummy
	if im.CachedCount() != 2 {
		t.Error("清理后缓存不可写 —— map 没有重建")
	}
}

// Initial 要把原版加载的贴图全都读进来。这些字段一直声明着却没人填，
// 于是效果被无效、连锁序号、限制卡标记、LP 条、猜拳手势统统画不出来。
//
// 场地图的编号尤其容易错：rule 0 用 field2.png、rule 1 用 field3.png，
// 文件名里的数字比下标大一。照字面写成 field/field2 就整体错位 ——
// 新大师规则会铺上旧规则的场地图，中间那两个额外怪兽区的格子根本不存在。
func TestInitialLoadsAllTextures(t *testing.T) {
	if _, err := os.Stat("../textures"); err != nil {
		t.Skipf("没有 textures 目录: %v", err)
	}
	old, _ := os.Getwd()
	os.Chdir("..")
	defer os.Chdir(old)

	im := &ImageManager{
		TMap:           [2]map[int]image.Image{make(map[int]image.Image), make(map[int]image.Image)},
		TThumb:         make(map[int]image.Image),
		TFields:        make(map[int]image.Image),
		TButton:        make(map[int]image.Image),
		TButtonDefense: make(map[int]image.Image),
		TRotated:       make(map[string]image.Image),
	}
	im.Initial()

	for name, got := range map[string]image.Image{
		"卡背":        im.TCover[0],
		"决斗背景":      im.TBackGround,
		"效果无效标记":    im.TNegated,
		"连锁序号":      im.TNumber,
		"LP 条":      im.TLPBar,
		"LP 边框":     im.TLPFrame,
		"遮罩":        im.TMask,
		"限制卡标记":     im.TLim,
		"OT 标记":     im.TOT,
		"场地(旧规则)":   im.TField[0],
		"场地(新大师规则)": im.TField[1],
		"场地透明(旧)":   im.TFieldTransparent[0],
		"场地透明(新)":   im.TFieldTransparent[1],
		"猜拳-石头":     im.THand[0],
		"猜拳-剪刀":     im.THand[1],
		"猜拳-布":      im.THand[2],
	} {
		if got == nil {
			t.Errorf("%s 没有被加载", name)
		}
	}
}

// 两套场地图必须是不同的图 —— 编号写错时两个下标会指向同一张。
func TestFieldTexturesDifferPerRule(t *testing.T) {
	if _, err := os.Stat("../textures"); err != nil {
		t.Skip("没有 textures 目录")
	}
	old, _ := os.Getwd()
	os.Chdir("..")
	defer os.Chdir(old)

	im := &ImageManager{
		TMap:           [2]map[int]image.Image{make(map[int]image.Image), make(map[int]image.Image)},
		TThumb:         make(map[int]image.Image),
		TFields:        make(map[int]image.Image),
		TButton:        make(map[int]image.Image),
		TButtonDefense: make(map[int]image.Image),
		TRotated:       make(map[string]image.Image),
	}
	im.Initial()

	a, b := im.TField[0], im.TField[1]
	if a == nil || b == nil {
		t.Fatal("场地图没加载全")
	}
	if sameImage(a, b) {
		t.Error("两套规则的场地图完全一样 —— 编号写错了")
	}

	// 只验证「两张不一样」挡不住整体错位（field/field2 也是两张不同的图），
	// 所以直接比对文件内容：rule 0 必须正好是 field2.png、rule 1 必须是 field3.png。
	for _, tc := range []struct {
		rule int
		file string
		got  image.Image
	}{
		{0, "textures/field2.png", a},
		{1, "textures/field3.png", b},
	} {
		want := im.loadImage(tc.file)
		if want == nil {
			t.Fatalf("读不到 %s", tc.file)
		}
		if !sameImage(tc.got, want) {
			t.Errorf("rule %d 用的不是 %s —— 场地图整体错位了", tc.rule, tc.file)
		}
	}
}

func sameImage(a, b image.Image) bool {
	ab, bb := a.Bounds(), b.Bounds()
	if ab != bb {
		return false
	}
	for y := ab.Min.Y; y < ab.Max.Y; y += 7 { // 抽样比对足够区分两张不同的图
		for x := ab.Min.X; x < ab.Max.X; x += 7 {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}
