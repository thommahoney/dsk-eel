package game

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/controller"
)

// The tests in this file feed joystick states to a game the way the
// controller does: by calling HandleControllerState from a goroutine other
// than the one the game runs on. Whole games run on the real tickers, with
// the tick shortened to a millisecond so that a game takes a few dozen
// milliseconds instead of a second or two.

const fastTick = time.Millisecond

// A neutral-stick game that does not eat sends at most 15 frames of play and
// 3 of the game-over animation; one that eats, at most 21 in all. A game
// that has sent this many frames and is still running got past a junction.
const framesPastFirstJunction = 30

var allDirections = [...]controller.Direction{
	controller.Dir_Neutral,
	controller.Dir_North, controller.Dir_NorthEast, controller.Dir_East, controller.Dir_SouthEast,
	controller.Dir_South, controller.Dir_SouthWest, controller.Dir_West, controller.Dir_NorthWest,
}

// input is a joystick state with the stick at dir and the given buttons down.
func input(dir controller.Direction, buttons controller.ButtonStatus) controller.ControllerState {
	return controller.ControllerState{Direction: dir, ButtonStatus: buttons}
}

// press sends a button going down and coming up again, stick neutral.
func press(g *Game, button controller.ButtonStatus) {
	g.HandleControllerState(input(controller.Dir_Neutral, button))
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
}

// startGame calls Run on its own goroutine and returns a channel that is
// closed when the game is over.
func startGame(g *Game) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		g.Run()
		close(done)
	}()
	return done
}

// waitGameOver fails the test if the game does not end.
func waitGameOver(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return")
	}
}

// drain throws away every frame already sent, and goes on until nothing has
// arrived for the length of wait. Whatever the test did before calling it
// (ending the game, pausing it) is meant to stop the frames, so it fails
// the test if they are still coming after a few seconds.
func (s *artnetSink) drain(wait time.Duration) {
	s.t.Helper()
	const atMost = 3 * time.Second
	deadline := time.Now().Add(atMost)
	for s.datagram(wait) != nil {
		if time.Now().After(deadline) {
			s.t.Fatalf("frames are still being sent after %v", atMost)
		}
	}
}

// count returns how many pixels of the frame have the given color.
func count(pixels [testPixels]Color, c Color) int {
	n := 0
	for _, p := range pixels {
		if p == c {
			n++
		}
	}
	return n
}

// TestInputBeforeFirstGame sends joystick states to a game that has never
// run, as happens when the stick is touched while the program starts up.
func TestInputBeforeFirstGame(t *testing.T) {
	g, sink := newTestGame(t)

	if got := g.controlDir.Load(); got != controller.Dir_Neutral {
		t.Fatalf("a new game holds direction %v, want Neutral", got)
	}
	for _, dir := range allDirections {
		for buttons := controller.ButtonStatus(0); buttons < 32; buttons++ {
			g.HandleControllerState(input(dir, buttons))
			if got := g.controlDir.Load(); got != dir {
				t.Fatalf("after input %v the held direction is %v", dir, got)
			}
		}
	}
	sink.expectNoFrame()
}

// TestWhiteButtonTogglesPause checks that every press of the white button
// flips pause, and that a colored button always pauses, before any game has
// run and after one has ended.
func TestWhiteButtonTogglesPause(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick

	check := func(when string) {
		t.Helper()
		for i := 0; i < 4; i++ {
			was := g.paused.Load()
			press(g, controller.Btn_White)
			if g.paused.Load() == was {
				t.Fatalf("%s: white press %d left paused at %v", when, i, was)
			}
		}
		g.Resume()
		press(g, controller.Btn_Red)
		if !g.paused.Load() {
			t.Fatalf("%s: red did not pause", when)
		}
		press(g, controller.Btn_Green)
		if !g.paused.Load() {
			t.Fatalf("%s: green resumed a paused game", when)
		}
		press(g, controller.Btn_White)
		if g.paused.Load() {
			t.Fatalf("%s: white did not resume", when)
		}
	}

	check("before the first game")
	waitGameOver(t, startGame(g))
	sink.drain(20 * time.Millisecond)
	check("after a game")
}

// TestPauseBeforeFirstGame pauses before the first game starts. The game
// must then sit still, sending nothing, until the white button resumes it,
// and the breathing brightness must not have moved in the meantime.
func TestPauseBeforeFirstGame(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick

	// North is held throughout, so that the eel turns at its first
	// junction instead of dying there while the test is still looking.
	g.HandleControllerState(input(controller.Dir_North, controller.Btn_Red))
	g.HandleControllerState(input(controller.Dir_North, controller.Btn_None))
	done := startGame(g)
	// 50 ms is 50 ticks, several whole games' worth.
	sink.expectNoFrame()
	select {
	case <-done:
		t.Fatal("a paused game ran to its end")
	default:
	}

	g.HandleControllerState(input(controller.Dir_North, controller.Btn_White))
	g.HandleControllerState(input(controller.Dir_North, controller.Btn_None))
	first := sink.mustFrame()
	// The food is painted last, in grey at the current brightness. On the
	// first tick of a game that is 1.0 - 0.05.
	if got := count(first, Color{242, 242, 242}); got != GrowthIncrement {
		t.Errorf("first frame after resuming has %d food pixels at brightness 0.95, want %d", got, GrowthIncrement)
	}

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	waitGameOver(t, done)
}

// TestPauseBetweenGames pauses after a game has ended. The next game must
// start paused, and one press of white must resume it.
func TestPauseBetweenGames(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick

	waitGameOver(t, startGame(g))
	sink.drain(20 * time.Millisecond)

	press(g, controller.Btn_Blue)
	done := startGame(g)
	sink.expectNoFrame()

	press(g, controller.Btn_White)
	if g.paused.Load() {
		t.Error("still paused after white was pressed")
	}
	sink.mustFrame()
	waitGameOver(t, done)
}

// TestPauseDuringGame pauses and resumes a game that is being played.
func TestPauseDuringGame(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick

	// Hold north so the eel does not die at the first junction.
	g.HandleControllerState(input(controller.Dir_North, controller.Btn_None))
	done := startGame(g)
	sink.mustFrame()

	g.HandleControllerState(input(controller.Dir_North, controller.Btn_Yellow))
	// A tick that had already started may still send its frame.
	sink.drain(20 * time.Millisecond)
	sink.expectNoFrame()

	g.HandleControllerState(input(controller.Dir_North, controller.Btn_White))
	sink.mustFrame()

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	waitGameOver(t, done)
}

// TestHeldDirectionCarriesIntoNextGame pushes the stick north while no game
// is running and keeps it there. The controller reports nothing more, since
// nothing changes, so the next game has to start with north already held:
// its eel must get past the first junction instead of dying there.
func TestHeldDirectionCarriesIntoNextGame(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick

	waitGameOver(t, startGame(g))
	sink.drain(20 * time.Millisecond)

	g.HandleControllerState(input(controller.Dir_North, controller.Btn_None))

	done := startGame(g)
	for frames := 0; frames < framesPastFirstJunction; frames++ {
		if _, ok := sink.frame(2 * time.Second); !ok {
			t.Fatalf("the game stopped after %d frames: the eel died at the first junction with north held", frames)
		}
	}
	select {
	case <-done:
		t.Fatal("the game ended at the first junction with north held")
	default:
	}

	// Letting go ends the game at the next junction.
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	waitGameOver(t, done)
}

// TestDemoModeEndsOnFirstInput runs two games with --demo. Each must start
// in demo mode, whatever happened in the one before, and leave it on the
// first input.
func TestDemoModeEndsOnFirstInput(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick
	g.Config.DemoMode = true

	for game := 0; game < 2; game++ {
		done := startGame(g)
		sink.mustFrame()
		if !g.demoMode.Load() {
			t.Fatalf("game %d: not in demo mode at the start", game)
		}

		// A neutral stick is input too: the player let go of it.
		g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
		if g.demoMode.Load() {
			t.Fatalf("game %d: still in demo mode after an input", game)
		}
		// Out of demo mode a neutral stick ends the game at the next junction.
		waitGameOver(t, done)
		sink.drain(20 * time.Millisecond)
	}
}

// TestDemoModeSteersAtJunction checks the two junction rules for a neutral
// stick: in demo mode the eel picks a direction itself, otherwise it dies.
func TestDemoModeSteersAtJunction(t *testing.T) {
	g, _ := newTestGame(t)

	atJunction := func() error {
		place(g, 0, 21, 7, 1.0, 20, 12, 6)
		return g.Eel.Move()
	}

	if err := atJunction(); err == nil {
		t.Fatal("neutral stick, no demo mode: the eel got past a junction")
	}

	// It picks north, south, east or west, and all four lead somewhere
	// from every segment.
	g.demoMode.Store(true)
	for try := 0; try < 64; try++ {
		if err := atJunction(); err != nil {
			t.Fatalf("demo mode, try %d: %v", try, err)
		}
	}

	// A held direction wins over demo mode.
	place(g, 0, 21, 7, 1.0, 20, 12, 6)
	g.controlDir.Store(controller.Dir_North)
	if err := g.Eel.Move(); err != nil {
		t.Fatalf("demo mode with north held: %v", err)
	}
	if head := g.Eel.Head(); head.Segment != g.Segments[1] {
		t.Fatalf("demo mode with north held: head went to %s, want %s", head.Segment.Label, g.Segments[1].Label)
	}
}

// TestInputRace hammers the game with random joystick states from another
// goroutine, starting before the first game and going on while whole games
// are played one after another. Run under the race detector, it fails if
// the input path and the game loop share anything unsynchronised.
func TestInputRace(t *testing.T) {
	g, _ := newTestGame(t)
	g.tick = fastTick

	// Hammer for at least a second, and for longer on a slow machine, until
	// a few whole games have been played.
	const (
		hammerAtLeast = time.Second
		hammerAtMost  = 10 * time.Second
		wantGames     = 5
	)

	var states atomic.Int64
	stop := make(chan struct{})
	var hammer sync.WaitGroup
	hammer.Add(1)
	go func() {
		defer hammer.Done()
		rng := rand.New(rand.NewPCG(1, 2))
		for {
			select {
			case <-stop:
				return
			default:
			}
			dir := allDirections[rng.IntN(len(allDirections))]
			g.HandleControllerState(input(dir, controller.ButtonStatus(rng.IntN(32))))
			states.Add(1)
			// Most button states pause. Resume half of the time,
			// with a fresh press of white, so that games keep moving and
			// come to an end.
			if g.paused.Load() && rng.IntN(2) == 0 {
				g.HandleControllerState(input(dir, controller.Btn_None))
				g.HandleControllerState(input(dir, controller.Btn_White))
				states.Add(2)
			}
		}
	}()

	// Let input arrive before the first game exists.
	for states.Load() < 100 {
		time.Sleep(time.Millisecond)
	}

	var games atomic.Int64
	var lastGame atomic.Bool
	played := make(chan struct{})
	go func() {
		defer close(played)
		for !lastGame.Load() {
			g.Run()
			games.Add(1)
		}
	}()

	start := time.Now()
	for time.Since(start) < hammerAtLeast || (games.Load() < wantGames && time.Since(start) < hammerAtMost) {
		time.Sleep(5 * time.Millisecond)
	}
	close(stop)
	hammer.Wait()

	// Leave the stick neutral and the game running, so the game being
	// played ends at its next junction.
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	g.Resume()
	lastGame.Store(true)
	waitGameOver(t, played)

	t.Logf("%d joystick states during %d games", states.Load(), games.Load())
	if games.Load() < wantGames {
		t.Errorf("only %d games were played in %v", games.Load(), hammerAtMost)
	}
}
