package game

import (
	"sync/atomic"

	"github.com/thommahoney/dsk-eel/controller"
)

// inputSnapshot is the input state at one moment: the direction the stick
// is held in, whether this game is a demo and whether the joystick is lost.
// It is a plain value: nothing that happens to the game afterwards changes
// it.
//
// The direction is in the low four bits, XORed with Dir_Neutral so that
// the zero value is a neutral stick in a game that is not a demo, with the
// joystick there. Four bits hold every direction: the joystick reports two
// bits for each axis of the stick, and the keyboard the same values.
type inputSnapshot uint32

const (
	inputDirectionBits inputSnapshot = 0b1111
	inputDemo          inputSnapshot = 1 << 4
	inputJoystickLost  inputSnapshot = 1 << 5
)

// stickHeld is the state that an input leaves: the stick held in dir,
// which may be neutral, in a game that is not a demo, with the joystick
// there.
func stickHeld(dir controller.Direction) inputSnapshot {
	return inputSnapshot(0).withDirection(dir)
}

// direction is the direction the stick is held in.
func (in inputSnapshot) direction() controller.Direction {
	return controller.Direction(in&inputDirectionBits) ^ controller.Dir_Neutral
}

// demo is whether this game is a demo. Run sets it from Config.DemoMode
// for every game and the first input clears it.
func (in inputSnapshot) demo() bool {
	return in&inputDemo != 0
}

// joystickLost is whether the joystick is lost, which makes every game a
// demo whatever the configuration says. It is cleared by the first input
// from a joystick that has come back, not by its return.
func (in inputSnapshot) joystickLost() bool {
	return in&inputJoystickLost != 0
}

// inDemoMode reports whether the eel steers itself at junctions while the
// stick is neutral: in a demo game, and in any game from the loss of the
// joystick until the first input after it is back.
func (in inputSnapshot) inDemoMode() bool {
	return in&(inputDemo|inputJoystickLost) != 0
}

// withDirection is the same state with the stick held in dir.
func (in inputSnapshot) withDirection(dir controller.Direction) inputSnapshot {
	return in&^inputDirectionBits | inputSnapshot(dir^controller.Dir_Neutral)&inputDirectionBits
}

// withDemo is the same state in a game that is a demo, or is not.
func (in inputSnapshot) withDemo(demo bool) inputSnapshot {
	if demo {
		return in | inputDemo
	}
	return in &^ inputDemo
}

// withJoystickLost is the state that losing the joystick leaves: nothing
// held and the joystick lost. Whether the game is a demo stays as it is.
func (in inputSnapshot) withJoystickLost() inputSnapshot {
	return (in | inputJoystickLost).withDirection(controller.Dir_Neutral)
}

// inputState is the input state of a game, shared with no lock between the
// goroutines that change it (the joystick's or the keyboard's, and Run's)
// and the game loop, which reads it on every tick.
//
// The stick, demo mode and the lost joystick are one atomic word and not
// three, because a tick decides by them together: a neutral stick ends the
// game at a junction unless the eel is steering itself. With three words a
// tick could load the stick before an input and the flags after it, and
// end the game on a combination that was never the state. No order of the
// stores, and no order of the loads, rules that out for every writer. With
// one word a tick loads a state that some writer left, whole.
//
// So a reader calls Load once and decides everything by that snapshot, and
// a writer replaces the whole word in one step: with Swap if the new state
// does not depend on the old one, and with Update if it does. A writer
// must never Load and then Swap, which would undo whatever another
// goroutine wrote in between.
type inputState struct {
	word atomic.Uint32
}

// Load returns the state as it is at this moment.
func (s *inputState) Load() inputSnapshot {
	return inputSnapshot(s.word.Load())
}

// Swap puts next in place of the state, whatever that was, and returns
// what it was.
func (s *inputState) Swap(next inputSnapshot) (old inputSnapshot) {
	return inputSnapshot(s.word.Swap(uint32(next)))
}

// Update puts change(old) in place of the state and returns old, the state
// it replaced. If another goroutine writes between the load and the swap,
// the swap does not happen and Update starts again from what was written,
// so neither write is lost. change is therefore called once for every try,
// and must do nothing but work out the new state.
func (s *inputState) Update(change func(inputSnapshot) inputSnapshot) (old inputSnapshot) {
	for {
		old = s.Load()
		if s.word.CompareAndSwap(uint32(old), uint32(change(old))) {
			return old
		}
	}
}
