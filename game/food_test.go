package game

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/controller"
)

// The tests spell out the food's shape in their own literals, like the wire
// layout in helpers_test.go: 7 points in a row on one of 49 segments of 22
// pixels, which can start at positions 6..21, so 49*16 = 784 placements.
const (
	testSegments      = 49
	testSegmentLength = 22
	testFoodLength    = 7
	testPlacements    = 784
)

// pixelBody returns a body with one point on each of the given pixels, head
// first. NewFood only looks at which pixels the eel covers, so a body built
// this way does not have to be one an eel could have crawled into.
func pixelBody(g *Game, pixels []int) []*Point {
	body := make([]*Point, len(pixels))
	for i, px := range pixels {
		body[i] = &Point{Segment: g.Segments[px/testSegmentLength], Position: px % testSegmentLength}
	}
	return body
}

// pixelsWhere returns the pixels for which keep is true, in order.
func pixelsWhere(keep func(px int) bool) []int {
	var pixels []int
	for px := 0; px < testPixels; px++ {
		if keep(px) {
			pixels = append(pixels, px)
		}
	}
	return pixels
}

// walkedBody returns a body of n points lying the way a moving eel leaves
// them: a path through the segment graph that turns at junctions and never
// crosses itself.
func walkedBody(t *testing.T, g *Game, n int) []*Point {
	t.Helper()
	turns := []controller.Direction{controller.Dir_North, controller.Dir_South, controller.Dir_East, controller.Dir_West}

	// A random path can box itself in before it is long enough, so try
	// again until one fits. For the lengths the tests use, a few attempts
	// are enough; the limit only stops a broken graph from looping forever.
	for attempt := 0; attempt < 100000; attempt++ {
		var taken [testPixels]bool
		var path []*Point
		seg, pos, dir := g.Segments[rand.N(testSegments)], rand.N(testSegmentLength), Direction(rand.N(2))
		for len(path) < n && !taken[seg.Offset+pos] {
			taken[seg.Offset+pos] = true
			path = append(path, &Point{Segment: seg, Position: pos})

			switch {
			case (dir == Greater && pos == testSegmentLength-1) || (dir == Lesser && pos == 0):
				var open []*Hop
				for _, turn := range turns {
					if h := seg.NextHop(dir, turn); h != nil && !taken[h.Point.Segment.Offset+h.Point.Position] {
						open = append(open, h)
					}
				}
				if len(open) == 0 {
					break
				}
				h := open[rand.N(len(open))]
				seg, pos, dir = h.Point.Segment, h.Point.Position, h.Direction
			case dir == Greater:
				pos++
			default:
				pos--
			}
		}
		if len(path) == n {
			slices.Reverse(path) // the last point laid is the head
			return path
		}
	}
	t.Fatalf("found no path of %d points through the segment graph", n)
	return nil
}

// coveredPixels marks the pixels the given points are on.
func coveredPixels(points []*Point) (covered [testPixels]bool) {
	for _, p := range points {
		covered[p.Segment.Offset+p.Position] = true
	}
	return covered
}

// freeFoodPlacements lists every placement of a food that touches none of
// the covered pixels. A placement is numbered segment*22 + the position of
// the food's first point.
func freeFoodPlacements(covered [testPixels]bool) []int {
	var free []int
	for seg := 0; seg < testSegments; seg++ {
	next:
		for start := testFoodLength - 1; start < testSegmentLength; start++ {
			for pos := start; pos > start-testFoodLength; pos-- {
				if covered[seg*testSegmentLength+pos] {
					continue next
				}
			}
			free = append(free, seg*testSegmentLength+start)
		}
	}
	return free
}

// checkFoodShape fails unless the food is fresh and is 7 points in a row on
// one segment with its first point at the highest position, which is what
// Chomp relies on. It returns the food's placement number.
func checkFoodShape(t *testing.T, g *Game, food *Food) int {
	t.Helper()
	if !food.Fresh {
		t.Fatalf("new food is not fresh")
	}
	if food.Game != g {
		t.Fatalf("new food does not belong to the game")
	}
	if len(food.Body) != testFoodLength {
		t.Fatalf("new food has %d points, want %d", len(food.Body), testFoodLength)
	}
	first := food.Body[0]
	if first.Position < testFoodLength-1 || first.Position >= testSegmentLength {
		t.Fatalf("new food starts at position %d, outside 6..21", first.Position)
	}
	for i, p := range food.Body {
		if p.Segment != first.Segment || p.Position != first.Position-i {
			t.Fatalf("food point %d is on %s at %d, want %s at %d",
				i, p.Segment.Label, p.Position, first.Segment.Label, first.Position-i)
		}
	}
	return first.Segment.Offset + first.Position
}

// touches reports whether any of the points is on a covered pixel.
func touches(points []*Point, covered [testPixels]bool) bool {
	for _, p := range points {
		if covered[p.Segment.Offset+p.Position] {
			return true
		}
	}
	return false
}

// TestNewFoodAvoidsEel asks for food many times around eels of different
// sizes and shapes. The food must never touch the eel, and every placement
// the eel leaves free must turn up, so the search does not quietly rule out
// part of the dome.
//
// calls is chosen so that a free placement going unseen by chance is out of
// the question. The first random try alone lands on any given free
// placement once in 784 calls, so in 40000 calls each is expected at least
// 51 times and missed with probability below e^-51; over 784 placements and
// all the cases that stays under 1e-18. The cases with fewer calls leave at
// most 16 placements free, and each of those comes up at least once in 19
// calls.
func TestNewFoodAvoidsEel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		body  func(t *testing.T, g *Game) []*Point
		calls int
		// wantFree, if not zero, is how many placements the eel leaves free.
		wantFree int
	}{
		{name: "new eel", calls: 40000, body: func(t *testing.T, g *Game) []*Point { return NewEel(g).Body }},
		{name: "path of 60", calls: 40000, body: func(t *testing.T, g *Game) []*Point { return walkedBody(t, g, 60) }},
		{name: "path of 300", calls: 40000, body: func(t *testing.T, g *Game) []*Point { return walkedBody(t, g, 300) }},
		{
			name:  "500 pixels in a block",
			calls: 40000,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, pixelsWhere(func(px int) bool { return px >= 100 && px < 600 }))
			},
		},
		{
			// Two eel pixels on every segment, at positions 0 and 11,
			// leave 8 placements on each.
			name:     "every 11th pixel",
			calls:    40000,
			wantFree: 8 * testSegments,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, pixelsWhere(func(px int) bool { return px%11 == 0 }))
			},
		},
		{
			name:     "all but one segment",
			calls:    2000,
			wantFree: 16,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, pixelsWhere(func(px int) bool { return px/testSegmentLength != 30 }))
			},
		},
		{
			// Only positions 9..15 of segment 12 are free.
			name:     "all but seven pixels",
			calls:    500,
			wantFree: 1,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, pixelsWhere(func(px int) bool {
					return px/testSegmentLength != 12 || px%testSegmentLength < 9 || px%testSegmentLength > 15
				}))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g, _ := newTestGame(t)
			body := c.body(t, g)
			g.Eel = &Eel{Body: body, Game: g}
			covered := coveredPixels(body)
			free := freeFoodPlacements(covered)
			if c.wantFree != 0 && len(free) != c.wantFree {
				t.Fatalf("the eel leaves %d placements free, want %d", len(free), c.wantFree)
			}
			if len(free) == 0 {
				t.Fatal("the eel leaves no placement free")
			}

			seen := map[int]int{}
			for i := 0; i < c.calls; i++ {
				food := NewFood(g)
				placement := checkFoodShape(t, g, food)
				if touches(food.Body, covered) {
					first := food.Body[0]
					t.Fatalf("call %d: food on %s at %d..%d touches the eel",
						i, first.Segment.Label, first.Position-testFoodLength+1, first.Position)
				}
				seen[placement]++
			}
			for _, placement := range free {
				if seen[placement] == 0 {
					t.Errorf("free placement on segment %d starting at %d never came up in %d calls",
						placement/testSegmentLength, placement%testSegmentLength, c.calls)
				}
			}
		})
	}
}

// TestNewFoodWithoutAnEel: with no eel on the dome, every placement is free.
func TestNewFoodWithoutAnEel(t *testing.T) {
	g, _ := newTestGame(t)
	seen := map[int]bool{}
	for i := 0; i < 40000; i++ {
		seen[checkFoodShape(t, g, NewFood(g))] = true
	}
	if len(seen) != testPlacements {
		t.Errorf("%d different placements came up, want all %d", len(seen), testPlacements)
	}
}

// TestNewFoodNoRoom covers an eel that leaves no 7 free pixels in a row
// anywhere: it sits on positions 6, 13 and 20 of every segment. NewFood must
// still come back, with a food of the usual shape, which then has to overlap
// the eel.
func TestNewFoodNoRoom(t *testing.T) {
	g, _ := newTestGame(t)
	body := pixelBody(g, pixelsWhere(func(px int) bool { return px%testSegmentLength%7 == 6 }))
	g.Eel = &Eel{Body: body, Game: g}
	covered := coveredPixels(body)
	if free := freeFoodPlacements(covered); len(free) != 0 {
		t.Fatalf("the eel leaves %d placements free, want none", len(free))
	}

	// NewFood runs on its own goroutine so that a search that never ends
	// fails the test instead of hanging it.
	foods := make(chan []*Food, 1)
	go func() {
		made := make([]*Food, 1000)
		for i := range made {
			made[i] = NewFood(g)
		}
		foods <- made
	}()
	select {
	case made := <-foods:
		for i, food := range made {
			checkFoodShape(t, g, food)
			if !touches(food.Body, covered) {
				t.Errorf("call %d: food does not touch an eel that leaves no room", i)
			}
		}
	case <-time.After(30 * time.Second):
		t.Fatal("NewFood did not return when there was no room for food")
	}
}

// TestFoodSpawnsOffEelInPlay plays demo-mode games tick by tick with the
// real Move and looks at every food the moment it appears: at the start of
// a game, as Run makes it, and in the tick where the eel finishes growing.
// A food that landed on the eel would be on its body, and already bitten.
//
// A wandering eel finds food too rarely to make a short test of it, so the
// test also makes the eel grow by one point every few ticks. Each growth
// spurt ends in Move asking for new food, exactly as after a meal.
func TestFoodSpawnsOffEelInPlay(t *testing.T) {
	t.Parallel()
	g, _ := newTestGame(t)
	g.demoMode.Store(true)

	spawns, games, longest := 0, 0, 0
	checkSpawn := func(tick int) {
		t.Helper()
		spawns++
		food, body := g.Food, g.Eel.Body
		covered := coveredPixels(body)
		if !touches(food.Body, covered) && food.Fresh && len(food.Body) == testFoodLength {
			return
		}
		if len(freeFoodPlacements(covered)) == 0 {
			return // an eel this big leaves no room; overlap is allowed
		}
		t.Fatalf("tick %d: new food appeared on a %d-point eel (fresh=%v, %d of %d points left)",
			tick, len(body), food.Fresh, len(food.Body), testFoodLength)
	}
	newGame := func(tick int) {
		games++
		g.Eel = NewEel(g)
		g.Food = NewFood(g)
		checkSpawn(tick)
	}

	newGame(0)
	for tick := 1; tick <= 4000; tick++ {
		eel := g.Eel
		if eel.Growth == 0 && rand.N(3) == 0 {
			eel.Growth = 1
		}
		before := g.Food
		err := eel.Move()
		if g.Food != before {
			checkSpawn(tick)
		}
		longest = max(longest, len(eel.Body))
		if err != nil {
			newGame(tick)
		}
	}
	// Guard against the test passing because nothing happened.
	if spawns < 500 || longest < 50 {
		t.Errorf("only %d food spawns in %d games, longest eel %d points; the run did not exercise much",
			spawns, games, longest)
	}
	t.Logf("%d food spawns in %d games, longest eel %d points", spawns, games, longest)
}

// BenchmarkNewFood measures a call with eels of different lengths on the
// dome. The eel is a solid block of pixels, so at 1000 points only 54 of
// the 784 placements are free and most calls have to look at all of them.
func BenchmarkNewFood(b *testing.B) {
	for _, n := range []int{7, 50, 300, 1000} {
		b.Run(fmt.Sprintf("eel=%d", n), func(b *testing.B) {
			g := &Game{Segments: InitSegments()}
			g.Eel = &Eel{Body: pixelBody(g, pixelsWhere(func(px int) bool { return px < n })), Game: g}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				NewFood(g)
			}
		})
	}
}
