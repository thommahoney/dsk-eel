package game

import (
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// fiveButtons is the five buttons of the joystick, each by itself.
var fiveButtons = [...]controller.ButtonStatus{
	controller.Btn_Red,
	controller.Btn_Green,
	controller.Btn_Blue,
	controller.Btn_Yellow,
	controller.Btn_White,
}

// isOneButton reports whether buttons is exactly one of the five.
func isOneButton(buttons controller.ButtonStatus) bool {
	for _, b := range fiveButtons {
		if buttons == b {
			return true
		}
	}
	return false
}

// TestFirstButtonIsOneButtonOrNone goes through every value the button byte
// of a report can have. firstButton must answer each with one button that
// is down, or with no button when none of the five is down, and never with
// several at once. HandleControllerState passes its answer on as the press,
// so this is why HandleButtonPress has nothing to do for several buttons.
func TestFirstButtonIsOneButtonOrNone(t *testing.T) {
	few := reportFew(t)
	for v := 0; v < 256; v++ {
		buttons := controller.ButtonStatus(v)
		got := firstButton(buttons)

		if buttons&allFive == 0 {
			if got != controller.Btn_None {
				few.Errorf("with the button byte at %08b none of the five buttons is down, but firstButton says %05b, want no button",
					v, int(got))
			}
			continue
		}
		if !isOneButton(got) {
			few.Errorf("with the button byte at %08b firstButton says %05b, want exactly one of the five buttons",
				v, int(got))
			continue
		}
		if buttons&got == 0 {
			few.Errorf("with the button byte at %08b firstButton says %05b, a button that is not down",
				v, int(got))
		}
	}
}

// TestPressOfNoSingleButtonDoesNothing calls HandleButtonPress with every
// value of the button byte that is not one button by itself: no button,
// several together, and bits that are no button. The game never does that
// (see TestFirstButtonIsOneButtonOrNone), and if something else did, it
// must not send anything, pause the game or use up a turn of the two
// faders a color alternates between.
func TestPressOfNoSingleButtonDoesNothing(t *testing.T) {
	g, sink := newOSCTestGame(t)
	indexBefore := g.buttonIndex

	for v := 0; v < 256; v++ {
		buttons := controller.ButtonStatus(v)
		if isOneButton(buttons) {
			continue
		}
		g.HandleButtonPress(buttons)
		if g.paused.Load() {
			t.Fatalf("a press of %08b, which is not one button, paused the game", v)
		}
		if g.buttonIndex != indexBefore {
			t.Fatalf("a press of %08b, which is not one button, moved on to the other fader of a pair", v)
		}
	}
	sink.expectNone("pressing everything that is not one button")

	// The game still takes a real press afterwards, as its first.
	g.HandleButtonPress(controller.Btn_Blue)
	sink.expectPress("pressing blue afterwards", fader6, 3)
}
