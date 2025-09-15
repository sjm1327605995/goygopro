package main

import (
	"image/color"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
)

// InputField 表示一个输入框组件
type InputField struct {
	text        string        // 输入的文本
	x, y        float64       // 位置坐标
	width       float64       // 宽度
	height      float64       // 高度
	active      bool          // 是否激活（获得焦点）
	cursorPos   int           // 光标位置
	lastBlink   time.Time     // 上次光标闪烁时间
	cursorVisible bool        // 光标是否可见
	font        font.Face     // 字体
	borderColor color.Color   // 边框颜色
	bgColor     color.Color   // 背景颜色
	textColor   color.Color   // 文本颜色
}

// NewInputField 创建一个新的输入框
func NewInputField(x, y, width, height float64, face font.Face) *InputField {
	return &InputField{
		x:           x,
		y:           y,
		width:       width,
		height:      height,
		font:        face,
		borderColor: color.RGBA{0, 0, 0, 255},
		bgColor:     color.RGBA{255, 255, 255, 255},
		textColor:   color.RGBA{0, 0, 0, 255},
		lastBlink:   time.Now(),
		cursorVisible: true,
	}
}

// Update 更新输入框状态
func (i *InputField) Update() {
	// 处理光标闪烁（每500毫秒切换一次可见性）
	if i.active && time.Since(i.lastBlink) > 500*time.Millisecond {
		i.cursorVisible = !i.cursorVisible
		i.lastBlink = time.Now()
	}
}

// Draw 绘制输入框
func (i *InputField) Draw(screen *ebiten.Image) {
	// 绘制背景
	ebitenutil.DrawRect(screen, i.x, i.y, i.width, i.height, i.bgColor)

	// 绘制边框（激活时边框颜色变深）
	borderColor := i.borderColor
	if i.active {
		borderColor = color.RGBA{0, 0, 255, 255}
	}
	ebitenutil.DrawLine(screen, i.x, i.y, i.x+i.width, i.y, borderColor) // 上
	ebitenutil.DrawLine(screen, i.x, i.y+i.height, i.x+i.width, i.y+i.height, borderColor) // 下
	ebitenutil.DrawLine(screen, i.x, i.y, i.x, i.y+i.height, borderColor) // 左
	ebitenutil.DrawLine(screen, i.x+i.width, i.y, i.x+i.width, i.y+i.height, borderColor) // 右

	// 绘制文本
	textX := i.x + 5 // 左边距
	textY := i.y + i.height - 5 // 底部边距（调整垂直位置）
	text.Draw(screen, i.text, i.font, int(textX), int(textY), i.textColor)

	// 绘制光标（仅当激活且可见时）
	if i.active && i.cursorVisible {
		// 计算光标位置
		cursorX := textX
		if i.cursorPos > 0 && i.cursorPos <= len(i.text) {
			// 测量光标前的文本宽度
			preText := i.text[:i.cursorPos]
			cursorX += float64(font.MeasureString(i.font, preText).Ceil())
		}

		// 绘制光标
		cursorHeight := i.height - 10
		cursorY := i.y + 5
		ebitenutil.DrawLine(screen, cursorX, cursorY, cursorX, cursorY+cursorHeight, i.textColor)
	}
}

// HandleMouse 处理鼠标事件
func (i *InputField) HandleMouse(x, y int) {
	// 检查鼠标是否点击了输入框区域
	inBounds := float64(x) >= i.x && float64(x) <= i.x+i.width &&
		float64(y) >= i.y && float64(y) <= i.y+i.height

	if inBounds {
		i.active = true
		// 重置光标状态
		i.cursorVisible = true
		i.lastBlink = time.Now()
	} else {
		i.active = false
	}
}

// HandleKey 处理键盘事件
func (i *InputField) HandleKey(r rune) {
	if !i.active {
		return
	}

	// 处理退格键
	if r == 0x08 { // 退格的ASCII码
		if i.cursorPos > 0 {
			// 删除光标前的字符
			i.text = i.text[:i.cursorPos-1] + i.text[i.cursorPos:]
			i.cursorPos--
		}
		return
	}

	// 处理回车键
	if r == 0x0D { // 回车的ASCII码
		// 可以在这里添加处理回车的逻辑
		return
	}

	// 忽略控制字符
	if r < 0x20 {
		return
	}

	// 插入字符到光标位置
	i.text = i.text[:i.cursorPos] + string(r) + i.text[i.cursorPos:]
	i.cursorPos++
}

// HandleSpecialKey 处理特殊按键（左右方向键）
func (i *InputField) HandleSpecialKey(key ebiten.Key) {
	if !i.active {
		return
	}

	switch key {
	case ebiten.KeyLeft:
		if i.cursorPos > 0 {
			i.cursorPos--
		}
	case ebiten.KeyRight:
		if i.cursorPos < len(i.text) {
			i.cursorPos++
		}
	}
}

// Game 实现ebiten.Game接口
type Game struct {
	inputField *InputField
	font       font.Face
}

func (g *Game) Update() error {
	g.inputField.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// 填充背景色
	screen.Fill(color.RGBA{240, 240, 240, 255})

	// 绘制输入框
	g.inputField.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 640, 480
}

func main() {
	// 加载默认字体（这里使用Ebiten提供的默认字体）
	// 实际项目中建议使用自定义字体
	fontFace, _, err := ebitenutil.GetFont("wqy-microhei.ttc")
	if err != nil {
		panic(err)
	}

	// 创建输入框
	inputField := NewInputField(100, 200, 300, 40, fontFace)

	// 设置游戏
	game := &Game{
		inputField: inputField,
		font:       fontFace,
	}

	// 配置窗口
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Ebiten Input Field Example")

	// 处理输入事件
	ebiten.SetMouseButtonHandler(func(button ebiten.MouseButton, action ebiten.Action, x, y int) {
		if button == ebiten.MouseButtonLeft && action == ebiten.Press {
			game.inputField.HandleMouse(x, y)
		}
	})

	ebiten.SetKeyDownHandler(func(key ebiten.Key) {
		game.inputField.HandleSpecialKey(key)
	})

	ebiten.SetRuneHandler(func(r rune) {
		game.inputField.HandleKey(r)
	})

	// 启动游戏
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
package main

import (
"image/color"
"strings"
"time"

"github.com/hajimehoshi/ebiten/v2"
"github.com/hajimehoshi/ebiten/v2/ebitenutil"
"github.com/hajimehoshi/ebiten/v2/text"
"golang.org/x/image/font"
)

// InputField 表示一个输入框组件
type InputField struct {
	text        string        // 输入的文本
	x, y        float64       // 位置坐标
	width       float64       // 宽度
	height      float64       // 高度
	active      bool          // 是否激活（获得焦点）
	cursorPos   int           // 光标位置
	lastBlink   time.Time     // 上次光标闪烁时间
	cursorVisible bool        // 光标是否可见
	font        font.Face     // 字体
	borderColor color.Color   // 边框颜色
	bgColor     color.Color   // 背景颜色
	textColor   color.Color   // 文本颜色
}

// NewInputField 创建一个新的输入框
func NewInputField(x, y, width, height float64, face font.Face) *InputField {
	return &InputField{
		x:           x,
		y:           y,
		width:       width,
		height:      height,
		font:        face,
		borderColor: color.RGBA{0, 0, 0, 255},
		bgColor:     color.RGBA{255, 255, 255, 255},
		textColor:   color.RGBA{0, 0, 0, 255},
		lastBlink:   time.Now(),
		cursorVisible: true,
	}
}

// Update 更新输入框状态
func (i *InputField) Update() {
	// 处理光标闪烁（每500毫秒切换一次可见性）
	if i.active && time.Since(i.lastBlink) > 500*time.Millisecond {
		i.cursorVisible = !i.cursorVisible
		i.lastBlink = time.Now()
	}
}

// Draw 绘制输入框
func (i *InputField) Draw(screen *ebiten.Image) {
	// 绘制背景
	ebitenutil.DrawRect(screen, i.x, i.y, i.width, i.height, i.bgColor)

	// 绘制边框（激活时边框颜色变深）
	borderColor := i.borderColor
	if i.active {
		borderColor = color.RGBA{0, 0, 255, 255}
	}
	ebitenutil.DrawLine(screen, i.x, i.y, i.x+i.width, i.y, borderColor) // 上
	ebitenutil.DrawLine(screen, i.x, i.y+i.height, i.x+i.width, i.y+i.height, borderColor) // 下
	ebitenutil.DrawLine(screen, i.x, i.y, i.x, i.y+i.height, borderColor) // 左
	ebitenutil.DrawLine(screen, i.x+i.width, i.y, i.x+i.width, i.y+i.height, borderColor) // 右

	// 绘制文本
	textX := i.x + 5 // 左边距
	textY := i.y + i.height - 5 // 底部边距（调整垂直位置）
	text.Draw(screen, i.text, i.font, int(textX), int(textY), i.textColor)

	// 绘制光标（仅当激活且可见时）
	if i.active && i.cursorVisible {
		// 计算光标位置
		cursorX := textX
		if i.cursorPos > 0 && i.cursorPos <= len(i.text) {
			// 测量光标前的文本宽度
			preText := i.text[:i.cursorPos]
			cursorX += float64(font.MeasureString(i.font, preText).Ceil())
		}

		// 绘制光标
		cursorHeight := i.height - 10
		cursorY := i.y + 5
		ebitenutil.DrawLine(screen, cursorX, cursorY, cursorX, cursorY+cursorHeight, i.textColor)
	}
}

// HandleMouse 处理鼠标事件
func (i *InputField) HandleMouse(x, y int) {
	// 检查鼠标是否点击了输入框区域
	inBounds := float64(x) >= i.x && float64(x) <= i.x+i.width &&
		float64(y) >= i.y && float64(y) <= i.y+i.height

	if inBounds {
		i.active = true
		// 重置光标状态
		i.cursorVisible = true
		i.lastBlink = time.Now()
	} else {
		i.active = false
	}
}

// HandleKey 处理键盘事件
func (i *InputField) HandleKey(r rune) {
	if !i.active {
		return
	}

	// 处理退格键
	if r == 0x08 { // 退格的ASCII码
		if i.cursorPos > 0 {
			// 删除光标前的字符
			i.text = i.text[:i.cursorPos-1] + i.text[i.cursorPos:]
			i.cursorPos--
		}
		return
	}

	// 处理回车键
	if r == 0x0D { // 回车的ASCII码
		// 可以在这里添加处理回车的逻辑
		return
	}

	// 忽略控制字符
	if r < 0x20 {
		return
	}

	// 插入字符到光标位置
	i.text = i.text[:i.cursorPos] + string(r) + i.text[i.cursorPos:]
	i.cursorPos++
}

// HandleSpecialKey 处理特殊按键（左右方向键）
func (i *InputField) HandleSpecialKey(key ebiten.Key) {
	if !i.active {
		return
	}

	switch key {
	case ebiten.KeyLeft:
		if i.cursorPos > 0 {
			i.cursorPos--
		}
	case ebiten.KeyRight:
		if i.cursorPos < len(i.text) {
			i.cursorPos++
		}
	}
}

// Game 实现ebiten.Game接口
type Game struct {
	inputField *InputField
	font       font.Face
}

func (g *Game) Update() error {
	g.inputField.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// 填充背景色
	screen.Fill(color.RGBA{240, 240, 240, 255})

	// 绘制输入框
	g.inputField.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 640, 480
}

func main() {
	// 加载默认字体（这里使用Ebiten提供的默认字体）
	// 实际项目中建议使用自定义字体
	fontFace, _, err := ebitenutil.GetFont("wqy-microhei.ttc")
	if err != nil {
		panic(err)
	}

	// 创建输入框
	inputField := NewInputField(100, 200, 300, 40, fontFace)

	// 设置游戏
	game := &Game{
		inputField: inputField,
		font:       fontFace,
	}

	// 配置窗口
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Ebiten Input Field Example")

	// 处理输入事件
	ebiten.SetMouseButtonHandler(func(button ebiten.MouseButton, action ebiten.Action, x, y int) {
		if button == ebiten.MouseButtonLeft && action == ebiten.Press {
			game.inputField.HandleMouse(x, y)
		}
	})

	ebiten.SetKeyDownHandler(func(key ebiten.Key) {
		game.inputField.HandleSpecialKey(key)
	})

	ebiten.SetRuneHandler(func(r rune) {
		game.inputField.HandleKey(r)
	})

	// 启动游戏
	if err := ebiten.RunGame(game); err != nil {
		panic(err)
	}
}
