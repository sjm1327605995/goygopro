package duel

import (
	"github.com/antlabs/timer"
)

var timerWheel timer.Timer

func init() {
	timerWheel = timer.NewTimer(timer.WithTimeWheel())
	go timerWheel.Run()
}
