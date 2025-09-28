package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/joelschutz/stagehand"
)

type State int

type BaseScene struct {
	count State
	sm    *stagehand.SceneManager[State]
}

func (s *BaseScene) Layout(w, h int) (int, int) {
	return w, h
}

func (s *BaseScene) Load(st State, sm stagehand.SceneController[State]) {
	s.count = st
	s.sm = sm.(*stagehand.SceneManager[State])
}

func (s *BaseScene) Unload() State {
	return s.count
}

type FirstScene struct {
	BaseScene
}

func (s *FirstScene) Update() error {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		s.count++
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		//s.sm.SwitchTo(&SecondScene{})
	}
	return nil
}

func (s *FirstScene) Draw(screen *ebiten.Image) {

}
