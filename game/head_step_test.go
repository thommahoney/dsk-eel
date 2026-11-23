package game

import (
	"fmt"
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// Where the head of the eel goes on a tick depends on four things: the
// pixel it is on, the way it is travelling, where the stick is, and
// whether the eel steers itself. The head can be on any of 49 x 22 pixels
// travelling either way, 2,156 head states. The stick reports one of 16
// values. The eel steers itself in a demo and while the joystick is lost,
// which makes four combinations of the two. That is few enough to try
// every one, and the test in this file does: it puts an eel of one point
// in each state, plays one tick with the real Eel.Move, and compares where
// the head is afterwards with the rule written out in headStepRule.
//
// The rule shares no code with Move or with Segment.NextHop. It takes the
// links of the dome from the hop table as graph_test.go reads it, and the
// turns a stick position asks for from the table in steering_test.go. So
// a mistake in Move or in NextHop cannot hide in the rule.
//
// With every step seen, the test then follows the steps themselves: from
// each of the 49 places a new eel starts, the head can come to be in every
// one of the 2,156 states, and with the stick held up, down, left or right
// there is no state in which a tick ends the game.
//
// What this cannot show: that the rule is the rule the game should have,
// which is the owner's to say, or that the hop table matches the dome. And
// it tries an eel of one point, by itself and with a second point growing
// behind it. That the head of a long eel steps the same way rests on
// reading Move, which looks at nothing but the head to decide the step;
// the random play in tick_test.go is what compares long eels with a
// reference.

// headStepStates is how many head states there are.
const headStepStates = testSegments * testSegmentLength * 2

// headStepState is a head state as a number from 0 to 2,155: the segment,
// the position on it and the way of travelling. headStepDied stands for
// the game having ended, and headStepNotTried for a step the test did not
// play.
type headStepState int16

const (
	headStepDied     headStepState = -1
	headStepNotTried headStepState = -2
)

func headStepAt(segment, position int, travel Direction) headStepState {
	s := (segment*testSegmentLength + position) * 2
	if travel == Lesser {
		s++
	}
	return headStepState(s)
}

func (s headStepState) segment() int  { return int(s) / 2 / testSegmentLength }
func (s headStepState) position() int { return int(s) / 2 % testSegmentLength }

func (s headStepState) travel() Direction {
	if s%2 == 1 {
		return Lesser
	}
	return Greater
}

// headStepRule is one tick of the head as this test states it. It returns
// the states the head may be in after the tick, ways of them:
//
//   - one, for a head that is not on the last pixel of its segment in the
//     way it is travelling: the next pixel that way, whatever the stick
//     says;
//   - on that last pixel, the pixel the stick leads to. A stick held along
//     an axis leads through the link of its turn. A diagonal leads on if
//     the links of its two turns come onto the same end of the same
//     segment, and nowhere if they do not. The head comes on at the end
//     the link names, travelling away from it;
//   - none, where the stick leads nowhere: a diagonal that asks for two
//     ways at once, a centred stick, and the seven values that are no
//     stick position. The tick ends the game;
//   - but an eel that steers itself takes one of its four turns in place
//     of a centred stick, and which one is left to chance. The four states
//     they lead to are returned, and chosen is true. A stick that is not
//     centred steers such an eel like any other.
func headStepRule(d *dome, from headStepState, stick controller.Direction, steersItself bool) (next [turnCount]headStepState, ways int, chosen bool) {
	segment, position, travel := from.segment(), from.position(), from.travel()
	if travel == Greater && position < testSegmentLength-1 {
		next[0] = headStepAt(segment, position+1, Greater)
		return next, 1, false
	}
	if travel == Lesser && position > 0 {
		next[0] = headStepAt(segment, position-1, Lesser)
		return next, 1, false
	}

	leads := d.leads[endOf(segment, travel == Greater)]
	arriving := func(onto domeEnd) headStepState {
		if onto.high() {
			return headStepAt(onto.segment(), testSegmentLength-1, Lesser)
		}
		return headStepAt(onto.segment(), 0, Greater)
	}
	if stick == controller.Dir_Neutral && steersItself {
		for turn, onto := range leads {
			next[turn] = arriving(onto)
		}
		return next, turnCount, true
	}
	turns := stickTurns[stick]
	if len(turns) == 0 {
		return next, 0, false
	}
	onto := leads[turns[0]]
	for _, turn := range turns[1:] {
		if leads[turn] != onto {
			return next, 0, false
		}
	}
	next[0] = arriving(onto)
	return next, 1, false
}

// headStepDraws is how many times a tick is played when the eel chooses
// its own turn. A turn is one of four picked at random, and where two ways
// lead on, two of the four turns lead each way. 64 ticks that all go the
// same way are one chance in 2^63, so the test can ask to see both.
// headStepDrawsUnderRace is how many it is under the race detector, too
// few to ask that.
const (
	headStepDraws          = 64
	headStepDrawsUnderRace = 8
)

// headStepPlay plays single ticks of an eel of one point on a game of its
// own.
type headStepPlay struct {
	g   *Game
	eel *Eel
	// own is the eel's one point, and body the array its Body is a slice
	// of, with room for the point a growing eel adds.
	own  Point
	body [2]*Point
	// food is the food as it is put back before every tick.
	food []*Point
	// index is the number of each segment of the dome, and graph is every
	// Point that belongs to the hop table.
	index map[*Segment]int
	graph map[*Point]bool
	// ate counts the ticks on which the head landed on the food.
	ate int
}

func headStepNewPlay(t *testing.T) *headStepPlay {
	t.Helper()
	g := newLoopbackGame(t)
	// Nothing reads the frames in this test. With the socket closed the
	// seven sends of a tick fail at once, which makes the run much shorter.
	g.Chromatik.ArtNetConnection.Close()

	p := &headStepPlay{g: g, index: map[*Segment]int{}, graph: hopPoints(g)}
	for i, s := range g.Segments {
		p.index[s] = i
	}
	// The food lies on the first seven pixels of a segment, so that some
	// steps along a segment and some steps onto one land on it. A step
	// must be the same step whether or not the eel eats on it.
	p.food = line(g.Segments[testSegments/2], testFoodLength-1, 0)
	g.Food = &Food{Game: g}
	p.eel = &Eel{Game: g}
	g.Eel = p.eel
	return p
}

// describe says where a head state is, in words.
func (p *headStepPlay) describe(s headStepState) string {
	return fmt.Sprintf("%s position %d %s", p.g.Segments[s.segment()].Label, s.position(), towards(s.travel()))
}

// outcome says what a tick did, in words.
func (p *headStepPlay) outcome(after headStepState) string {
	if after == headStepDied {
		return "the tick ended the game"
	}
	return "the head went to " + p.describe(after)
}

// step puts the eel in the state from, as one point that is growing or
// not, plays one tick of the real game with the input in, and returns the
// state of the head after it, or headStepDied if the tick ended the game.
// problem is "" unless the eel is left in a way no tick may leave it:
//
//   - after a tick that ends the game, an eel that is not as it was, in
//     its body, its segment or the way it is travelling: the game-over
//     animation shrinks the eel where it lies;
//   - a head that is not on one of the 22 pixels of one of the 49 segments;
//   - an Eel.Segment that is not the segment the head is on;
//   - a body that is not the head alone, or, for a growing eel, the head
//     and behind it the point the eel was;
//   - a Point of the body that belongs to the hop table. The body's points
//     are written to on later ticks, and writing to one of the table's
//     would move a link of the dome.
func (p *headStepPlay) step(from headStepState, in inputSnapshot, growing bool) (after headStepState, problem string) {
	g, eel := p.g, p.eel
	segment := g.Segments[from.segment()]
	start := Point{Segment: segment, Position: from.position()}
	// A growth of two leaves one to go after the tick, so the tick does
	// not get to the end of growing, where Move places a new food.
	growth, length := 0, 1
	if growing {
		growth, length = 2, 2
	}

	p.own = start
	p.body[0], p.body[1] = &p.own, nil
	eel.Body = p.body[:1]
	eel.Segment, eel.TravelDir, eel.Growth = segment, from.travel(), growth
	g.Food.Body, g.Food.Fresh = p.food, true
	g.input.Swap(in)

	err := eel.Move()

	if !g.Food.Fresh {
		p.ate++
	}
	if err != nil {
		if len(eel.Body) != 1 || *eel.Body[0] != start || eel.Segment != segment || eel.TravelDir != from.travel() {
			return headStepDied, "the tick ended the game and did not leave the eel where it was, travelling the way it was"
		}
		return headStepDied, ""
	}

	head := eel.Head()
	number, onDome := p.index[head.Segment]
	if !onDome {
		return headStepNotTried, "the head is on a segment that is not one of the dome's 49"
	}
	if head.Position < 0 || head.Position >= testSegmentLength {
		return headStepNotTried, fmt.Sprintf("the head is at position %d of %s, which has positions 0 to 21", head.Position, head.Segment.Label)
	}
	after = headStepAt(number, head.Position, eel.TravelDir)
	switch {
	case eel.TravelDir != Greater && eel.TravelDir != Lesser:
		problem = fmt.Sprintf("the eel is travelling in direction %d, which is neither of the two there are", eel.TravelDir)
	case eel.Segment != head.Segment:
		problem = fmt.Sprintf("Eel.Segment is %s but the head is on %s", eel.Segment.Label, head.Segment.Label)
	case len(eel.Body) != length:
		problem = fmt.Sprintf("the eel is %d points long after the tick, want %d", len(eel.Body), length)
	case p.graph[head]:
		problem = "the head is a Point of the hop table and not one of the eel's own"
	case growing && (eel.Body[1] != &p.own || p.own != start):
		problem = "the point behind the head of the growing eel is not the point the eel was"
	}
	return after, problem
}

// TestHeadStepFromEveryStateWithEveryStick plays one tick from each of the
// 2,156 head states with each of the 16 stick values, in a demo and not,
// with the joystick lost and not, for an eel that is growing and one that
// is not: 275,968 steps and every one must be the step headStepRule gives.
// Where the eel chooses its own turn the tick is played headStepDraws
// times: it must never end the game, must always take one of the eel's
// four turns, and over the draws must go every way those turns lead.
//
// The count of the ticks that end the game is pinned. They are all on the
// 98 last pixels before a segment end, and there are, for an eel that is
// growing and again for one that is not,
//
//   - 972 where the player steers: at each of the 98 ends the centred
//     stick and the seven values that are no stick position, 784, and the
//     188 of the 392 diagonals that ask for two ways at once;
//   - 874 where the eel steers itself, in each of the three combinations
//     of demo and lost joystick: the same but for the centred stick, with
//     which it takes a turn of its own at all 98 ends.
//
// The steps of a game that is no demo, with the joystick there, are then
// followed from state to state; see the last two subtests. Those two have
// no steps of their own to follow: chosen by name with -run, without the
// first, they are skipped and say so.
//
// Under the race detector, which makes a tick many times slower and has
// nothing to find in one goroutine, 196 of the states are tried: the 98 on
// the last pixel before a segment end, the ones in which the stick decides
// anything, and the 98 on a pixel in the middle of a segment. An eel that
// chooses its own turn is given 8 ticks there and not 64. The run without
// it is the one that tries every state.
func TestHeadStepFromEveryStateWithEveryStick(t *testing.T) {
	d := wellFormedDome(t)
	p := headStepNewPlay(t)

	// tried says whether the steps from a state are played in this run.
	// wantStates is how many states that makes, and chosenDraws how many
	// ticks an eel that chooses its own turn is given.
	wantStates, chosenDraws := headStepStates, headStepDraws
	if raceDetector {
		wantStates, chosenDraws = 4*testSegments, headStepDrawsUnderRace
	}
	tried := func(from headStepState) bool {
		return !raceDetector || headStepOnLastPixel(from) || from.position() == testSegmentLength/2
	}

	// steps[from][stick] is the state the head was in after the tick from
	// the state from with that stick value, in a game that is no demo with
	// the joystick there and an eel that is not growing.
	steps := make([][stickValues]headStepState, headStepStates)
	for from := range steps {
		for stick := range steps[from] {
			steps[from][stick] = headStepNotTried
		}
	}
	// stepsPlayed is whether the first subtest ran and filled steps. The
	// other two follow what it played. Without it they would have nothing
	// to follow, and would report an eel that cannot get anywhere.
	stepsPlayed := false
	needsSteps := func(t *testing.T) {
		t.Helper()
		if !stepsPlayed {
			t.Skip("this follows the steps the first subtest plays, and that one did not run: run the whole of TestHeadStepFromEveryStateWithEveryStick")
		}
	}

	t.Run("every step is the one the rule gives", func(t *testing.T) {
		few := reportFew(t)
		states, played, chosenSteps := 0, 0, 0
		for _, demo := range []bool{false, true} {
			for _, lost := range []bool{false, true} {
				for _, growing := range []bool{false, true} {
					steersItself := demo || lost
					died := 0
					for s := range headStepStates {
						from := headStepState(s)
						if !tried(from) {
							continue
						}
						states++
						for v := range stickValues {
							stick := controller.Direction(v)
							in := inputOf(stick, demo, lost)
							want, ways, chosen := headStepRule(d, from, stick, steersItself)
							// A step that goes wrong is reported once, however
							// many times it is played.
							reported := false
							fail := func(format string, args ...any) {
								if reported {
									return
								}
								reported = true
								grows := ""
								if growing {
									grows = ", growing"
								}
								few.Errorf("from %s%s, with %s (%04b): %s",
									p.describe(from), grows, describeInput(in), v, fmt.Sprintf(format, args...))
							}

							draws := 1
							if chosen {
								draws = chosenDraws
								chosenSteps++
							}
							var went [turnCount]bool
							for range draws {
								got, problem := p.step(from, in, growing)
								played++
								if problem != "" {
									fail("%s", problem)
									continue
								}
								if got == headStepDied {
									died++
								}
								if !demo && !lost && !growing {
									steps[from][v] = got
								}
								switch {
								case ways == 0 && got != headStepDied:
									fail("%s, want the tick to end the game", p.outcome(got))
								case ways == 1 && got != want[0]:
									fail("%s, want it to go to %s", p.outcome(got), p.describe(want[0]))
								case chosen:
									one := false
									for turn, w := range want {
										if got == w {
											went[turn], one = true, true
										}
									}
									if !one {
										fail("%s, want the eel to take a turn of its own, to %s, %s, %s or %s",
											p.outcome(got), p.describe(want[turnUp]), p.describe(want[turnDown]), p.describe(want[turnLeft]), p.describe(want[turnRight]))
									}
								}
							}
							if chosen && draws == headStepDraws {
								for turn, w := range want {
									if !went[turn] {
										fail("in %d ticks the eel never took a turn of its own to %s, where its %s turn leads",
											draws, p.describe(w), turnNames[turn])
									}
								}
							}
						}
					}

					wantDied, who := 972, "the player steers"
					if steersItself {
						wantDied, who = 874, "the eel steers itself"
					}
					if died != wantDied {
						t.Errorf("demo %v, joystick lost %v, growing %v: %d ticks ended the game, want the %d there are where %s",
							demo, lost, growing, died, wantDied, who)
					}
				}
			}
		}
		stepsPlayed = true

		// Eight kinds of game and eel: a demo or not, the joystick lost
		// or not, the eel growing or not. In six of them the eel steers
		// itself, and chooses its turn on the 98 last pixels.
		if headStepStates != 2156 || states != 8*wantStates {
			t.Errorf("%d of the %d head states were tried for each of the 8 kinds of game and eel, want %d of 49 x 22 x 2 = 2156",
				states/8, headStepStates, wantStates)
		}
		if once, want := played-chosenSteps*chosenDraws, 8*wantStates*stickValues-6*98; once != want {
			t.Errorf("%d ticks were played once, want %d: 8 x %d states x 16 stick values, less the 6 x 98 on which the eel chooses", once, want, wantStates)
		}
		if raceDetector {
			t.Logf("the race detector is on: only %d of the %d head states were tried, those on the last pixel before a segment end and on a pixel in the middle, with %d ticks where the eel chooses and not %d; every state needs a run without -race",
				wantStates, headStepStates, chosenDraws, headStepDraws)
		}
		t.Logf("%d ticks, %d of them on the food", played, p.ate)
		if chosenSteps != 6*98 {
			t.Errorf("the eel chose its own turn in %d states, want 98 in each of the 6 kinds of game and eel in which it steers itself", chosenSteps)
		}
		if p.ate == 0 {
			t.Errorf("the head never landed on the food in %d ticks", played)
		}
		// The rule reads a hop table built for it, so the game's own must
		// still be what a newly built one is after all these ticks.
		checkGraphIntact(t, p.g)
	})

	t.Run("every state is reached from every start", func(t *testing.T) {
		needsSteps(t)
		if raceDetector {
			t.Skip("not every state is tried under the race detector; the run without it follows the steps")
		}
		// A new eel is on any of the 49 segments with its head at position
		// 6, travelling towards the high end (eel_start_test.go). Each of
		// the steps seen above leads from one state to the next, so the
		// states a start leads to, by any stick at all, are found by
		// following them.
		few := reportFew(t)
		furthest := 0
		for segment := range testSegments {
			start := headStepAt(segment, 6, Greater)
			ticks := make([]int, headStepStates)
			for s := range ticks {
				ticks[s] = -1
			}
			ticks[start] = 0
			queue := []headStepState{start}
			for i := 0; i < len(queue); i++ {
				for _, next := range steps[queue[i]] {
					if next >= 0 && ticks[next] < 0 {
						ticks[next] = ticks[queue[i]] + 1
						queue = append(queue, next)
					}
				}
			}
			for s, n := range ticks {
				if n < 0 {
					few.Errorf("an eel that starts at %s can never have its head at %s", p.describe(start), p.describe(headStepState(s)))
				}
				furthest = max(furthest, n)
			}
		}
		t.Logf("every one of the %d head states is reached from every one of the %d starts, the furthest in %d ticks",
			headStepStates, testSegments, furthest)
	})

	t.Run("a stick held along an axis never ends the game", func(t *testing.T) {
		// "With an arrow held it never dies there", the README says of
		// the junctions under --keyboard. No state has a tick that ends
		// the game with the stick up, down, left or right, and every such
		// tick leads to a state again, so however long the stick is held
		// no junction ends the game. The eel can still run into itself.
		needsSteps(t)
		few := reportFew(t)
		held := 0
		for s := range steps {
			from := headStepState(s)
			if !tried(from) {
				continue
			}
			for _, stick := range cardinalTurns {
				held++
				switch steps[from][stick] {
				case headStepDied:
					few.Errorf("from %s with the stick held %v the tick ends the game", p.describe(from), stick)
				case headStepNotTried:
					few.Errorf("from %s with the stick held %v no tick was played", p.describe(from), stick)
				}
			}
		}
		if held != 4*wantStates {
			t.Errorf("%d steps with the stick held along an axis were looked at, want 4 from each of the %d states tried", held, wantStates)
		}
	})
}

// headStepOnLastPixel says whether a head in the state from is on the last
// pixel of its segment in the way it is travelling, where the next tick
// takes it onto another segment or ends the game.
func headStepOnLastPixel(from headStepState) bool {
	if from.travel() == Greater {
		return from.position() == testSegmentLength-1
	}
	return from.position() == 0
}
