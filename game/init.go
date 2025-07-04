package game

const (
	SegmentLength = 22
)

// Ridiculously long function for initializing the Game
func (g *Game) Init() {
	segments := [49]Segment{}

	// blue - loop 3
	b1 := NewSegment("b1")
	segments[0] = b1
	b1Start := NewStartHop(&b1)
	b1End := NewEndHop(&b1)
	b2 := NewSegment("b2")
	segments[1] = b2
	b2Start := NewStartHop(&b2)
	b2End := NewEndHop(&b2)
	b3 := NewSegment("b3")
	segments[2] = b3
	b3Start := NewStartHop(&b3)
	b3End := NewEndHop(&b3)
	b4 := NewSegment("b4")
	segments[3] = b4
	b4Start := NewStartHop(&b4)
	b4End := NewEndHop(&b4)
	b5 := NewSegment("b5")
	segments[4] = b5
	b5Start := NewStartHop(&b5)
	b5End := NewEndHop(&b5)
	b6 := NewSegment("b6")
	segments[5] = b6
	b6Start := NewStartHop(&b6)
	b6End := NewEndHop(&b6)
	b7 := NewSegment("b7")
	segments[6] = b7
	b7Start := NewStartHop(&b7)
	b7End := NewEndHop(&b7)
	b8 := NewSegment("b8")
	segments[7] = b8
	b8Start := NewStartHop(&b8)
	b8End := NewEndHop(&b8)
	b9 := NewSegment("b9")
	segments[8] = b9
	b9Start := NewStartHop(&b9)
	b9End := NewEndHop(&b9)

	// red - loop 4
	r1 := NewSegment("r1")
	segments[9] = r1
	r1Start := NewStartHop(&r1)
	r1End := NewEndHop(&r1)
	r2 := NewSegment("r2")
	segments[10] = r2
	r2Start := NewStartHop(&r2)
	r2End := NewEndHop(&r2)
	r3 := NewSegment("r3")
	segments[11] = r3
	r3Start := NewStartHop(&r3)
	r3End := NewEndHop(&r3)
	r4 := NewSegment("r4")
	segments[12] = r4
	r4Start := NewStartHop(&r4)
	r4End := NewEndHop(&r4)
	r5 := NewSegment("r5")
	segments[13] = r5
	r5Start := NewStartHop(&r5)
	r5End := NewEndHop(&r5)
	r6 := NewSegment("r6")
	segments[14] = r6
	r6Start := NewStartHop(&r6)
	r6End := NewEndHop(&r6)
	r7 := NewSegment("r7")
	segments[15] = r7
	r7Start := NewStartHop(&r7)
	r7End := NewEndHop(&r7)
	r8 := NewSegment("r8")
	segments[16] = r8
	r8Start := NewStartHop(&r8)
	r8End := NewEndHop(&r8)
	r9 := NewSegment("r9")
	segments[17] = r9
	r9Start := NewStartHop(&r9)
	r9End := NewEndHop(&r9)
	r10 := NewSegment("r10")
	segments[18] = r10
	r10Start := NewStartHop(&r10)
	r10End := NewEndHop(&r10)

	// purple - loop 5
	p1 := NewSegment("p1")
	segments[19] = p1
	p1Start := NewStartHop(&p1)
	p1End := NewEndHop(&p1)
	p2 := NewSegment("p2")
	segments[20] = p2
	p2Start := NewStartHop(&p2)
	p2End := NewEndHop(&p2)
	p3 := NewSegment("p3")
	segments[21] = p3
	p3Start := NewStartHop(&p3)
	p3End := NewEndHop(&p3)
	p4 := NewSegment("p4")
	segments[22] = p4
	p4Start := NewStartHop(&p4)
	p4End := NewEndHop(&p4)
	p5 := NewSegment("p5")
	segments[23] = p5
	p5Start := NewStartHop(&p5)
	p5End := NewEndHop(&p5)
	p6 := NewSegment("p6")
	segments[24] = p6
	p6Start := NewStartHop(&p6)
	p6End := NewEndHop(&p6)
	p7 := NewSegment("p7")
	segments[25] = p7
	p7Start := NewStartHop(&p7)
	p7End := NewEndHop(&p7)
	p8 := NewSegment("p8")
	segments[26] = p8
	p8Start := NewStartHop(&p8)
	p8End := NewEndHop(&p8)
	p9 := NewSegment("p9")
	segments[27] = p9
	p9Start := NewStartHop(&p9)
	p9End := NewEndHop(&p9)
	p10 := NewSegment("p10")
	segments[28] = p10
	p10Start := NewStartHop(&p10)
	p10End := NewEndHop(&p10)

	// yellow - loop 1
	y1 := NewSegment("y1")
	segments[29] = y1
	y1Start := NewStartHop(&y1)
	y1End := NewEndHop(&y1)
	y2 := NewSegment("y2")
	segments[30] = y2
	y2Start := NewStartHop(&y2)
	y2End := NewEndHop(&y2)
	y3 := NewSegment("y3")
	segments[31] = y3
	y3Start := NewStartHop(&y3)
	y3End := NewEndHop(&y3)
	y4 := NewSegment("y4")
	segments[32] = y4
	y4Start := NewStartHop(&y4)
	y4End := NewEndHop(&y4)
	y5 := NewSegment("y5")
	segments[33] = y5
	y5Start := NewStartHop(&y5)
	y5End := NewEndHop(&y5)
	y6 := NewSegment("y6")
	segments[34] = y6
	y6Start := NewStartHop(&y6)
	y6End := NewEndHop(&y6)
	y7 := NewSegment("y7")
	segments[35] = y7
	y7Start := NewStartHop(&y7)
	y7End := NewEndHop(&y7)
	y8 := NewSegment("y8")
	segments[36] = y8
	y8Start := NewStartHop(&y8)
	y8End := NewEndHop(&y8)
	y9 := NewSegment("y9")
	segments[37] = y9
	y9Start := NewStartHop(&y9)
	y9End := NewEndHop(&y9)
	y10 := NewSegment("y10")
	segments[38] = y10
	y10Start := NewStartHop(&y10)
	y10End := NewEndHop(&y10)

	// green - loop 2
	g1 := NewSegment("g1")
	segments[39] = g1
	g1Start := NewStartHop(&g1)
	g1End := NewEndHop(&g1)
	g2 := NewSegment("g2")
	segments[40] = g2
	g2Start := NewStartHop(&g2)
	g2End := NewEndHop(&g2)
	g3 := NewSegment("g3")
	segments[41] = g3
	g3Start := NewStartHop(&g3)
	g3End := NewEndHop(&g3)
	g4 := NewSegment("g4")
	segments[42] = g4
	g4Start := NewStartHop(&g4)
	g4End := NewEndHop(&g4)
	g5 := NewSegment("g5")
	segments[43] = g5
	g5Start := NewStartHop(&g5)
	g5End := NewEndHop(&g5)
	g6 := NewSegment("g6")
	segments[44] = g6
	g6Start := NewStartHop(&g6)
	g6End := NewEndHop(&g6)
	g7 := NewSegment("g7")
	segments[45] = g7
	g7Start := NewStartHop(&g7)
	g7End := NewEndHop(&g7)
	g8 := NewSegment("g8")
	segments[46] = g8
	g8Start := NewStartHop(&g8)
	g8End := NewEndHop(&g8)
	g9 := NewSegment("g9")
	segments[47] = g9
	g9Start := NewStartHop(&g9)
	g9End := NewEndHop(&g9)
	g10 := NewSegment("g10")
	segments[48] = g10
	g10Start := NewStartHop(&g10)
	g10End := NewEndHop(&g10)

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

	g.Segments = segments
}

func NewSegment(label string) Segment {
	return Segment{Label: label, Length: SegmentLength}
}

func NewStartHop(segment *Segment) Hop {
	return Hop{Point: &Point{Segment: segment, Position: 0}, Direction: Greater}
}

func NewEndHop(segment *Segment) Hop {
	return Hop{Point: &Point{Segment: segment, Position: SegmentLength - 1}, Direction: Lesser}
}
