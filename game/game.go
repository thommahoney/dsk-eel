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
	Eel        *Eel
	Food       *Food
	Segments   [SegmentCount]*Segment

	// Input state. The joystick goroutine writes it in HandleControllerState
	// and the game loop reads it, at any time: before the first game, during
	// a game and between games. It belongs to the Game and not to one Eel, so
	// what the player is doing carries over from one game to the next.
	//
	// controlDir is the direction the stick is held in right now.
	// demoMode is whether the eel steers itself at junctions; Run sets it
	// from Config.DemoMode for every game and the first input clears it.
	// paused stops Mover from advancing the game.
	controlDir heldDirection
	demoMode   atomic.Bool
	paused     atomic.Bool

	// buttonIndex alternates between the two faders of a colored button on
	// each press. Only HandleButtonPress uses it, on the joystick goroutine.
	buttonIndex int

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

	if !game.Config.NoJoy {
		c, err := controller.NewController(config, game.HandleControllerState)
		if err != nil {
			return nil, fmt.Errorf("error in NewController: %s", err)
		}

		game.Controller = c
	}

	return game, nil
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

	// reset demo mode to configured value
	g.demoMode.Store(g.Config.DemoMode)

	var wg sync.WaitGroup

	g.Ableton.FireScene(0)

	wg.Add(1)
	go g.Mover(&wg)

	wg.Wait()
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

// HandleControllerState takes a new joystick state. The controller calls it
// on its own goroutine, one call at a time, whenever the state changes, and
// that can be at any moment: before the first game, in the middle of a tick
// or between games. It only touches the input state on Game, which the game
// loop reads with atomic loads, and never the eel or the frame.
func (g *Game) HandleControllerState(state controller.ControllerState) {
	g.Config.Logger.Info("HandleControllerState", "joystick", state.Direction.String(), "buttons", state.ButtonStatus.String())

	if g.demoMode.CompareAndSwap(true, false) {
		// we have a contender!
		g.Config.Logger.Info("HandleControllerState setting DemoMode = false")
	}

	g.controlDir.Store(state.Direction)

	// All five buttons held together does nothing.
	if state.ButtonStatus&controller.Btn_Red > 0 &&
		state.ButtonStatus&controller.Btn_Green > 0 &&
		state.ButtonStatus&controller.Btn_Blue > 0 &&
		state.ButtonStatus&controller.Btn_Yellow > 0 &&
		state.ButtonStatus&controller.Btn_White > 0 {
		return
	}

	g.HandleButtonPress(state.ButtonStatus)
}

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
