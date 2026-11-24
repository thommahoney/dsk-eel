package game

import (
	"testing"
)

// TestDemoModeTurnsAtRandom pins how an eel that steers itself chooses at a
// junction: by chance, each time anew. An eel that always turned the same
// way, or took the turns in a fixed order, would go round the same few
// segments of the dome for as long as the demo ran.
//
// The game's source of chance cannot be given to it by a test, so the test
// takes the eel through the same junction 4096 times and counts.
//
// What can be seen of a choice is the segment the eel comes onto, and a
// junction has only two ways out. The four turns share them in pairs, and
// there are two pairings on the dome, so the test stands at a junction of
// each: leaving p1 by its high end, north and east lead to p2 and south and
// west to r9; leaving p2 by its low end, north and west lead to r9 and
// south and east to p1. (Four segment ends have one way out, and no
// junction tells the turns apart any further.) At each of the two:
//
//   - The way north leads is taken about half of the time. An eel that
//     picks one of north, south, east and west with equal chances does
//     that. One that leaves out a turn does not: without west, say, it
//     goes north's way from p1 two times in three. What this cannot tell
//     from the real eel is one that is as likely to pick south as north,
//     and west as east, but does not pick north as often as east: on this
//     dome such an eel goes each way just as often as the real one.
//   - About half of the time the way is the one taken the time before.
//   - Of the 16 runs of four ways in a row that there are, none comes up
//     much more often than the one time in 16 that chance gives it. The
//     tries are taken four at a time for this, 1024 runs. An eel that
//     takes the turns in an order, whichever, goes the same few runs over
//     and over, although it may well go each way half of the time.
//
// The limits are 256 either side of half the tries for the first two
// counts, which is eight standard deviations, and 128 over the 64 that is
// a sixteenth of the runs for the third. Of n fair tries, the number that
// come out one way is d or more from what is expected with a probability
// of at most 2*exp(-2*d*d/n), or half of that on one side (Hoeffding's
// bound): here exp(-32) or twice that, 1.3 or 2.5 in 10^14, for each of
// the 36 counts of the two junctions. Added up, the test fails by bad luck
// less than once in 10^12 runs. The same limits leave no room for an eel
// with a turn missing: two thirds of 4096 is 427 more than is allowed.
func TestDemoModeTurnsAtRandom(t *testing.T) {
	const (
		tries      = 4096
		margin     = 256
		runs       = tries / 4
		runsMargin = 128
	)
	junctions := []struct {
		name string
		// put places the eel with its head on the last pixel before the
		// junction.
		put func(g *Game)
		// The pixels the head lands on when it goes north's way and
		// south's way.
		north, south int
	}{
		{
			"leaving p1 by its high end",
			func(g *Game) { place(g, 0, 21, 7, 1.0, 20, 12, 6) },
			22,  // p2 at its low end
			417, // r9 at its high end
		},
		{
			"leaving p2 by its low end",
			func(g *Game) { pinPlaceLesser(g, 1, 0, 7, 1.0, 20, 12, 6) },
			417, // r9 at its high end
			21,  // p1 at its high end
		},
	}
	for _, j := range junctions {
		t.Run(j.name, func(t *testing.T) {
			t.Parallel()
			g := newLoopbackGame(t)
			// Nothing reads the frames. With the socket closed the seven
			// sends of a tick fail at once.
			g.Chromatik.ArtNetConnection.Close()
			setDemo(g, true)

			northWay, sameAsBefore := 0, 0
			wentNorth := false
			// The ways of the last tries, a bit for each, 1 for north's
			// way, and how often each run of four came up.
			lastWays := 0
			var runsOfFour [16]int
			for try := range tries {
				j.put(g)
				if err := g.Eel.Move(); err != nil {
					t.Fatalf("try %d: %v", try, err)
				}
				head := g.Eel.Head()
				px := head.Segment.Offset + head.Position
				if px != j.north && px != j.south {
					t.Fatalf("try %d: the head went to pixel %d (%s position %d), want pixel %d or %d",
						try, px, head.Segment.Label, head.Position, j.north, j.south)
				}
				north := px == j.north
				lastWays <<= 1
				if north {
					northWay++
					lastWays |= 1
				}
				if try > 0 && north == wentNorth {
					sameAsBefore++
				}
				wentNorth = north
				if try%4 == 3 {
					runsOfFour[lastWays&0b1111]++
				}
			}

			if northWay < tries/2-margin || northWay > tries/2+margin {
				t.Errorf("the eel went the way north leads %d times in %d, want %d give or take %d: it does not pick each of the four turns as often as the others",
					northWay, tries, tries/2, margin)
			}
			// There are tries-1 pairs of one try and the next.
			if sameAsBefore < tries/2-margin || sameAsBefore > tries/2+margin {
				t.Errorf("the eel went the same way as the time before %d times in %d, want %d give or take %d: its choice depends on the one before",
					sameAsBefore, tries-1, tries/2, margin)
			}
			for run, n := range runsOfFour {
				if n > runs/16+runsMargin {
					t.Errorf("of %d runs of four tries, %d went the ways %04b (1 is the way north leads), want about %d and no more than %d: the choices follow a pattern",
						runs, n, run, runs/16, runs/16+runsMargin)
				}
			}
		})
	}
}
