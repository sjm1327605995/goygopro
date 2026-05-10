package client

import "github.com/sjm1327605995/tenon/pkg/engine"

// PushScene navigates to the named route via the global navigator key.
// Safe to call from non-UI callbacks (network handlers, etc.).
func PushScene(name string) {
	key := MainGame.GetNavigatorKey()
	if key == nil {
		return
	}
	state := key.CurrentState()
	if state == nil {
		return
	}
	if nav, ok := state.(engine.NavigatorState); ok {
		nav.Push(name)
	}
}

// PopScene pops the current route via the global navigator key.
func PopScene() {
	key := MainGame.GetNavigatorKey()
	if key == nil {
		return
	}
	state := key.CurrentState()
	if state == nil {
		return
	}
	if nav, ok := state.(engine.NavigatorState); ok {
		nav.Pop()
	}
}
