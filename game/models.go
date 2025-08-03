package game

import (
	"fmt"
	"maps"
	"math/rand/v2"

	"github.com/thommahoney/dsk-eel/controller"
)

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
	TravelDir  Direction
}

func (g *Game) NewBody() []*Point {
	startingSegment := g.RandomSegment()
	body := []*Point{}
	start := Max(rand.N(SegmentLength), GrowthIncrement-1)
	for i := start; i > start-GrowthIncrement; i-- {
		body = append(body, &Point{Segment: startingSegment, Position: i})
	}
	return body
}

func NewEel(g *Game) *Eel {
	return &Eel{
		Body:       g.NewBody(),
		ControlDir: controller.Dir_Neutral,
		Game:       g,
		TravelDir:  Greater,
	}
}

// @todo don't spawn food on top of eel
func NewFood(g *Game) *Food {
	return &Food{
		Body: g.NewBody(),
		Game: g,
		Fresh: true,
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
	// @todo trigger sound!
}

func (e *Eel) Move() error {
	head := e.Head()
	travelDir := e.TravelDir

	var nextPoint *Point

	if (head.Position == 0 && travelDir == Lesser) ||
		(head.Position == SegmentLength-1 && travelDir == Greater) {

		var cDir controller.Direction
		if e.Game.Config.DemoMode && e.ControlDir == controller.Dir_Neutral {
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
		nextHop := head.Segment.NextHop(travelDir, cDir)

		if nextHop == nil {
			return fmt.Errorf("no next hop")
		}

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
		e.Body = append([]*Point{nextPoint}, e.Body...)
		e.Growth--

		if e.Growth == 0 {
			e.Game.Food = NewFood(e.Game)
		}
	} else {
		e.Body = append([]*Point{nextPoint}, e.Body[0:e.Length()-1]...)
	}

	eelBody := e.BodyPixels()
	food := e.Game.Food
	foodBody := food.BodyPixels()

	if hasCommonKeys(foodBody, eelBody) {
		if food.IsFresh() {
			e.Eat()
		}
		food.Chomp(e.TravelDir)
	}

	maps.Copy(eelBody, foodBody)

	// eelBody now includes foodBody too
	e.Game.Draw(eelBody)

	return nil
}

func (e *Eel) BodyPixels() map[int]Color {
	pixels := map[int]Color{}

	for i, point := range e.Body {
		pixels[point.Segment.Offset+point.Position] = hueToRGB((float64(i) * 360.0 / float64(len(e.Body))))
	}

	return pixels
}

// Represents the Food that the Eel encounters on its journey
// When the Eel encounters Food, its length is increased
type Food struct {
	Body []*Point
	Game *Game
	Fresh bool
}

func (f *Food) BodyPixels() map[int]Color {
	pixels := map[int]Color{}

	for i, point := range f.Body {
		pixels[point.Segment.Offset+point.Position] = hueToRGB((float64(i) * 360.0 / GrowthIncrement))
	}

	return pixels
}

func (f *Food) IsFresh() bool {
	return f.Fresh
}

func (f *Food) Chomp(d Direction) {
	if d == Greater {
	 	f.Body = f.Body[0:len(f.Body)-1]
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
	// @todo: handle NorthEast, SouthEast, SouthWest, NorthWest directions
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
