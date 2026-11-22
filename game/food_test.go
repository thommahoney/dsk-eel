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
// first. NewFood only looks at which pixels the eel covers, which segment
// its first point, the head, is on and which way the eel says it is
// travelling, so a body built this way does not have to be one an eel
// could have crawled into. The eels of these tests travel towards the high
// end of the head's segment unless a test says otherwise.
func pixelBody(g *Game, pixels []int) []*Point {
	body := make([]*Point, len(pixels))
	for i, px := range pixels {
		body[i] = &Point{Segment: g.Segments[px/testSegmentLength], Position: px % testSegmentLength}
	}
	return body
}

// headFirst returns the pixels with head moved to the front, which makes it
// the head of a body built from them.
func headFirst(pixels []int, head int) []int {
	i := slices.Index(pixels, head)
	if i < 0 {
		panic(fmt.Sprintf("pixel %d is not one of the eel's", head))
	}
	return append([]int{head}, slices.Delete(slices.Clone(pixels), i, i+1)...)
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

// segmentsAhead returns the segments that join the end of the eel's head
// segment that the eel is travelling towards: the ones it can be on after
// its next junction. They are read straight from the segment's hops.
func segmentsAhead(eel *Eel) []*Segment {
	return segmentsPast(eel.Body[0].Segment, eel.TravelDir)
}

// segmentsPast returns the segments that join the high end of segment, for
// Greater, or its low end, for Lesser.
func segmentsPast(head *Segment, dir Direction) []*Segment {
	hops := []*Hop{head.GreaterUp, head.GreaterDown, head.GreaterLeft, head.GreaterRight}
	if dir == Lesser {
		hops = []*Hop{head.LesserUp, head.LesserDown, head.LesserLeft, head.LesserRight}
	}
	var ahead []*Segment
	for _, h := range hops {
		if h != nil && !slices.Contains(ahead, h.Point.Segment) {
			ahead = append(ahead, h.Point.Segment)
		}
	}
	return ahead
}

// allowedFoodPlacements lists the placements a new food may take around the
// eel: the free ones that are neither on the segment the head is on nor on
// a segment one junction ahead of it. If there are none of those, the free
// ones one junction ahead are allowed, and if there are none of those
// either, the free ones on the head's segment.
func allowedFoodPlacements(eel *Eel) []int {
	covered := coveredPixels(eel.Body)
	if len(eel.Body) == 0 {
		return freeFoodPlacements(covered)
	}
	block := func(pixels [testPixels]bool, segments ...*Segment) [testPixels]bool {
		for _, seg := range segments {
			for pos := 0; pos < testSegmentLength; pos++ {
				pixels[seg.Offset+pos] = true
			}
		}
		return pixels
	}
	offHeadSegment := block(covered, eel.Body[0].Segment)
	if far := freeFoodPlacements(block(offHeadSegment, segmentsAhead(eel)...)); len(far) > 0 {
		return far
	}
	if ahead := freeFoodPlacements(offHeadSegment); len(ahead) > 0 {
		return ahead
	}
	return freeFoodPlacements(covered)
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
// sizes and shapes. The food must never touch the eel, and never be on the
// segment the eel's head is on, or on one a junction ahead of it, while a
// segment further away has room. Every allowed placement, which is every
// one those rules leave, must turn up, so the search does not quietly rule
// out more of the dome.
//
// calls is chosen so that an allowed placement going unseen by chance is
// out of the question. The first random try alone lands on any given
// allowed placement once in 784 calls, so in 40000 calls each is expected
// at least 51 times and missed with probability below e^-51; over 784
// placements and all the cases that stays under 1e-18. The cases with fewer
// calls allow at most 16 placements, and each of those comes up at least
// once in 19 calls.
func TestNewFoodAvoidsEel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		body  func(t *testing.T, g *Game) []*Point
		calls int
		// wantAllowed, if not zero, is how many placements the eel allows.
		wantAllowed int
		// lesser has the eel travel towards the low end of its head's
		// segment instead of the high end.
		lesser bool
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
			// leave 8 placements on each. The head is on segment 0, whose
			// high end joins segments 1 and 18.
			name:        "every 11th pixel",
			calls:       40000,
			wantAllowed: 8 * (testSegments - 3),
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, pixelsWhere(func(px int) bool { return px%11 == 0 }))
			},
		},
		{
			name:        "all but one segment",
			calls:       2000,
			wantAllowed: 16,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, pixelsWhere(func(px int) bool { return px/testSegmentLength != 30 }))
			},
		},
		{
			// Only positions 9..15 of segment 12 are free.
			name:        "all but seven pixels",
			calls:       500,
			wantAllowed: 1,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, pixelsWhere(func(px int) bool {
					return px/testSegmentLength != 12 || px%testSegmentLength < 9 || px%testSegmentLength > 15
				}))
			},
		},
		{
			// The same, with the head at position 0 of segment 12: the
			// only room is on the head's segment, so the food goes there.
			name:        "room only on the head's segment",
			calls:       500,
			wantAllowed: 1,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, headFirst(pixelsWhere(func(px int) bool {
					return px/testSegmentLength != 12 || px%testSegmentLength < 9 || px%testSegmentLength > 15
				}), 12*testSegmentLength))
			},
		},
		{
			// The head is alone on segment 12, at position 0, and the only
			// other room is segment 30, which is far from it. Most calls
			// have to look at all the placements, and must pick one of the
			// 16 on segment 30, not one of the 15 ahead of the head on
			// segment 12.
			name:        "room on the head's segment and one other",
			calls:       2000,
			wantAllowed: 16,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, headFirst(pixelsWhere(func(px int) bool {
					return px == 12*testSegmentLength || (px/testSegmentLength != 12 && px/testSegmentLength != 30)
				}), 12*testSegmentLength))
			},
		},
		{
			// The high end of segment 12 joins segments 16 and 17. With
			// room on 16 and on the head's own segment, the food goes on
			// 16: a junction away is better than straight ahead.
			name:        "room one junction ahead and on the head's segment",
			calls:       2000,
			wantAllowed: 16,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, headFirst(pixelsWhere(func(px int) bool {
					return px == 12*testSegmentLength || (px/testSegmentLength != 12 && px/testSegmentLength != 16)
				}), 12*testSegmentLength))
			},
		},
		{
			// With room on 17, one junction ahead, and on 30, far away,
			// the food goes on 30.
			name:        "room one junction ahead and far away",
			calls:       2000,
			wantAllowed: 16,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, headFirst(pixelsWhere(func(px int) bool {
					return px/testSegmentLength != 17 && px/testSegmentLength != 30
				}), 12*testSegmentLength))
			},
		},
		{
			// Travelling the other way, the eel is heading for the low end
			// of segment 12, which joins segments 11 and 13. Segment 16 is
			// behind it now and counts as far away, so with room on 11 and
			// on 16 the food goes on 16.
			name:        "room ahead and behind, travelling to the low end",
			calls:       2000,
			wantAllowed: 16,
			lesser:      true,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, headFirst(pixelsWhere(func(px int) bool {
					return px/testSegmentLength != 11 && px/testSegmentLength != 16
				}), 12*testSegmentLength))
			},
		},
		{
			// Only the dome's first segment has room, and the head is on
			// the second.
			name:        "room only on the first segment",
			calls:       2000,
			wantAllowed: 16,
			body: func(t *testing.T, g *Game) []*Point {
				return pixelBody(g, headFirst(pixelsWhere(func(px int) bool {
					return px/testSegmentLength != 0
				}), testSegmentLength))
			},
		},
		{
			// Only the last segment has room, and the head is in the
			// middle of it, at position 10. That leaves the placements
			// starting at 6..9 and 17..21, which only the look at all the
			// placements can find: every one of them has to come up, the
			// first and the last of the dome's last segment included.
			name:        "room on both sides of the head",
			calls:       500,
			wantAllowed: 9,
			body: func(t *testing.T, g *Game) []*Point {
				head := (testSegments-1)*testSegmentLength + 10
				return pixelBody(g, headFirst(pixelsWhere(func(px int) bool {
					return px == head || px/testSegmentLength != testSegments-1
				}), head))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g, _ := newTestGame(t)
			body := c.body(t, g)
			g.Eel = &Eel{Body: body, Game: g}
			if c.lesser {
				g.Eel.TravelDir = Lesser
			}
			covered := coveredPixels(body)
			allowed := allowedFoodPlacements(g.Eel)
			if c.wantAllowed != 0 && len(allowed) != c.wantAllowed {
				t.Fatalf("the eel allows %d placements, want %d", len(allowed), c.wantAllowed)
			}
			if len(allowed) == 0 {
				t.Fatal("the eel leaves no placement free")
			}

			seen := map[int]int{}
			for i := 0; i < c.calls; i++ {
				food := NewFood(g)
				placement := checkFoodShape(t, g, food)
				first := food.Body[0]
				if touches(food.Body, covered) {
					t.Fatalf("call %d: food on %s at %d..%d touches the eel",
						i, first.Segment.Label, first.Position-testFoodLength+1, first.Position)
				}
				if !slices.Contains(allowed, placement) {
					t.Fatalf("call %d: food on %s at %d..%d is too close to the eel's head, and there is room further away",
						i, first.Segment.Label, first.Position-testFoodLength+1, first.Position)
				}
				seen[placement]++
			}
			for _, placement := range allowed {
				if seen[placement] == 0 {
					t.Errorf("allowed placement on segment %d starting at %d never came up in %d calls",
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

// TestNewFoodWithAnEmptyEel: an eel with no body has no head either, and
// rules nothing out.
func TestNewFoodWithAnEmptyEel(t *testing.T) {
	g, _ := newTestGame(t)
	g.Eel = &Eel{Game: g}
	seen := map[int]bool{}
	for i := 0; i < 40000; i++ {
		seen[checkFoodShape(t, g, NewFood(g))] = true
	}
	if len(seen) != testPlacements {
		t.Errorf("%d different placements came up, want all %d", len(seen), testPlacements)
	}
}

// TestNewFoodAfterCrossingAJunction grows an eel by its last point on the
// tick that takes its head from segment 0 onto another segment, which is
// when Move asks for new food. The head's segment is then the one it has
// just moved onto, so the food must not be there or a junction ahead of
// there, and it may be on the segment the head has left.
func TestNewFoodAfterCrossingAJunction(t *testing.T) {
	g, _ := newTestGame(t)
	g.demoMode.Store(true)

	onSegmentLeft := 0
	for i := 0; i < 3000; i++ {
		// Head on the last pixel of segment 0; the old food is far away.
		place(g, 0, testSegmentLength-1, testFoodLength, 1.0, 30, 12, 6)
		old := g.Food
		g.Eel.Growth = 1
		if err := g.Eel.Move(); err != nil {
			t.Fatalf("round %d: the move through the junction ended the game: %v", i, err)
		}
		head := g.Eel.Head()
		if head.Segment == g.Segments[0] {
			t.Fatalf("round %d: the head is still on segment 0", i)
		}
		if g.Food == old {
			t.Fatalf("round %d: no new food when the eel finished growing", i)
		}
		checkFoodShape(t, g, g.Food)
		switch on := g.Food.Body[0].Segment; {
		case on == head.Segment:
			t.Fatalf("round %d: new food on %s, the segment the head has just moved onto", i, on.Label)
		case slices.Contains(segmentsAhead(g.Eel), on):
			t.Fatalf("round %d: new food on %s, one junction ahead of the head on %s", i, on.Label, head.Segment.Label)
		case on == g.Segments[0]:
			onSegmentLeft++
		}
	}
	// The eel still covers positions 15..21 of segment 0, which leaves 9
	// of the 729 allowed placements there, so at least 37 of the foods
	// are expected on it, and none with probability below e^-37.
	if onSegmentLeft == 0 {
		t.Error("no food ever appeared on the segment the head had just left")
	}
}

// towards says in words which end of its segment an eel travelling in dir
// is heading for.
func towards(dir Direction) string {
	if dir == Lesser {
		return "travelling towards the low end"
	}
	return "travelling towards the high end"
}

// TestNewFoodScanPicksEvenly leaves room only one junction ahead of the
// head: all of segment 16 and positions 15..21 of segment 17, with the
// head on segment 12. No random try is accepted there, so every call looks
// at all the placements, and each of the 17 must be as likely as the
// others: 16 on one segment do not share the chance of the single one on
// the other.
//
// With 20000 calls each placement is expected 1176 times with a standard
// deviation of 33; the test accepts 880..1470, about 9 of those either way.
func TestNewFoodScanPicksEvenly(t *testing.T) {
	g, _ := newTestGame(t)
	body := pixelBody(g, headFirst(pixelsWhere(func(px int) bool {
		switch px / testSegmentLength {
		case 16:
			return false
		case 17:
			return px%testSegmentLength < 15
		}
		return true
	}), 12*testSegmentLength))
	g.Eel = &Eel{Body: body, Game: g}
	allowed := allowedFoodPlacements(g.Eel)
	if len(allowed) != 17 {
		t.Fatalf("the eel allows %d placements, want 17", len(allowed))
	}

	const calls = 20000
	seen := map[int]int{}
	for i := 0; i < calls; i++ {
		seen[checkFoodShape(t, g, NewFood(g))]++
	}
	for _, placement := range allowed {
		if n := seen[placement]; n < 880 || n > 1470 {
			t.Errorf("the placement on segment %d starting at %d came up %d times in %d calls, want about %d",
				placement/testSegmentLength, placement%testSegmentLength, n, calls, calls/17)
		}
	}
	if len(seen) != len(allowed) {
		t.Errorf("%d different placements came up, want the %d allowed ones", len(seen), len(allowed))
	}
}

// TestRunMakesTheFoodAfterTheEel plays whole games through Run, with nobody
// at the stick, so that each ends at the eel's first junction. Run has to
// make the eel before the food, or the food cannot keep away from it: the
// first food of every game must be far from where the new eel started. The
// eel cannot have eaten it, so it is still there when the game is over.
//
// A food placed without looking at the new eel is too close to it in 3
// games out of 49, and never in 100 games with probability under 0.2%.
func TestRunMakesTheFoodAfterTheEel(t *testing.T) {
	t.Parallel()
	g, sink := newTestGame(t)
	g.tick = fastTick

	for game := 0; game < 100; game++ {
		waitGameOver(t, startGame(g))
		// The frames are not looked at; keep the socket from filling up.
		sink.drain(time.Millisecond)

		start := g.Eel.Segment
		if !g.Food.Fresh {
			t.Fatalf("game %d: the eel ate before its first junction", game)
		}
		on := g.Food.Body[0].Segment
		if on == start || slices.Contains(segmentsPast(start, Greater), on) {
			t.Fatalf("game %d: the eel started on %s and the first food was on %s", game, start.Label, on.Label)
		}
	}
}

// TestNewFoodIsNeverOneJunctionAhead puts a new eel's worth of points on
// every segment in turn, travelling each way, with the head on the last
// pixel before the junction. The next tick takes the head onto one of the
// segments that join there, so new food must be on none of them.
func TestNewFoodIsNeverOneJunctionAhead(t *testing.T) {
	g, _ := newTestGame(t)
	for _, segment := range g.Segments {
		for _, dir := range []Direction{Greater, Lesser} {
			body := line(segment, testSegmentLength-1, testSegmentLength-testFoodLength)
			if dir == Lesser {
				body = line(segment, 0, testFoodLength-1)
			}
			g.Eel = &Eel{Body: body, Game: g, Segment: segment, TravelDir: dir}
			ahead := segmentsAhead(g.Eel)
			if len(ahead) == 0 {
				t.Fatalf("segment %s, %s: the junction leads nowhere", segment.Label, towards(dir))
			}
			// Without the rule a food lands on a given segment once in
			// 48 calls, and never in 500 with probability e^-10.
			for i := 0; i < 500; i++ {
				on := NewFood(g).Body[0].Segment
				if on == segment || slices.Contains(ahead, on) {
					t.Fatalf("head on the last pixel of %s, %s: new food on %s", segment.Label, towards(dir), on.Label)
				}
			}
		}
	}
}

// TestNewFoodPastAOneWayJunction names the two junctions of the dome that
// lead to one segment only, whichever way the stick points, each approached
// from either side: b2 and b3 join end to end with nothing else there, and
// so do b6 and b7. An eel heading for one of them reaches the next segment
// with no turn to choose, so food there would be as good as straight
// ahead. The labels are written out here so that the test does not depend
// on how the code reads the hops.
func TestNewFoodPastAOneWayJunction(t *testing.T) {
	g, _ := newTestGame(t)
	byLabel := map[string]*Segment{}
	for _, s := range g.Segments {
		byLabel[s.Label] = s
	}
	for _, c := range []struct {
		from string
		dir  Direction
		next string
	}{
		{"b2", Greater, "b3"},
		{"b3", Lesser, "b2"},
		{"b6", Greater, "b7"},
		{"b7", Lesser, "b6"},
	} {
		from, next := byLabel[c.from], byLabel[c.next]
		if from == nil || next == nil {
			t.Fatalf("no segments labelled %s and %s", c.from, c.next)
		}
		// The head has just come onto the segment, a whole segment away
		// from the junction.
		body := line(from, 0, 0)
		if c.dir == Lesser {
			body = line(from, testSegmentLength-1, testSegmentLength-1)
		}
		g.Eel = &Eel{Body: body, Game: g, Segment: from, TravelDir: c.dir}
		for i := 0; i < 5000; i++ {
			if on := NewFood(g).Body[0].Segment; on == next || on == from {
				t.Fatalf("head on %s, heading for the one-way junction to %s: new food on %s", c.from, c.next, on.Label)
			}
		}
	}
}

// TestNewFoodLooksAtAllFourTurns rewires one junction so that each of the
// four turns leads to a segment of its own. On the dome as built, up and
// down already lead to every segment a junction joins, so only a changed
// junction shows that left and right are looked at too.
func TestNewFoodLooksAtAllFourTurns(t *testing.T) {
	g, _ := newTestGame(t)
	from := g.Segments[12]
	to := func(segment int) *Hop {
		return &Hop{Point: &Point{Segment: g.Segments[segment], Position: 0}, Direction: Greater}
	}
	from.GreaterUp, from.GreaterDown, from.GreaterLeft, from.GreaterRight = to(20), to(30), to(40), to(45)
	from.LesserUp, from.LesserDown, from.LesserLeft, from.LesserRight = to(21), to(31), to(41), to(46)

	for _, c := range []struct {
		dir   Direction
		ahead []int
	}{
		{Greater, []int{20, 30, 40, 45}},
		{Lesser, []int{21, 31, 41, 46}},
	} {
		g.Eel = &Eel{Body: line(from, 10, 10), Game: g, Segment: from, TravelDir: c.dir}
		for i := 0; i < 5000; i++ {
			on := NewFood(g).Body[0].Segment.Offset / testSegmentLength
			if on == 12 || slices.Contains(c.ahead, on) {
				t.Fatalf("on segment 12, %s, which leads to segments %v: new food on segment %d", towards(c.dir), c.ahead, on)
			}
		}
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
// A food on the segment of the eel's head, or one junction ahead of it,
// could be eaten without a real turn.
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
		if len(freeFoodPlacements(covered)) == 0 {
			return // an eel this big leaves no room; overlap is allowed
		}
		if touches(food.Body, covered) || !food.Fresh || len(food.Body) != testFoodLength {
			t.Fatalf("tick %d: new food appeared on a %d-point eel (fresh=%v, %d of %d points left)",
				tick, len(body), food.Fresh, len(food.Body), testFoodLength)
		}
		first := food.Body[0]
		if !slices.Contains(allowedFoodPlacements(g.Eel), first.Segment.Offset+first.Position) {
			t.Fatalf("tick %d: new food appeared on %s, too close to the head of a %d-point eel on %s, and there was room further away",
				tick, first.Segment.Label, len(body), body[0].Segment.Label)
		}
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
