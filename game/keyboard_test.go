package game

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

// TestOpenInput checks which of the keyboard and the joystick NewGame opens
// for each combination of --keyboard and --nojoy, with stand-ins for the
// two functions that open them. The keyboard takes the joystick's place.
func TestOpenInput(t *testing.T) {
	realKeyboard, realController := newKeyboard, newController
	t.Cleanup(func() { newKeyboard, newController = realKeyboard, realController })

	for _, c := range []struct {
		name            string
		keyboard, noJoy bool
		wantKeyboard    bool
		wantJoystick    bool
	}{
		{name: "no flags", wantJoystick: true},
		{name: "--nojoy", noJoy: true},
		{name: "--keyboard", keyboard: true, wantKeyboard: true},
		{name: "--keyboard --nojoy", keyboard: true, noJoy: true, wantKeyboard: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			g, _ := newTestGame(t)
			g.Config.Keyboard, g.Config.NoJoy = c.keyboard, c.noJoy

			var keyboards, joysticks int
			var handler func(controller.ControllerState)
			var lost func()
			newKeyboard = func(cfg *config.Config, handle func(controller.ControllerState)) (*controller.Keyboard, error) {
				keyboards++
				handler = handle
				return nil, nil
			}
			newController = func(cfg *config.Config, handle func(controller.ControllerState), lostFunc func()) (*controller.Controller, error) {
				joysticks++
				handler, lost = handle, lostFunc
				return nil, nil
			}

			if err := g.openInput(); err != nil {
				t.Fatalf("openInput: %v", err)
			}
			if (keyboards == 1) != c.wantKeyboard || keyboards > 1 {
				t.Errorf("the keyboard was opened %d times, want opened: %v", keyboards, c.wantKeyboard)
			}
			if (joysticks == 1) != c.wantJoystick || joysticks > 1 {
				t.Errorf("the joystick was opened %d times, want opened: %v", joysticks, c.wantJoystick)
			}

			// Whatever was opened has to be given the game's own handlers.
			if c.wantKeyboard || c.wantJoystick {
				if handler == nil {
					t.Fatal("the input was opened with no handler")
				}
				handler(input(controller.Dir_West, controller.Btn_None))
				if got := g.input.Load().direction(); got != controller.Dir_West {
					t.Errorf("a state given to the handler left the stick held %v, want West", got)
				}
			}
			if c.wantJoystick {
				if lost == nil {
					t.Fatal("the joystick was opened with nothing to call when it is lost")
				}
				lost()
				if !g.input.Load().inDemoMode() {
					t.Error("the joystick's lost function did not put the game in demo mode")
				}
			}
		})
	}

	// A failure to open either one stops NewGame, with the reason.
	for _, keyboard := range []bool{true, false} {
		g, _ := newTestGame(t)
		g.Config.Keyboard = keyboard
		failure := errors.New("cannot open that")
		newKeyboard = func(*config.Config, func(controller.ControllerState)) (*controller.Keyboard, error) {
			return nil, failure
		}
		newController = func(*config.Config, func(controller.ControllerState), func()) (*controller.Controller, error) {
			return nil, failure
		}
		if err := g.openInput(); err == nil || !strings.Contains(err.Error(), failure.Error()) {
			t.Errorf("--keyboard=%v: openInput returned %v, want an error that says %q", keyboard, err, failure)
		}
	}
}

// TestKeyboardReachesGame wires a keyboard to a game the way NewGame does
// and types at it. The arrows steer, and a button key pressed twice is two
// presses: the keyboard sends the release that the game needs to see
// between them.
func TestKeyboardReachesGame(t *testing.T) {
	g, sink := newOSCTestGame(t)
	setDemo(g, true)

	r, w := io.Pipe()
	quit := make(chan struct{})
	controller.StartKeyboard(g.Config.Logger, r, g.HandleControllerState, func() { close(quit) })
	t.Cleanup(func() { _ = w.Close() })

	// A write returns when the reader has taken it, which is before the
	// reader has acted on it. A second write of a key that means nothing
	// returns only when the reader comes back for more, and by then the
	// game has been given everything the first one led to.
	keys := func(s string) {
		t.Helper()
		for _, part := range []string{s, "x"} {
			if _, err := w.Write([]byte(part)); err != nil {
				t.Fatalf("typing %q: %v", part, err)
			}
		}
	}

	keys("\x1b[A") // up
	if got := g.input.Load().direction(); got != controller.Dir_North {
		t.Fatalf("after the up arrow the stick is held %v, want North", got)
	}
	if g.input.Load().inDemoMode() {
		t.Error("still in demo mode after a key")
	}
	sink.expectNone("an arrow key")

	keys("r")
	sink.expectPress("r", fader2, 1)
	if !g.paused.Load() {
		t.Error("the red button did not pause the game")
	}
	keys("r")
	sink.expectPress("r again", fader1, 1)

	keys("w")
	sink.expectPress("w", fader9, 0)
	if g.paused.Load() {
		t.Error("the white button did not resume the game")
	}
	if got := g.input.Load().direction(); got != controller.Dir_North {
		t.Errorf("after the buttons the stick is held %v, want North still", got)
	}

	keys(" ") // centre the stick
	if got := g.input.Load().direction(); got != controller.Dir_Neutral {
		t.Errorf("after space the stick is held %v, want Neutral", got)
	}

	if _, err := w.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-quit:
	case <-time.After(5 * time.Second):
		t.Fatal("q did not call the quit function")
	}
}
