package game

import (
	"fmt"
	"slices"
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// Segment.NextHop is the whole steering rule: which hop an eel takes at
// the end of its segment, for the way it is travelling and where the stick
// is. The joystick reports the stick in four bits, so there are 16 values
// it can have: nine have a name, centred and the eight directions, and
// seven are values no stick position gives. The tests in this file call
// NextHop with all 16 for both ways of travelling: on a segment made up so
// that a mix-up cannot hide, on every way four turns can share hops, and
// on every segment of the dome.
//
// The tests state the rule a second time, as the table below, and show
// that NextHop follows it in every case. They cannot show that it is the
// rule the game should have, or that pushing the stick north takes the eel
// the way a player at the dome would call up.

// stickTurns is the rule as these tests state it: the turns each stick
// position asks for. A stick straight along an axis asks for one turn, a
// diagonal for two, and a centred stick for none.
var stickTurns = map[controller.Direction][]int{
	controller.Dir_Neutral:   nil,
	controller.Dir_North:     {turnUp},
	controller.Dir_South:     {turnDown},
	controller.Dir_East:      {turnRight},
	controller.Dir_West:      {turnLeft},
	controller.Dir_NorthEast: {turnUp, turnRight},
	controller.Dir_SouthEast: {turnDown, turnRight},
	controller.Dir_SouthWest: {turnDown, turnLeft},
	controller.Dir_NorthWest: {turnUp, turnLeft},
}

// stickValues is how many values the four bits of the stick can have.
const stickValues = 16

// wantNextHop is where the rule says the stick leads: to the hop of the
// turn it asks for, if it asks for one, and to the hop its two turns share
// if it asks for two and they lead the same way. A diagonal whose two
// turns lead different ways leads nowhere, and so do a centred stick and a
// value that is no stick position.
func wantNextHop(s *Segment, travel Direction, stick controller.Direction) *Hop {
	turns := stickTurns[stick]
	if len(turns) == 0 {
		return nil
	}
	hop := turnHop(s, travel, turns[0])
	for _, turn := range turns[1:] {
		if turnHop(s, travel, turn) != hop {
			return nil
		}
	}
	return hop
}

// slotHolding names the field of s that holds the hop, for a message.
func slotHolding(s *Segment, hop *Hop) string {
	if hop == nil {
		return "no hop"
	}
	for _, slot := range hopSlots {
		if slot.hop(s) == hop {
			return "the " + slot.name + " hop"
		}
	}
	return "a hop that is not one of the segment's"
}

// madeUpSegment returns a segment that is not part of the dome, with the
// given hops for its four turns, up, down, left and right, travelling
// towards the high end and towards the low end. The hops can be empty:
// NextHop hands one back and never looks inside it.
func madeUpSegment(greater, lesser [turnCount]*Hop) *Segment {
	return &Segment{
		Label:        "made up",
		GreaterUp:    greater[turnUp],
		GreaterDown:  greater[turnDown],
		GreaterLeft:  greater[turnLeft],
		GreaterRight: greater[turnRight],
		LesserUp:     lesser[turnUp],
		LesserDown:   lesser[turnDown],
		LesserLeft:   lesser[turnLeft],
		LesserRight:  lesser[turnRight],
	}
}

// TestStickHasNineNamedValues: of the 16 values the stick can report,
// nine are positions of the stick and seven are not. The tests below rely
// on having looked at all 16.
func TestStickHasNineNamedValues(t *testing.T) {
	named := 0
	for v := range stickValues {
		if _, ok := stickTurns[controller.Direction(v)]; ok {
			named++
		}
	}
	if named != 9 || len(stickTurns) != 9 {
		t.Errorf("%d of the 16 stick values are named, and the test's rule names %d, want 9 and 9", named, len(stickTurns))
	}
}

// TestNextHopCardinalsTakeTheirOwnHop gives a segment eight hops that are
// all different, which no segment of the dome has: there left and right
// lead where up or down does, so an exchange of two turns can go unseen.
// Here every turn has a hop of its own, and the test writes out which one
// each stick position must take. Everything else leads nowhere: the
// centred stick, the seven values that are no stick position, and all four
// diagonals, since no two turns share a hop.
func TestNextHopCardinalsTakeTheirOwnHop(t *testing.T) {
	s := madeUpSegment([turnCount]*Hop{{}, {}, {}, {}}, [turnCount]*Hop{{}, {}, {}, {}})
	want := map[Direction]map[controller.Direction]*Hop{
		Greater: {
			controller.Dir_North: s.GreaterUp,
			controller.Dir_South: s.GreaterDown,
			controller.Dir_East:  s.GreaterRight,
			controller.Dir_West:  s.GreaterLeft,
		},
		Lesser: {
			controller.Dir_North: s.LesserUp,
			controller.Dir_South: s.LesserDown,
			controller.Dir_East:  s.LesserRight,
			controller.Dir_West:  s.LesserLeft,
		},
	}
	few := reportFew(t)
	for _, travel := range []Direction{Greater, Lesser} {
		for v := range stickValues {
			stick := controller.Direction(v)
			got, wantHop := s.NextHop(travel, stick), want[travel][stick]
			if got != wantHop {
				few.Errorf("%s with the stick at %v (%04b): NextHop returns %s, want %s",
					towards(travel), stick, v, slotHolding(s, got), slotHolding(s, wantHop))
			}
		}
	}
}

// TestNextHopDiagonalNeedsBothTurnsToAgree goes through every way the four
// turns of a junction can share hops: each turn takes one of four hops, 256
// junctions in all, from four turns that lead four ways to four that lead
// one way. For each, and for all 16 stick values, NextHop must do what the
// rule says: a diagonal leads on exactly when its two turns hold the same
// hop, and then to that hop. The junction is tried for both ways of
// travelling, with four other hops in the turns of the way not travelled,
// which must never come back.
func TestNextHopDiagonalNeedsBothTurnsToAgree(t *testing.T) {
	ways := [turnCount]*Hop{{}, {}, {}, {}}
	notTravelled := [turnCount]*Hop{{}, {}, {}, {}}
	few := reportFew(t)
	which := func(hop *Hop) string {
		if hop == nil {
			return "no hop"
		}
		if i := slices.Index(ways[:], hop); i >= 0 {
			return fmt.Sprintf("hop %d", i)
		}
		return "a hop of the way not travelled"
	}
	for _, travel := range []Direction{Greater, Lesser} {
		led := 0
		for n := range 256 {
			// The hop each turn takes: up, down, left, right.
			takes := [turnCount]int{n % 4, n / 4 % 4, n / 16 % 4, n / 64 % 4}
			turns := [turnCount]*Hop{ways[takes[turnUp]], ways[takes[turnDown]], ways[takes[turnLeft]], ways[takes[turnRight]]}
			s := madeUpSegment(turns, notTravelled)
			if travel == Lesser {
				s = madeUpSegment(notTravelled, turns)
			}
			for v := range stickValues {
				stick := controller.Direction(v)
				got, want := s.NextHop(travel, stick), wantNextHop(s, travel, stick)
				if got != want {
					few.Errorf("%s with the stick at %v (%04b), where up, down, left and right take hops %d of four: NextHop returns %s, want %s",
						towards(travel), stick, v, takes, which(got), which(want))
				}
				if len(stickTurns[stick]) == 2 && got != nil {
					led++
				}
			}
		}
		// The two turns of a diagonal take the same one of the four hops
		// in a quarter of the junctions, 64, and there are four diagonals.
		if led != 256 {
			t.Errorf("%s: a diagonal led on %d times over the 256 junctions, want 4 x 64 = 256", towards(travel), led)
		}
	}
}

// TestNextHopOnEveryJunctionOfTheDome calls NextHop for every segment of
// the dome, both ways of travelling and all 16 stick values, 1568 calls,
// and compares each with the rule. It then pins what that comes to on the
// dome as it is wired:
//
//   - a stick held along an axis always leads on, at all 392 turns. Demo
//     mode steers by such a turn picked at random, so it never leaves the
//     eel without a way on;
//   - a centred stick and the seven values that are no stick position
//     never do;
//   - a diagonal leads on at 204 of the 392 places it can be held. At the
//     four ends of the two-way joints every diagonal leads the one way
//     there is. At each of the other 94 ends two diagonals lead on and two
//     end the game: left and right there lead the same two ways as up and
//     down, so a diagonal whose two turns agree leads on, the opposite
//     diagonal does too, and the other two ask for both ways at once.
//
// This pins what the game does today. Whether a diagonal that asks for
// both ways should end the game is the owner's to say.
func TestNextHopOnEveryJunctionOfTheDome(t *testing.T) {
	opposite := map[controller.Direction]controller.Direction{
		controller.Dir_NorthEast: controller.Dir_SouthWest,
		controller.Dir_SouthWest: controller.Dir_NorthEast,
		controller.Dir_NorthWest: controller.Dir_SouthEast,
		controller.Dir_SouthEast: controller.Dir_NorthWest,
	}

	calls, alongAxis, diagonals, noTurn := 0, 0, 0, 0
	allFour, oppositePairs := 0, 0
	few := reportFew(t)
	for _, s := range InitSegments() {
		for _, travel := range []Direction{Greater, Lesser} {
			var leading []controller.Direction
			for v := range stickValues {
				stick := controller.Direction(v)
				got, want := s.NextHop(travel, stick), wantNextHop(s, travel, stick)
				calls++
				if got != want {
					few.Errorf("%s %s with the stick at %v (%04b): NextHop leads to %s, want %s",
						s.Label, towards(travel), stick, v, hopName(got), hopName(want))
				}
				if got == nil {
					continue
				}
				switch len(stickTurns[stick]) {
				case 1:
					alongAxis++
				case 2:
					diagonals++
					leading = append(leading, stick)
				default:
					noTurn++
				}
			}
			switch {
			case len(leading) == 4:
				allFour++
			case len(leading) == 2 && opposite[leading[0]] == leading[1]:
				oppositePairs++
			default:
				few.Errorf("%s %s: the diagonals that lead on are %v, want all four or two opposite ones", s.Label, towards(travel), leading)
			}
		}
	}
	if calls != 1568 {
		t.Errorf("NextHop was called %d times, want 49 x 2 x 16 = 1568", calls)
	}
	if alongAxis != 392 {
		t.Errorf("a stick held along an axis leads on at %d of the 392 turns, want all of them", alongAxis)
	}
	if noTurn != 0 {
		t.Errorf("a centred stick or a value that is no stick position led on %d times, want never", noTurn)
	}
	if diagonals != 204 {
		t.Errorf("a diagonal leads on at %d of the 392 places it can be held, want 204", diagonals)
	}
	if allFour != 4 || oppositePairs != 94 {
		t.Errorf("all four diagonals lead on at %d segment ends and two opposite ones at %d, want 4 and 94", allFour, oppositePairs)
	}
}
