package game

import "testing"

// TestNewFoodPositionIsUniformOnAnEmptyDome counts where new foods start
// when no eel is in the way. A food is 7 points long and lies below its
// first point, so the first point can be at positions 6..21 of a 22-pixel
// segment, and each of those 16 positions should be as likely as any other.
//
// With 160000 samples each position is expected 10000 times, with a
// standard deviation of 96.8. The test accepts 9000..11000, more than 10
// standard deviations either way. The segment is checked the same way
// below. By the Chernoff bound a correct NewFood fails all of this by
// chance less than once in 1e17 runs. The clamping this replaced put 32%
// of the foods on position 6 (about 50900 of 160000) and 4.5% on each of
// the others (about 7300).
func TestNewFoodPositionIsUniformOnAnEmptyDome(t *testing.T) {
	g := &Game{Segments: InitSegments()}

	const samples = 160000
	var starts [testSegmentLength]int
	segments := map[*Segment]int{}
	for range samples {
		body := NewFood(g).Body
		if len(body) != testFoodLength {
			t.Fatalf("food has %d points, want %d", len(body), testFoodLength)
		}
		segment := body[0].Segment
		for j, p := range body {
			if p.Segment != segment || p.Position != body[0].Position-j {
				t.Fatalf("point %d is on %s at %d, want %s at %d",
					j, p.Segment.Label, p.Position, segment.Label, body[0].Position-j)
			}
		}
		if last := body[len(body)-1].Position; last < 0 || body[0].Position >= testSegmentLength {
			t.Fatalf("food covers positions %d..%d, outside the segment", last, body[0].Position)
		}
		starts[body[0].Position]++
		segments[segment]++
	}

	for pos, n := range starts {
		switch {
		case pos < testFoodLength-1:
			if n != 0 {
				t.Errorf("%d foods start at position %d, where a food does not fit", n, pos)
			}
		case n < 9000 || n > 11000:
			t.Errorf("%d of %d foods start at position %d, want about %d", n, samples, pos, samples/16)
		}
	}

	// The segment should be uniform too: 3265 expected on each of the 49,
	// standard deviation 56.6, and 2700..3830 is 10 of those either way.
	if len(segments) != testSegments {
		t.Errorf("food landed on %d segments, want all %d", len(segments), testSegments)
	}
	for segment, n := range segments {
		if n < 2700 || n > 3830 {
			t.Errorf("%d of %d foods are on segment %s, want about %d", n, samples, segment.Label, samples/testSegments)
		}
	}
}

// TestNewFoodPositionIsUniform checks that the spread stays even when an
// eel is in the way. The eel fills segment 0 with its head at the high
// end, which joins segments 1 and 18. New food keeps off the eel and off
// the segments one junction ahead of its head, so it has the other 46
// segments, and on each of them all 16 positions.
//
// The numbers are those of the empty-dome test above, except that each of
// 46 segments is expected 3478 times (standard deviation 58.3) and
// 2890..4070 is accepted.
func TestNewFoodPositionIsUniform(t *testing.T) {
	g := &Game{Segments: InitSegments()}
	g.Eel = &Eel{Body: line(g.Segments[0], testSegmentLength-1, 0), Game: g}
	covered := coveredPixels(g.Eel.Body)

	const samples = 160000
	var starts [testSegmentLength]int
	segments := map[*Segment]int{}
	for i := range samples {
		food := NewFood(g)
		if touches(food.Body, covered) {
			t.Fatalf("call %d: food touches the eel", i)
		}
		starts[food.Body[0].Position]++
		segments[food.Body[0].Segment]++
	}

	for pos, n := range starts {
		switch {
		case pos < testFoodLength-1:
			if n != 0 {
				t.Errorf("%d foods start at position %d, where a food does not fit", n, pos)
			}
		case n < 9000 || n > 11000:
			t.Errorf("%d of %d foods start at position %d, want about %d", n, samples, pos, samples/16)
		}
	}
	if len(segments) != testSegments-3 {
		t.Errorf("food landed on %d segments, want the %d far from the eel's head", len(segments), testSegments-3)
	}
	for segment, n := range segments {
		if n < 2890 || n > 4070 {
			t.Errorf("%d of %d foods are on segment %s, want about %d", n, samples, segment.Label, samples/(testSegments-3))
		}
	}
}
