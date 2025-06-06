package game

type Direction int

const (
	// Greater direction means that indexes are increasing
	Greater = Direction(iota)

	// Lesser direction means that indexes are decreasing
	Lesser  = Direction(iota)
)

type Eel struct {
	Head      *Point
	Tail      *Point
	Length    int
	Direction Direction
}

// Represents the Food that the Eel encounters on its journey
// When the Eel encounters Food, its length is increased
type Food struct {
	Point *Point
}

// Hop describes a junction point between Segments
//
// When moving from one Segment to the next, the eel either
// starts at the beginning or end of the next segment and
// therefore may travel in a different direction after a junction
// eg. B1 -> G9 is a transition from Greater to Lesser direction travel
type Hop struct {
	Point *Point
	Direction Direction
}

// Represents a physical dome pipe segment and the lights on it
type Segment struct {
	Label        string
	Length       int

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

// Represents a specific LED on a Segment
type Point struct {
	Segment  *Segment

	// LED closest to the ground cable is Position 0
	// LED furthest from the ground cable is Position (Segment.Length - 1)
	Position int
}
