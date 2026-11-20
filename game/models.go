package game

import (
	"fmt"
	"math/rand/v2"

	"github.com/thommahoney/dsk-eel/controller"
)

// initialBodyCap is the capacity Eel.Body starts with. Move shifts the body
// in place, so the slice is not reallocated until the eel outgrows this.
const initialBodyCap = 256

type Direction int

const (
	// Greater direction means that indexes are increasing
	Greater = Direction(iota)

	// Lesser direction means that indexes are decreasing
	Lesser = Direction(iota)

	// Initial length of Eel, length of food
	GrowthIncrement = 7

	// eelStartPosition is where the head of a new eel starts on its
	// segment: the lowest position that leaves room for the body below it.
	// A new eel sets off towards the high end of its segment, so this is
	// the longest run-up a segment allows: SegmentLength-1-eelStartPosition
	// moves, a little over a second, before the tick that takes the eel
	// through its first junction. A game therefore never ends before the
	// player has had that long to choose a turn.
	eelStartPosition = GrowthIncrement - 1
)

type Eel struct {
	Body      []*Point
	Game      *Game
	Growth    int
	Segment   *Segment
	TravelDir Direction
}

// randomPlacement picks where a new food goes: a random segment and the
// position of the food's first point. The food runs from there towards
// lower positions, so the first point needs GrowthIncrement-1 positions
// below it. Every position that leaves that much room is equally likely.
func (g *Game) randomPlacement() (*Segment, int) {
	start := GrowthIncrement - 1 + rand.N(SegmentLength-GrowthIncrement+1)
	return g.RandomSegment(), start
}

// newBodyAt returns GrowthIncrement points in a row on segment, the first
// at position start and the rest below it, in a slice with room for
// capacity points.
func newBodyAt(segment *Segment, start, capacity int) []*Point {
	body := make([]*Point, 0, capacity)
	for i := start; i > start-GrowthIncrement; i-- {
		body = append(body, &Point{Segment: segment, Position: i})
	}
	return body
}

// NewEel returns a new eel on a random segment, with its head at
// eelStartPosition and travelling towards the high end.
func NewEel(g *Game) *Eel {
	segment := g.RandomSegment()
	return &Eel{
		Body:      newBodyAt(segment, eelStartPosition, initialBodyCap),
		Game:      g,
		Segment:   segment,
		TravelDir: Greater,
	}
}

// foodPlacementTries is how many random placements NewFood tries before it
// looks at every placement in turn. An eel has to cover a large part of the
// dome before eight random placements in a row all touch it or land on a
// segment that is too close to its head.
const foodPlacementTries = 8

// How close a segment is to the eel's head, for placing new food, from
// furthest to nearest. foodPlacement takes the first level that has room,
// and far is the zero value, which is what every segment is with no eel.
//
// Far from the head, the food is at least two junctions away, the stick
// decides at one of them at least, and the eel needs more than a second
// and a half to get there. A segment one junction ahead joins the end of
// the head's segment that the head is travelling towards. Food there can
// be one pixel away when the head is about to cross, and two junctions of
// the dome lead to one segment only, whatever the stick says. Food on the
// head's own segment can be straight ahead, to be eaten without a turn.
const (
	foodFarFromHead = iota
	foodOneJunctionAhead
	foodOnHeadSegment
	foodNearnessLevels
)

// NewFood returns a fresh food that does not touch the eel, as long as the
// dome still has GrowthIncrement free pixels in a row on some segment. The
// food is not on the segment the eel's head is on, nor on a segment that
// joins the end the head is travelling towards, unless only those segments
// have room.
func NewFood(g *Game) *Food {
	segment, start := g.foodPlacement()

	return &Food{
		// Food only ever shrinks, so it needs no spare capacity.
		Body:  newBodyAt(segment, start, GrowthIncrement),
		Game:  g,
		Fresh: true,
		Hue:   rand.Float64() * 360.0,
	}
}

// foodPlacement picks the segment and first position for a new food, away
// from every pixel the eel is on and as far from its head as the levels
// above allow: on a segment far from the head if one has room, or else one
// junction ahead of it, or else on the head's own segment.
//
// If the eel leaves no room anywhere, the food is placed at random and will
// overlap the eel: a game that far along gets a food it bites at once rather
// than no food at all.
func (g *Game) foodPlacement() (*Segment, int) {
	// Mark the eel's pixels from its body, and take the head from it too.
	// g.frameSet cannot be used: Move makes new food after it has advanced
	// the body but before it has redrawn, when frameSet still describes
	// the previous frame.
	var taken [SegmentCount * SegmentLength]bool
	var nearness [SegmentCount]uint8
	if g.Eel != nil && len(g.Eel.Body) > 0 {
		for _, p := range g.Eel.Body {
			taken[p.Segment.Offset+p.Position] = true
		}

		// A diagonal stick leads where one of these four does.
		head := g.Eel.Head().Segment
		for _, turn := range [...]controller.Direction{
			controller.Dir_North, controller.Dir_South, controller.Dir_East, controller.Dir_West,
		} {
			if hop := head.NextHop(g.Eel.TravelDir, turn); hop != nil {
				nearness[hop.Point.Segment.Offset/SegmentLength] = foodOneJunctionAhead
			}
		}
		nearness[head.Offset/SegmentLength] = foodOnHeadSegment
	}

	for range foodPlacementTries {
		segment, start := g.randomPlacement()
		if nearness[segment.Offset/SegmentLength] == foodFarFromHead && placementFree(&taken, segment, start) {
			return segment, start
		}
	}

	// Every random try touched the eel or landed too close to its head, so
	// look at all the placements, in one group for each level of nearness.
	// Each free one replaces its group's choice so far with probability
	// one in the number of free ones seen in the group, which leaves every
	// free placement of a group equally likely to be the group's choice.
	// The choice of the group furthest from the head that has one is used.
	var choices [foodNearnessLevels]struct {
		segment *Segment
		start   int
		free    int
	}
	for _, s := range g.Segments {
		c := &choices[nearness[s.Offset/SegmentLength]]
		for i := GrowthIncrement - 1; i < SegmentLength; i++ {
			if !placementFree(&taken, s, i) {
				continue
			}
			c.free++
			if rand.N(c.free) == 0 {
				c.segment, c.start = s, i
			}
		}
	}
	for _, c := range choices {
		if c.free > 0 {
			return c.segment, c.start
		}
	}
	return g.randomPlacement()
}

// placementFree reports whether none of the GrowthIncrement pixels of a body
// on segment, from position start downwards, is marked in taken.
func placementFree(taken *[SegmentCount * SegmentLength]bool, segment *Segment, start int) bool {
	for i := start; i > start-GrowthIncrement; i-- {
		if taken[segment.Offset+i] {
			return false
		}
	}
	return true
}

func (e *Eel) Length() int {
	return len(e.Body)
}

func (e *Eel) Head() *Point {
	return e.Body[0]
}

func (e *Eel) Tail() *Point {
	return e.Body[e.Length()-1]
}

func (e *Eel) Eat() {
	e.Growth += GrowthIncrement
	// One of the first eight clips on track 6, at random.
	e.Game.Ableton.FireClip(6, rand.Int32N(8))
}

func (e *Eel) Move() error {
	head := e.Head()
	travelDir := e.TravelDir

	// The joystick goroutine can change the direction at any moment. Read
	// it once, so the whole tick steers and signals by the same direction.
	controlDir := e.Game.controlDir.Load()

	// next is where the head goes on this tick.
	next := *head
	nextHop := head.Segment.NextHop(travelDir, controlDir)

	if (head.Position == 0 && travelDir == Lesser) ||
		(head.Position == SegmentLength-1 && travelDir == Greater) {

		var cDir controller.Direction
		if controlDir == controller.Dir_Neutral && e.Game.inDemoMode() {
			switch rand.N(4) {
			case 0:
				cDir = controller.Dir_North
			case 1:
				cDir = controller.Dir_South
			case 2:
				cDir = controller.Dir_East
			case 3:
				cDir = controller.Dir_West
			}
		} else {
			cDir = controlDir
		}

		nextHop = head.Segment.NextHop(travelDir, cDir)

		if nextHop == nil {
			return fmt.Errorf("no next hop")
		}

		e.Segment = nextHop.Point.Segment

		// Copy the hop's point rather than keep the pointer. That Point is
		// part of the segment graph, and the points in the body are written
		// to again when they are reused below.
		next = *nextHop.Point
		e.TravelDir = nextHop.Direction
	} else if e.TravelDir == Greater {
		next.Position++
	} else {
		next.Position--
	}

	// Advance the body in place. NewEel gave the slice spare capacity, so
	// growing only reallocates once the eel is longer than initialBodyCap.
	if e.Growth > 0 {
		e.Body = append(e.Body, nil)
		copy(e.Body[1:], e.Body[:len(e.Body)-1])
		e.Body[0] = &Point{Segment: next.Segment, Position: next.Position}
		e.Growth--

		if e.Growth == 0 {
			e.Game.Food = NewFood(e.Game)
		}
	} else {
		// The tail leaves its pixel on this tick, so its Point becomes the
		// new head and a tick without growth allocates nothing. This works
		// because the body owns every Point in it: nothing else holds on to
		// one from tick to tick.
		tail := e.Tail()
		*tail = next
		copy(e.Body[1:], e.Body[:len(e.Body)-1])
		e.Body[0] = tail
	}

	// Build the frame in g.frame. Later layers paint over earlier ones:
	// turn signals, then the eel, then the food.
	g := e.Game
	g.resetFrame()
	e.drawTurnSignals(nextHop)

	// drawBody also finds self-collisions. That has to happen before the
	// food is looked at, so an eel that dies on this tick does not eat.
	if err := e.drawBody(); err != nil {
		return err
	}

	// The eel bites the food if any part of its body is on a food pixel.
	// The food is painted as it was before this tick's bite: Chomp only
	// reslices Body, so foodBody still includes the pixel being eaten.
	food := g.Food
	foodBody := food.Body
	for _, fp := range foodBody {
		if g.frameSet[fp.Segment.Offset+fp.Position] {
			if food.IsFresh() {
				e.Eat()
			}
			food.Chomp(e.TravelDir)
			break
		}
	}
	food.draw(foodBody)

	g.Draw()

	return nil
}

// drawTurnSignals paints the turn-signal preview pixels into g.frame.
// With a non-nil Hop it shows a yellow "turn here" indicator at the
// approaching junction; with a nil Hop it shows red "straight ahead"
// indicators on the up/down neighbors of the current segment.
func (e *Eel) drawTurnSignals(h *Hop) {
	g := e.Game
	if h != nil {
		base := h.Point.Segment.Offset
		for i := 0; i < 5; i++ {
			var pos int
			if h.Direction == Greater {
				pos = base + i
			} else {
				pos = base + SegmentLength - i - 1
			}
			g.frame[pos] = hsvToRGB(60, 1.0, g.Brightness-(float64(i)*.1))
		}
		return
	}

	var up, down *Hop
	if e.TravelDir == Greater {
		up = e.Segment.GreaterUp
		down = e.Segment.GreaterDown
	} else {
		up = e.Segment.LesserUp
		down = e.Segment.LesserDown
	}
	upBase := up.Point.Segment.Offset
	downBase := down.Point.Segment.Offset
	for i := 0; i < 5; i++ {
		v := g.Brightness - (float64(i) * .1)
		red := hsvToRGB(0, 1.0, v)

		var pos int
		if up.Direction == Greater {
			pos = upBase + i
		} else {
			pos = upBase + SegmentLength - i - 1
		}
		g.frame[pos] = red

		if down.Direction == Greater {
			pos = downBase + i
		} else {
			pos = downBase + SegmentLength - i - 1
		}
		g.frame[pos] = red
	}
}

// drawBody paints the live eel body into g.frame, tracking occupancy in
// g.frameSet so we can detect self-collision (two body points at the same
// pixel) with an O(1) lookup. Returns an error on self-collision, which
// the Mover loop converts into a Game Over.
func (e *Eel) drawBody() error {
	g := e.Game
	bodyLength := len(e.Body)
	if bodyLength == 0 {
		return nil
	}
	for i, point := range e.Body {
		idx := point.Segment.Offset + point.Position
		if g.frameSet[idx] {
			return fmt.Errorf("eel body overlaps")
		}
		g.frameSet[idx] = true
		g.frame[idx] = hsvToRGB(float64(i)*360.0/float64(bodyLength), 1.0, 1.0)
	}
	return nil
}

// drawBodyGameOver paints the eel body into g.frame for the game-over
// animation: head and tail are white, middle is the rainbow hsv ramp
// scaled by the current brightness. Overlaps are allowed (the body is
// shrinking and may briefly self-intersect).
func (e *Eel) drawBodyGameOver(brightness float64) bool {
	g := e.Game
	bodyLength := len(e.Body)
	if bodyLength == 0 {
		return false
	}
	for i, point := range e.Body {
		idx := point.Segment.Offset + point.Position
		if i == 0 || i == bodyLength-1 {
			g.frame[idx] = White
		} else {
			g.frame[idx] = hsvToRGB(float64(i)*360.0/float64(bodyLength), 1.0, brightness)
		}
	}
	return true
}

func (e *Eel) Shrink() {
	bodyLength := len(e.Body)
	if bodyLength >= 2 {
		e.Body = e.Body[1 : bodyLength-1]
	} else {
		e.Body = nil
	}
}

// Represents the Food that the Eel encounters on its journey
// When the Eel encounters Food, its length is increased
type Food struct {
	Body  []*Point
	Fresh bool
	Game  *Game
	Hue   float64
}

// draw paints the given food points into the frame, over whatever is
// already there.
func (f *Food) draw(body []*Point) {
	color := hsvToRGB(0.0, 0.0, f.Game.Brightness)
	for _, point := range body {
		f.Game.frame[point.Segment.Offset+point.Position] = color
	}
}

func (f *Food) IsFresh() bool {
	return f.Fresh
}

func (f *Food) Chomp(d Direction) {
	if d == Greater {
		f.Body = f.Body[0 : len(f.Body)-1]
	} else {
		f.Body = f.Body[1:len(f.Body)]
	}
	f.Fresh = false
}

// Hop describes a junction point between Segments
//
// When moving from one Segment to the next, the eel either
// starts at the beginning or end of the next segment and
// therefore may travel in a different direction after a junction
// eg. B1 -> G9 is a transition from Greater to Lesser direction travel
type Hop struct {
	Point     *Point
	Direction Direction
}

// Represents a physical dome pipe segment and the lights on it
type Segment struct {
	Label  string
	Length int
	Offset int

	Color Color

	// [Greater|Lesser][Up|Down|Left|Right] hops
	// are akin to traversing a graph or a tree structure
	// with additional metadata for the game
	//
	// ie. If the eel is traveling in the Greater direction
	// and the joystick is in the "up" position... go here
	// and potentially change direction
	GreaterUp    *Hop
	GreaterDown  *Hop
	GreaterLeft  *Hop
	GreaterRight *Hop
	LesserUp     *Hop
	LesserDown   *Hop
	LesserLeft   *Hop
	LesserRight  *Hop
}

// NextHop returns the correct next Hop for the Eel to follow
//
// This function should only be called when the head of the eel meets a vertex.
// If NextHop returns nil it means either there's a bug in the program or the
// game is over due to incorrect controller direction
func (s *Segment) NextHop(tDir Direction, cDir controller.Direction) *Hop {
	switch tDir {
	case Greater:
		switch cDir {
		case controller.Dir_North:
			return s.GreaterUp
		case controller.Dir_South:
			return s.GreaterDown
		case controller.Dir_East:
			return s.GreaterRight
		case controller.Dir_West:
			return s.GreaterLeft
		case controller.Dir_NorthEast:
			if s.GreaterUp == s.GreaterRight {
				return s.GreaterUp
			}
		case controller.Dir_SouthEast:
			if s.GreaterDown == s.GreaterRight {
				return s.GreaterDown
			}
		case controller.Dir_SouthWest:
			if s.GreaterDown == s.GreaterLeft {
				return s.GreaterDown
			}
		case controller.Dir_NorthWest:
			if s.GreaterUp == s.GreaterLeft {
				return s.GreaterUp
			}
		}
	case Lesser:
		switch cDir {
		case controller.Dir_North:
			return s.LesserUp
		case controller.Dir_South:
			return s.LesserDown
		case controller.Dir_East:
			return s.LesserRight
		case controller.Dir_West:
			return s.LesserLeft
		case controller.Dir_NorthEast:
			if s.LesserUp == s.LesserRight {
				return s.LesserUp
			}
		case controller.Dir_SouthEast:
			if s.LesserDown == s.LesserRight {
				return s.LesserDown
			}
		case controller.Dir_SouthWest:
			if s.LesserDown == s.LesserLeft {
				return s.LesserDown
			}
		case controller.Dir_NorthWest:
			if s.LesserUp == s.LesserLeft {
				return s.LesserUp
			}
		}
	}

	return nil
}

// Represents a specific LED on a Segment
type Point struct {
	Segment *Segment

	// LED closest to the ground cable is Position 0
	// LED furthest from the ground cable is Position (Segment.Length - 1)
	Position int
}
