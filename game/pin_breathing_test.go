package game

import (
	"slices"
	"sync"
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// The brightness of the food and of the turn signals breathes: from full it
// goes down a twentieth on every tick to 0.6, turns, comes back up to full
// and turns again, 16 ticks for a breath. The game-over animation dims the
// eel the same way, a step for every frame. The tests in this file follow
// both through several whole breaths, because the turns are where it can go
// wrong: a brightness that misses one keeps going, and leaves the range of
// levels there are.

// pinBreath is the level of white on each tick of a breath, starting with
// the tick after full brightness: 0.95, 0.90 and so on down to 0.60 on the
// eighth tick, and back up to 1 on the sixteenth. The food is drawn in
// white, so this is its grey.
var pinBreath = [16]byte{
	242, 229, 216, 203, 191, 178, 165, 152,
	165, 178, 191, 203, 216, 229, 242, 255,
}

// pinGreys returns the grey levels a frame shows other than the background,
// each as many times as it has pixels, lowest first. The eel and the turn
// signals are never grey, so in a frame of play these are the food's pixels.
func pinGreys(pixels [testPixels]Color) []byte {
	var greys []byte
	for _, c := range pixels {
		if c[0] == c[1] && c[1] == c[2] && c[0] != testBackground {
			greys = append(greys, c[0])
		}
	}
	slices.Sort(greys)
	return greys
}

// pinFood returns the greys of a food of seven pixels drawn at the level.
func pinFood(level byte) []byte {
	return slices.Repeat([]byte{level}, GrowthIncrement)
}

// TestMoverBreathes runs Mover for three breaths, 48 ticks, and looks at
// every frame: the food must be at the level of its tick, and so must the
// brightest pixel of the turn signal.
//
// North is held, which takes the eel from p1 round a loop of five segments
// (p2, p4, p5, then r6, b5, g6, y6 and p6 again and again). It never comes
// near the food on b1, so every frame has the whole food, and the yellow
// signal of the turn ahead. The signal is five pixels, the first at the
// brightness of the tick; on the tick the eel goes through a junction its
// head is on that first pixel, and the frame shows four.
func TestMoverBreathes(t *testing.T) {
	g, sink := newTestGame(t)
	place(g, 0, 6, 7, 1.0, 20, 12, 6)
	setStick(g, controller.Dir_North)
	g.brightnessIncrement = -0.05
	g.tick = fastTick

	var wg sync.WaitGroup
	wg.Add(1)
	go g.Mover(&wg)
	// Letting go of the stick ends the game at the next junction, and Mover
	// then waits for the animation.
	endGame := func() {
		setStick(g, controller.Dir_Neutral)
		pinWait(t, &wg, "the game")
	}
	t.Cleanup(endGame)

	frames := make([][testPixels]Color, 3*len(pinBreath))
	for i := range frames {
		frames[i] = sink.mustFrame()
	}
	endGame()

	wholeSignals := 0
	for i, pixels := range frames {
		level := pinBreath[i%len(pinBreath)]
		if got, want := pinGreys(pixels), pinFood(level); !slices.Equal(got, want) {
			t.Fatalf("tick %d: the food's pixels are at the levels %v, want %v", i+1, got, want)
		}

		var signal []byte
		for _, c := range pixels {
			if c[0] == c[1] && c[2] == 0 && c[0] != 0 {
				signal = append(signal, c[0])
			}
		}
		switch len(signal) {
		case 5:
			wholeSignals++
			if brightest := slices.Max(signal); brightest != level {
				t.Fatalf("tick %d: the brightest pixel of the yellow signal is at level %d, want %d, the level of the food", i+1, brightest, level)
			}
		case 4:
			// A junction tick: the head covers the brightest one.
		default:
			t.Fatalf("tick %d: the frame has %d yellow signal pixels, want 5, or 4 on a junction tick", i+1, len(signal))
		}
	}
	// The eel starts 15 moves from the end of p1 and a segment is 22 long,
	// so ticks 16 and 38 are the junction ticks.
	if want := len(frames) - 2; wholeSignals != want {
		t.Errorf("%d of %d frames showed the whole turn signal, want %d: all but the two junction ticks", wholeSignals, len(frames), want)
	}

	if len(g.Food.Body) != GrowthIncrement || !g.Food.Fresh {
		t.Errorf("the eel got to the food (%d points left, fresh=%v): this test needs a food that is left alone",
			len(g.Food.Body), g.Food.Fresh)
	}
}

// TestRunStartsTheBreathFromFull starts a game with the breath as the game
// before can leave it. When the eel dies Mover puts the brightness back to
// full but not the step, and a game that ends while the brightness is on
// its way up (one that ends at its third junction, for example) leaves the
// step pointing up. Run has to turn it down again, or the first tick of the
// next game would ask for more than full brightness.
//
// The game is one with nobody at the stick: 15 frames of play, and then
// three of the animation, in which the only grey is the white of the eel's
// ends.
func TestRunStartsTheBreathFromFull(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick
	g.brightnessIncrement = 0.05

	waitGameOver(t, startGame(g))

	for i := range 15 {
		level := pinBreath[i]
		if got, want := pinGreys(sink.mustFrame()), pinFood(level); !slices.Equal(got, want) {
			t.Fatalf("tick %d of the game: the food's pixels are at the levels %v, want %v", i+1, got, want)
		}
	}
	for i, ends := range []int{2, 2, 1} {
		if got, want := pinGreys(sink.mustFrame()), slices.Repeat([]byte{255}, ends); !slices.Equal(got, want) {
			t.Fatalf("frame %d of the animation has the greys %v, want only the white of the eel's ends, %v", i+1, got, want)
		}
	}
	pinExpectNothingMore(t, sink)
}

// TestGameOverAnimationBreathes runs the animation on an eel of 80 points,
// which takes 39 frames to disappear: two breaths and seven frames more.
// Every frame is the eel two points shorter, white at both ends, and in
// between in colors at the brightness of the frame: the strongest of the
// three channels of such a color is the level, and the weakest is 0.
//
// The frames are read while the animation runs. Read afterwards, all 273
// datagrams of it would have to wait in the sink's socket, and how many a
// socket keeps before it drops them is up to the machine.
func TestGameOverAnimationBreathes(t *testing.T) {
	const length = 80

	g, sink := newTestGame(t)
	g.tick = fastTick
	// The eel lies on pixels 0 to 79, head on pixel 0. The animation does
	// not care that no eel could have crawled there.
	place(g, 0, 10, 7, 1.0, 5, 12, 6)
	g.Eel.Body = pixelBody(g, pixelsWhere(func(px int) bool { return px < length }))

	var wg sync.WaitGroup
	wg.Add(1)
	go g.GameOverAnimation(&wg)
	// The animation ends by itself when the eel is gone. It is waited for
	// even when the test fails, so that it is not still drawing when the
	// game's sockets are closed.
	ended := func() { pinWait(t, &wg, "the animation") }
	t.Cleanup(ended)

	for frame := 1; frame < length/2; frame++ {
		pixels := sink.mustFrame()
		level := pinBreath[(frame-1)%len(pinBreath)]
		first, last := frame, length-1-frame
		for px, c := range pixels {
			strongest, weakest := max(c[0], c[1], c[2]), min(c[0], c[1], c[2])
			switch {
			case px < first || px > last:
				if c != (Color{testBackground, testBackground, testBackground}) {
					t.Fatalf("frame %d: pixel %d is %v, want the background: the eel is on pixels %d to %d", frame, px, c, first, last)
				}
			case px == first || px == last:
				if c != White {
					t.Fatalf("frame %d: pixel %d, an end of the eel, is %v, want white", frame, px, c)
				}
			case strongest != level || weakest != 0:
				t.Fatalf("frame %d: pixel %d is %v, want a color at level %d: its strongest channel %d and its weakest 0",
					frame, px, c, level, level)
			}
		}
	}
	ended()
	pinExpectNothingMore(t, sink)
	if g.Brightness != 1.0 {
		t.Errorf("Brightness is %v after the animation, want 1", g.Brightness)
	}
}
