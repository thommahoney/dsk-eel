package game

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// Move reuses the Point of the tail as the next head. The tests in this
// file check the three things that could go wrong with that: a tick that
// allocates after all, a Point of the segment graph getting into the body
// and being overwritten there, and play that differs in any way from play
// with a new Point for every head.

var (
	cardinalTurns = []controller.Direction{
		controller.Dir_North, controller.Dir_South, controller.Dir_East, controller.Dir_West,
	}
	allStickPositions = []controller.Direction{
		controller.Dir_Neutral,
		controller.Dir_North, controller.Dir_South, controller.Dir_East, controller.Dir_West,
		controller.Dir_NorthEast, controller.Dir_SouthEast, controller.Dir_SouthWest, controller.Dir_NorthWest,
	}
)

// TestMoveAllocationsByLength: an ordinary tick allocates nothing, however
// long the eel is.
func TestMoveAllocationsByLength(t *testing.T) {
	skipWhereSendsAllocate(t)
	for _, n := range []int{7, 50, 300, 1000} {
		g := newLoopbackGame(t)
		lapEel(g, n)
		// AllocsPerRun calls the function once more than it is asked to.
		allocs := testing.AllocsPerRun(straightRun-1, func() {
			if err := g.Eel.Move(); err != nil {
				t.Fatalf("eel of %d: Move: %v", n, err)
			}
		})
		if allocs != 0 {
			t.Errorf("eel of %d: a tick allocates %v times, want 0", n, allocs)
		}
	}
}

// TestMoveAllocationsAtJunction: a tick that takes the eel through a
// junction allocates nothing either, whether the player chose the turn or
// demo mode did.
func TestMoveAllocationsAtJunction(t *testing.T) {
	skipWhereSendsAllocate(t)
	for _, demo := range []bool{false, true} {
		g := newLoopbackGame(t)
		place(g, 0, 21, 7, 0.75, 5, 12, 6)
		g.demoMode.Store(demo)
		if !demo {
			g.controlDir.Store(controller.Dir_North)
		}
		seg := g.Segments[0]

		allocs := testing.AllocsPerRun(50, func() {
			// Put the eel back at the end of segment 0, so that every
			// call is a junction tick.
			eel := g.Eel
			for i, p := range eel.Body {
				p.Segment, p.Position = seg, 21-i
			}
			eel.Segment, eel.TravelDir = seg, Greater
			if err := eel.Move(); err != nil {
				t.Fatalf("Move: %v", err)
			}
			if eel.Head().Segment == seg {
				t.Fatalf("the eel did not leave segment %s", seg.Label)
			}
		})
		if allocs != 0 {
			t.Errorf("demo mode %v: a junction tick allocates %v times, want 0", demo, allocs)
		}
	}
}

// TestMoveAllocationsWhileGrowing: a tick on which the eel grows needs one
// new Point, and nothing else while the body has spare capacity.
func TestMoveAllocationsWhileGrowing(t *testing.T) {
	skipWhereSendsAllocate(t)
	g := newLoopbackGame(t)
	place(g, 0, 6, 7, 0.75, 5, 12, 6)
	g.Eel.Body = append(make([]*Point, 0, initialBodyCap), g.Eel.Body...)
	g.Eel.Growth = 1000

	// 11 calls in all, which keeps the head on segment 0.
	allocs := testing.AllocsPerRun(10, func() {
		if err := g.Eel.Move(); err != nil {
			t.Fatalf("Move: %v", err)
		}
	})
	if allocs != 1 {
		t.Errorf("a growing tick allocates %v times, want 1", allocs)
	}
	if got := len(g.Eel.Body); got != 7+11 {
		t.Errorf("the eel is %d points long after 11 growing ticks, want 18", got)
	}
}

// hops returns the eight hops of a segment in a fixed order.
func hops(s *Segment) [8]*Hop {
	return [8]*Hop{
		s.GreaterUp, s.GreaterDown, s.GreaterLeft, s.GreaterRight,
		s.LesserUp, s.LesserDown, s.LesserLeft, s.LesserRight,
	}
}

// hopPoints returns the Points that belong to the segment graph.
func hopPoints(g *Game) map[*Point]bool {
	points := map[*Point]bool{}
	for _, s := range g.Segments {
		for _, h := range hops(s) {
			points[h.Point] = true
		}
	}
	return points
}

// checkGraphIntact compares the game's segment graph with a newly built
// one: every hop must still lead to the same pixel of the same segment, in
// the same direction.
func checkGraphIntact(t *testing.T, g *Game) {
	t.Helper()
	fresh := InitSegments()
	for i, s := range g.Segments {
		want := fresh[i]
		if s.Label != want.Label || s.Offset != want.Offset || s.Length != want.Length || s.Color != want.Color {
			t.Errorf("segment %d is %s at offset %d, want %s at offset %d", i, s.Label, s.Offset, want.Label, want.Offset)
		}
		wantHops := hops(want)
		for k, h := range hops(s) {
			w := wantHops[k]
			wantSegment := g.Segments[w.Point.Segment.Offset/testSegmentLength]
			if h.Point.Segment != wantSegment || h.Point.Position != w.Point.Position || h.Direction != w.Direction {
				t.Errorf("hop %d of segment %s leads to %s position %d direction %d, want %s position %d direction %d",
					k, s.Label, h.Point.Segment.Label, h.Point.Position, h.Direction,
					wantSegment.Label, w.Point.Position, w.Direction)
			}
		}
	}
}

// checkBodyOwnsItsPoints fails if a Point of the segment graph is in the
// eel's body, or if the same Point is in the body twice.
func checkBodyOwnsItsPoints(t *testing.T, eel *Eel, graph map[*Point]bool) {
	t.Helper()
	seen := make(map[*Point]bool, len(eel.Body))
	for i, p := range eel.Body {
		if graph[p] {
			t.Fatalf("body point %d is a Point of the segment graph (%s position %d)", i, p.Segment.Label, p.Position)
		}
		if seen[p] {
			t.Fatalf("body point %d is in the body twice", i)
		}
		seen[p] = true
	}
}

// TestSegmentGraphSurvivesPlay plays many demo-mode games in which the stick
// is also pushed to random turns and the eel grows a lot, and then checks
// that the segment graph is what it was. If Move ever kept a Point of the
// graph in the body, that Point would be overwritten when it came round to
// being reused as a head, and a hop would lead somewhere else from then on.
func TestSegmentGraphSurvivesPlay(t *testing.T) {
	t.Parallel()
	g := newLoopbackGame(t)
	// Nothing reads the frames in this test. With the socket closed the
	// seven sends of a tick fail at once, which makes the run much shorter.
	g.Chromatik.ArtNetConnection.Close()
	g.demoMode.Store(true)

	ticks := 100000
	if raceDetector {
		ticks = 6000
	}
	games, junctions, longest := 0, 0, 0
	newGame := func() {
		games++
		g.Eel = NewEel(g)
		g.Food = NewFood(g)
	}
	newGame()
	for tick := 0; tick < ticks; tick++ {
		eel := g.Eel
		switch rand.N(40) {
		case 0:
			g.controlDir.Store(cardinalTurns[rand.N(len(cardinalTurns))])
		case 1:
			g.controlDir.Store(controller.Dir_Neutral)
		}
		if rand.N(10) == 0 && len(eel.Body)+eel.Growth < 400 {
			eel.Growth += 1 + rand.N(5)
		}
		segment := eel.Segment
		err := eel.Move()
		if eel.Segment != segment {
			junctions++
		}
		longest = max(longest, len(eel.Body))
		if err != nil {
			newGame()
		}
	}
	checkGraphIntact(t, g)

	// Guard against the test passing because nothing happened. Both
	// limits are far below what a run produces.
	if junctions < ticks/200 || longest < 30 {
		t.Errorf("only %d junctions and a longest eel of %d points in %d ticks", junctions, longest, ticks)
	}
	t.Logf("%d ticks, %d games, %d junctions, longest eel %d points", ticks, games, junctions, longest)
}

// referenceMove is Eel.Move as it was when every tick made a new Point for
// the head, kept to compare against. It runs on a second game with a
// segment graph of its own, and reads the stick and the demo flag from that
// game. It is that code line for line except for the three numbered places,
// none of which is about how the body advances: the test has to give it the
// same luck as the game under test, and it talks to no one.
func referenceMove(e *Eel, demoTurn controller.Direction, newFood func() *Food) error {
	head := e.Head()
	travelDir := e.TravelDir

	controlDir := e.Game.controlDir.Load()

	var nextPoint *Point
	nextHop := head.Segment.NextHop(travelDir, controlDir)

	if (head.Position == 0 && travelDir == Lesser) ||
		(head.Position == SegmentLength-1 && travelDir == Greater) {

		var cDir controller.Direction
		if controlDir == controller.Dir_Neutral && e.Game.demoMode.Load() {
			// 1. The original picked one of the four turns at random.
			cDir = demoTurn
		} else {
			cDir = controlDir
		}

		nextHop = head.Segment.NextHop(travelDir, cDir)

		if nextHop == nil {
			return fmt.Errorf("no next hop")
		}

		e.Segment = nextHop.Point.Segment

		nextPoint = nextHop.Point
		e.TravelDir = nextHop.Direction
	} else {
		p := *head
		nextPoint = &p
		if e.TravelDir == Greater {
			nextPoint.Position++
		} else {
			nextPoint.Position--
		}
	}

	if e.Growth > 0 {
		e.Body = append(e.Body, nil)
		copy(e.Body[1:], e.Body[:len(e.Body)-1])
		e.Body[0] = nextPoint
		e.Growth--

		if e.Growth == 0 {
			// 2. The original called NewFood, which places it at random.
			e.Game.Food = newFood()
		}
	} else {
		copy(e.Body[1:], e.Body[:len(e.Body)-1])
		e.Body[0] = nextPoint
	}

	g := e.Game
	g.resetFrame()
	e.drawTurnSignals(nextHop)

	if err := e.drawBody(); err != nil {
		return err
	}

	food := g.Food
	foodBody := food.Body
	for _, fp := range foodBody {
		if g.frameSet[fp.Segment.Offset+fp.Position] {
			if food.IsFresh() {
				// 3. The original called Eat, which also fires an Ableton
				// clip, and ended by sending the frame with g.Draw.
				e.Growth += GrowthIncrement
			}
			food.Chomp(e.TravelDir)
			break
		}
	}
	food.draw(foodBody)

	return nil
}

// playStats counts what a run of random play went through.
type playStats struct {
	ticks, games                int
	demoJunctions, heldJunction int // junction ticks: turn picked by demo mode, by the stick
	noHopDeaths, collisions     int
	meals, foodsMadeByMove      int
	growthPast256, reusePast256 int // growing and ordinary ticks of an eel longer than 256
	longest                     int
}

func (s *playStats) add(o playStats) {
	s.ticks += o.ticks
	s.games += o.games
	s.demoJunctions += o.demoJunctions
	s.heldJunction += o.heldJunction
	s.noHopDeaths += o.noHopDeaths
	s.collisions += o.collisions
	s.meals += o.meals
	s.foodsMadeByMove += o.foodsMadeByMove
	s.growthPast256 += o.growthPast256
	s.reusePast256 += o.reusePast256
	s.longest = max(s.longest, o.longest)
}

// mirroredPlay plays the same games on two Games at once: g with the real
// Move, and ref with referenceMove. After every tick it compares them.
type mirroredPlay struct {
	t      *testing.T
	g, ref *Game
	// wire, if set, receives g's frames, and every frame is read back and
	// compared too.
	wire *artnetSink

	graph       map[*Point]bool                      // the Points of g's segment graph
	lastPackets [universeCount][artnetPacketLen]byte // g's packets after the last frame it sent

	stats playStats

	// The game being played.
	stick          int // how the stick moves: stickNeutral, stickNudged or stickBusy
	feedEvery      int // the eel is fed about once in this many ticks; 0 for never
	ticksLeft      int // the game is abandoned after this many more ticks
	brightnessStep float64
}

const (
	stickNeutral = iota // never touched
	stickNudged         // now and then moved to a random position
	stickBusy           // moved every few ticks, mostly to a turn that exists
)

// samePixel reports whether a point of one game and a point of the other
// are the same pixel of the dome.
func samePixel(a, b *Point) bool {
	return a.Segment.Offset+a.Position == b.Segment.Offset+b.Position
}

// mirrorPoints copies points onto the same pixels of another game's
// segments.
func mirrorPoints(to *Game, points []*Point, capacity int) []*Point {
	out := make([]*Point, len(points), max(capacity, len(points)))
	for i, p := range points {
		out[i] = &Point{Segment: to.Segments[p.Segment.Offset/testSegmentLength], Position: p.Position}
	}
	return out
}

// roomAhead returns how many times the eel can move before it reaches the
// end of its segment or runs into its own body.
func roomAhead(body []*Point, dir Direction) int {
	covered := coveredPixels(body)
	head := body[0]
	step := 1
	if dir == Lesser {
		step = -1
	}
	room := 0
	for pos := head.Position + step; pos >= 0 && pos < testSegmentLength && !covered[head.Segment.Offset+pos]; pos += step {
		room++
	}
	return room
}

// start puts an eel and a food on g and copies both to ref, along with the
// stick position g has at that moment. With a nil food the game gets a new
// one from NewFood, as in Run.
func (m *mirroredPlay) start(eel *Eel, food *Food, demo bool, stick, feedEvery, ticks int) {
	g, ref := m.g, m.ref
	m.stats.games++

	eel.Game = g
	g.Eel = eel
	g.demoMode.Store(demo)
	if food == nil {
		food = NewFood(g)
	}
	g.Food = food
	ref.Eel = &Eel{
		Body:      mirrorPoints(ref, eel.Body, cap(eel.Body)),
		Game:      ref,
		Growth:    eel.Growth,
		Segment:   ref.Segments[eel.Segment.Offset/testSegmentLength],
		TravelDir: eel.TravelDir,
	}
	ref.Food = &Food{Body: mirrorPoints(ref, food.Body, 0), Game: ref, Fresh: food.Fresh}
	ref.demoMode.Store(demo)
	ref.controlDir.Store(g.controlDir.Load())

	g.Brightness, m.brightnessStep = 1.0, -0.05
	m.stick, m.feedEvery, m.ticksLeft = stick, feedEvery, ticks
}

// walked returns an n-point eel lying along a path through the graph, with
// room to move at least eight times before anything can happen to it.
func (m *mirroredPlay) walked(n, growth int) *Eel {
	m.t.Helper()
	// About one path in three ends with that much room, so the limit is
	// never reached.
	for range 1000 {
		body, dir := walkedEel(m.t, m.g, n)
		if roomAhead(body, dir) < 8 {
			continue
		}
		body = append(make([]*Point, 0, max(initialBodyCap, n)), body...)
		return &Eel{Body: body, Growth: growth, Segment: body[0].Segment, TravelDir: dir}
	}
	m.t.Fatalf("found no path of %d points with room ahead of its head", n)
	return nil
}

// startGame starts game number n. The first seven games are set pieces that
// between them are certain to go through everything the comparison should
// see; the games after that are random.
func (m *mirroredPlay) startGame(n int) {
	g := m.g
	// Every game starts with the stick let go.
	g.controlDir.Store(controller.Dir_Neutral)
	straight := func(seg, head int) *Eel {
		s := g.Segments[seg]
		body := append(make([]*Point, 0, initialBodyCap), line(s, head, head-6)...)
		return &Eel{Body: body, Segment: s, TravelDir: Greater}
	}

	switch n {
	case 0:
		// A new eel in demo mode: it takes a random turn at its first
		// junction, within 16 ticks.
		m.start(NewEel(g), nil, true, stickNeutral, 0, 60)
	case 1:
		// A player holding the stick north takes the eel through its
		// first junction.
		g.controlDir.Store(controller.Dir_North)
		m.start(NewEel(g), nil, false, stickNeutral, 0, 30)
	case 2:
		// A player who does not touch the stick dies at the first
		// junction: no next hop.
		m.start(NewEel(g), nil, false, stickNeutral, 0, 60)
	case 3:
		// An eel bent back on itself dies on its first tick: the pixel
		// ahead of its head is taken.
		eel := straight(4, 10)
		for i, pos := range []int{10, 9, 8, 11, 12, 13, 14} {
			eel.Body[i].Position = pos
		}
		m.start(eel, nil, true, stickNeutral, 0, 60)
	case 4:
		// A meal: the food lies two pixels ahead. The eel bites on its
		// second tick, grows for seven ticks without leaving the segment,
		// and Move makes the next food.
		food := &Food{Body: line(g.Segments[7], 16, 10), Game: g, Fresh: true}
		m.start(straight(7, 8), food, true, stickNeutral, 0, 12)
	case 5:
		// A 250-point eel that grows by 40: its body outgrows the slice
		// NewEel would have given it.
		m.start(m.walked(250, 40), nil, true, stickNeutral, 0, 200)
	case 6:
		// A 300-point eel that does not grow: ordinary ticks of a long
		// eel.
		m.start(m.walked(300, 0), nil, true, stickNeutral, 0, 200)
	default:
		var eel *Eel
		if rand.N(2) == 0 {
			eel = NewEel(g)
		} else {
			lengths := []int{8 + rand.N(40), 100, 250, 300, 450}
			eel = m.walked(lengths[rand.N(len(lengths))], rand.N(2)*rand.N(40))
		}
		stick := rand.N(3)
		feeds := []int{0, 40, 8}
		m.start(eel, nil, stick != stickBusy, stick, feeds[rand.N(len(feeds))], 200+rand.N(1500))
	}
}

// tick moves both games on by one tick and compares them. It reports
// whether the game is over.
func (m *mirroredPlay) tick() (over bool) {
	t, g, ref := m.t, m.g, m.ref
	eel, refEel := g.Eel, ref.Eel
	m.stats.ticks++
	tick := m.stats.ticks

	// The same input for both.
	switch m.stick {
	case stickNudged:
		if rand.N(30) == 0 {
			g.controlDir.Store(allStickPositions[rand.N(len(allStickPositions))])
		}
	case stickBusy:
		if rand.N(4) == 0 {
			g.controlDir.Store(cardinalTurns[rand.N(len(cardinalTurns))])
			if rand.N(10) == 0 {
				g.controlDir.Store(allStickPositions[rand.N(len(allStickPositions))])
			}
		}
	}
	ref.controlDir.Store(g.controlDir.Load())
	if m.feedEvery > 0 && rand.N(m.feedEvery) == 0 && len(eel.Body)+eel.Growth < 550 {
		extra := 1 + rand.N(12)
		eel.Growth += extra
		refEel.Growth += extra
	}
	// The brightness breathes as it does in Mover.
	g.Brightness += m.brightnessStep
	if g.Brightness <= MinBrightness || g.Brightness >= MaxBrightness {
		m.brightnessStep = -m.brightnessStep
	}
	ref.Brightness = g.Brightness

	refHead := refEel.Head()
	atJunction := (refHead.Position == 0 && refEel.TravelDir == Lesser) ||
		(refHead.Position == testSegmentLength-1 && refEel.TravelDir == Greater)
	demoPicks := atJunction && ref.demoMode.Load() && ref.controlDir.Load() == controller.Dir_Neutral
	lengthBefore, growing := len(eel.Body), eel.Growth > 0
	foodBefore, wasFresh := g.Food, g.Food.Fresh

	err := eel.Move()

	// Where demo mode picked the turn, the reference has to be told which
	// one. It is the turn that leads to the pixel the real head is on now.
	demoTurn := controller.Dir_Neutral
	if demoPicks {
		for _, turn := range cardinalTurns {
			if h := refHead.Segment.NextHop(refEel.TravelDir, turn); h != nil && samePixel(h.Point, eel.Head()) {
				demoTurn = turn
			}
		}
		if demoTurn == controller.Dir_Neutral {
			t.Fatalf("tick %d: at the end of %s the eel went to %s position %d, where no turn leads",
				tick, refHead.Segment.Label, eel.Head().Segment.Label, eel.Head().Position)
		}
	}
	// Where Move made new food, the reference gets a copy of it.
	refMadeFood := false
	refErr := referenceMove(refEel, demoTurn, func() *Food {
		refMadeFood = true
		if g.Food == foodBefore {
			t.Fatalf("tick %d: the eel finished growing and Move made no new food", tick)
		}
		if !g.Food.Fresh || len(g.Food.Body) != testFoodLength {
			t.Fatalf("tick %d: the new food was bitten on the tick it appeared", tick)
		}
		return &Food{Body: mirrorPoints(ref, g.Food.Body, 0), Game: ref, Fresh: true}
	})
	if g.Food != foodBefore && !refMadeFood {
		t.Fatalf("tick %d: Move made new food when the eel had not finished growing", tick)
	}

	// The outcome of the tick.
	if (err == nil) != (refErr == nil) || (err != nil && err.Error() != refErr.Error()) {
		t.Fatalf("tick %d: Move returned %v, the reference %v", tick, err, refErr)
	}
	// The eel.
	if len(eel.Body) != len(refEel.Body) {
		t.Fatalf("tick %d: the eel is %d points long, the reference %d", tick, len(eel.Body), len(refEel.Body))
	}
	for i, p := range eel.Body {
		if r := refEel.Body[i]; !samePixel(p, r) {
			t.Fatalf("tick %d: body point %d of %d is on %s position %d, the reference on %s position %d",
				tick, i, len(eel.Body), p.Segment.Label, p.Position, r.Segment.Label, r.Position)
		}
	}
	if eel.Segment.Offset != refEel.Segment.Offset || eel.TravelDir != refEel.TravelDir || eel.Growth != refEel.Growth {
		t.Fatalf("tick %d: the eel is on %s going %d with %d to grow, the reference on %s going %d with %d",
			tick, eel.Segment.Label, eel.TravelDir, eel.Growth, refEel.Segment.Label, refEel.TravelDir, refEel.Growth)
	}
	// The food.
	if g.Food.Fresh != ref.Food.Fresh || len(g.Food.Body) != len(ref.Food.Body) {
		t.Fatalf("tick %d: the food has %d points left (fresh=%v), the reference %d (fresh=%v)",
			tick, len(g.Food.Body), g.Food.Fresh, len(ref.Food.Body), ref.Food.Fresh)
	}
	for i, p := range g.Food.Body {
		if !samePixel(p, ref.Food.Body[i]) {
			t.Fatalf("tick %d: food point %d differs from the reference", tick, i)
		}
	}
	// The frame, and the packets it was sent in. A tick on which the eel
	// dies sends nothing, so the packets must be those of the frame before.
	if g.frame != ref.frame || g.frameSet != ref.frameSet {
		t.Fatalf("tick %d: the frame differs from the reference", tick)
	}
	if err == nil {
		for px, want := range ref.frame {
			packet := &g.artnetBufs[px/testFullUniv]
			at := testHeaderLen + px%testFullUniv*3
			if got := (Color{packet[at], packet[at+1], packet[at+2]}); got != want {
				t.Fatalf("tick %d: pixel %d was sent as %v, the reference has %v", tick, px, got, want)
			}
		}
		m.lastPackets = g.artnetBufs
		if m.wire != nil {
			if got := m.wire.mustFrame(); got != ref.frame {
				t.Fatalf("tick %d: the frame received differs from the reference", tick)
			}
		}
	} else if g.artnetBufs != m.lastPackets {
		t.Fatalf("tick %d: the packets changed on a tick that sent no frame", tick)
	}
	// The body owns its points. Points only enter the body at the head, so
	// checking the head on every tick covers them all; the full check is
	// there to catch a Point that is in the body twice.
	if m.graph[eel.Head()] {
		t.Fatalf("tick %d: the head is a Point of the segment graph", tick)
	}
	if tick%256 == 0 || err != nil {
		checkBodyOwnsItsPoints(t, eel, m.graph)
	}
	if tick%4096 == 0 {
		checkGraphIntact(t, g)
	}

	s := &m.stats
	s.longest = max(s.longest, len(eel.Body))
	switch {
	case err == nil:
	case err.Error() == "no next hop":
		s.noHopDeaths++
	default:
		s.collisions++
	}
	if atJunction && (err == nil || err.Error() != "no next hop") {
		if demoPicks {
			s.demoJunctions++
		} else {
			s.heldJunction++
		}
	}
	if wasFresh && g.Food == foodBefore && !g.Food.Fresh {
		s.meals++
	}
	if refMadeFood {
		s.foodsMadeByMove++
	}
	if len(eel.Body) > initialBodyCap {
		if growing && len(eel.Body) > lengthBefore {
			s.growthPast256++
		} else if !growing && err == nil {
			s.reusePast256++
		}
	}

	m.ticksLeft--
	return err != nil || m.ticksLeft <= 0
}

// run plays games until the given number of ticks is reached, and then
// checks that the set pieces did what they are there for.
func (m *mirroredPlay) run(ticks int) playStats {
	t := m.t
	m.graph = hopPoints(m.g)
	m.lastPackets = m.g.artnetBufs

	for game := 0; m.stats.ticks < ticks; game++ {
		m.startGame(game)
		for !m.tick() {
		}
	}
	checkGraphIntact(t, m.g)
	checkGraphIntact(t, m.ref)
	if m.wire != nil {
		m.wire.expectNoFrame()
	}

	s := m.stats
	if s.demoJunctions == 0 || s.heldJunction == 0 || s.noHopDeaths == 0 || s.collisions == 0 ||
		s.meals == 0 || s.foodsMadeByMove == 0 || s.growthPast256 == 0 || s.reusePast256 == 0 {
		t.Errorf("the run missed something it should have gone through: %+v", s)
	}
	return s
}

// TestMoveMatchesReference plays random games on two Games side by side,
// one with Move and one with referenceMove, and compares them after every
// tick: the result of the move, the body point by point, the food, the
// frame and the Art-Net packets built from it. It also keeps checking that
// the body holds only Points of its own and that the segment graph is
// intact.
//
// Demo mode and a player's stick both get their turn, eels start at up to
// 450 points and are fed until they pass 550, and they die both ways. The four
// workers play at least 400000 ticks between them, or 16000 under the race
// detector, which a comparison on one goroutine gains nothing from.
func TestMoveMatchesReference(t *testing.T) {
	t.Parallel()
	const workers = 4
	ticks := 100000
	if raceDetector {
		ticks = 4000
	}

	results := make([]playStats, workers)
	t.Run("workers", func(t *testing.T) {
		for w := range workers {
			t.Run(fmt.Sprint(w), func(t *testing.T) {
				t.Parallel()
				m := &mirroredPlay{t: t, g: newLoopbackGame(t), ref: &Game{Segments: InitSegments()}}
				// The packets are compared where they are built, so
				// nothing needs to arrive anywhere. With the socket closed
				// the seven sends of a tick fail at once, which is what
				// makes this many ticks affordable.
				m.g.Chromatik.ArtNetConnection.Close()
				results[w] = m.run(ticks)
			})
		}
	})
	var total playStats
	for _, s := range results {
		total.add(s)
	}
	t.Logf("%+v", total)
}

// TestMoveSendsReferenceFrames is the same comparison over fewer ticks, with
// the socket open: every frame Move sends is read back from the loopback
// socket and compared with the reference's frame.
func TestMoveSendsReferenceFrames(t *testing.T) {
	t.Parallel()
	g, sink := newTestGame(t)
	m := &mirroredPlay{t: t, g: g, ref: &Game{Segments: InitSegments()}, wire: sink}
	t.Logf("%+v", m.run(2000))
}
