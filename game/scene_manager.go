package game

import (
	"github.com/TotallyGamerJet/clay"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"os"
)

// Scene 定义场景接口，所有具体场景都需要实现这些方法
type Scene interface {
	// Update 处理场景逻辑更新，返回错误表示需要终止
	Update() error
	// Draw 负责场景的渲染
	Draw(screen *ebiten.Image)
	// OnEnter 当场景被激活时调用（进入场景时）
	OnEnter()
	// OnExit 当场景被切换出去时调用（离开场景时）
	OnExit()
}

// GameState 包含游戏的共享状态，提供给场景使用
type GameState struct {
	SceneManager *SceneManager
	// 可以添加其他需要在场景间共享的数据

}

// SceneManager 场景管理器，负责场景的切换、历史管理和回退
type SceneManager struct {
	current Scene   // 当前活跃的场景
	history []Scene // 场景历史栈，保存之前的场景

}

var (
	Fonts       []text.Face
	FontSource  *text.GoTextFaceSource
	ScaleFactor float32
	Cmds        clay.RenderCommandArray
	Width       float64
	Height      float64
)

// NewSceneManager 创建一个新的场景管理器
func NewSceneManager(initialScene Scene) *SceneManager {
	manager := &SceneManager{
		history: make([]Scene, 0),
	}
	manager.current = initialScene
	manager.current.OnEnter()
	return manager
}

// Update 更新当前场景
func (s *SceneManager) Update() error {
	if s.current == nil {
		return nil // 没有当前场景，无需更新
	}

	// 创建游戏状态并传递给当前场景
	return s.current.Update()
}

// Draw 绘制当前场景
func (s *SceneManager) Draw(screen *ebiten.Image) {
	if s.current == nil {
		return // 没有当前场景，无需绘制
	}

	// 直接绘制当前场景
	s.current.Draw(screen)
}

// GoTo 切换到指定场景，将当前场景压入历史栈
func (s *SceneManager) GoTo(scene Scene) {
	if s.current != nil {
		// 通知当前场景即将退出
		s.current.OnExit()
		// 将当前场景压入历史栈
		s.history = append(s.history, s.current)
	}

	// 切换到新场景
	s.current = scene
	s.current.OnEnter()
}

// Replace 替换当前场景，不将当前场景压入历史栈
// 适用于不希望用户回退到当前场景的情况
func (s *SceneManager) Replace(scene Scene) {
	if s.current != nil {
		s.current.OnExit() // 仅执行退出逻辑，不保存到历史
	}

	// 切换到新场景
	s.current = scene
	s.current.OnEnter()
}

// GoBack 回退到上一个场景
// 返回值表示是否成功回退
func (s *SceneManager) GoBack() {
	if len(s.history) == 0 {
		os.Exit(0)
		return
	}

	// 通知当前场景退出
	s.current.OnExit()

	// 从历史栈获取上一个场景（栈顶元素）
	lastIdx := len(s.history) - 1
	prevScene := s.history[lastIdx]

	// 移除栈顶元素
	s.history = s.history[:lastIdx]

	// 切换到上一个场景
	s.current = prevScene
	s.current.OnEnter()

}

// ClearHistory 清空场景历史
func (s *SceneManager) ClearHistory() {
	s.history = make([]Scene, 0)
}

// CurrentScene 获取当前活跃的场景
func (s *SceneManager) CurrentScene() Scene {
	return s.current
}
