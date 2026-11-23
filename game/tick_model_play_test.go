package game

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/controller"
)

// The tests in this file drive the tick model of tick_model_test.go: every
// real tick they play is compared with the model's. They differ in how
// they choose the ticks.
//
//   - TestTickMatchesModelForEverySteeringChoice goes through every input
//     at every junction, and so every route, to a fixed number of
//     junctions from every segment an eel can start on. Between the
//     junctions it plays each tick by one input only, picked so that every
//     input comes up at every pixel on some route or other.
//   - TestTickMatchesModelInLongRandomGames plays games far longer than
//     that search can reach, with long eels and many meals. It is a
//     sample.
//   - TestNewFoodObeysTheRulesOnCrowdedDomes asks for food on domes with
//     little room or none, and holds every answer to all 784 placements.
//     The domes are a sample.
//   - TestTickBitesFoodMadeWhereThereIsNoRoom plays ticks from made-up
//     domes that an eel leaves no room on, which no game the others play
//     has come to.
//
// Two things the game decides by chance: the turn it takes at a junction
// in demo mode, and where new food goes. They come from Go's shared random
// source, which a test can neither set nor replay, so no test here goes
// through them case by case. Each one that comes up is checked to be one
// the rules allow, and the tests count how many came up.

var (
	tickModelSeed = flag.Uint64("tick-model-seed", 0,
		"the seed for the random choices of the tick model tests; 0 picks a new one for every run")
	tickModelTick = flag.String("tick-model-tick", "",
		"a tick for TestTickModelPlaysATickAgain to play, as a failure of a tick model test prints it")
)

// tickModelSeedFor returns the seed a test makes its own choices by, and
// logs it. The seed fixes what the test chooses: where the stick goes, and
// the eels and domes it makes up. It does not fix what the game chooses by
// chance, so a second run with the same seed sets out the same way and
// then goes its own. What plays a failure again exactly is the tick the
// failure prints, with TestTickModelPlaysATickAgain.
func tickModelSeedFor(t *testing.T) uint64 {
	t.Helper()
	seed := *tickModelSeed
	if seed == 0 {
		seed = rand.Uint64() | 1
	}
	t.Logf("the test's own random choices are made with -tick-model-seed=%d", seed)
	return seed
}

// tickModelBreath is the brightness of the dome from tick to tick, stepped
// the way Game.Mover steps it before every move: down from 1 by a
// twentieth a tick until it reaches 0.6, and up again.
type tickModelBreath struct{ brightness, step float64 }

func tickModelNewBreath() tickModelBreath { return tickModelBreath{brightness: 1.0, step: -0.05} }

func (b *tickModelBreath) next() float64 {
	b.brightness += b.step
	if b.brightness <= 0.6 || b.brightness >= 1.0 {
		b.step = -b.step
	}
	return b.brightness
}

// tickModelNewEel is a new eel on the given segment, as the game starts
// one: seven points from position 6 down to 0, travelling towards the high
// end. The food is still to be put down.
func tickModelNewEel(segment int) tickModelState {
	m := tickModelState{high: true, foodLow: 1}
	for position := range tickModelFoodLength {
		m.trail = append(m.trail, segment*tickModelSegmentLength+position)
	}
	return m
}

// tickModelSearch is the search of TestTickMatchesModelForEverySteeringChoice
// for one kind of game.
type tickModelSearch struct {
	r      *tickModelRun
	breath tickModelBreath

	// first is the input this search plays the first time it is at a pixel
	// between junctions; see explore. The three searches each have their
	// own, so that between them they get round all 64 sooner.
	first int

	// What the search went through. The way an eel travels is named by
	// the segment end it travels towards.
	steered [tickModelEnds][tickModelStickValues]bool // the stick at each value on the tick through the junction
	// along counts the ticks played from each pixel between junctions, by
	// the end the eel travels towards and the position of its head, and
	// alongPlayed marks which of the 64 inputs they were played by.
	along       [tickModelEnds][tickModelSegmentLength]int
	alongPlayed [tickModelEnds][tickModelSegmentLength][tickModelInputs]bool
	// demoWent is the ends the game came onto when it picked the turn
	// itself, and demoTurns how often it was asked to.
	demoWent  [tickModelEnds][]int
	demoTurns [tickModelEnds]int
}

// tickModelSearchPoint is a moment the search comes back to.
type tickModelSearchPoint struct {
	m             tickModelState
	length, meals int
	breath        tickModelBreath
}

func (s *tickModelSearch) save() tickModelSearchPoint {
	return tickModelSearchPoint{m: s.r.m.clone(), length: s.r.length, meals: s.r.meals, breath: s.breath}
}

func (s *tickModelSearch) restore(p tickModelSearchPoint) {
	s.r.set(p.m)
	s.r.length, s.r.meals, s.breath = p.length, p.meals, p.breath
}

// tickModelInputs is how many inputs a tick can be played by: the 16 values
// of the stick, in a demo or not, with the joystick lost or not.
const tickModelInputs = 64

// tickModelAlongCases is how many ways there are to play a tick between
// junctions: towards each of the 98 segment ends, with the head on each of
// the 21 pixels short of that end, by each of the 64 inputs.
const tickModelAlongCases = tickModelEnds * (tickModelSegmentLength - 1) * tickModelInputs // 131,712

// tickModelDemoDraws is how many more times the search lets the game pick
// the turn at every junction it comes to, after the three times that are
// part of going through every input.
const tickModelDemoDraws = 5

// explore plays on from where the game is until the eel has been through
// the given number of junctions on every way there is.
//
// Along a segment the input does not decide where the eel goes, but the
// stick decides what the turn signals show, and a tick there is played by
// one input only: to play each by all 64 would make the search many times
// as long. Which one is counted round at every pixel by itself, for each
// way of travelling: the first tick the search plays from there is played
// by one input, the next time it comes there by the next, and so on. The
// input a round starts with moves on from pixel to pixel, which as a rule
// has the stick somewhere else from one tick to the next as the eel
// crosses a segment, in demo mode and out of it. That gets every input
// played at every pixel, but on some route or other and not on every one:
// the test counts that it did.
//
// At the end of the segment the search plays the tick through the
// junction with each of the 16 values the stick can have, in a game that
// is a demo or not and with the joystick lost or not: 64 ticks from the
// same moment, all compared. Then it goes on from that moment once for
// each segment end the junction leads onto.
func (s *tickModelSearch) explore(junctions int) {
	r := s.r
	end, atEnd := r.m.leaving()
	for !atEnd {
		position := r.m.head() % tickModelSegmentLength
		number := (s.first + 13*position + s.along[end][position]) % tickModelInputs
		s.along[end][position]++
		s.alongPlayed[end][position][number] = true
		in := tickModelInput{
			stick:      controller.Direction(number % tickModelStickValues),
			demo:       number/tickModelStickValues%2 == 1,
			lost:       number/tickModelStickValues/2%2 == 1,
			brightness: s.breath.next(),
		}
		if r.tick(in) {
			return // a growing eel ran into itself
		}
		end, atEnd = r.m.leaving()
	}
	if junctions == 0 {
		return
	}

	here := s.save()
	turnPicked := func(in tickModelInput) {
		s.restore(here)
		in.brightness = s.breath.next()
		if r.tick(in) {
			return
		}
		s.demoTurns[end]++
		onto := 2 * (r.m.head() / tickModelSegmentLength)
		if !r.m.high {
			onto++
		}
		if !slices.Contains(s.demoWent[end], onto) {
			s.demoWent[end] = append(s.demoWent[end], onto)
		}
	}
	for stick := range tickModelStickValues {
		s.steered[end][stick] = true
		for flags := range 4 {
			in := tickModelInput{stick: controller.Direction(stick), demo: flags&1 != 0, lost: flags&2 != 0}
			if in.stick == controller.Dir_Neutral && (in.demo || in.lost) {
				turnPicked(in)
				continue
			}
			s.restore(here)
			in.brightness = s.breath.next()
			r.tick(in)
		}
	}
	for range tickModelDemoDraws {
		turnPicked(tickModelInput{stick: controller.Dir_Neutral, demo: true})
	}

	var gone []int
	for turn, onto := range r.dome.leads[end] {
		if slices.Contains(gone, onto) {
			continue
		}
		gone = append(gone, onto)
		s.restore(here)
		if r.tick(tickModelInput{stick: tickModelCardinals[turn], brightness: s.breath.next()}) {
			continue // a growing eel ran into itself
		}
		s.explore(junctions - 1)
	}
}

// TestTickMatchesModelForEverySteeringChoice plays every route an eel can
// take through a number of junctions, with every input at each of them,
// and compares every tick of it with the model.
//
// A game starts with a new eel on each of the 49 segments in turn, as
// NewEel puts one down. From there the search goes through every input at
// every junction (see explore) until the eel has been through seven
// junctions on every way the dome offers, which is 169 ticks, or about 12
// seconds of play. It does that for three kinds of game:
//
//   - with the first food straight ahead of the eel, so that every game
//     has a meal, seven ticks of growing and a new food made by the tick
//     that ends them, before the first junction;
//   - with the first food on the segment furthest from the eel, so that
//     nearly every game is played by an eel of seven points that never
//     eats. Nothing in those is left to chance but the turns the game is
//     asked to pick;
//   - with an eel that never stops growing, which no game has: it is what
//     makes eels long enough, within seven junctions, to go round a loop
//     of the dome and run into themselves.
//
// Under the race detector the search stops after the first junction. The
// detector makes the comparison some thirty times slower and has nothing
// to find in it, as it all happens on one goroutine; the run without the
// detector is the one that counts.
//
// What this leaves out: anything past the depth; the positions new food
// can take and the turns the game picks (each one that comes up is
// checked, but they come up by chance); and whatever the model has wrong
// in the same way as the game. And the ticks between junctions are each
// played by one input of the 64. There the input changes nothing but the
// turn signals drawn. Between them the three searches play every input
// with the head on every pixel, travelling either way, 131,712
// combinations, and the test counts them. They do not play every input at
// every pixel on every route, or in each of the three kinds of game: a
// mistake in the signals that shows only with a long eel, or with an eel
// that came a certain way, is left to the random games.
func TestTickMatchesModelForEverySteeringChoice(t *testing.T) {
	t.Parallel()
	depth := 7
	if raceDetector {
		depth = 1
	}
	t.Logf("the search goes %d junctions deep", depth)

	kinds := []struct {
		name       string
		growth     int
		foodAhead  bool
		wantsMeals bool
	}{
		{name: "a meal straight ahead", foodAhead: true, wantsMeals: true},
		{name: "food far away"},
		{name: "an eel that never stops growing", growth: 1 << 30},
	}
	searches := make([]*tickModelSearch, len(kinds))
	t.Run("kinds", func(t *testing.T) {
		for k, kind := range kinds {
			t.Run(kind.name, func(t *testing.T) {
				t.Parallel()
				g := newLoopbackGame(t)
				// The packets are compared where they are built. With
				// the socket closed the seven sends of a tick fail at
				// once, which is what makes this many ticks affordable.
				g.Chromatik.ArtNetConnection.Close()
				r := tickModelNewRun(t, g, fmt.Sprintf("every steering choice %d junctions deep, %s", depth, kind.name))
				r.anyGrowth = kind.growth > tickModelFoodLength
				s := &tickModelSearch{r: r, first: k * tickModelInputs / len(kinds)}
				searches[k] = s

				for segment := range tickModelSegments {
					m := tickModelNewEel(segment)
					m.growth, m.fresh = kind.growth, true
					if kind.foodAhead {
						// Four pixels ahead of the head, which is at
						// position 6.
						m.foodSegment, m.foodLow, m.foodHigh = segment, 10, 16
					} else {
						far := 0
						for other, junctions := range r.dome.junctionsTo[2*segment+1] {
							if junctions > r.dome.junctionsTo[2*segment+1][far] {
								far = other
							}
						}
						m.foodSegment, m.foodLow, m.foodHigh = far, 8, 14
					}
					r.start(m)
					s.breath = tickModelNewBreath()
					s.explore(depth)
				}
				r.checkGraph()
				t.Logf("%v", r.stats)

				if kind.wantsMeals && (r.stats.meals < tickModelSegments || r.stats.newFood[tickModelFar] < tickModelSegments) {
					t.Errorf("%d meals and %d new foods far from the head, want at least one of each for each of the 49 starts",
						r.stats.meals, r.stats.newFood[tickModelFar])
				}
				if r.anyGrowth && depth >= 7 && r.stats.collisions == 0 {
					t.Errorf("no eel ran into itself, so the search never played the tick that ends a game that way")
				}
			})
		}
	})
	if t.Failed() {
		return
	}

	// What the three searches went through between them.
	var total tickModelStats
	steered, along, forksBothWays, forksAsked := 0, 0, 0, 0
	for end := range tickModelEnds {
		var went []int
		asked := 0
		for stick := range tickModelStickValues {
			for _, s := range searches {
				if s.steered[end][stick] {
					steered++
					break
				}
			}
		}
		for position := range tickModelSegmentLength {
			for number := range tickModelInputs {
				for _, s := range searches {
					if s.alongPlayed[end][position][number] {
						along++
						break
					}
				}
			}
		}
		for _, s := range searches {
			asked += s.demoTurns[end]
			for _, onto := range s.demoWent[end] {
				if !slices.Contains(went, onto) {
					went = append(went, onto)
				}
			}
		}
		// The ways on from this end, by the listing.
		var ways []int
		for _, onto := range searches[0].r.dome.leads[end] {
			if !slices.Contains(ways, onto) {
				ways = append(ways, onto)
			}
		}
		// A turn picked 60 times that leads two ways misses one of them
		// with a chance of one in 2^59, so that would be a rule broken
		// and not bad luck. At the full depth the fork asked least is
		// asked 456 times.
		if len(ways) > 1 && asked >= 60 {
			forksAsked++
			if len(went) == len(ways) {
				forksBothWays++
			}
		}
	}
	for _, s := range searches {
		total.add(s.r.stats)
	}
	t.Logf("%d junctions deep from 49 starts, three kinds of game: %v", depth, total)
	t.Logf("the tick through a junction was played with %d of the 1568 pairs of a way of travelling and a stick value, "+
		"and a tick between junctions in %d of the %d combinations of a way of travelling, a pixel and an input; "+
		"at %d of the %d forks where the game was asked for a turn 60 times or more it took both ways",
		steered, along, tickModelAlongCases, forksBothWays, forksAsked)

	if forksBothWays != forksAsked {
		t.Errorf("at %d of %d forks the game, picking the turn itself 60 times or more, only ever went one way", forksAsked-forksBothWays, forksAsked)
	}
	if total.junctions == 0 || total.demoTurns == 0 || total.noWayOn == 0 {
		t.Errorf("the search missed something it should have gone through: %v", total)
	}
	if depth >= 7 {
		// At this depth every way of travelling there is has come up at a
		// junction, and every pixel often enough between junctions for the
		// three searches to have got round all the inputs there.
		if steered != tickModelEnds*tickModelStickValues {
			t.Errorf("the search played the junction tick with %d of the 98 x 16 = 1568 pairs of a way of travelling and a stick value, want all of them", steered)
		}
		if along != tickModelAlongCases {
			t.Errorf("the search played a tick between junctions in %d of the 98 x 21 x 64 = %d combinations of a way of travelling, a pixel and an input, want all of them",
				along, tickModelAlongCases)
		}
		if forksAsked != 94 {
			t.Errorf("the game was asked for a turn 60 times or more at %d forks, want all 94", forksAsked)
		}
	}
}

// The three players of the random games.
const (
	tickModelHandsOff = iota // a demo, or a lost joystick: the game steers
	tickModelWanderer        // moves the stick now and then, anywhere
	tickModelSeeker          // steers for the food and round its own body
	tickModelPlayers
)

// tickModelPlayer plays random games on a run.
type tickModelPlayer struct {
	r      *tickModelRun
	rng    *rand.Rand
	breath tickModelBreath
	games  int
	kind   int
	in     tickModelInput
	// A game is given up after 200 ticks and up to longest more; left is
	// how many it still has.
	longest, left int
}

// tickModelWalk lays an eel along a random path through the dome that
// never crosses itself, as a moving eel leaves its body, and returns its
// pixels, tail first, and the way its head travels. The eel has n points
// unless the path boxes itself in first.
//
// A path that turns at random boxes itself in after a few hundred pixels.
// So that eels come out long enough to crowd the dome, three turns in
// four are taken with care: onto the segment that has the fewest ways on
// from its far end, but at least one, which leaves the roomier parts of
// the dome for later.
func tickModelWalk(d *tickModelDome, rng *rand.Rand, n int) (trail []int, high bool) {
	var on [tickModelPixels]bool
	m := tickModelState{high: rng.IntN(2) == 0}
	next := tickModelWay{pixel: rng.IntN(tickModelPixels), high: m.high}
	// open returns the ways on from an end that the eel is not in the way
	// of, each once.
	open := func(end int) (ways []tickModelWay) {
		for _, onto := range d.leads[end] {
			if w := tickModelArriving(onto); !on[w.pixel] && !slices.Contains(ways, w) {
				ways = append(ways, w)
			}
		}
		return ways
	}
	for len(m.trail) < n && !on[next.pixel] {
		on[next.pixel] = true
		m.trail, m.high = append(m.trail, next.pixel), next.high

		end, now := m.leaving()
		if !now {
			next.pixel = m.head() - 1
			if m.high {
				next.pixel = m.head() + 1
			}
			continue
		}
		ways := open(end)
		if len(ways) == 0 {
			break
		}
		next = ways[rng.IntN(len(ways))]
		if rng.IntN(4) != 0 {
			fewest := 0
			for _, w := range ways {
				// An eel that comes on at an end leaves by the other.
				if onward := len(open(w.onto ^ 1)); onward > 0 && (fewest == 0 || onward < fewest) {
					next, fewest = w, onward
				}
			}
		}
	}
	return m.trail, m.high
}

// newGame starts a game: two times in three as Game.Run starts one, and
// otherwise with an eel of up to 700 points already lying on the dome, so
// that long eels and little room for food come up far more often than
// play alone brings them about.
func (p *tickModelPlayer) newGame() {
	r, rng := p.r, p.rng
	p.breath = tickModelNewBreath()
	p.kind = p.games % tickModelPlayers
	p.games++
	p.in = tickModelInput{stick: controller.Dir_Neutral}
	if p.kind == tickModelHandsOff {
		p.in.demo = rng.IntN(4) != 0
		p.in.lost = !p.in.demo || rng.IntN(4) == 0
	}
	p.left = 200 + rng.IntN(p.longest)

	if rng.IntN(3) != 0 {
		r.startAsRunDoes()
		return
	}
	m := tickModelState{foodLow: 1}
	for len(m.trail) < tickModelFoodLength {
		m.trail, m.high = tickModelWalk(r.dome, rng, tickModelFoodLength*(1+rng.IntN(100)))
	}
	r.start(m)
	r.g.Food = NewFood(r.g)
	r.takeNewFood(false)
}

// seek returns the stick position that leads towards the food by the
// fewest junctions. A way onto a segment the eel's own body is on counts
// as longer than any way round, and one whose first pixel the body is on,
// which would end the game, as longer still.
func (p *tickModelPlayer) seek() controller.Direction {
	r := p.r
	end, _ := r.m.leaving()
	var best []int
	lowest := 0
	for turn, onto := range r.dome.leads[end] {
		cost := r.dome.junctionsTo[onto^1][r.m.foodSegment]
		for position := range tickModelSegmentLength {
			if r.frame.eel[onto/2*tickModelSegmentLength+position] {
				cost += 100
				break
			}
		}
		if r.frame.eel[tickModelArriving(onto).pixel] {
			cost += 1000
		}
		if len(best) == 0 || cost < lowest {
			best, lowest = best[:0], cost
		}
		if cost == lowest {
			best = append(best, turn)
		}
	}
	return tickModelCardinals[best[p.rng.IntN(len(best))]]
}

// play plays one tick and reports whether the game is over.
func (p *tickModelPlayer) play() (over bool) {
	rng := p.rng
	switch p.kind {
	case tickModelWanderer:
		// The stick starts centred, which ends the game at the first
		// junction unless it has moved by then, and it can go back
		// there, or to a diagonal that leads nowhere.
		switch n := rng.IntN(60); {
		case n < 10:
			p.in.stick = tickModelCardinals[rng.IntN(tickModelTurns)]
		case n == 10:
			p.in.stick = controller.Direction(rng.IntN(tickModelStickValues))
		}
	case tickModelSeeker:
		p.in.stick = p.seek()
		if rng.IntN(500) == 0 {
			p.in.stick = controller.Direction(rng.IntN(tickModelStickValues))
		}
	}
	p.in.brightness = p.breath.next()
	p.left--
	return p.r.tick(p.in) || p.left <= 0
}

// TestTickMatchesModelInLongRandomGames plays games of up to 3200 ticks
// and compares every tick with the model: 400,000 ticks between four
// players. Under the race detector, which has nothing to find in a
// comparison on one goroutine, it is 6,000 ticks of games of up to 600.
//
// A third of the games are left to the game to steer, as a demo or with
// the joystick lost. In a third the stick wanders, and those end at a
// junction sooner or later. In a third the player steers for the food and
// round its own body, which is what brings many meals and long eels. One
// game in three starts with a long eel already on the dome.
//
// This is a sample: it shows that the tick and the model agreed on what
// came up, and says nothing about what did not.
func TestTickMatchesModelInLongRandomGames(t *testing.T) {
	t.Parallel()
	const players = 4
	ticks, longest := 100000, 3000
	if raceDetector {
		ticks, longest = 1500, 400
	}
	seed := tickModelSeedFor(t)

	results := make([]tickModelStats, players)
	t.Run("players", func(t *testing.T) {
		for n := range players {
			t.Run(fmt.Sprint(n), func(t *testing.T) {
				t.Parallel()
				g := newLoopbackGame(t)
				// As in the search: the packets are compared where they
				// are built, and a closed socket makes a send cost
				// nothing.
				g.Chromatik.ArtNetConnection.Close()
				r := tickModelNewRun(t, g, fmt.Sprintf("random games with -tick-model-seed=%d, player %d", seed, n))
				p := &tickModelPlayer{r: r, rng: rand.New(rand.NewPCG(seed, uint64(n))), longest: longest}
				for r.stats.ticks < ticks {
					p.newGame()
					for !p.play() {
					}
				}
				r.checkGraph()
				results[n] = r.stats
			})
		}
	})
	if t.Failed() {
		return
	}
	var total tickModelStats
	for _, s := range results {
		total.add(s)
	}
	t.Logf("%v; %d pixels could rightly be one of two colors", total, total.eitherColor)

	// Guard against passing because nothing happened. Every player plays
	// each kind of game, so even the short run has turns the game picked
	// and meals; the limits for the full run are far below what it goes
	// through.
	missed := total.junctions < total.ticks/100 || total.demoTurns == 0 || total.meals == 0 || total.newFood[tickModelFar] == 0
	if !raceDetector {
		missed = missed || total.demoTurns < 1000 || total.noWayOn < 20 || total.collisions < 20 ||
			total.meals < 300 || total.newFood[tickModelFar] < 300 || total.longest < 300
	}
	if missed {
		t.Errorf("the games missed something they should have gone through: %v", total)
	}
}

// TestTickSendsTheModelsPackets plays random games with the socket open,
// and reads every frame back from it: seven datagrams for every tick the
// eel survives, each the bytes the model has for its universe, and none
// for the tick that ends a game.
//
// Whether a random game ends in the ticks played here is left to chance,
// and about one run in seven has none that does. So after them the test
// plays the two ticks that end a game, one each way, from the states
// written down for TestTickModelPlaysATickAgain, and looks for datagrams
// after each.
func TestTickSendsTheModelsPackets(t *testing.T) {
	t.Parallel()
	seed := tickModelSeedFor(t)
	g, sink := newTestGame(t)
	r := tickModelNewRun(t, g, fmt.Sprintf("random games on the wire with -tick-model-seed=%d", seed))
	r.wire = sink
	p := &tickModelPlayer{r: r, rng: rand.New(rand.NewPCG(seed, 0)), longest: 300}
	for r.stats.ticks < 1500 {
		p.newGame()
		for !p.play() {
		}
	}
	// Had a tick that ended one of those games sent anything, the frame
	// after it would have been read out of step, or it is still waiting
	// here.
	sink.expectNoFrame()
	if r.stats.games < 2 || r.stats.junctions == 0 {
		t.Errorf("the games missed something they should have gone through: %v", r.stats)
	}

	for _, ending := range []struct {
		tick  string
		ended *int
		how   string
	}{
		{tickModelTickIntoOwnTail, &r.stats.collisions, "the eel running into itself"},
		{tickModelTickWithNoWayOn, &r.stats.noWayOn, "the stick leading nowhere"},
	} {
		m, in, _, err := tickModelParseTick(ending.tick)
		if err != nil {
			t.Fatalf("the tick kept for a game ended by %s: %v", ending.how, err)
		}
		r.start(m)
		before := *ending.ended
		if over := r.tick(in); !over || *ending.ended != before+1 {
			t.Fatalf("the tick kept for a game ended by %s did not end one that way (over=%v)", ending.how, over)
		}
		if d := sink.datagram(50 * time.Millisecond); d != nil {
			t.Fatalf("the tick that ended a game by %s sent a datagram of %d bytes, want nothing sent", ending.how, len(d))
		}
	}
	t.Logf("%v", r.stats)
}

// tickModelCrowdedDome makes up a dome with little room for food: an eel
// whose pixels leave free placements on the wanted level of nearness to
// its head and on none further away, or none at all for tickModelNoRoom.
// The pixels are not a path an eel could have crawled along. The food
// rules only ask which pixels the eel is on, which segment its head is on
// and which way it travels.
//
// A segment is closed by a pixel taken in every seven, and others at
// random. It is left open by keeping seven pixels in a row free, with any
// of the others taken.
func tickModelCrowdedDome(d *tickModelDome, rng *rand.Rand, wanted int) tickModelState {
	headSegment := rng.IntN(tickModelSegments)
	m := tickModelState{high: rng.IntN(2) == 0, foodLow: 1}
	end := 2 * headSegment
	if m.high {
		end++
	}
	var level [tickModelSegments]int
	for _, onto := range d.leads[end] {
		level[onto/2] = tickModelOneJunctionAhead
	}
	level[headSegment] = tickModelOnHeadSegment

	var taken [tickModelPixels]bool
	mustOpen := -1 // one segment of the wanted level always has room
	for _, s := range rng.Perm(tickModelSegments) {
		if level[s] == wanted {
			mustOpen = s
			break
		}
	}
	for s := range tickModelSegments {
		open := level[s] > wanted && rng.IntN(2) == 0
		if level[s] == wanted {
			open = s == mustOpen || rng.IntN(3) == 0
		}
		first := s * tickModelSegmentLength
		if open {
			free := rng.IntN(tickModelSegmentLength - tickModelFoodLength + 1)
			for position := range tickModelSegmentLength {
				taken[first+position] = (position < free || position >= free+tickModelFoodLength) && rng.IntN(3) == 0
			}
			continue
		}
		every := rng.IntN(tickModelFoodLength)
		for position := range tickModelSegmentLength {
			taken[first+position] = position%tickModelFoodLength == every || rng.IntN(4) == 0
		}
	}

	// The head is one of the taken pixels of its segment.
	var onHeadSegment []int
	for position := range tickModelSegmentLength {
		if taken[headSegment*tickModelSegmentLength+position] {
			onHeadSegment = append(onHeadSegment, headSegment*tickModelSegmentLength+position)
		}
	}
	// With nothing taken on it, position 21 leaves the room at 0 to 6.
	head := headSegment*tickModelSegmentLength + tickModelSegmentLength - 1
	if len(onHeadSegment) > 0 {
		head = onHeadSegment[rng.IntN(len(onHeadSegment))]
	}
	taken[head] = false
	for pixel, is := range taken {
		if is {
			m.trail = append(m.trail, pixel)
		}
	}
	rng.Shuffle(len(m.trail), func(i, j int) { m.trail[i], m.trail[j] = m.trail[j], m.trail[i] })
	m.trail = append(m.trail, head)
	return m
}

// TestNewFoodObeysTheRulesOnCrowdedDomes asks the game for new food on
// domes where the eel leaves little room, and holds every answer to all
// 784 placements there are: the food is never on the eel while any
// placement is free, and it is on the head's segment, or one junction
// ahead of the head, only when nothing further away has room.
//
// Half the domes hold an eel lying as a moving eel leaves its body, along
// a random path that never crosses itself, of several hundred points. A
// path covers the segments it takes whole and leaves the others free, and
// no random path came near taking them all, so these domes have always
// had room far from the head: what they show is that the food keeps off a
// long eel. The other half are made up, pixel by pixel, so that the
// furthest room is far from the head, one junction ahead, on the head's
// segment or nowhere, a quarter each; the two middle cases and the last
// only come up that way. Food is asked for four times on every dome,
// since the game picks among the allowed placements by chance.
//
// The domes are a sample, 10,000 of them, or 400 under the race detector.
// The check of each answer is not: it looks at every placement. Whether
// the game picks evenly among the allowed ones is not looked at here.
func TestNewFoodObeysTheRulesOnCrowdedDomes(t *testing.T) {
	t.Parallel()
	domes := 10000
	if raceDetector {
		domes = 400
	}
	seed := tickModelSeedFor(t)
	rng := rand.New(rand.NewPCG(seed, 0))
	r := tickModelNewRun(t, newLoopbackGame(t), fmt.Sprintf("food on crowded domes with -tick-model-seed=%d", seed))

	var walked, madeUp [tickModelFoodOutcomes]int
	longest := 0
	for dome := range domes {
		var m tickModelState
		wanted := -1
		if dome%2 == 0 {
			m = tickModelState{foodLow: 1}
			m.trail, m.high = tickModelWalk(r.dome, rng, 300+rng.IntN(tickModelPixels-299))
			longest = max(longest, len(m.trail))
		} else {
			wanted = dome / 2 % tickModelFoodOutcomes
			m = tickModelCrowdedDome(r.dome, rng, wanted)
		}
		r.start(m)
		r.stats.ticks++ // a dome counts as a tick in a failure message
		for range 4 {
			r.g.Food = NewFood(r.g)
			outcome := r.takeNewFood(false)
			if wanted < 0 {
				walked[outcome]++
				continue
			}
			madeUp[outcome]++
			if outcome != wanted {
				r.failf("the test made up a dome whose furthest room is %s, and the model finds it %s: the test is wrong",
					tickModelFoodOutcomeNames[wanted], tickModelFoodOutcomeNames[outcome])
			}
		}
	}
	t.Logf("%d domes, food asked for 4 times on each. With an eel lying along a path (the longest of %d points): "+
		"%d far from the head, %d one junction ahead, %d on the head's segment, %d with no room. Made up: %d, %d, %d and %d",
		domes, longest, walked[0], walked[1], walked[2], walked[3], madeUp[0], madeUp[1], madeUp[2], madeUp[3])
	for outcome, n := range madeUp {
		if n != domes/2/tickModelFoodOutcomes*4 {
			t.Errorf("food was asked for %d times where the furthest room is %s, want %d", n, tickModelFoodOutcomeNames[outcome], domes/2/tickModelFoodOutcomes*4)
		}
	}
	if got := walked[0] + walked[1] + walked[2] + walked[3]; got != domes/2*4 || longest < 400 {
		t.Errorf("food was asked for %d times round an eel lying along a path, the longest of %d points, want %d times and an eel of 400 or more",
			got, longest, domes/2*4)
	}
}

// TestTickBitesFoodMadeWhereThereIsNoRoom plays the ticks that come when
// the eel leaves no seven free pixels in a row anywhere on the dome, and
// compares them with the model. The game then puts new food down anywhere,
// on the eel, for the eel to bite at once (see foodPlacement). By the model
// that is a meal on the very tick that made the food: the eel has seven
// more to grow, and the food loses the pixel at the end the eel's way of
// travelling meets first, wherever on the food the eel is. A food the eel
// is still on is bitten again on each tick after that, by whichever point
// of the eel is on it and not by the head alone.
//
// No game the other tests play has come to this, since an eel that crawls
// leaves whole segments free. So the domes are made up pixel by pixel, as
// in TestNewFoodObeysTheRulesOnCrowdedDomes, with an eel that is in pieces
// as no eel could be, and the check that the eel is in one piece is left
// out. Each dome starts on the last tick of the eel's growing, which is
// the tick that makes new food, and is played on until the eel runs into
// itself, which it soon does on a dome that full, or for 30 ticks.
//
// The domes are a sample, and the place of each food is the game's choice
// by chance.
func TestTickBitesFoodMadeWhereThereIsNoRoom(t *testing.T) {
	t.Parallel()
	const domes = 300
	seed := tickModelSeedFor(t)
	rng := rand.New(rand.NewPCG(seed, 0))
	r := tickModelNewRun(t, newLoopbackGame(t), fmt.Sprintf("food with no room on made-up domes with -tick-model-seed=%d", seed))
	r.scattered = true

	// bitten counts the ticks the eel survived that took a pixel off the
	// food, and awayFromHead those of them on which the head was on none
	// of the food's pixels; each of the two for a food made on that tick
	// and for one made before.
	var bitten, awayFromHead [2]int
	for range domes {
		m := tickModelCrowdedDome(r.dome, rng, tickModelNoRoom)
		// The food before is all eaten, and its meal has one tick of
		// growing left.
		m.growth, m.fresh = 1, false
		r.start(m)
		breath := tickModelNewBreath()
		for range 30 {
			made := r.stats.newFood[tickModelNoRoom]
			left := r.m.foodHigh - r.m.foodLow
			in := tickModelInput{stick: tickModelCardinals[rng.IntN(tickModelTurns)], brightness: breath.next()}
			if r.tick(in) {
				break
			}
			m := &r.m
			old := 1
			if r.stats.newFood[tickModelNoRoom] > made {
				old = 0 // made on this tick, and bitten or the comparison had failed
			} else if m.foodHigh-m.foodLow == left {
				continue // the eel is not on what is left of the food
			}
			bitten[old]++
			// The food as it was before the bite.
			low, high := m.foodLow, m.foodHigh
			if m.high {
				low--
			} else {
				high++
			}
			position := m.head() % tickModelSegmentLength
			if m.head()/tickModelSegmentLength != m.foodSegment || position < low || position > high {
				awayFromHead[old]++
			}
		}
	}
	r.checkGraph()
	t.Logf("%v", r.stats)
	t.Logf("%d foods made with no room were bitten on the tick that made them, %d of them with the head somewhere else; "+
		"%d bites of a food made on a tick before, %d of them with the head somewhere else",
		bitten[0], awayFromHead[0], bitten[1], awayFromHead[1])

	if got := r.stats.newFood[tickModelFar] + r.stats.newFood[tickModelOneJunctionAhead] + r.stats.newFood[tickModelOnHeadSegment]; got != 0 {
		t.Errorf("%d foods found room on domes the test made up to have none: the test is wrong", got)
	}
	// Two eels in three get through the first tick, and a food covers 7 of
	// the 1078 pixels the head can be on, so these limits are far below
	// what a run goes through.
	if awayFromHead[0] < domes/4 || awayFromHead[1] < domes/4 {
		t.Errorf("the eel bit a food made with no room, with its head somewhere else, on %d of the ticks that made one and on %d later ticks, want at least %d of each",
			awayFromHead[0], awayFromHead[1], domes/4)
	}
}

// again writes down the tick being checked, the state it started from and
// what it was played by, in the form -tick-model-tick takes.
func (r *tickModelRun) again() string {
	m, in := &r.before, r.input
	var b strings.Builder
	fmt.Fprintf(&b, "stick=%d demo=%v lost=%v brightness=%v growth=%d high=%v fresh=%v food=%d:%d:%d ",
		int(in.stick), in.demo, in.lost, in.brightness, m.growth, m.high, m.fresh, m.foodSegment, m.foodLow, m.foodHigh)
	if r.scattered {
		b.WriteString("scattered=true ")
	}
	b.WriteString("eel=")
	for i := len(m.trail) - 1; i >= 0; i-- {
		if i < len(m.trail)-1 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(m.trail[i]))
	}
	return b.String()
}

// tickModelParseTick reads what again wrote. scattered says the eel was
// made up in pieces, and is only written down when it was.
func tickModelParseTick(text string) (m tickModelState, in tickModelInput, scattered bool, err error) {
	seen := map[string]bool{}
	for _, field := range strings.Fields(text) {
		name, value, _ := strings.Cut(field, "=")
		seen[name] = true
		switch name {
		case "stick":
			var stick int
			stick, err = strconv.Atoi(value)
			in.stick = controller.Direction(stick)
		case "demo":
			in.demo, err = strconv.ParseBool(value)
		case "lost":
			in.lost, err = strconv.ParseBool(value)
		case "brightness":
			in.brightness, err = strconv.ParseFloat(value, 64)
		case "growth":
			m.growth, err = strconv.Atoi(value)
		case "high":
			m.high, err = strconv.ParseBool(value)
		case "fresh":
			m.fresh, err = strconv.ParseBool(value)
		case "food":
			_, err = fmt.Sscanf(value, "%d:%d:%d", &m.foodSegment, &m.foodLow, &m.foodHigh)
		case "scattered":
			scattered, err = strconv.ParseBool(value)
		case "eel":
			for _, number := range strings.Split(value, ",") {
				var pixel int
				if pixel, err = strconv.Atoi(number); err != nil || pixel < 0 || pixel >= tickModelPixels {
					return m, in, scattered, fmt.Errorf("eel pixel %q is not a pixel of the dome", number)
				}
				m.trail = append(m.trail, pixel)
			}
			slices.Reverse(m.trail) // written head first, kept tail first
		default:
			err = fmt.Errorf("unknown field")
		}
		if err != nil {
			return m, in, scattered, fmt.Errorf("%q: %v", field, err)
		}
	}
	for _, name := range []string{"stick", "demo", "lost", "brightness", "growth", "high", "fresh", "food", "eel"} {
		if !seen[name] {
			return m, in, scattered, fmt.Errorf("%s= is missing", name)
		}
	}
	if m.foodSegment < 0 || m.foodSegment >= tickModelSegments || m.foodLow < 0 || m.foodHigh >= tickModelSegmentLength {
		return m, in, scattered, fmt.Errorf("the food is not on a segment of the dome")
	}
	return m, in, scattered, nil
}

// Ticks worth playing on every run, written as a failure would print them.
const (
	// An eel crosses a junction onto a food and bites it.
	tickModelTickOntoFood = "stick=7 demo=false lost=false brightness=0.95 growth=0 high=true fresh=true food=1:0:6 eel=21,20,19,18,17,16,15"
	// An eel that has been round the three segments p1, r9 and r10 runs
	// into its own tail, which ends the game.
	tickModelTickIntoOwnTail = "stick=4 demo=false lost=false brightness=0.8 growth=3 high=true fresh=true food=29:8:14 eel=" +
		"439,438,437,436,435,434,433,432,431,430,429,428,427,426,425,424,423,422,421,420,419,418," +
		"396,397,398,399,400,401,402,403,404,405,406,407,408,409,410,411,412,413,414,415,416,417," +
		"21,20,19,18,17,16,15,14,13,12,11,10,9,8,7,6,5,4,3,2,1,0"
	// An eel at the end of a segment with the stick centred has nowhere to
	// go, which ends the game.
	tickModelTickWithNoWayOn = "stick=5 demo=false lost=false brightness=0.6 growth=0 high=false fresh=true food=9:8:14 eel=22,23,24,25,26,27,28"
	// The last tick of an eel's growing, on which the game makes new food.
	tickModelTickThatMakesFood = "stick=3 demo=true lost=false brightness=1 growth=1 high=false fresh=false food=4:10:9 eel=50,51,52,53,54,55,56,57,58,59,60,61,62"
)

var tickModelTicksToKeep = []string{
	tickModelTickOntoFood, tickModelTickIntoOwnTail, tickModelTickWithNoWayOn, tickModelTickThatMakesFood,
}

// TestTickModelPlaysATickAgain plays single ticks from a written-down
// state and compares them with the model. A failure of any tick model test
// prints the tick it failed on in that form, and
//
//	dev/run go test ./game/ -run TestTickModelPlaysATickAgain -tick-model-tick='...'
//
// plays it again, alone. The tick is played 50 times from the same state,
// so that what the game picks by chance on it, a turn or a place for new
// food, comes up more than once. Without the flag the test plays the few
// ticks of tickModelTicksToKeep.
func TestTickModelPlaysATickAgain(t *testing.T) {
	ticks := tickModelTicksToKeep
	if *tickModelTick != "" {
		ticks = []string{*tickModelTick}
	}
	r := tickModelNewRun(t, newLoopbackGame(t), "one tick again")
	for _, text := range ticks {
		m, in, scattered, err := tickModelParseTick(text)
		if err != nil {
			t.Fatalf("-tick-model-tick: %v", err)
		}
		r.anyGrowth, r.scattered = m.growth > tickModelFoodLength, scattered
		for range 50 {
			r.start(m)
			over := r.tick(in)
			if *tickModelTick != "" {
				t.Logf("the game is over=%v, and leaves %s", over, r.m.describe(r.dome))
			} else if got := r.again(); got != text {
				// What a failure prints has to be what this test reads.
				t.Fatalf("the tick %q is written down again as %q", text, got)
			}
		}
	}
}

// TestTickModelColorsOfEveryEel compares the model's colors with the
// game's for everything a frame can show: each point of an eel of every
// length up to all 1078 pixels, 581,581 colors, and the turn signals and
// the food at each of the brightnesses a game goes through.
//
// The model's eel colors are worked out in whole numbers. Where one could
// rightly be either of two levels (see tickModelRainbow) the game must
// have one of the two, and everywhere else the one.
func TestTickModelColorsOfEveryEel(t *testing.T) {
	colors, either, low := 0, 0, 0
	for n := 1; n <= tickModelPixels; n++ {
		for i := range n {
			colors++
			got := [3]byte(hsvToRGB(float64(i)*360.0/float64(n), 1.0, 1.0))
			want, also := tickModelRainbow(i, n)
			if want != also {
				either++
				if got == also {
					low++
				}
			}
			if got != want && got != also {
				t.Fatalf("point %d of an eel of %d is %v in the game, and %v or %v by the model", i, n, got, want, also)
			}
		}
	}
	if colors != 581581 || either != 9780 {
		t.Errorf("%d colors, of which %d could be either of two, want 581,581 and 9,780", colors, either)
	}
	t.Logf("of %d eel colors, %d could rightly be one level lower, and in the game %d are", colors, either, low)

	breath := tickModelNewBreath()
	values := map[float64]bool{}
	for range 64 {
		brightness := breath.next()
		values[brightness] = true
		for i := range tickModelSignalLength {
			want, also := tickModelSignalLevels(brightness, i)
			if want != also {
				t.Errorf("pixel %d of a turn signal at brightness %v is %d or %d depending on how the processor rounds; the model says they agree", i, brightness, want, also)
			}
			dimmed := brightness - (float64(i) * .1)
			if got := [3]byte(hsvToRGB(60, 1.0, dimmed)); got != [3]byte{want, want, 0} && got != [3]byte{also, also, 0} {
				t.Errorf("pixel %d of a yellow turn signal at brightness %v is %v in the game, want red and green at %d", i, brightness, got, want)
			}
			if got := [3]byte(hsvToRGB(0, 1.0, dimmed)); got != [3]byte{want, 0, 0} && got != [3]byte{also, 0, 0} {
				t.Errorf("pixel %d of a red turn signal at brightness %v is %v in the game, want red at %d", i, brightness, got, want)
			}
		}
		grey := byte(brightness * 255)
		if got := [3]byte(hsvToRGB(0, 0, brightness)); got != [3]byte{grey, grey, grey} {
			t.Errorf("food at brightness %v is %v in the game, want grey at %d", brightness, got, grey)
		}
	}
	if len(values) != 9 {
		t.Errorf("the brightness takes %d values in a game, want 9", len(values))
	}
}
