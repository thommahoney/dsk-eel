package eel

type Direction int

const (
	Greater = iota
	Lesser  = iota
)

type Eel struct {
	Head      *Point
	Tail      *Point
	Length    int
	Direction Direction
}

type Food struct {
	Point *Point
}

type Dome struct {
	Segments []Segment
}

type Hop struct {
	Point *Point
	Direction Direction
}

type Segment struct {
	Label        string
	Length       int
	GreaterUp    *Hop
	GreaterDown  *Hop
	GreaterLeft  *Hop
	GreaterRight *Hop
	LesserUp     *Hop
	LesserDown   *Hop
	LesserLeft   *Hop
	LesserRight  *Hop
}

// LED closest to the ground is Position 0
// LED closest to the sky is Position X
type Point struct {
	Segment  *Segment
	Position int
}
