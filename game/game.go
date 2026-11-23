package game

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

const (
	MaxBrightness     = 1.0
	MinBrightness     = 0.6
	MovementFrequency = 70 * time.Millisecond
	RGB               = 3
	SegmentCount      = 49
)

// Tracks game state
type Game struct {
	Ableton    *Ableton
	Brightness float64
	Chromatik  *Chromatik
	Config     *config.Config
	Controller *controller.Controller
	Keyboard   *controller.Keyboard
	Eel        *Eel
	Food       *Food
	Segments   [SegmentCount]*Segment

	// Input state. The joystick goroutine writes it in HandleControllerState
	// and HandleControllerLost, and the game loop reads it, at any time:
	// before the first game, during a game and between games. It belongs to
	// the Game and not to one Eel, so what the player is doing carries over
	// from one game to the next.
	//
	// input is the direction the stick is held in right now, whether this
	// game is a demo, in which the eel steers itself at junctions, and
	// whether the joystick is lost, which makes every game a demo. Run
	// writes it too: it sets demo mode from Config.DemoMode for every game.
	// The three are one atomic word, so that a tick sees them as they were
	// together at one moment; see inputState.
	// paused stops Mover from advancing the game.
	input  inputState
	paused atomic.Bool

	// buttonIndex alternates between the two faders of a colored button on
	// each press. Only HandleButtonPress uses it, on the joystick goroutine.
	buttonIndex int

	// prevButtons is the buttons that were down in the previous joystick
	// state, used to tell a button going down from one being held. Only
	// HandleControllerState and HandleControllerLost use it, on the
	// joystick goroutine.
	prevButtons controller.ButtonStatus

	// tick is the time between eel moves. Zero means MovementFrequency;
	// tests set it lower so that whole games take milliseconds.
	tick time.Duration

	// Render state. Only one goroutine touches it at a time: Mover while a
	// game is being played, then GameOverAnimation, which Mover starts as
	// its last act. The joystick goroutine never does.
	//
	// frame is the pixel buffer for the frame being built. frameSet marks
	// the pixels the eel body occupies in that frame; it finds
	// self-collisions and food hits without searching the body.
	// artnetBufs holds one ArtDMX packet per universe. The headers are
	// filled in once by initDrawBuffers and Draw rewrites only pixel bytes.
	frame      [SegmentCount * SegmentLength]Color
	frameSet   [SegmentCount * SegmentLength]bool
	artnetBufs [universeCount][artnetPacketLen]byte

	// brightnessIncrement is the step Mover applies to Brightness each tick.
	brightnessIncrement float64
}

func NewGame(config *config.Config) (*Game, error) {
	game := &Game{
		Brightness: 1.0,
		Config:     config,
	}

	err := game.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %s", err)
	}

	if err := game.openInput(); err != nil {
		return nil, err
	}

	return game, nil
}

// How the keyboard and the joystick are opened. Tests put their own
// functions here.
var (
	newKeyboard   = controller.NewKeyboard
	newController = controller.NewController
)

// openInput opens what the game is steered with: the keyboard with
// --keyboard, nothing with --nojoy, and the joystick otherwise. The
// keyboard takes the joystick's place, so with --keyboard the joystick is
// not opened, whether or not --nojoy is given as well.
func (g *Game) openInput() error {
	switch {
	case g.Config.Keyboard:
		k, err := newKeyboard(g.Config, g.HandleControllerState)
		if err != nil {
			return fmt.Errorf("error in NewKeyboard: %s", err)
		}

		g.Keyboard = k
	case !g.Config.NoJoy:
		c, err := newController(g.Config, g.HandleControllerState, g.HandleControllerLost)
		if err != nil {
			return fmt.Errorf("error in NewController: %s", err)
		}

		g.Controller = c
	}
	return nil
}

func (g *Game) RandomSegment() *Segment {
	return g.Segments[rand.N(SegmentCount)]
}

func (g *Game) Run() {
	g.Config.Logger.Info("Starting game")

	g.Eel = NewEel(g)
	g.Food = NewFood(g)
	g.Brightness = 1.0
	g.brightnessIncrement = -0.05

	g.resetDemoMode()

	var wg sync.WaitGroup

	g.Ableton.FireScene(0)

	wg.Add(1)
	go g.Mover(&wg)

	wg.Wait()
}

// resetDemoMode sets demo mode to the configured value, for a new game.
// The stick and the lost joystick stay as they are, and the joystick
// goroutine can write either at this very moment, so this is an Update: a
// store of the state as it was loaded, changed, could undo an input or a
// loss.
func (g *Game) resetDemoMode() {
	demo := g.Config.DemoMode
	g.input.Update(func(in inputSnapshot) inputSnapshot { return in.withDemo(demo) })
}

func (g *Game) GameOver() {
	g.Config.Logger.Info("GameOver")
	g.Ableton.FireScene(7)
}

// Pause stops the eel. It may be called from any goroutine at any time;
// Mover looks at the flag on every tick, so a pause requested between games
// holds the next game still from its first tick.
func (g *Game) Pause() {
	g.paused.Store(true)
}

// Resume lets the eel move again from the next tick.
func (g *Game) Resume() {
	g.paused.Store(false)
}

// tickInterval is the time between eel moves.
func (g *Game) tickInterval() time.Duration {
	if g.tick > 0 {
		return g.tick
	}
	return MovementFrequency
}

// Mover calls Eel.Move() on an interval and steps the breathing brightness
// before each move. When the eel dies it hands over to GameOverAnimation.
// While the game is paused the ticks pass with nothing done: the eel does
// not move, no frame is sent and the brightness stays where it is.
func (g *Game) Mover(wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(g.tickInterval())
	defer ticker.Stop()

	for range ticker.C {
		if g.paused.Load() {
			continue
		}

		g.Brightness += g.brightnessIncrement
		if g.Brightness <= MinBrightness || g.Brightness >= MaxBrightness {
			g.brightnessIncrement = -g.brightnessIncrement
		}

		err := g.Eel.Move()
		if err != nil {
			g.Config.Logger.Info("Mover sent game over", "err", err)
			g.Brightness = 1.0
			g.GameOver()
			wg.Add(1)
			go g.GameOverAnimation(wg)
			return
		}
	}
}

func (g *Game) GameOverAnimation(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(g.tickInterval() * 2)
	defer ticker.Stop()

	increment := -0.05

	for range ticker.C {
		g.Brightness += increment
		if g.Brightness <= MinBrightness || g.Brightness >= MaxBrightness {
			increment = increment * -1
		}

		g.Eel.Shrink()
		g.resetFrame()
		if !g.Eel.drawBodyGameOver(g.Brightness) {
			g.Brightness = 1.0
			return
		}
		g.Draw()
	}
}

// HandleControllerLost is called by the controller, on its goroutine, when
// the joystick stops answering. The game carries on in demo mode with
// nothing held, until the first input from a joystick that has come back;
// that input ends demo mode the way any first input does. Pause is left as
// it is.
func (g *Game) HandleControllerLost() {
	// The stick let go and the joystick lost go into the input state in
	// one step, so no tick sees the stick let go without the eel steering
	// itself, which would end the game at a junction. Whether the game is
	// a demo is kept, and Run can write that at this very moment from its
	// own goroutine, so this is an Update and not a Swap.
	was := g.input.Update(inputSnapshot.withJoystickLost)
	if !was.joystickLost() {
		g.Config.Logger.Warn("joystick lost: running in demo mode until it is back and someone uses it")
	}
	g.prevButtons = controller.Btn_None
}

// HandleControllerState takes a new joystick state. The controller calls it
// on its own goroutine, one call at a time, whenever the state changes, and
// that can be at any moment: before the first game, in the middle of a tick
// or between games. It only touches the input state on Game, which the game
// loop reads with atomic loads (the stick, demo mode and the lost joystick
// in one load a tick, and pause in another), and never the eel or the
// frame.
func (g *Game) HandleControllerState(state controller.ControllerState) {
	g.Config.Logger.Info("HandleControllerState", "joystick", state.Direction.String(), "buttons", state.ButtonStatus.String())

	// The direction, and the end of demo mode and of a joystick loss, go
	// into the input state in one step, so no tick sees demo mode ended
	// without the direction that ended it: with the stick still neutral,
	// that would end the game at a junction. Nothing of the old state is
	// kept, so a Swap is enough, whatever Run is writing at this moment.
	was := g.input.Swap(stickHeld(state.Direction))
	if was.inDemoMode() {
		// we have a contender!
		g.Config.Logger.Info("HandleControllerState setting DemoMode = false")
	}

	// Buttons act once per press: only a state in which a button has gone
	// down since the previous state counts. The controller also reports a
	// state when the stick moves or a button is let go, and neither of
	// those is a press of the buttons still held.
	wentDown := state.ButtonStatus &^ g.prevButtons
	g.prevButtons = state.ButtonStatus
	if wentDown == controller.Btn_None {
		return
	}

	// All five buttons held together does nothing.
	if state.ButtonStatus&controller.Btn_Red > 0 &&
		state.ButtonStatus&controller.Btn_Green > 0 &&
		state.ButtonStatus&controller.Btn_Blue > 0 &&
		state.ButtonStatus&controller.Btn_Yellow > 0 &&
		state.ButtonStatus&controller.Btn_White > 0 {
		return
	}

	// The press is of the buttons that went down, not of everything that
	// is held. A second button pressed before the first is let go (a rolled
	// press) therefore acts as that button by itself, and the last button
	// pressed is the one in effect.
	//
	// Buttons that go down in the same state are a press of one of them.
	// Taken together they match no fader, so every channel would go to 0
	// and stay there, with nothing showing, until the next press. A bit
	// that is none of the five buttons is not a press at all.
	g.HandleButtonPress(firstButton(wentDown))
}

// buttonOrder is the order in which firstButton looks for a button.
var buttonOrder = [...]controller.ButtonStatus{
	controller.Btn_Red,
	controller.Btn_Green,
	controller.Btn_Blue,
	controller.Btn_Yellow,
	controller.Btn_White,
}

// firstButton picks one button out of several: a color before white, and
// the colors in the order red, green, blue, yellow. It returns Btn_None
// if none of the five is set.
func firstButton(buttons controller.ButtonStatus) controller.ButtonStatus {
	for _, b := range buttonOrder {
		if buttons&b != 0 {
			return b
		}
	}
	return controller.Btn_None
}

// HandleButtonPress runs the action of a button. HandleControllerState
// always passes it a single button, so what it does with several at once
// is only reached by calling it directly.
func (g *Game) HandleButtonPress(status controller.ButtonStatus) {
	if status == controller.Btn_None {
		// No-Op
		return
	}

	// @todo add back an easter egg for multiple buttons pressed

	g.buttonIndex += 1
	if g.buttonIndex >= 2 {
		g.buttonIndex = 0
	}

	for bs, addrs := range buttonToOSCAddress {
		for idx, a := range addrs {
			val := float32(0.0)
			if bs == status && (idx == g.buttonIndex || status == controller.Btn_White) {
				val = 1.0
			}

			err := g.Chromatik.OscSend(a, val)
			if err != nil {
				g.Config.Logger.Error("error sending channel OSC:", "error", err)
			}
		}

	}

	if status == controller.Btn_White {
		// Only this goroutine writes the flag, so reading it and then
		// setting it cannot lose an update.
		if g.paused.Load() {
			g.Resume()
		} else {
			g.Pause()
		}
		// no need to change color swatch
		return
	} else {
		g.Pause()
	}

	var swatch int
	switch status {
	case controller.Btn_Red:
		swatch = 1
	case controller.Btn_Green:
		swatch = 2
	case controller.Btn_Blue:
		swatch = 3
	case controller.Btn_Yellow:
		swatch = 4
	default:
		// multiple buttons pressed?
		swatch = 1
	}

	err := g.Chromatik.OscSend_swatch(swatch)
	if err != nil {
		g.Config.Logger.Error("error sending swatch OSC:", "error", err)
	}
}
