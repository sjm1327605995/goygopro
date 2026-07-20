package client

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"sync"

	_ "image/jpeg"
	_ "image/png"
)

// ImageManager corresponds to C++ class ImageManager in image_manager.h
// Loads and caches card images, field backgrounds, and UI textures.
type ImageManager struct {
	mu sync.RWMutex

	// Caches for card images
	TMap           [2]map[int]image.Image // [normal, big]
	TThumb         map[int]image.Image
	TFields        map[int]image.Image
	TButton        map[int]image.Image
	TButtonDefense map[int]image.Image
	TRotated       map[string]image.Image // 预旋转的卡图，见 Rotated

	// System textures
	TCover             [2]image.Image
	TButtonFacedown    [2]image.Image
	TButtonFacedownDef [2]image.Image
	TUnknown           image.Image
	TUnknownFit        image.Image
	TUnknownThumb      image.Image
	TAct               image.Image
	TAttack            image.Image
	TNegated           image.Image
	TChain             image.Image
	TNumber            image.Image
	TLPFrame           image.Image
	TLPBar             image.Image
	TMask              image.Image
	TEquip             image.Image
	TTarget            image.Image
	TChainTarget       image.Image
	TLim               image.Image
	TOT                image.Image
	THand              [3]image.Image
	TBackGround        image.Image
	TBackGroundMenu    image.Image
	TBackGroundDeck    image.Image
	TField             [2]image.Image
	TFieldTransparent  [2]image.Image
}

var ImageMgr = &ImageManager{
	TMap:           [2]map[int]image.Image{make(map[int]image.Image), make(map[int]image.Image)},
	TThumb:         make(map[int]image.Image),
	TFields:        make(map[int]image.Image),
	TButton:        make(map[int]image.Image),
	TButtonDefense: make(map[int]image.Image),
	TRotated:       make(map[string]image.Image),
}

// Initial loads all system textures.
func (im *ImageManager) Initial() bool {
	im.TCover[0] = im.loadImage("textures/cover.jpg")
	im.TCover[1] = im.loadImage("textures/cover2.jpg")
	im.TBackGround = im.loadImage("textures/bg.jpg")
	im.TBackGroundMenu = im.loadImage("textures/bg_menu.jpg")
	im.TBackGroundDeck = im.loadImage("textures/bg_deck.jpg")
	im.TAct = im.loadImage("textures/act.png")
	im.TAttack = im.loadImage("textures/attack.png")
	im.TChain = im.loadImage("textures/chain.png")
	im.TChainTarget = im.loadImage("textures/chaintarget.png")
	im.TEquip = im.loadImage("textures/equip.png")
	im.TTarget = im.loadImage("textures/target.png")
	im.TUnknown = im.makePlaceholder(CardImgWidth, CardImgHeight, color.RGBA{80, 80, 80, 255})
	im.TUnknownThumb = im.makePlaceholder(CardThumbWidth, CardThumbHeight, color.RGBA{80, 80, 80, 255})

	// 场地贴图按规则分两套。注意编号：rule 0 用 field2.png、rule 1 用 field3.png
	// （C++ image_manager.cpp 的 tField[0]/tField[1]）。文件名里的数字比下标大一 ——
	// 此前照字面写成 field/field2，整体错了一位，新大师规则铺的其实是旧规则的场地图，
	// 中间那两个额外怪兽区的格子根本不存在。field.png 是更早的遗留文件，原版不再加载。
	im.TField[0] = im.loadImage("textures/field2.png")
	im.TField[1] = im.loadImage("textures/field3.png")
	im.TFieldTransparent[0] = im.loadImage("textures/field-transparent2.png")
	im.TFieldTransparent[1] = im.loadImage("textures/field-transparent3.png")

	// 决斗中的各种标记与仪表。这些字段一直声明着却没人加载，于是界面上
	// 效果被无效、连锁序号、限制卡标记、LP 条统统画不出来。
	im.TNegated = im.loadImage("textures/negated.png")
	im.TNumber = im.loadImage("textures/number.png")
	im.TLPBar = im.loadImage("textures/lp.png")
	im.TLPFrame = im.loadImage("textures/lpf.png")
	im.TMask = im.loadImage("textures/mask.png")
	im.TLim = im.loadImage("textures/lim.png")
	im.TOT = im.loadImage("textures/ot.png")

	// 猜拳的手势图（石头/剪刀/布）
	im.THand[0] = im.loadImage("textures/f1.jpg")
	im.THand[1] = im.loadImage("textures/f2.jpg")
	im.THand[2] = im.loadImage("textures/f3.jpg")
	return true
}

func (im *ImageManager) makePlaceholder(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	return img
}

func (im *ImageManager) loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

// GetTexture loads a card image by code and caches it.
func (im *ImageManager) GetTexture(code int) image.Image {
	im.mu.RLock()
	if img, ok := im.TMap[0][code]; ok {
		im.mu.RUnlock()
		return img
	}
	im.mu.RUnlock()

	path := GetCardImagePath(code)
	img := im.loadImage(path)
	if img == nil {
		return im.TUnknown
	}
	im.mu.Lock()
	im.TMap[0][code] = img
	im.mu.Unlock()
	return img
}

// Rotated 返回把 img 顺时针转 quarter 个 90° 后的图（quarter 取 0..3），并缓存。
//
// 为什么在这里转图、而不是让 GUI 去转节点：卡的朝向只有四种（0/±90/180，见
// GetCardLocation），而它们是**桌面平面内**的旋转 —— 守备表示的怪兽是平躺着横过来，
// 不是立起来。GUI 层若用 tenon 的 ui.Rotate，旋转会发生在投影之后、且绕场景中心，
// 卡会被甩出自己的格子并丢掉前缩（见 docs/tenon-needs.md 第 4 节）。
// 预转位图 + 宽高对调则天然是平面内旋转，且徽标/边框仍保持正立、命中区也正确。
//
// 必须缓存：cardFace 每帧都会被调用，177x254 的图每帧转一次、几十张卡一起，直接卡死。
func (im *ImageManager) Rotated(key string, img image.Image, quarter int) image.Image {
	quarter = ((quarter % 4) + 4) % 4
	if quarter == 0 || img == nil {
		return img
	}
	ck := fmt.Sprintf("%s:q%d", key, quarter)

	im.mu.RLock()
	if got, ok := im.TRotated[ck]; ok {
		im.mu.RUnlock()
		return got
	}
	im.mu.RUnlock()

	out := rotateQuarter(img, quarter)
	im.mu.Lock()
	im.TRotated[ck] = out
	im.mu.Unlock()
	return out
}

// rotateQuarter 顺时针转 quarter 个 90°。奇数次旋转宽高互换。
func rotateQuarter(src image.Image, quarter int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if quarter%2 == 1 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			switch quarter {
			case 1: // 顺时针 90°：源的 (x,y) -> 目标的 (h-1-y, x)
				dst.Set(h-1-y, x, c)
			case 2:
				dst.Set(w-1-x, h-1-y, c)
			case 3:
				dst.Set(y, w-1-x, c)
			}
		}
	}
	return dst
}

// GetTextureThumb returns a thumbnail image for a card.
func (im *ImageManager) GetTextureThumb(code int) image.Image {
	im.mu.RLock()
	if img, ok := im.TThumb[code]; ok {
		im.mu.RUnlock()
		return img
	}
	im.mu.RUnlock()

	path := GetCardThumbPath(code)
	img := im.loadImage(path)
	if img == nil {
		return im.TUnknownThumb
	}
	im.mu.Lock()
	im.TThumb[code] = img
	im.mu.Unlock()
	return img
}

func GetCardThumbPath(code int) string {
	return "pics/thumbnail/" + itoa(code) + ".jpg"
}

func GetCardImagePath(code int) string {
	return "pics/" + itoa(code) + ".jpg"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	n := 0
	for i > 0 {
		buf[n] = byte('0' + i%10)
		i /= 10
		n++
	}
	if neg {
		buf[n] = '-'
		n++
	}
	// reverse
	for i := 0; i < n/2; i++ {
		buf[i], buf[n-1-i] = buf[n-1-i], buf[i]
	}
	return string(buf[:n])
}

// ClearTexture 清空卡图缓存（对应 C++ ImageManager::ClearTexture）。
//
// 缓存是三个没有上限的 map：卡图、缩略图、预转过的卡图。一局决斗见到的卡越多，
// 它们只增不减 —— 一张 177x254 的 RGBA 约 180KB，几百张就是几十 MB，
// 预转的图还要再翻一倍。C++ 在关闭对局时清一次，这里照做。
//
// 系统贴图（卡背、场地、背景）不在此列：它们在 Initial 时加载，整个进程都用得着。
func (im *ImageManager) ClearTexture() {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.TMap[0] = make(map[int]image.Image)
	im.TMap[1] = make(map[int]image.Image)
	im.TThumb = make(map[int]image.Image)
	im.TFields = make(map[int]image.Image)
	im.TButton = make(map[int]image.Image)
	im.TButtonDefense = make(map[int]image.Image)
	im.TRotated = make(map[string]image.Image)
}

// CachedCount 返回当前缓存的图片张数，用于观察内存占用。
func (im *ImageManager) CachedCount() int {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return len(im.TMap[0]) + len(im.TMap[1]) + len(im.TThumb) +
		len(im.TFields) + len(im.TButton) + len(im.TButtonDefense) + len(im.TRotated)
}
