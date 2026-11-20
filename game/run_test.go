package game

import (
	"slices"
	"sync"
	"testing"
	"time"
)

// TestRunPlaysOneGame runs a whole game on the real tickers. A new eel
// moves 15 times along its segment, and with a neutral joystick and demo
// mode off it dies on the tick after that, at its first junction. Mover
// must then start the game-over animation exactly once, and Run must not
// return before it has finished.
func TestRunPlaysOneGame(t *testing.T) {
	g, sink := newTestGame(t)

	for game := 0; game < 2; game++ {
		done := make(chan struct{})
		go func() {
			g.Run()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("game %d: Run did not return", game)
		}

		var frames [][testPixels]Color
		var counts []int
		for {
			pixels, ok := sink.frame(100 * time.Millisecond)
			if !ok {
				break
			}
			frames = append(frames, pixels)
			counts = append(counts, lit(pixels))
		}
		// The animation draws only the eel, two points shorter in every
		// frame until nothing is left, so the game ends with frames of
		// 5, 3 and 1 pixels (6, 4 and 2 if the eel ate and its length is
		// even). Frames from play also show food and turn signals.
		//
		// Before that come the 15 frames of play, one for each move. An
		// eel that did not eat is 7 points long and its animation is
		// exactly those three frames, 18 in all; one that ate has a longer
		// animation.
		const playFrames = 15
		n := len(counts)
		if n < playFrames+3 {
			t.Fatalf("game %d: only %d frames were sent, want at least %d: %d of play and 3 or more of animation",
				game, n, playFrames+3, playFrames)
		}
		last := counts[n-1]
		if (last != 1 && last != 2) || counts[n-2] != last+2 || counts[n-3] != last+4 {
			t.Errorf("game %d: the last three frames light %v pixels, want [5 3 1] or [6 4 2]", game, counts[n-3:])
		}
		// For an eel that did not eat, the first animation frame is the
		// one with 5 pixels, drawn one step down from full brightness.
		// That fixes its colors: white ends around these three, in either
		// direction. (An eel that ate and died at an odd length also ends
		// 5, 3, 1, but its 5-pixel frame comes later and is dimmer.)
		if n == playFrames+3 && counts[n-3] == 5 {
			var middle []Color
			for _, c := range frames[n-3] {
				if c != (Color{testBackground, testBackground, testBackground}) && c != White {
					middle = append(middle, c)
				}
			}
			a := []Color{{0, 96, 242}, {0, 242, 96}, {193, 242, 0}}
			b := []Color{a[2], a[1], a[0]}
			if !slices.Equal(middle, a) && !slices.Equal(middle, b) {
				t.Errorf("game %d: first animation frame has colors %v, want %v in either order", game, middle, a)
			}
		}
		if counts[0] <= 7 {
			t.Errorf("game %d: the first frame lights only %d pixels", game, counts[0])
		}

		if len(g.Eel.Body) != 0 {
			t.Errorf("game %d: Run returned with %d eel points still to animate", game, len(g.Eel.Body))
		}
		if g.Brightness != 1.0 {
			t.Errorf("game %d: Brightness is %v after the game, want 1", game, g.Brightness)
		}
	}
}

// TestGameOverAnimationStartsAtFullBrightness kills the eel in the middle
// of a breath, at brightness 0.7. Mover must put the brightness back to 1
// before the animation, so its first frame is drawn at 0.95 like the first
// frame of any animation, and not at 0.65.
func TestGameOverAnimationStartsAtFullBrightness(t *testing.T) {
	g, sink := newTestGame(t)
	// Head on the last pixel of segment 0 with a neutral stick: the first
	// tick ends the game without drawing.
	place(g, 0, 21, 7, 0.7, 5, 12, 6)
	g.brightnessIncrement = -0.05
	g.tick = fastTick

	var wg sync.WaitGroup
	wg.Add(1)
	go g.Mover(&wg)
	wg.Wait()

	checkFrame(t, "first animation frame", sink.mustFrame(), map[int]Color{
		16: White, 17: {0, 96, 242}, 18: {0, 242, 96}, 19: {193, 242, 0}, 20: White,
	})
}
