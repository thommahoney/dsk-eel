package game

import (
	"sync"
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// The expected frames in this file were recorded from the map-based
// renderer that the frame buffer replaced, for the same game states. They
// pin what a tick puts on the wire: colors, which layer wins where layers
// overlap, and what the food looks like while it is being eaten.

var (
	grey75 = Color{191, 191, 191} // food at brightness 0.75

	// Red "no turn selected" signals at brightness 0.75, on the first
	// pixels of segment 1 and the last pixels of segment 18.
	redSignals75 = map[int]Color{
		22: {191, 0, 0}, 23: {165, 0, 0}, 24: {140, 0, 0}, 25: {114, 0, 0}, 26: {89, 0, 0},
		413: {89, 0, 0}, 414: {114, 0, 0}, 415: {140, 0, 0}, 416: {165, 0, 0}, 417: {191, 0, 0},
	}
)

// merge returns the union of the given pixel maps.
func merge(maps ...map[int]Color) map[int]Color {
	out := map[int]Color{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

func TestMovePlainTick(t *testing.T) {
	g, sink := newTestGame(t)
	place(g, 0, 10, 7, 0.75, 5, 12, 6)

	if err := g.Eel.Move(); err != nil {
		t.Fatalf("Move: %v", err)
	}
	checkFrame(t, "plain tick", sink.mustFrame(), merge(redSignals75, map[int]Color{
		// eel, tail to head
		5: {255, 0, 218}, 6: {72, 0, 255}, 7: {0, 145, 255}, 8: {0, 255, 145},
		9: {72, 255, 0}, 10: {255, 218, 0}, 11: {255, 0, 0},
		// food
		116: grey75, 117: grey75, 118: grey75, 119: grey75, 120: grey75, 121: grey75, 122: grey75,
	}))
	sink.expectNoFrame()
}

// TestMoveEatingTicks follows an eel through its first three bites. On
// each tick the pixel being eaten is still drawn as food, over the eel's
// head, and disappears on the next tick.
func TestMoveEatingTicks(t *testing.T) {
	g, sink := newTestGame(t)
	place(g, 0, 11, 7, 0.75, 0, 18, 12)

	want := []map[int]Color{
		{
			6: {255, 0, 218}, 7: {72, 0, 255}, 8: {0, 145, 255}, 9: {0, 255, 145},
			10: {72, 255, 0}, 11: {255, 218, 0},
			12: grey75, 13: grey75, 14: grey75, 15: grey75, 16: grey75, 17: grey75, 18: grey75,
		},
		{
			6: {255, 0, 191}, 7: {127, 0, 255}, 8: {0, 63, 255}, 9: {0, 255, 255},
			10: {0, 255, 63}, 11: {127, 255, 0}, 12: {255, 191, 0},
			13: grey75, 14: grey75, 15: grey75, 16: grey75, 17: grey75, 18: grey75,
		},
		{
			6: {255, 0, 170}, 7: {170, 0, 255}, 8: {0, 0, 255}, 9: {0, 169, 255},
			10: {0, 255, 169}, 11: {0, 255, 0}, 12: {170, 255, 0}, 13: {255, 170, 0},
			14: grey75, 15: grey75, 16: grey75, 17: grey75, 18: grey75,
		},
	}
	for tick, w := range want {
		if err := g.Eel.Move(); err != nil {
			t.Fatalf("tick %d: Move: %v", tick, err)
		}
		checkFrame(t, "eating tick", sink.mustFrame(), merge(redSignals75, w))

		if got, want := len(g.Eel.Body), 7+tick; got != want {
			t.Errorf("tick %d: eel is %d long, want %d", tick, got, want)
		}
		if got, want := g.Eel.Growth, GrowthIncrement-tick; got != want {
			t.Errorf("tick %d: Growth is %d, want %d", tick, got, want)
		}
		if got, want := len(g.Food.Body), 6-tick; got != want {
			t.Errorf("tick %d: food has %d points left, want %d", tick, got, want)
		}
		if g.Food.Fresh {
			t.Errorf("tick %d: food is still fresh", tick)
		}
	}
}

// TestMoveHeldDirection has a 17-point eel with the joystick held north,
// so the turn it will take is signalled in yellow on the next segment.
// Seventeen points is the shortest body where computing the hue as
// i*(360/n) instead of i*360/n changes a color byte (pixel 77).
func TestMoveHeldDirection(t *testing.T) {
	g, sink := newTestGame(t)
	place(g, 3, 17, 17, 1.0, 20, 12, 6)
	g.controlDir.Store(controller.Dir_North)

	if err := g.Eel.Move(); err != nil {
		t.Fatalf("Move: %v", err)
	}
	checkFrame(t, "held direction", sink.mustFrame(), map[int]Color{
		// eel, tail to head
		68: {255, 0, 90}, 69: {255, 0, 180}, 70: {240, 0, 255}, 71: {150, 0, 255},
		72: {60, 0, 255}, 73: {0, 30, 255}, 74: {0, 120, 255}, 75: {0, 209, 255},
		76: {0, 255, 209}, 77: {0, 255, 120}, 78: {0, 255, 30}, 79: {60, 255, 0},
		80: {150, 255, 0}, 81: {240, 255, 0}, 82: {255, 179, 0}, 83: {255, 89, 0},
		84: {255, 0, 0},
		// yellow turn signal
		88: {255, 255, 0}, 89: {229, 229, 0}, 90: {204, 204, 0}, 91: {178, 178, 0}, 92: {153, 153, 0},
		// food at full brightness
		446: White, 447: White, 448: White, 449: White, 450: White, 451: White, 452: White,
	})
}

// TestMoveThroughJunction hops from the end of segment 0 onto segment 1.
// The yellow signal starts on the pixel the head lands on, and the head
// is drawn over it.
func TestMoveThroughJunction(t *testing.T) {
	g, sink := newTestGame(t)
	place(g, 0, 21, 7, 0.9, 20, 12, 6)
	g.controlDir.Store(controller.Dir_North)

	if err := g.Eel.Move(); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if head := g.Eel.Head(); head.Segment != g.Segments[1] || head.Position != 0 {
		t.Fatalf("head is on %s at %d, want segment %s at 0", head.Segment.Label, head.Position, g.Segments[1].Label)
	}
	grey90 := Color{229, 229, 229}
	checkFrame(t, "junction", sink.mustFrame(), map[int]Color{
		16: {255, 0, 218}, 17: {72, 0, 255}, 18: {0, 145, 255}, 19: {0, 255, 145},
		20: {72, 255, 0}, 21: {255, 218, 0},
		22: {255, 0, 0}, // head, over the first signal pixel
		23: {204, 204, 0}, 24: {178, 178, 0}, 25: {153, 153, 0}, 26: {127, 127, 0},
		446: grey90, 447: grey90, 448: grey90, 449: grey90, 450: grey90, 451: grey90, 452: grey90,
	})
}

// TestMoveSelfCollision checks the tick on which the eel runs into itself:
// Move reports it, sends nothing, and leaves the food alone even though
// the body is lying on it.
func TestMoveSelfCollision(t *testing.T) {
	g, sink := newTestGame(t)
	place(g, 0, 10, 7, 1.0, 0, 9, 3)
	// Bend the body so that the pixel ahead of the head is already taken.
	seg := g.Segments[0]
	g.Eel.Body = []*Point{
		{Segment: seg, Position: 10}, {Segment: seg, Position: 9}, {Segment: seg, Position: 8},
		{Segment: seg, Position: 11}, {Segment: seg, Position: 12}, {Segment: seg, Position: 13},
		{Segment: seg, Position: 14},
	}

	if err := g.Eel.Move(); err == nil {
		t.Fatal("Move returned no error for an eel that overlaps itself")
	}
	sink.expectNoFrame()
	if g.Eel.Growth != 0 {
		t.Errorf("Growth is %d: the eel ate on the tick it died", g.Eel.Growth)
	}
	if len(g.Food.Body) != 7 || !g.Food.Fresh {
		t.Errorf("food was bitten on the tick the eel died: %d points left, fresh=%v", len(g.Food.Body), g.Food.Fresh)
	}
}

// TestGameOverAnimationFrames runs the real animation on a 7-point eel.
// Each frame loses a point from both ends, and the ends are white.
func TestGameOverAnimationFrames(t *testing.T) {
	g, sink := newTestGame(t)
	place(g, 0, 10, 7, 1.0, 5, 12, 6)

	var wg sync.WaitGroup
	wg.Add(1)
	g.GameOverAnimation(&wg)

	want := []map[int]Color{
		{5: White, 6: {0, 96, 242}, 7: {0, 242, 96}, 8: {193, 242, 0}, 9: White},
		{6: White, 7: {0, 229, 0}, 8: White},
		{7: White},
	}
	for _, w := range want {
		checkFrame(t, "game over frame", sink.mustFrame(), w)
	}
	sink.expectNoFrame()
	if g.Brightness != 1.0 {
		t.Errorf("Brightness is %v after the animation, want 1", g.Brightness)
	}
}

// TestGameOverHue pins the hue arithmetic of the game-over frames at a
// body length where rounding matters (see TestMoveHeldDirection).
func TestGameOverHue(t *testing.T) {
	g, _ := newTestGame(t)
	place(g, 3, 17, 17, 1.0, 20, 12, 6)

	g.resetFrame()
	if !g.Eel.drawBodyGameOver(1.0) {
		t.Fatal("drawBodyGameOver drew nothing")
	}
	at := func(i int) Color {
		p := g.Eel.Body[i]
		return g.frame[p.Segment.Offset+p.Position]
	}
	if at(0) != White || at(16) != White {
		t.Errorf("ends are %v and %v, want white", at(0), at(16))
	}
	if want := (Color{0, 255, 120}); at(7) != want {
		t.Errorf("point 7 is %v, want %v", at(7), want)
	}
}

// TestMoveAllocations keeps an ordinary tick, rendering and the seven
// sends included, free of allocations.
func TestMoveAllocations(t *testing.T) {
	g, _ := newTestGame(t)
	place(g, 0, 6, 7, 0.75, 5, 12, 6)

	// 11 calls in all, which keeps the head on segment 0.
	allocs := testing.AllocsPerRun(10, func() {
		if err := g.Eel.Move(); err != nil {
			t.Fatalf("Move: %v", err)
		}
	})
	if allocs != 0 {
		t.Errorf("a tick allocates %v times, want 0", allocs)
	}
}
