package game

import (
	"slices"
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// The eels of move_test.go all travel towards the high end of their segment.
// About half of all travel on the dome is the other way, towards position 0,
// and Move has code of its own for it: which end of a food is bitten off,
// which neighbours get the red signals, and where on the next segment a
// yellow signal goes when the eel will come onto it at its high end. The
// tests in this file are those of move_test.go for an eel travelling that
// way.
//
// The eel is on p3 (pixels 44 to 65). Leaving it by the low end, north and
// east lead onto p4 at its low end (pixels 66 and up) and south and west
// onto p2 at its high end (pixels 43 and down), so both ways of drawing a
// signal are seen. An eel leaving p3 by its high end would signal on p7 and
// p8 instead.
//
// The colors of the eel are those of move_test.go, which depend only on how
// far a point is from the head. The signals have the levels recorded there.

// Red "no turn selected" signals at brightness 0.75 for an eel travelling
// towards the low end of p3: on the first pixels of p4 and the last pixels
// of p2, brightest next to the junction.
var pinRedSignalsLesser75 = map[int]Color{
	66: {191, 0, 0}, 67: {165, 0, 0}, 68: {140, 0, 0}, 69: {114, 0, 0}, 70: {89, 0, 0},
	43: {191, 0, 0}, 42: {165, 0, 0}, 41: {140, 0, 0}, 40: {114, 0, 0}, 39: {89, 0, 0},
}

// pinPlaceLesser is place for an eel travelling towards lower positions: n
// points on segment seg with the head at headPos and the body above it, a
// neutral joystick, and a fresh food on segment foodSeg covering positions
// foodFrom..foodTo.
func pinPlaceLesser(g *Game, seg, headPos, n int, brightness float64, foodSeg, foodFrom, foodTo int) {
	s := g.Segments[seg]
	g.Eel = &Eel{
		Body:      line(s, headPos, headPos+n-1),
		Game:      g,
		Segment:   s,
		TravelDir: Lesser,
	}
	setStick(g, controller.Dir_Neutral)
	g.Food = &Food{Body: line(g.Segments[foodSeg], foodFrom, foodTo), Game: g, Fresh: true}
	g.Brightness = brightness
}

// pinPositions returns the positions of the points, in order.
func pinPositions(points []*Point) []int {
	positions := make([]int, len(points))
	for i, p := range points {
		positions[i] = p.Position
	}
	return positions
}

func TestMoveLesserPlainTick(t *testing.T) {
	g, sink := newTestGame(t)
	pinPlaceLesser(g, 2, 10, 7, 0.75, 5, 12, 6)

	if err := g.Eel.Move(); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if head := g.Eel.Head(); head.Segment != g.Segments[2] || head.Position != 9 {
		t.Fatalf("head is on %s at %d, want %s at 9, one position lower", head.Segment.Label, head.Position, g.Segments[2].Label)
	}
	checkFrame(t, "plain tick towards the low end", sink.mustFrame(), merge(pinRedSignalsLesser75, map[int]Color{
		// eel, head to tail
		53: {255, 0, 0}, 54: {255, 218, 0}, 55: {72, 255, 0}, 56: {0, 255, 145},
		57: {0, 145, 255}, 58: {72, 0, 255}, 59: {255, 0, 218},
		// food
		116: grey75, 117: grey75, 118: grey75, 119: grey75, 120: grey75, 121: grey75, 122: grey75,
	}))
	pinExpectNothingMore(t, sink)
}

// TestMoveLesserHeldDirection holds the stick while the eel travels towards
// the low end of p3. The turn it will take is signalled in yellow on the
// segment it leads to, at the end the eel will come onto: the first pixels
// of p4 for north and east, the last pixels of p2 for south and west.
func TestMoveLesserHeldDirection(t *testing.T) {
	yellowOnP4 := map[int]Color{
		66: {255, 255, 0}, 67: {229, 229, 0}, 68: {204, 204, 0}, 69: {178, 178, 0}, 70: {153, 153, 0},
	}
	yellowOnP2 := map[int]Color{
		43: {255, 255, 0}, 42: {229, 229, 0}, 41: {204, 204, 0}, 40: {178, 178, 0}, 39: {153, 153, 0},
	}
	tests := []struct {
		stick  controller.Direction
		signal map[int]Color
	}{
		{controller.Dir_North, yellowOnP4},
		{controller.Dir_East, yellowOnP4},
		{controller.Dir_South, yellowOnP2},
		{controller.Dir_West, yellowOnP2},
	}
	for _, tc := range tests {
		t.Run(tc.stick.String(), func(t *testing.T) {
			g, sink := newTestGame(t)
			pinPlaceLesser(g, 2, 10, 7, 1.0, 5, 12, 6)
			setStick(g, tc.stick)

			if err := g.Eel.Move(); err != nil {
				t.Fatalf("Move: %v", err)
			}
			checkFrame(t, "held "+tc.stick.String(), sink.mustFrame(), merge(tc.signal, map[int]Color{
				// eel, head to tail
				53: {255, 0, 0}, 54: {255, 218, 0}, 55: {72, 255, 0}, 56: {0, 255, 145},
				57: {0, 145, 255}, 58: {72, 0, 255}, 59: {255, 0, 218},
				// food at full brightness
				116: White, 117: White, 118: White, 119: White, 120: White, 121: White, 122: White,
			}))
		})
	}
}

// TestMoveLesserThroughJunction hops from the low end of p3 onto the high
// end of p2. The eel goes on travelling towards the low end there. The
// yellow signal starts on the pixel the head lands on, the last of p2, and
// the head is drawn over it.
func TestMoveLesserThroughJunction(t *testing.T) {
	g, sink := newTestGame(t)
	pinPlaceLesser(g, 2, 0, 7, 0.9, 5, 12, 6)
	setStick(g, controller.Dir_South)

	if err := g.Eel.Move(); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if head := g.Eel.Head(); head.Segment != g.Segments[1] || head.Position != 21 {
		t.Fatalf("head is on %s at %d, want %s at 21", head.Segment.Label, head.Position, g.Segments[1].Label)
	}
	if g.Eel.Segment != g.Segments[1] || g.Eel.TravelDir != Lesser {
		t.Errorf("the eel is on %s %s, want %s %s",
			g.Eel.Segment.Label, towards(g.Eel.TravelDir), g.Segments[1].Label, towards(Lesser))
	}
	grey90 := Color{229, 229, 229}
	checkFrame(t, "junction towards the low end", sink.mustFrame(), map[int]Color{
		39: {127, 127, 0}, 40: {153, 153, 0}, 41: {178, 178, 0}, 42: {204, 204, 0},
		43: {255, 0, 0}, // head, over the first signal pixel
		44: {255, 218, 0}, 45: {72, 255, 0}, 46: {0, 255, 145},
		47: {0, 145, 255}, 48: {72, 0, 255}, 49: {255, 0, 218},
		116: grey90, 117: grey90, 118: grey90, 119: grey90, 120: grey90, 121: grey90, 122: grey90,
	})
}

// TestMoveLesserEatingTicks is TestMoveEatingTicks for an eel that comes at
// the food from above: head at position 14 of p3, food on positions 13 down
// to 7. It follows the eel through the whole meal, seven bites on seven
// ticks.
//
// The eel bites off the end of the food it came in by. So after every tick
// the food that is left is exactly the positions ahead of the head, nearest
// first, and the frame shows grey from the head's pixel (the bite of this
// tick, still drawn) to the far end of the food and nowhere else: never on
// the body behind the head. The first three frames are compared pixel for
// pixel.
func TestMoveLesserEatingTicks(t *testing.T) {
	g, sink := newTestGame(t)
	pinPlaceLesser(g, 2, 14, 7, 0.75, 2, 13, 7)
	const (
		p3      = 44 // the pixel of position 0 of p3
		foodEnd = 7  // the food's lowest position
	)

	golden := []map[int]Color{
		{
			51: grey75, 52: grey75, 53: grey75, 54: grey75, 55: grey75, 56: grey75, 57: grey75,
			58: {255, 218, 0}, 59: {72, 255, 0}, 60: {0, 255, 145},
			61: {0, 145, 255}, 62: {72, 0, 255}, 63: {255, 0, 218},
		},
		{
			51: grey75, 52: grey75, 53: grey75, 54: grey75, 55: grey75, 56: grey75,
			57: {255, 191, 0}, 58: {127, 255, 0}, 59: {0, 255, 63}, 60: {0, 255, 255},
			61: {0, 63, 255}, 62: {127, 0, 255}, 63: {255, 0, 191},
		},
		{
			51: grey75, 52: grey75, 53: grey75, 54: grey75, 55: grey75,
			56: {255, 170, 0}, 57: {170, 255, 0}, 58: {0, 255, 0}, 59: {0, 255, 169},
			60: {0, 169, 255}, 61: {0, 0, 255}, 62: {170, 0, 255}, 63: {255, 0, 170},
		},
	}

	for tick := range GrowthIncrement {
		if err := g.Eel.Move(); err != nil {
			t.Fatalf("tick %d: Move: %v", tick, err)
		}
		frame := sink.mustFrame()

		head := g.Eel.Head()
		if head.Segment != g.Segments[2] || head.Position != 13-tick {
			t.Fatalf("tick %d: head is on %s at %d, want %s at %d",
				tick, head.Segment.Label, head.Position, g.Segments[2].Label, 13-tick)
		}

		// The food that is left: everything from the position ahead of the
		// head down to the food's far end.
		var ahead []int
		for pos := head.Position - 1; pos >= foodEnd; pos-- {
			ahead = append(ahead, pos)
		}
		if left := pinPositions(g.Food.Body); !slices.Equal(left, ahead) {
			t.Errorf("tick %d: with the head at %d the food left is on positions %v, want %v: the positions ahead of the head and no others",
				tick, head.Position, left, ahead)
		}
		for _, p := range g.Food.Body {
			if p.Segment != g.Segments[2] {
				t.Errorf("tick %d: a food point is on %s, want %s", tick, p.Segment.Label, g.Segments[2].Label)
			}
		}

		// The frame: grey on the head's pixel and on what is left, and on
		// nothing else.
		var grey, wantGrey []int
		for px, c := range frame {
			if c == grey75 {
				grey = append(grey, px)
			}
		}
		for pos := foodEnd; pos <= head.Position; pos++ {
			wantGrey = append(wantGrey, p3+pos)
		}
		if !slices.Equal(grey, wantGrey) {
			t.Errorf("tick %d: with the head on pixel %d the frame shows food on pixels %v, want %v: from the far end of the food up to the head",
				tick, p3+head.Position, grey, wantGrey)
		}
		if tick < len(golden) {
			checkFrame(t, "eating tick towards the low end", frame, merge(pinRedSignalsLesser75, golden[tick]))
		}

		if got, want := len(g.Eel.Body), 7+tick; got != want {
			t.Errorf("tick %d: eel is %d long, want %d", tick, got, want)
		}
		if got, want := g.Eel.Growth, GrowthIncrement-tick; got != want {
			t.Errorf("tick %d: Growth is %d, want %d", tick, got, want)
		}
		if g.Food.Fresh {
			t.Errorf("tick %d: food is still fresh", tick)
		}
	}
	if len(g.Food.Body) != 0 {
		t.Errorf("after %d bites the food still has %d points", GrowthIncrement, len(g.Food.Body))
	}
}
