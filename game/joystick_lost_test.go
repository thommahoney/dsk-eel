package game

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/controller"
)

// From anywhere on a segment the next junction is at most 21 moves away,
// and the game-over animation of an eel that has eaten twice is 10 frames.
// A neutral-stick game that sends this many more frames and is still
// running is steering itself.
const framesPastNextJunction = 60

// wantSelfSteering fails unless the running game gets past its next
// junction with nobody steering.
func wantSelfSteering(t *testing.T, sink *artnetSink, done <-chan struct{}) {
	t.Helper()
	for frames := 0; frames < framesPastNextJunction; frames++ {
		if _, ok := sink.frame(2 * time.Second); !ok {
			t.Fatalf("the game stopped after %d frames: the eel died at a junction", frames)
		}
	}
	select {
	case <-done:
		t.Fatal("the game ended: the eel died at a junction")
	default:
	}
}

// TestJoystickLostDuringGame loses the joystick while the stick is held.
// The held direction must be dropped and the game must carry on in demo
// mode, until the first input from the joystick once it is back.
func TestJoystickLostDuringGame(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick

	g.HandleControllerState(input(controller.Dir_North, controller.Btn_None))
	done := startGame(g)
	sink.mustFrame()
	if g.inDemoMode() {
		t.Fatal("in demo mode with a joystick and no --demo")
	}

	g.HandleControllerLost()
	if got := g.controlDir.Load(); got != controller.Dir_Neutral {
		t.Errorf("held direction is %v after the joystick was lost, want Neutral", got)
	}
	if !g.inDemoMode() {
		t.Error("not in demo mode after the joystick was lost")
	}
	wantSelfSteering(t, sink, done)

	// A neutral stick is input, and out of demo mode it ends the game at
	// the next junction.
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	if g.inDemoMode() {
		t.Error("still in demo mode after an input")
	}
	waitGameOver(t, done)
}

// TestJoystickLostBeforeFirstGame loses the joystick between startup and
// the first game. The games must be demos although --demo was not given,
// and go back to what was configured once the joystick has been used.
func TestJoystickLostBeforeFirstGame(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick

	g.HandleControllerLost()
	done := startGame(g)
	wantSelfSteering(t, sink, done)

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	waitGameOver(t, done)
	sink.drain(20 * time.Millisecond)

	// The next game is an ordinary one: with the stick neutral it ends at
	// the first junction.
	if g.inDemoMode() {
		t.Error("in demo mode after the joystick was used")
	}
	waitGameOver(t, startGame(g))
}

// TestJoystickLostForgetsHeldButtons loses the joystick with a button down.
// Nothing is sent to Chromatik and pause stays as it was. If the button is
// still down when the joystick is back, that counts as a press, because a
// lost joystick counts as let go.
func TestJoystickLostForgetsHeldButtons(t *testing.T) {
	g, sink := newOSCTestGame(t)

	g.HandleControllerState(input(controller.Dir_North, controller.Btn_Red))
	sink.expectPress("pressing red", fader2, 1)

	g.HandleControllerLost()
	sink.expectNone("losing the joystick")
	if !g.paused.Load() {
		t.Fatal("losing the joystick resumed a paused game")
	}

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red))
	sink.expectPress("red, down when the joystick came back", fader1, 1)
}

// fakeJoystick is a controller.Device that reports what the test sends.
type fakeJoystick struct {
	reports chan []byte
	unplug  chan struct{}
}

func (f *fakeJoystick) Read(p []byte) (int, error) {
	select {
	case r := <-f.reports:
		return copy(p, r), nil
	case <-f.unplug:
		return -1, errors.New("fake joystick unplugged")
	}
}

func (f *fakeJoystick) Close() error                  { return nil }
func (f *fakeJoystick) Manufacturer() (string, error) { return "fake", nil }
func (f *fakeJoystick) Product() (string, error)      { return "fake", nil }

// TestControllerLossReachesGame wires a controller with a fake device to a
// game the way NewGame does, and goes through the life of a joystick that
// is used, unplugged, plugged back in and used again. The demo that starts
// with the loss must last until the joystick is touched, not just until it
// is back.
func TestControllerLossReachesGame(t *testing.T) {
	g, _ := newTestGame(t)

	newJoystick := func() *fakeJoystick {
		return &fakeJoystick{reports: make(chan []byte), unplug: make(chan struct{})}
	}
	var mu sync.Mutex
	plugged := newJoystick()
	opens := 0
	open := func() (controller.Device, error) {
		mu.Lock()
		defer mu.Unlock()
		if plugged == nil {
			return nil, errors.New("fake joystick is not plugged in")
		}
		opens++
		return plugged, nil
	}
	plug := func(j *fakeJoystick) {
		mu.Lock()
		defer mu.Unlock()
		plugged = j
	}
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(time.Millisecond)
		}
	}
	send := func(j *fakeJoystick, report []byte) {
		t.Helper()
		select {
		case j.reports <- report:
		case <-time.After(5 * time.Second):
			t.Fatal("the controller is not reading the joystick")
		}
	}
	// The reports for the stick untouched, and pushed north and east, with
	// no buttons down.
	neutral := []byte{0x40, 0x40, 0, 0, 0, 0, 0, 0}
	north := []byte{0x40, 0xc0, 0, 0, 0, 0, 0, 0}
	east := []byte{0x00, 0x40, 0, 0, 0, 0, 0, 0}

	first := plugged
	c, err := controller.Start(g.Config.Logger, open, 2*time.Millisecond, g.HandleControllerState, g.HandleControllerLost)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(c.Stop)
	if g.inDemoMode() {
		t.Fatal("in demo mode with the joystick there and --demo not given")
	}
	send(first, north)
	waitFor("north to reach the game", func() bool { return g.controlDir.Load() == controller.Dir_North })

	plug(nil)
	close(first.unplug)
	waitFor("demo mode and a neutral stick after the joystick is gone", func() bool {
		return g.inDemoMode() && g.controlDir.Load() == controller.Dir_Neutral
	})

	// Back, but nobody has touched it: still a demo, also if the joystick
	// reports its untouched state. The second report is taken only after
	// the first has been dealt with.
	second := newJoystick()
	// Stop first, so that the read this ends is not taken for another loss.
	t.Cleanup(func() {
		c.Stop()
		close(second.unplug)
	})
	plug(second)
	waitFor("the joystick to be opened again", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return opens == 2
	})
	send(second, neutral)
	send(second, neutral)
	if !g.inDemoMode() {
		t.Error("demo mode ended when the joystick came back, before any input")
	}

	send(second, east)
	waitFor("east to reach the game", func() bool { return g.controlDir.Load() == controller.Dir_East })
	if g.inDemoMode() {
		t.Error("still in demo mode after an input from the joystick that came back")
	}
}
