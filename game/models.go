package game

import (
	"fmt"
	"math/rand/v2"

	"github.com/thommahoney/dsk-eel/controller"
)

// initialBodyCap is the starting capacity allocated for Eel.Body. Body
// mutation in Move shifts in place, so once cap is large enough the body
// slice never re-allocates. 256 comfortably handles the longest games.
const initialBodyCap = 256

type Direction int

const (
	// Greater direction means that indexes are increasing
	Greater = Direction(iota)

	// Lesser direction means that indexes are decreasing
	Lesser = Direction(iota)

	// Initial length of Eel, length of food
	GrowthIncrement = 7
)

type Eel struct {
	Body       []*Point
	ControlDir controller.Direction
	Game       *Game
	Growth     int
	Segment    *Segment
	TravelDir  Direction
}

func (g *Game) NewBody() (*Segment, []*Point) {
	startingSegment := g.RandomSegment()
	body := make([]*Point, 0, initialBodyCap)
	start := Max(rand.N(SegmentLength), GrowthIncrement-1)
	for i := start; i > start-GrowthIncrement; i-- {
		body = append(body, &Point{Segment: startingSegment, Position: i})
	}
	return startingSegment, body
}

func NewEel(g *Game) *Eel {
	segment, body := g.NewBody()
	return &Eel{
		Body:       body,
		ControlDir: controller.Dir_Neutral,
		Game:       g,
		Segment:    segment,
		TravelDir:  Greater,
	}
}

// @todo don't spawn food on top of eel
func NewFood(g *Game) *Food {
	_, body := g.NewBody()

	return &Food{
		Body:  body,
		Game:  g,
		Fresh: true,
		Hue:   rand.Float64()*360.0,
	}
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
	e.Game.Ableton.FireClip(6, rand.Int32N(8)+1)
}

func (e *Eel) Move() error {
	head := e.Head()
	travelDir := e.TravelDir

	var nextPoint *Point
	nextHop := head.Segment.NextHop(travelDir, e.ControlDir)

	if (head.Position == 0 && travelDir == Lesser) ||
		(head.Position == SegmentLength-1 && travelDir == Greater) {

		var cDir controller.Direction
		if e.Game.DemoMode && e.ControlDir == controller.Dir_Neutral {
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
			cDir = e.ControlDir
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

	// Advance the body in place. Body.cap is pre-allocated in NewBody, so
	// normal growth stays on the same backing array; only an extremely long
	// game would hit the append path and grow the slice.
	if e.Growth > 0 {
		if len(e.Body) < cap(e.Body) {
			e.Body = e.Body[:len(e.Body)+1]
		} else {
			e.Body = append(e.Body, nil)
		}
		copy(e.Body[1:], e.Body[:len(e.Body)-1])
		e.Body[0] = nextPoint
		e.Growth--

		if e.Growth == 0 {
			e.Game.Food = NewFood(e.Game)
		}
	} else {
		copy(e.Body[1:], e.Body[:len(e.Body)-1])
		e.Body[0] = nextPoint
	}

	// Food collision: any eel body pixel touching any food pixel counts,
	// matching the pre-refactor hasCommonKeys semantics. Food and body
	// are both small so this is negligible work.
	food := e.Game.Food
	for _, bp := range e.Body {
		bIdx := bp.Segment.Offset + bp.Position
		hit := false
		for _, fp := range food.Body {
			if fp.Segment.Offset+fp.Position == bIdx {
				hit = true
				break
			}
		}
		if hit {
			if food.IsFresh() {
				e.Eat()
			}
			food.Chomp(e.TravelDir)
			break
		}
	}

	// Render the frame directly into g.frame. Order matters: turn signals
	// go down first so eel/food can paint over them (matching the old
	// maps.Copy precedence of food > eel > turn signals).
	g := e.Game
	g.resetFrame()
	e.drawTurnSignals(nextHop)
	if err := e.drawBody(); err != nil {
		return err
	}
	g.Food.draw()
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
	hueStep := 360.0 / float64(bodyLength)
	for i, point := range e.Body {
		idx := point.Segment.Offset + point.Position
		if g.frameSet[idx] {
			return fmt.Errorf("eel body overlaps")
		}
		g.frameSet[idx] = true
		g.frame[idx] = hsvToRGB(float64(i)*hueStep, 1.0, 1.0)
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
	hueStep := 360.0 / float64(bodyLength)
	for i, point := range e.Body {
		idx := point.Segment.Offset + point.Position
		if i == 0 || i == bodyLength-1 {
			g.frame[idx] = White
		} else {
			g.frame[idx] = hsvToRGB(float64(i)*hueStep, 1.0, brightness)
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

// draw paints the food pixels into f.Game.frame. Food is drawn last so
// it overwrites any eel body or turn signal pixels that share the same
// index, matching the pre-refactor precedence.
func (f *Food) draw() {
	color := hsvToRGB(0.0, 0.0, f.Game.Brightness)
	for _, point := range f.Body {
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
