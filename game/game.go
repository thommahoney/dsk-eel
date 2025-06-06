package eel

import "log/slog"

const (
	SegmentLength = 22
)

type Game struct {
	logger *slog.Logger

	Dome *Dome
}

func NewGame(logger *slog.Logger) *Game {
	segments := make([]Segment, 49)

	// blue - loop 3
	b1 := Segment{Label: "b1", Length: SegmentLength}
	b1Start := Hop{Point: &Point{Segment: &b1, Position: 0}, Direction: Greater}
	b1End := Hop{Point: &Point{Segment: &b1, Position: SegmentLength - 1}, Direction: Lesser}
	b2 := Segment{Label: "b2", Length: SegmentLength}
	b2Start := Hop{Point: &Point{Segment: &b2, Position: 0}, Direction: Greater}
	b2End := Hop{Point: &Point{Segment: &b2, Position: SegmentLength - 1}, Direction: Lesser}
	b3 := Segment{Label: "b3", Length: SegmentLength}
	b3Start := Hop{Point: &Point{Segment: &b3, Position: 0}, Direction: Greater}
	b3End := Hop{Point: &Point{Segment: &b3, Position: SegmentLength - 1}, Direction: Lesser}
	b4 := Segment{Label: "b4", Length: SegmentLength}
	b4Start := Hop{Point: &Point{Segment: &b4, Position: 0}, Direction: Greater}
	b4End := Hop{Point: &Point{Segment: &b4, Position: SegmentLength - 1}, Direction: Lesser}
	b5 := Segment{Label: "b5", Length: SegmentLength}
	b5Start := Hop{Point: &Point{Segment: &b5, Position: 0}, Direction: Greater}
	b5End := Hop{Point: &Point{Segment: &b5, Position: SegmentLength - 1}, Direction: Lesser}
	b6 := Segment{Label: "b6", Length: SegmentLength}
	b6Start := Hop{Point: &Point{Segment: &b6, Position: 0}, Direction: Greater}
	b6End := Hop{Point: &Point{Segment: &b6, Position: SegmentLength - 1}, Direction: Lesser}
	b7 := Segment{Label: "b7", Length: SegmentLength}
	b7Start := Hop{Point: &Point{Segment: &b7, Position: 0}, Direction: Greater}
	b7End := Hop{Point: &Point{Segment: &b7, Position: SegmentLength - 1}, Direction: Lesser}
	b8 := Segment{Label: "b8", Length: SegmentLength}
	b8Start := Hop{Point: &Point{Segment: &b8, Position: 0}, Direction: Greater}
	b8End := Hop{Point: &Point{Segment: &b8, Position: SegmentLength - 1}, Direction: Lesser}
	b9 := Segment{Label: "b9", Length: SegmentLength}
	b9Start := Hop{Point: &Point{Segment: &b9, Position: 0}, Direction: Greater}
	b9End := Hop{Point: &Point{Segment: &b9, Position: SegmentLength - 1}, Direction: Lesser}

	// red - loop 4
	r1 := Segment{Label: "r1", Length: SegmentLength}
	r1Start := Hop{Point: &Point{Segment: &r1, Position: 0}, Direction: Greater}
	r1End := Hop{Point: &Point{Segment: &r1, Position: SegmentLength - 1}, Direction: Lesser}
	r2 := Segment{Label: "r2", Length: SegmentLength}
	r2Start := Hop{Point: &Point{Segment: &r2, Position: 0}, Direction: Greater}
	r2End := Hop{Point: &Point{Segment: &r2, Position: SegmentLength - 1}, Direction: Lesser}
	r3 := Segment{Label: "r3", Length: SegmentLength}
	r3Start := Hop{Point: &Point{Segment: &r3, Position: 0}, Direction: Greater}
	r3End := Hop{Point: &Point{Segment: &r3, Position: SegmentLength - 1}, Direction: Lesser}
	r4 := Segment{Label: "r4", Length: SegmentLength}
	r4Start := Hop{Point: &Point{Segment: &r4, Position: 0}, Direction: Greater}
	r4End := Hop{Point: &Point{Segment: &r4, Position: SegmentLength - 1}, Direction: Lesser}
	r5 := Segment{Label: "r5", Length: SegmentLength}
	r5Start := Hop{Point: &Point{Segment: &r5, Position: 0}, Direction: Greater}
	r5End := Hop{Point: &Point{Segment: &r5, Position: SegmentLength - 1}, Direction: Lesser}
	r6 := Segment{Label: "r6", Length: SegmentLength}
	r6Start := Hop{Point: &Point{Segment: &r6, Position: 0}, Direction: Greater}
	r6End := Hop{Point: &Point{Segment: &r6, Position: SegmentLength - 1}, Direction: Lesser}
	r7 := Segment{Label: "r7", Length: SegmentLength}
	r7Start := Hop{Point: &Point{Segment: &r7, Position: 0}, Direction: Greater}
	r7End := Hop{Point: &Point{Segment: &r7, Position: SegmentLength - 1}, Direction: Lesser}
	r8 := Segment{Label: "r8", Length: SegmentLength}
	r8Start := Hop{Point: &Point{Segment: &r8, Position: 0}, Direction: Greater}
	r8End := Hop{Point: &Point{Segment: &r8, Position: SegmentLength - 1}, Direction: Lesser}
	r9 := Segment{Label: "r9", Length: SegmentLength}
	r9Start := Hop{Point: &Point{Segment: &r9, Position: 0}, Direction: Greater}
	r9End := Hop{Point: &Point{Segment: &r9, Position: SegmentLength - 1}, Direction: Lesser}
	r10 := Segment{Label: "r10", Length: SegmentLength}
	r10Start := Hop{Point: &Point{Segment: &r10, Position: 0}, Direction: Greater}
	r10End := Hop{Point: &Point{Segment: &r10, Position: SegmentLength - 1}, Direction: Lesser}

	// purple - loop 5
	p1 := Segment{Label: "p1", Length: SegmentLength}
	p1Start := Hop{Point: &Point{Segment: &p1, Position: 0}, Direction: Greater}
	p1End := Hop{Point: &Point{Segment: &p1, Position: SegmentLength - 1}, Direction: Lesser}
	p2 := Segment{Label: "p2", Length: SegmentLength}
	p2Start := Hop{Point: &Point{Segment: &p2, Position: 0}, Direction: Greater}
	p2End := Hop{Point: &Point{Segment: &p2, Position: SegmentLength - 1}, Direction: Lesser}
	p3 := Segment{Label: "p3", Length: SegmentLength}
	p3Start := Hop{Point: &Point{Segment: &p3, Position: 0}, Direction: Greater}
	p3End := Hop{Point: &Point{Segment: &p3, Position: SegmentLength - 1}, Direction: Lesser}
	p4 := Segment{Label: "p4", Length: SegmentLength}
	p4Start := Hop{Point: &Point{Segment: &p4, Position: 0}, Direction: Greater}
	p4End := Hop{Point: &Point{Segment: &p4, Position: SegmentLength - 1}, Direction: Lesser}
	p5 := Segment{Label: "p5", Length: SegmentLength}
	p5Start := Hop{Point: &Point{Segment: &p5, Position: 0}, Direction: Greater}
	p5End := Hop{Point: &Point{Segment: &p5, Position: SegmentLength - 1}, Direction: Lesser}
	p6 := Segment{Label: "p6", Length: SegmentLength}
	p6Start := Hop{Point: &Point{Segment: &p6, Position: 0}, Direction: Greater}
	p6End := Hop{Point: &Point{Segment: &p6, Position: SegmentLength - 1}, Direction: Lesser}
	p7 := Segment{Label: "p7", Length: SegmentLength}
	p7Start := Hop{Point: &Point{Segment: &p7, Position: 0}, Direction: Greater}
	p7End := Hop{Point: &Point{Segment: &p7, Position: SegmentLength - 1}, Direction: Lesser}
	p8 := Segment{Label: "p8", Length: SegmentLength}
	p8Start := Hop{Point: &Point{Segment: &p8, Position: 0}, Direction: Greater}
	p8End := Hop{Point: &Point{Segment: &p8, Position: SegmentLength - 1}, Direction: Lesser}
	p9 := Segment{Label: "p9", Length: SegmentLength}
	p9Start := Hop{Point: &Point{Segment: &p9, Position: 0}, Direction: Greater}
	p9End := Hop{Point: &Point{Segment: &p9, Position: SegmentLength - 1}, Direction: Lesser}
	p10 := Segment{Label: "p10", Length: SegmentLength}
	p10Start := Hop{Point: &Point{Segment: &p10, Position: 0}, Direction: Greater}
	p10End := Hop{Point: &Point{Segment: &p10, Position: SegmentLength - 1}, Direction: Lesser}

	// yellow - loop 1
	y1 := Segment{Label: "y1", Length: SegmentLength}
	y1Start := Hop{Point: &Point{Segment: &y1, Position: 0}, Direction: Greater}
	y1End := Hop{Point: &Point{Segment: &y1, Position: SegmentLength - 1}, Direction: Lesser}
	y2 := Segment{Label: "y2", Length: SegmentLength}
	y2Start := Hop{Point: &Point{Segment: &y2, Position: 0}, Direction: Greater}
	y2End := Hop{Point: &Point{Segment: &y2, Position: SegmentLength - 1}, Direction: Lesser}
	y3 := Segment{Label: "y3", Length: SegmentLength}
	y3Start := Hop{Point: &Point{Segment: &y3, Position: 0}, Direction: Greater}
	y3End := Hop{Point: &Point{Segment: &y3, Position: SegmentLength - 1}, Direction: Lesser}
	y4 := Segment{Label: "y4", Length: SegmentLength}
	y4Start := Hop{Point: &Point{Segment: &y4, Position: 0}, Direction: Greater}
	y4End := Hop{Point: &Point{Segment: &y4, Position: SegmentLength - 1}, Direction: Lesser}
	y5 := Segment{Label: "y5", Length: SegmentLength}
	y5Start := Hop{Point: &Point{Segment: &y5, Position: 0}, Direction: Greater}
	y5End := Hop{Point: &Point{Segment: &y5, Position: SegmentLength - 1}, Direction: Lesser}
	y6 := Segment{Label: "y6", Length: SegmentLength}
	y6Start := Hop{Point: &Point{Segment: &y6, Position: 0}, Direction: Greater}
	y6End := Hop{Point: &Point{Segment: &y6, Position: SegmentLength - 1}, Direction: Lesser}
	y7 := Segment{Label: "y7", Length: SegmentLength}
	y7Start := Hop{Point: &Point{Segment: &y7, Position: 0}, Direction: Greater}
	y7End := Hop{Point: &Point{Segment: &y7, Position: SegmentLength - 1}, Direction: Lesser}
	y8 := Segment{Label: "y8", Length: SegmentLength}
	y8Start := Hop{Point: &Point{Segment: &y8, Position: 0}, Direction: Greater}
	y8End := Hop{Point: &Point{Segment: &y8, Position: SegmentLength - 1}, Direction: Lesser}
	y9 := Segment{Label: "y9", Length: SegmentLength}
	y9Start := Hop{Point: &Point{Segment: &y9, Position: 0}, Direction: Greater}
	y9End := Hop{Point: &Point{Segment: &y9, Position: SegmentLength - 1}, Direction: Lesser}
	y10 := Segment{Label: "y10", Length: SegmentLength}
	y10Start := Hop{Point: &Point{Segment: &y10, Position: 0}, Direction: Greater}
	y10End := Hop{Point: &Point{Segment: &y10, Position: SegmentLength - 1}, Direction: Lesser}

	// green - loop 2
	g1 := Segment{Label: "g1", Length: SegmentLength}
	g1Start := Hop{Point: &Point{Segment: &g1, Position: 0}, Direction: Greater}
	g1End := Hop{Point: &Point{Segment: &g1, Position: SegmentLength - 1}, Direction: Lesser}
	g2 := Segment{Label: "g2", Length: SegmentLength}
	g2Start := Hop{Point: &Point{Segment: &g2, Position: 0}, Direction: Greater}
	g2End := Hop{Point: &Point{Segment: &g2, Position: SegmentLength - 1}, Direction: Lesser}
	g3 := Segment{Label: "g3", Length: SegmentLength}
	g3Start := Hop{Point: &Point{Segment: &g3, Position: 0}, Direction: Greater}
	g3End := Hop{Point: &Point{Segment: &g3, Position: SegmentLength - 1}, Direction: Lesser}
	g4 := Segment{Label: "g4", Length: SegmentLength}
	g4Start := Hop{Point: &Point{Segment: &g4, Position: 0}, Direction: Greater}
	g4End := Hop{Point: &Point{Segment: &g4, Position: SegmentLength - 1}, Direction: Lesser}
	g5 := Segment{Label: "g5", Length: SegmentLength}
	g5Start := Hop{Point: &Point{Segment: &g5, Position: 0}, Direction: Greater}
	g5End := Hop{Point: &Point{Segment: &g5, Position: SegmentLength - 1}, Direction: Lesser}
	g6 := Segment{Label: "g6", Length: SegmentLength}
	g6Start := Hop{Point: &Point{Segment: &g6, Position: 0}, Direction: Greater}
	g6End := Hop{Point: &Point{Segment: &g6, Position: SegmentLength - 1}, Direction: Lesser}
	g7 := Segment{Label: "g7", Length: SegmentLength}
	g7Start := Hop{Point: &Point{Segment: &g7, Position: 0}, Direction: Greater}
	g7End := Hop{Point: &Point{Segment: &g7, Position: SegmentLength - 1}, Direction: Lesser}
	g8 := Segment{Label: "g8", Length: SegmentLength}
	g8Start := Hop{Point: &Point{Segment: &g8, Position: 0}, Direction: Greater}
	g8End := Hop{Point: &Point{Segment: &g8, Position: SegmentLength - 1}, Direction: Lesser}
	g9 := Segment{Label: "g9", Length: SegmentLength}
	g9Start := Hop{Point: &Point{Segment: &g9, Position: 0}, Direction: Greater}
	g9End := Hop{Point: &Point{Segment: &g9, Position: SegmentLength - 1}, Direction: Lesser}
	g10 := Segment{Label: "g10", Length: SegmentLength}
	g10Start := Hop{Point: &Point{Segment: &g10, Position: 0}, Direction: Greater}
	g10End := Hop{Point: &Point{Segment: &g10, Position: SegmentLength - 1}, Direction: Lesser}

	// connections
	b1.GreaterUp = &b2Start
	b1.GreaterDown = &g9End
	b1.GreaterLeft = &g9End
	b1.GreaterRight = &b2Start
	b1.LesserUp = &b9End
	b1.LesserDown = &g10End
	b1.LesserLeft = &g10End
	b1.LesserRight = &b9End

	b2.GreaterUp = &b3Start
	b2.GreaterDown = &b3Start
	b2.GreaterLeft = &b3Start
	b2.GreaterRight = &b3Start
	b2.LesserUp = &g9End
	b2.LesserDown = &b1End
	b2.LesserLeft = &g9End
	b2.LesserRight = &b1End

	b3.GreaterUp = &b4Start
	b3.GreaterDown = &g7Start
	b3.GreaterLeft = &g7Start
	b3.GreaterRight = &b4Start
	b3.LesserUp = &b2End
	b3.LesserDown = &b2End
	b3.LesserLeft = &b2End
	b3.LesserRight = &b2End

	b4.GreaterUp = &g6End
	b4.GreaterDown = &b5Start
	b4.GreaterLeft = &g6End
	b4.GreaterRight = &b5Start
	b4.LesserUp = &g7Start
	b4.LesserDown = &b3End
	b4.LesserLeft = &g7Start
	b4.LesserRight = &b3End

	b5.GreaterUp = &r6Start
	b5.GreaterDown = &r5End
	b5.GreaterLeft = &r6Start
	b5.GreaterRight = &r5End
	b5.LesserUp = &g6End
	b5.LesserDown = &b4End
	b5.LesserLeft = &b4End
	b5.LesserRight = &g6End

	b6.GreaterUp = &b7Start
	b6.GreaterDown = &b7Start
	b6.GreaterLeft = &b7Start
	b6.GreaterRight = &b7Start
	b6.LesserUp = &r5Start
	b6.LesserDown = &r4End
	b6.LesserLeft = &r5Start
	b6.LesserRight = &r4End

	b7.GreaterUp = &b8Start
	b7.GreaterDown = &b9Start
	b7.GreaterLeft = &b9Start
	b7.GreaterRight = &b8Start
	b7.LesserUp = &b6End
	b7.LesserDown = &b6End
	b7.LesserLeft = &b6End
	b7.LesserRight = &b6End

	b8.GreaterUp = &r2Start
	b8.GreaterDown = &r1End
	b8.GreaterLeft = &r2Start
	b8.GreaterRight = &r1End
	b8.LesserUp = &b7End
	b8.LesserDown = &b9Start
	b8.LesserLeft = &b9Start
	b8.LesserRight = &b7End

	b9.GreaterUp = &b1Start
	b9.GreaterDown = &r1Start
	b9.GreaterLeft = &b1Start
	b9.GreaterRight = &r1Start
	b9.LesserUp = &b7End
	b9.LesserDown = &b8Start
	b9.LesserLeft = &b7End
	b9.LesserRight = &b8Start

	// red
	r1.GreaterUp = &r2Start
	r1.GreaterDown = &b8End
	r1.GreaterLeft = &b8End
	r1.GreaterRight = &r2Start
	r1.LesserUp = &r10End
	r1.LesserDown = &b9End
	r1.LesserLeft = &b9End
	r1.LesserRight = &r10End

	r2.GreaterUp = &r4Start
	r2.GreaterDown = &r3Start
	r2.GreaterLeft = &r4Start
	r2.GreaterRight = &r3Start
	r2.LesserUp = &b8End
	r2.LesserDown = &r1End
	r2.LesserLeft = &b8End
	r2.LesserRight = &r1End

	r3.GreaterUp = &r7End
	r3.GreaterDown = &r8Start
	r3.GreaterLeft = &r7End
	r3.GreaterRight = &r8Start
	r3.LesserUp = &r4Start
	r3.LesserDown = &r2End
	r3.LesserLeft = &r2End
	r3.LesserRight = &r4Start

	r4.GreaterUp = &r5Start
	r4.GreaterDown = &b6Start
	r4.GreaterLeft = &b6Start
	r4.GreaterRight = &r5Start
	r4.LesserUp = &r3Start
	r4.LesserDown = &r2End
	r4.LesserLeft = &r2End
	r4.LesserRight = &r3Start

	r5.GreaterUp = &b5End
	r5.GreaterDown = &r6Start
	r5.GreaterLeft = &b5End
	r5.GreaterRight = &r6Start
	r5.LesserUp = &b6Start
	r5.LesserDown = &r4End
	r5.LesserLeft = &b6Start
	r5.LesserRight = &r4End

	r6.GreaterUp = &p6Start
	r6.GreaterDown = &p5End
	r6.GreaterLeft = &p6Start
	r6.GreaterRight = &p5End
	r6.LesserUp = &b5End
	r6.LesserDown = &r5End
	r6.LesserLeft = &r5End
	r6.LesserRight = &b5End

	r7.GreaterUp = &r3End
	r7.GreaterDown = &r8Start
	r7.GreaterLeft = &r3End
	r7.GreaterRight = &r8Start
	r7.LesserUp = &p5Start
	r7.LesserDown = &p4End
	r7.LesserLeft = &p5Start
	r7.LesserRight = &p4End

	r8.GreaterUp = &r9Start
	r8.GreaterDown = &r10Start
	r8.GreaterLeft = &r10Start
	r8.GreaterRight = &r9Start
	r8.LesserUp = &r7End
	r8.LesserDown = &r3End
	r8.LesserLeft = &r3End
	r8.LesserRight = &r7End

	r9.GreaterUp = &p2Start
	r9.GreaterDown = &p1End
	r9.GreaterLeft = &p2Start
	r9.GreaterRight = &p1End
	r9.LesserUp = &r8End
	r9.LesserDown = &r10Start
	r9.LesserLeft = &r10Start
	r9.LesserRight = &r8End

	r10.GreaterUp = &r1Start
	r10.GreaterDown = &p1Start
	r10.GreaterLeft = &r1Start
	r10.GreaterRight = &p1Start
	r10.LesserUp = &r8End
	r10.LesserDown = &r9Start
	r10.LesserLeft = &r8End
	r10.LesserRight = &r9Start

	// purple
	p1.GreaterUp = &p2Start
	p1.GreaterDown = &r9End
	p1.GreaterLeft = &r9End
	p1.GreaterRight = &p2Start
	p1.LesserUp = &p10End
	p1.LesserDown = &r10End
	p1.LesserLeft = &r10End
	p1.LesserRight = &p10End

	p2.GreaterUp = &p4Start
	p2.GreaterDown = &p3Start
	p2.GreaterLeft = &p4Start
	p2.GreaterRight = &p3Start
	p2.LesserUp = &r9End
	p2.LesserDown = &p1End
	p2.LesserLeft = &r9End
	p2.LesserRight = &p1End

	p3.GreaterUp = &p7End
	p3.GreaterDown = &p8Start
	p3.GreaterLeft = &p7End
	p3.GreaterRight = &p8Start
	p3.LesserUp = &p4Start
	p3.LesserDown = &p2End
	p3.LesserLeft = &p2End
	p3.LesserRight = &p4Start

	p4.GreaterUp = &p5Start
	p4.GreaterDown = &r7Start
	p4.GreaterLeft = &r7Start
	p4.GreaterRight = &p5Start
	p4.LesserUp = &p3Start
	p4.LesserDown = &p2End
	p4.LesserLeft = &p2End
	p4.LesserRight = &p3Start

	p5.GreaterUp = &r6End
	p5.GreaterDown = &p6Start
	p5.GreaterLeft = &r6End
	p5.GreaterRight = &p6Start
	p5.LesserUp = &r7Start
	p5.LesserDown = &p4End
	p5.LesserLeft = &r7Start
	p5.LesserRight = &p4End

	p6.GreaterUp = &y6Start
	p6.GreaterDown = &y5End
	p6.GreaterLeft = &y6Start
	p6.GreaterRight = &y5End
	p6.LesserUp = &r6End
	p6.LesserDown = &p5End
	p6.LesserLeft = &p5End
	p6.LesserRight = &r6End

	p7.GreaterUp = &p3End
	p7.GreaterDown = &p8Start
	p7.GreaterLeft = &p3End
	p7.GreaterRight = &p8Start
	p7.LesserUp = &y5Start
	p7.LesserDown = &y4End
	p7.LesserLeft = &y5Start
	p7.LesserRight = &y4End

	p8.GreaterUp = &p9Start
	p8.GreaterDown = &p10Start
	p8.GreaterLeft = &p10Start
	p8.GreaterRight = &p9Start
	p8.LesserUp = &p7End
	p8.LesserDown = &p3End
	p8.LesserLeft = &p3End
	p8.LesserRight = &p7End

	p9.GreaterUp = &y2Start
	p9.GreaterDown = &y1End
	p9.GreaterLeft = &y2Start
	p9.GreaterRight = &y1End
	p9.LesserUp = &p8End
	p9.LesserDown = &p10Start
	p9.LesserLeft = &p10Start
	p9.LesserRight = &p8End

	p10.GreaterUp = &p1Start
	p10.GreaterDown = &y1Start
	p10.GreaterLeft = &p1Start
	p10.GreaterRight = &y1Start
	p10.LesserUp = &p8End
	p10.LesserDown = &p9Start
	p10.LesserLeft = &p8End
	p10.LesserRight = &p9Start

	// yellow
	y1.GreaterUp = &y2Start
	y1.GreaterDown = &p9End
	y1.GreaterLeft = &p9End
	y1.GreaterRight = &y2Start
	y1.LesserUp = &y10End
	y1.LesserDown = &p10End
	y1.LesserLeft = &p10End
	y1.LesserRight = &y10End

	y2.GreaterUp = &y4Start
	y2.GreaterDown = &y3Start
	y2.GreaterLeft = &y4Start
	y2.GreaterRight = &y3Start
	y2.LesserUp = &p9End
	y2.LesserDown = &y1End
	y2.LesserLeft = &p9End
	y2.LesserRight = &y1End

	y3.GreaterUp = &y7End
	y3.GreaterDown = &y8Start
	y3.GreaterLeft = &y7End
	y3.GreaterRight = &y8Start
	y3.LesserUp = &y4Start
	y3.LesserDown = &y2End
	y3.LesserLeft = &y2End
	y3.LesserRight = &y4Start

	y4.GreaterUp = &y5Start
	y4.GreaterDown = &p7Start
	y4.GreaterLeft = &p7Start
	y4.GreaterRight = &y5Start
	y4.LesserUp = &y3Start
	y4.LesserDown = &y2End
	y4.LesserLeft = &y2End
	y4.LesserRight = &y3Start

	y5.GreaterUp = &p6End
	y5.GreaterDown = &y6Start
	y5.GreaterLeft = &p6End
	y5.GreaterRight = &y6Start
	y5.LesserUp = &p7Start
	y5.LesserDown = &y4End
	y5.LesserLeft = &p7Start
	y5.LesserRight = &y4End

	y6.GreaterUp = &g6Start
	y6.GreaterDown = &g5End
	y6.GreaterLeft = &g6Start
	y6.GreaterRight = &g5End
	y6.LesserUp = &p6End
	y6.LesserDown = &y5End
	y6.LesserLeft = &y5End
	y6.LesserRight = &p6End

	y7.GreaterUp = &y3End
	y7.GreaterDown = &y8Start
	y7.GreaterLeft = &y3End
	y7.GreaterRight = &y8Start
	y7.LesserUp = &g5Start
	y7.LesserDown = &g4End
	y7.LesserLeft = &g5Start
	y7.LesserRight = &g4End

	y8.GreaterUp = &y9Start
	y8.GreaterDown = &y10Start
	y8.GreaterLeft = &y10Start
	y8.GreaterRight = &y9Start
	y8.LesserUp = &y7End
	y8.LesserDown = &y3End
	y8.LesserLeft = &y3End
	y8.LesserRight = &y7End

	y9.GreaterUp = &g2Start
	y9.GreaterDown = &g1End
	y9.GreaterLeft = &g2Start
	y9.GreaterRight = &g1End
	y9.LesserUp = &y8End
	y9.LesserDown = &y10Start
	y9.LesserLeft = &y10Start
	y9.LesserRight = &y8End

	y10.GreaterUp = &y1Start
	y10.GreaterDown = &g1Start
	y10.GreaterLeft = &y1Start
	y10.GreaterRight = &g1Start
	y10.LesserUp = &y8End
	y10.LesserDown = &y9Start
	y10.LesserLeft = &y8End
	y10.LesserRight = &y9Start

	// green
	g1.GreaterUp = &g2Start
	g1.GreaterDown = &y9End
	g1.GreaterLeft = &y9End
	g1.GreaterRight = &g2Start
	g1.LesserUp = &g10End
	g1.LesserDown = &y10End
	g1.LesserLeft = &y10End
	g1.LesserRight = &g10End

	g2.GreaterUp = &g4Start
	g2.GreaterDown = &g3Start
	g2.GreaterLeft = &g4Start
	g2.GreaterRight = &g3Start
	g2.LesserUp = &y9End
	g2.LesserDown = &g1End
	g2.LesserLeft = &y9End
	g2.LesserRight = &g1End

	g3.GreaterUp = &g7End
	g3.GreaterDown = &g8Start
	g3.GreaterLeft = &g7End
	g3.GreaterRight = &g8Start
	g3.LesserUp = &g4Start
	g3.LesserDown = &g2End
	g3.LesserLeft = &g2End
	g3.LesserRight = &g4Start

	g4.GreaterUp = &g5Start
	g4.GreaterDown = &y7Start
	g4.GreaterLeft = &y7Start
	g4.GreaterRight = &g5Start
	g4.LesserUp = &g3Start
	g4.LesserDown = &g2End
	g4.LesserLeft = &g2End
	g4.LesserRight = &g3Start

	g5.GreaterUp = &y6End
	g5.GreaterDown = &g6Start
	g5.GreaterLeft = &y6End
	g5.GreaterRight = &g6Start
	g5.LesserUp = &y7Start
	g5.LesserDown = &g4End
	g5.LesserLeft = &y7Start
	g5.LesserRight = &g4End

	g6.GreaterUp = &b5Start
	g6.GreaterDown = &b4End
	g6.GreaterLeft = &b5Start
	g6.GreaterRight = &b4End
	g6.LesserUp = &y6End
	g6.LesserDown = &g5End
	g6.LesserLeft = &g5End
	g6.LesserRight = &y6End

	g7.GreaterUp = &g3End
	g7.GreaterDown = &g8Start
	g7.GreaterLeft = &g3End
	g7.GreaterRight = &g8Start
	g7.LesserUp = &b4Start
	g7.LesserDown = &b3End
	g7.LesserLeft = &b4Start
	g7.LesserRight = &b3End

	g8.GreaterUp = &g9Start
	g8.GreaterDown = &g10Start
	g8.GreaterLeft = &g10Start
	g8.GreaterRight = &g9Start
	g8.LesserUp = &g7End
	g8.LesserDown = &g3End
	g8.LesserLeft = &g3End
	g8.LesserRight = &g7End

	g9.GreaterUp = &b2Start
	g9.GreaterDown = &b1End
	g9.GreaterLeft = &b2Start
	g9.GreaterRight = &b1End
	g9.LesserUp = &g8End
	g9.LesserDown = &g10Start
	g9.LesserLeft = &g10Start
	g9.LesserRight = &g8End

	g10.GreaterUp = &g1Start
	g10.GreaterDown = &b1Start
	g10.GreaterLeft = &g1Start
	g10.GreaterRight = &b1Start
	g10.LesserUp = &g8End
	g10.LesserDown = &g9Start
	g10.LesserLeft = &g8End
	g10.LesserRight = &g9Start

	dome := &Dome{
		Segments: segments,
	}

	return &Game{
		logger: logger,
		Dome:   dome,
	}
}
