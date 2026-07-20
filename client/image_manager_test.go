package client

import (
	"image"
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
