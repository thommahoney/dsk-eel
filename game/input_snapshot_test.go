package game

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// The stick, demo mode and the lost joystick are one atomic word, so that a
// tick never plays by the stick of one moment and the demo mode of another.
// When they were three words it could: a tick that loaded a neutral stick,
// then had the first input of a demo game arrive, then loaded the flags,
// saw a neutral stick in a game that was no demo, and ended the game at a
// junction with the stick held.
//
// The tests in this file check what that rests on. The word holds each of
// the three without disturbing the others. Everything that changes the
// input leaves the whole state it should, logs what it should, and gets
// there in one step, with no state in between for a tick to load. A change
// that keeps part of the old state loses nothing another goroutine writes
// meanwhile. And a tick plays by the snapshot it loaded and by nothing
// else.

// inputOf is the state with the stick held in dir, in a demo or not, with
// the joystick lost or not.
func inputOf(dir controller.Direction, demo, lost bool) inputSnapshot {
	in := stickHeld(dir).withDemo(demo)
	if lost {
		in |= inputJoystickLost
	}
	return in
}

// describeInput says what a state is, for a failure message.
func describeInput(in inputSnapshot) string {
	s := "stick " + in.direction().String()
	if in.demo() {
		s += ", demo"
	}
	if in.joystickLost() {
		s += ", joystick lost"
	}
	return s
}

// everyInput is every state there is with the stick in one of its nine
// positions.
func everyInput() []inputSnapshot {
	var all []inputSnapshot
	for _, dir := range allDirections {
		for _, demo := range []bool{false, true} {
			for _, lost := range []bool{false, true} {
				all = append(all, inputOf(dir, demo, lost))
			}
		}
	}
	return all
}

// inputEvent is something that changes the input state.
type inputEvent struct {
	name string
	// happen makes it happen, with the game's own code.
	happen func(g *Game)
	// leaves is the state it has to leave, given the state before it.
	leaves func(was inputSnapshot) inputSnapshot
	// logs is which of the two lines about demo mode it has to log, given
	// the state before it, or "" for neither.
	logs func(was inputSnapshot) string
}

// The two lines the log has about demo mode, as the text handler writes
// them. Someone watching the log of a dome tells by them when a player
// took over from the demo and when the joystick went away.
const (
	logDemoModeEnded = `level=INFO msg="HandleControllerState setting DemoMode = false"`
	logJoystickLost  = `level=WARN msg="joystick lost: running in demo mode until it is back and someone uses it"`
)

// everyInputEvent is every way the input state changes while the game
// runs: a joystick state for each stick position, the loss of the
// joystick, and the start of a game with and without --demo.
func everyInputEvent() []inputEvent {
	var events []inputEvent
	for _, dir := range allDirections {
		events = append(events, inputEvent{
			name:   "the stick goes to " + dir.String(),
			happen: func(g *Game) { g.HandleControllerState(input(dir, controller.Btn_None)) },
			// Any joystick state is input, and ends demo mode and a loss.
			leaves: func(inputSnapshot) inputSnapshot { return inputOf(dir, false, false) },
			// The end of demo mode is logged when there was one to end,
			// from --demo or from a lost joystick.
			logs: func(was inputSnapshot) string {
				if was.inDemoMode() {
					return logDemoModeEnded
				}
				return ""
			},
		})
	}
	events = append(events, inputEvent{
		name:   "the joystick is lost",
		happen: func(g *Game) { g.HandleControllerLost() },
		leaves: func(was inputSnapshot) inputSnapshot {
			return inputOf(controller.Dir_Neutral, was.demo(), true)
		},
		// The warning is for the loss, not for every report of it: a
		// joystick that is opened again and fails again before anyone has
		// used it is reported lost a second time.
		logs: func(was inputSnapshot) string {
			if !was.joystickLost() {
				return logJoystickLost
			}
			return ""
		},
	})
	for _, demo := range []bool{false, true} {
		events = append(events, inputEvent{
			name: fmt.Sprintf("a game starts with --demo=%v", demo),
			happen: func(g *Game) {
				g.Config.DemoMode = demo
				g.resetDemoMode()
			},
			leaves: func(was inputSnapshot) inputSnapshot {
				return inputOf(was.direction(), demo, was.joystickLost())
			},
			logs: func(inputSnapshot) string { return "" },
		})
	}
	return events
}

// TestInputSnapshotHoldsEveryState checks that a state reads back as the
// stick, demo mode and lost joystick it was made from, for every value the
// four bits of a direction can take, that no two states are the same word,
// and that the zero word is a game nobody has touched.
func TestInputSnapshotHoldsEveryState(t *testing.T) {
	var zero inputSnapshot
	if zero.direction() != controller.Dir_Neutral || zero.demo() || zero.joystickLost() || zero.inDemoMode() {
		t.Errorf("the zero state is %q, want a neutral stick, no demo, joystick there", describeInput(zero))
	}

	seen := map[inputSnapshot]string{}
	for dir := range controller.Direction(16) {
		for _, demo := range []bool{false, true} {
			for _, lost := range []bool{false, true} {
				name := fmt.Sprintf("stick %04b demo=%v lost=%v", int(dir), demo, lost)
				in := inputOf(dir, demo, lost)
				if in.direction() != dir || in.demo() != demo || in.joystickLost() != lost {
					t.Errorf("%s reads back as stick %04b demo=%v lost=%v",
						name, int(in.direction()), in.demo(), in.joystickLost())
				}
				if got, want := in.inDemoMode(), demo || lost; got != want {
					t.Errorf("%s: inDemoMode is %v, want %v: the eel steers itself in a demo and while the joystick is lost", name, got, want)
				}
				if other, taken := seen[in]; taken {
					t.Errorf("%s and %s are the same word %#b", name, other, uint32(in))
				}
				seen[in] = name

				// Changing one of the three leaves the other two alone.
				if got := in.withDirection(controller.Dir_West); got != inputOf(controller.Dir_West, demo, lost) {
					t.Errorf("%s with the stick moved west is %q", name, describeInput(got))
				}
				if got := in.withDemo(!demo); got != inputOf(dir, !demo, lost) {
					t.Errorf("%s with demo=%v is %q", name, !demo, describeInput(got))
				}
			}
		}
	}

	// A direction wider than four bits, which neither the joystick nor the
	// keyboard can report, must not reach the two flags above them.
	for _, dir := range []controller.Direction{0b010101, 0b100101, 0b110101} {
		if in := stickHeld(dir); in.demo() || in.joystickLost() {
			t.Errorf("a stick held in direction %#b is %q, want no demo and the joystick there", int(dir), describeInput(in))
		}
	}

	// The word a game holds gives back what was put in it.
	var state inputState
	for _, in := range everyInput() {
		state.Swap(in)
		if got := state.Load(); got != in {
			t.Errorf("stored %q, loaded %q", describeInput(in), describeInput(got))
		}
	}
}

// TestInputEventsLeaveTheWholeState starts from every state and makes each
// thing happen that changes the input. Each must leave the state it is
// meant to, in all three parts: an input holds the stick there and ends
// demo mode and a loss, a loss lets the stick go and keeps demo mode, and a
// new game sets demo mode and keeps the stick and a loss.
//
// Each must also log what it is meant to. The two lines about demo mode
// are decided by the state that was replaced: the end of demo mode is
// logged by an input that ended one, and the loss of the joystick the
// first time it is reported.
func TestInputEventsLeaveTheWholeState(t *testing.T) {
	g := newLoopbackGame(t)
	var log bytes.Buffer
	g.Config.Logger = slog.New(slog.NewTextHandler(&log, nil))
	for _, was := range everyInput() {
		for _, event := range everyInputEvent() {
			g.input.Swap(was)
			log.Reset()
			event.happen(g)
			if got, want := g.input.Load(), event.leaves(was); got != want {
				t.Errorf("from %q, %s: the state is %q, want %q",
					describeInput(was), event.name, describeInput(got), describeInput(want))
			}
			for _, line := range []string{logDemoModeEnded, logJoystickLost} {
				want := 0
				if line == event.logs(was) {
					want = 1
				}
				if got := strings.Count(log.String(), line); got != want {
					t.Errorf("from %q, %s: the log has the line %s %d times, want %d; log:\n%s",
						describeInput(was), event.name, line, got, want, log.String())
				}
			}
		}
	}
}

// TestInputHandlersWriteInOneStep watches the state from a second goroutine
// while a handler of the joystick writes it, as a tick can load it at any
// moment. Every state it loads must be the one before the event or the one
// after it, whole. A handler that got from the one to the other in two
// steps, the flags and then the stick or the other way round, would show
// the state in between, and one of those is a neutral stick with the eel
// not steering itself: the state that ends a game at a junction.
//
// The cases are the events that change the stick and a flag together, so
// that there is a state in between to see. Putting the state back for the
// next round is one Swap, so the watcher sees nothing in between there
// either, and the test cannot fail when the handlers are right.
func TestInputHandlersWriteInOneStep(t *testing.T) {
	// Enough rounds that, with two processors or more, a write in two
	// steps is seen on every run, thousands of times over. With one, the
	// watcher only gets to look where the writer is interrupted, and may
	// see nothing.
	rounds := 100000
	if raceDetector {
		rounds = 20000
	}
	const watcherLoads = 2000
	north := input(controller.Dir_North, controller.Btn_None)
	cases := []struct {
		name   string
		before inputSnapshot
		happen func(g *Game)
		after  inputSnapshot
	}{
		{
			name:   "the first input of a demo game",
			before: inputOf(controller.Dir_Neutral, true, false),
			happen: func(g *Game) { g.HandleControllerState(north) },
			after:  inputOf(controller.Dir_North, false, false),
		},
		{
			name:   "the first input after the joystick was lost",
			before: inputOf(controller.Dir_Neutral, false, true),
			happen: func(g *Game) { g.HandleControllerState(north) },
			after:  inputOf(controller.Dir_North, false, false),
		},
		{
			name:   "the joystick is lost with the stick held",
			before: inputOf(controller.Dir_North, false, false),
			happen: func(g *Game) { g.HandleControllerLost() },
			after:  inputOf(controller.Dir_Neutral, false, true),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := newLoopbackGame(t)
			g.input.Swap(c.before)

			var loads, between atomic.Int64
			var firstBetween atomic.Uint32
			var stop atomic.Bool
			var watcher sync.WaitGroup
			watcher.Go(func() {
				for !stop.Load() {
					in := g.input.Load()
					if in != c.before && in != c.after && between.Add(1) == 1 {
						firstBetween.Store(uint32(in))
					}
					loads.Add(1)
				}
			})

			// Go on until the watcher has had its share too, however the
			// two are scheduled.
			i := 0
			for ; i < rounds || loads.Load() < watcherLoads; i++ {
				g.input.Swap(c.before)
				c.happen(g)
			}
			stop.Store(true)
			watcher.Wait()

			if n := between.Load(); n > 0 {
				t.Errorf("%d of %d loads saw a state that is neither %q, from before, nor %q, from after; the first was %q",
					n, loads.Load(), describeInput(c.before), describeInput(c.after),
					describeInput(inputSnapshot(firstBetween.Load())))
			}
			t.Logf("%d rounds watched by %d loads", i, loads.Load())
		})
	}
}

// TestInputUpdateKeepsAWriteThatLandsInTheMiddle is every interleaving of
// two writers there is: one in the middle of an Update, between its load
// and its swap, and another whose whole write lands right there. The end
// state must be as if the second had happened first and the Update after
// it, with neither lost, and Update must report the state it really
// replaced. The second writer is each event with the game's own code; the
// Update is each change the game makes that way.
func TestInputUpdateKeepsAWriteThatLandsInTheMiddle(t *testing.T) {
	changes := []struct {
		name   string
		change func(inputSnapshot) inputSnapshot
	}{
		{"losing the joystick", inputSnapshot.withJoystickLost},
		{"starting a demo game", func(in inputSnapshot) inputSnapshot { return in.withDemo(true) }},
		{"starting a game that is no demo", func(in inputSnapshot) inputSnapshot { return in.withDemo(false) }},
	}

	g := newLoopbackGame(t)
	for _, was := range everyInput() {
		for _, c := range changes {
			for _, other := range everyInputEvent() {
				g.input.Swap(was)
				tries := 0
				replaced := g.input.Update(func(in inputSnapshot) inputSnapshot {
					tries++
					if tries == 1 {
						// Update has loaded the state and not yet swapped.
						other.happen(g)
					}
					return c.change(in)
				})

				between := other.leaves(was)
				name := fmt.Sprintf("from %q, %s while %s", describeInput(was), other.name, c.name)
				if got, want := g.input.Load(), c.change(between); got != want {
					t.Errorf("%s: the state is %q, want %q: one of the two writes was lost",
						name, describeInput(got), describeInput(want))
				}
				if replaced != between {
					t.Errorf("%s: Update says it replaced %q, but the state then was %q",
						name, describeInput(replaced), describeInput(between))
				}
				wantTries := 2
				if between == was {
					// Nothing changed under the Update, so its first try
					// stands.
					wantTries = 1
				}
				if tries != wantTries {
					t.Errorf("%s: Update tried %d times, want %d", name, tries, wantTries)
				}
			}
		}
	}
}

// TestTickPlaysByTheInputItLoaded is the tick that used to end a game: its
// head is on the last pixel of a segment, it has loaded the input state,
// and then something changes the input before the tick gets any further.
// The tick must play as it would have with nothing happening, by the state
// it loaded: that state existed, and what happened is for the next tick.
// Every state is tried with every event.
//
// One of the cases is the one this test was written for: a demo game with
// a neutral stick, and then the stick goes north. The tick loaded a neutral
// stick in a demo, so the eel must turn by itself. The tick as it was
// loaded the stick before the input and demo mode after it, played a
// neutral stick in a game that was no demo, and ended the game.
func TestTickPlaysByTheInputItLoaded(t *testing.T) {
	g := newLoopbackGame(t)
	// Nothing reads the frames. With the socket closed the sends of a tick
	// fail at once.
	g.Chromatik.ArtNetConnection.Close()
	seg := g.Segments[0]
	turns := []*Hop{seg.GreaterUp, seg.GreaterDown, seg.GreaterLeft, seg.GreaterRight}

	// check fails unless a tick from the end of seg ended the way the
	// state it loaded says.
	check := func(name string, loaded inputSnapshot, err error) {
		t.Helper()
		head := g.Eel.Head()
		landedOn := func(hop *Hop) bool {
			return head.Segment == hop.Point.Segment && head.Position == hop.Point.Position
		}
		dir := loaded.direction()
		switch {
		case dir == controller.Dir_Neutral && !loaded.inDemoMode():
			if err == nil {
				t.Errorf("%s: the eel got past the junction with a neutral stick and no demo mode", name)
			}
		case dir == controller.Dir_Neutral:
			if err != nil {
				t.Errorf("%s: the game ended (%v), want the eel to turn by itself", name, err)
				return
			}
			for _, hop := range turns {
				if landedOn(hop) {
					return
				}
			}
			t.Errorf("%s: the head is at %s %d, which none of the four turns leads to", name, head.Segment.Label, head.Position)
		default:
			hop := seg.NextHop(Greater, dir)
			if hop == nil {
				if err == nil {
					t.Errorf("%s: the eel got past the junction, but %v leads nowhere from there", name, dir)
				}
				return
			}
			if err != nil {
				t.Errorf("%s: the game ended (%v), want the eel to turn %v", name, err, dir)
			} else if !landedOn(hop) {
				t.Errorf("%s: the head is at %s %d, want %s %d, where %v leads",
					name, head.Segment.Label, head.Position, hop.Point.Segment.Label, hop.Point.Position, dir)
			}
		}
	}

	for _, was := range everyInput() {
		// With nothing happening, through Move itself.
		place(g, 0, 21, 7, 1.0, 20, 12, 6)
		g.input.Swap(was)
		check(fmt.Sprintf("%q", describeInput(was)), was, g.Eel.Move())

		for _, event := range everyInputEvent() {
			place(g, 0, 21, 7, 1.0, 20, 12, 6)
			g.input.Swap(was)

			loaded := g.input.Load() // the tick starts, as in Move
			event.happen(g)
			err := g.Eel.move(loaded) // and goes on

			check(fmt.Sprintf("%q loaded, then %s", describeInput(was), event.name), loaded, err)
		}
	}
}

// TestInputWritersOnTwoGoroutines runs the writers that keep part of the
// state against each other, as they run in the game: the joystick goroutine
// and Run. One goroutine hammers the state with its write. The other writes
// its own part, and before every write looks whether what it wrote last is
// still there. A write that loaded the state and stored it back changed,
// in two steps, would undo it.
func TestInputWritersOnTwoGoroutines(t *testing.T) {
	// Enough writes that, with two processors or more, a write in two
	// steps is caught on every run, hundreds of times over. With one it is
	// caught only where a writer is interrupted between its steps, which
	// may be on no write at all. Under the race detector fewer are enough:
	// it makes a write slower and its two steps further apart.
	const hammerWrites = 2000
	writes := 100000
	if raceDetector {
		writes = 20000
	}
	cases := []struct {
		name string
		// hammer is the write of the one goroutine, number i.
		hammer func(g *Game, i int)
		// write is write number i of the other. It returns what part of
		// the state the hammer has to leave alone after it.
		write func(g *Game, i int) (name string, kept func(inputSnapshot) bool)
	}{
		{
			name: "games starting while the joystick is used and lost",
			hammer: func(g *Game, i int) {
				g.Config.DemoMode = i%2 == 0
				g.resetDemoMode()
			},
			write: func(g *Game, i int) (string, func(inputSnapshot) bool) {
				if i%3 == 2 {
					g.HandleControllerLost()
					return "the joystick lost and the stick neutral", func(in inputSnapshot) bool {
						return in.joystickLost() && in.direction() == controller.Dir_Neutral
					}
				}
				dir := allDirections[i%len(allDirections)]
				g.HandleControllerState(input(dir, controller.Btn_None))
				return "the stick held " + dir.String() + " and the joystick there", func(in inputSnapshot) bool {
					return !in.joystickLost() && in.direction() == dir
				}
			},
		},
		{
			name:   "the joystick lost over and over while games start",
			hammer: func(g *Game, i int) { g.HandleControllerLost() },
			write: func(g *Game, i int) (string, func(inputSnapshot) bool) {
				demo := i%2 == 0
				g.Config.DemoMode = demo
				g.resetDemoMode()
				return fmt.Sprintf("demo=%v", demo), func(in inputSnapshot) bool { return in.demo() == demo }
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := newLoopbackGame(t)

			var hammered atomic.Int64
			var stop atomic.Bool
			var hammer sync.WaitGroup
			hammer.Go(func() {
				for i := 0; !stop.Load(); i++ {
					c.hammer(g, i)
					hammered.Add(1)
				}
			})

			undone := 0
			name, kept := "", func(inputSnapshot) bool { return true }
			stillThere := func(i int) {
				if in := g.input.Load(); !kept(in) {
					undone++
					if undone == 1 {
						t.Errorf("write %d left %s, and then the state was %q: the other goroutine undid it",
							i, name, describeInput(in))
					}
				}
			}
			// Go on until the hammer has had its share too, however the
			// two are scheduled.
			i := 0
			for ; i < writes || hammered.Load() < hammerWrites; i++ {
				stillThere(i - 1)
				name, kept = c.write(g, i)
			}
			stop.Store(true)
			hammer.Wait()
			stillThere(i - 1)

			if undone > 1 {
				t.Errorf("%d of %d writes were undone", undone, i)
			}
			t.Logf("%d writes against %d from the other goroutine", i, hammered.Load())
		})
	}
}
