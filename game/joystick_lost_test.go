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

// TestJoystickLostBeforeFirstGame is the program starting with no joystick
// plugged in. The games must be demos although --demo was not given, and go
// back to what was configured once the joystick has been used.
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
		t.Error("losing the joystick resumed a paused game")
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
// game the way NewGame does, and goes through the life of a joystick that is
// not there at startup, is plugged in, used and unplugged.
func TestControllerLossReachesGame(t *testing.T) {
	g, _ := newTestGame(t)

	var mu sync.Mutex
	var plugged *fakeJoystick
	open := func() (controller.Device, error) {
		mu.Lock()
		defer mu.Unlock()
		if plugged == nil {
			return nil, errors.New("fake joystick is not plugged in")
		}
		return plugged, nil
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

	c := controller.Start(g.Config.Logger, open, 2*time.Millisecond, g.HandleControllerState, g.HandleControllerLost)
	t.Cleanup(c.Stop)
	if !g.inDemoMode() {
		t.Fatal("not in demo mode after starting without a joystick")
	}

	joystick := &fakeJoystick{reports: make(chan []byte), unplug: make(chan struct{})}
	mu.Lock()
	plugged = joystick
	mu.Unlock()

	// The report for the stick pushed north with no buttons down.
	north := []byte{0x40, 0xc0, 0, 0, 0, 0, 0, 0}
	select {
	case joystick.reports <- north:
	case <-time.After(5 * time.Second):
		t.Fatal("the controller did not open the joystick once it was plugged in")
	}
	waitFor("north to reach the game", func() bool { return g.controlDir.Load() == controller.Dir_North })
	if g.inDemoMode() {
		t.Error("still in demo mode after an input")
	}

	mu.Lock()
	plugged = nil
	mu.Unlock()
	close(joystick.unplug)
	waitFor("demo mode and a neutral stick after the joystick is gone", func() bool {
		return g.inDemoMode() && g.controlDir.Load() == controller.Dir_Neutral
	})
}
