package game

import "testing"

// A new eel must never be about to die. Its head starts at position 6 of a
// 22-pixel segment, travelling towards position 21, with its other six
// points below it. That is the longest run-up the segment allows.
func TestNewEelStartsAtTheLowEndOfItsSegment(t *testing.T) {
	g := &Game{Segments: InitSegments()}

	seen := map[*Segment]bool{}
	for i := 0; i < 5000; i++ {
		eel := NewEel(g)
		seen[eel.Segment] = true

		if eel.TravelDir != Greater {
			t.Fatalf("eel %d travels in direction %v, want Greater", i, eel.TravelDir)
		}
		if len(eel.Body) != 7 {
			t.Fatalf("eel %d has %d points, want 7", i, len(eel.Body))
		}
		for j, p := range eel.Body {
			if p.Segment != eel.Segment || p.Position != 6-j {
				t.Fatalf("eel %d: point %d is on %s at position %d, want %s at position %d",
					i, j, p.Segment.Label, p.Position, eel.Segment.Label, 6-j)
			}
		}
	}
	// The segment is still random: 5000 eels miss one of the 49 segments
	// by chance less than once in 1e40 runs.
	if len(seen) != len(g.Segments) {
		t.Errorf("new eels appeared on %d segments, want all %d", len(seen), len(g.Segments))
	}
}

// With nobody touching the stick, a new eel moves 15 times and only the
// 16th tick, the one that reaches the first junction, ends the game. So no
// game ends before the player has had 16 ticks (1.12 s) to choose a turn.
// Every segment is tried, several times each so the food falls in
// different places, sometimes in the eel's path.
func TestNewGameNeverEndsBeforeTheFirstJunction(t *testing.T) {
	g, _ := newTestGame(t)

	for _, segment := range g.Segments {
		for game := 0; game < 6; game++ {
			// NewEel picks its own segment; move the eel to this one.
			g.Eel = NewEel(g)
			g.Eel.Segment = segment
			for _, p := range g.Eel.Body {
				p.Segment = segment
			}
			g.Food = NewFood(g)
			g.Brightness = 1.0

			for move := 1; move <= 15; move++ {
				if err := g.Eel.Move(); err != nil {
					t.Fatalf("segment %s: move %d ended the game: %v", segment.Label, move, err)
				}
			}
			if head := g.Eel.Head(); head.Segment != segment || head.Position != SegmentLength-1 {
				t.Fatalf("segment %s: after 15 moves the head is on %s at position %d, want position %d",
					segment.Label, head.Segment.Label, head.Position, SegmentLength-1)
			}
			if err := g.Eel.Move(); err == nil {
				t.Fatalf("segment %s: the eel went through a junction with a neutral stick", segment.Label)
			}
		}
	}
}
