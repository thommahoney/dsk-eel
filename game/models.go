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
	Segment    *Segment
	TravelDir  Direction
}

func (g *Game) NewBody() (*Segment, []*Point) {
	startingSegment := g.RandomSegment()
	body := []*Point{}
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
	nextHop := head.Segment.NextHop(travelDir, e.ControlDir)

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
		e.Body = append([]*Point{nextPoint}, e.Body...)
		e.Growth--

		if e.Growth == 0 {
			e.Game.Food = NewFood(e.Game)
		}
	} else {
		e.Body = append([]*Point{nextPoint}, e.Body[0:e.Length()-1]...)
	}

	eelBody, err := e.BodyPixels(1.0, true)
	if err != nil {
		return err
	}
	food := e.Game.Food
	foodBody := food.BodyPixels()

	if hasCommonKeys(foodBody, eelBody) {
		if food.IsFresh() {
			e.Eat()
		}
		food.Chomp(e.TravelDir)
	}

	pixels := e.TurnSignals(nextHop)

	maps.Copy(pixels, eelBody)
	maps.Copy(pixels, foodBody)

	e.Game.Draw(pixels)

	return nil
}

func (e *Eel) TurnSignals(h *Hop) map[int]Color {
	pixels := map[int]Color{}

	if h != nil {
		for i := 0; i < 5; i++ {
			var pos int
			if h.Direction == Greater {
				pos = h.Point.Segment.Offset + i
			} else {
				pos = h.Point.Segment.Offset + SegmentLength - i - 1
			}
			pixels[pos] = hsvToRGB(60, 1.0, e.Game.Brightness-(float64(i)*.1))
		}
	} else {
		for i := 0; i < 5; i++ {
			var pos int
			var up, down *Hop
			if e.TravelDir == Greater {
				up = e.Segment.GreaterUp
				down = e.Segment.GreaterDown
			} else {
				up = e.Segment.LesserUp
				down = e.Segment.LesserDown
			}

			if up.Direction == Greater {
				pos = up.Point.Segment.Offset + i
			} else {
				pos = up.Point.Segment.Offset + SegmentLength - i - 1
			}
			pixels[pos] = hsvToRGB(0, 1.0, e.Game.Brightness-(float64(i)*.1))

			if down.Direction == Greater {
				pos = down.Point.Segment.Offset + i
			} else {
				pos = down.Point.Segment.Offset + SegmentLength - i - 1
			}
			pixels[pos] = hsvToRGB(0, 1.0, e.Game.Brightness-(float64(i)*.1))
		}
	}

	return pixels
}

func (e *Eel) BodyPixels(brightness float64, validate bool) (map[int]Color, error) {
	pixels := map[int]Color{}

	for i, point := range e.Body {
		pixelPos := point.Segment.Offset + point.Position
		if _, k := pixels[pixelPos]; k && validate {
			return nil, fmt.Errorf("eel body overlaps")
		}
		pixels[pixelPos] = hsvToRGB((float64(i) * 360.0 / float64(len(e.Body))), 1.0, brightness)
	}

	return pixels, nil
}

// Represents the Food that the Eel encounters on its journey
// When the Eel encounters Food, its length is increased
type Food struct {
	Body  []*Point
	Game  *Game
	Fresh bool
}

func (f *Food) BodyPixels() map[int]Color {
	pixels := map[int]Color{}

	for i, point := range f.Body {
		pixels[point.Segment.Offset+point.Position] = hsvToRGB((float64(i) * 360.0 / GrowthIncrement), 1.0, f.Game.Brightness)
	}

	return pixels
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
