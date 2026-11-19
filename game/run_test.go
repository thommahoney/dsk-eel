package game

import (
	"slices"
	"testing"
	"time"
)

// TestRunPlaysOneGame runs a whole game on the real tickers. With a
// neutral joystick and demo mode off the eel dies at the first junction,
// within 16 ticks. Mover must then start the game-over animation exactly
// once, and Run must not return before it has finished.
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
		n := len(counts)
		if n < 4 {
			t.Fatalf("game %d: only %d frames were sent", game, n)
		}
		last := counts[n-1]
		if (last != 1 && last != 2) || counts[n-2] != last+2 || counts[n-3] != last+4 {
			t.Errorf("game %d: the last three frames light %v pixels, want [5 3 1] or [6 4 2]", game, counts[n-3:])
		}
		// The animation starts again from full brightness, whatever the
		// brightness was when the eel died. For an eel that did not eat,
		// that fixes the colors of the first animation frame: white ends
		// around these three, in either direction.
		if counts[n-3] == 5 {
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
