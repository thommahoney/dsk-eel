package game

import (
	"sync/atomic"

	"github.com/thommahoney/dsk-eel/controller"
)

// heldDirection is the direction the joystick is held in. The joystick
// goroutine stores it and the game loop loads it, with no lock between them.
// The zero value is a neutral stick.
type heldDirection struct {
	// The direction XORed with Dir_Neutral, which makes zero mean neutral.
	bits atomic.Int32
}

func (h *heldDirection) Load() controller.Direction {
	return controller.Direction(h.bits.Load()) ^ controller.Dir_Neutral
}

func (h *heldDirection) Store(d controller.Direction) {
	h.bits.Store(int32(d ^ controller.Dir_Neutral))
}
