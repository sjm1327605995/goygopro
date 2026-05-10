package client

import (
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
	TMap          [2]map[int]image.Image // [normal, big]
	TThumb        map[int]image.Image
	TFields       map[int]image.Image
	TButton       map[int]image.Image
	TButtonDefense map[int]image.Image

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
