package game

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
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
	Ableton      *Ableton
	Brightness   float64
	Chromatik    *Chromatik
	Config       *config.Config
	Controller   *controller.Controller
	DemoMode     bool // actually respected, Config.DemoMode is intention
	Eel          *Eel
	Food         *Food
	MoverTicker  *time.Ticker
	Paused       bool
	PrimaryColor Color
	QuitChan     chan struct{}
	Segments     [SegmentCount]*Segment

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
		Brightness:   1.0,
		Config:       config,
		PrimaryColor: RandomColor(),
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
	g.DemoMode = g.Config.DemoMode

	var wg sync.WaitGroup
	g.QuitChan = make(chan struct{})

	g.Ableton.FireScene(0)

	wg.Add(1)
	go g.Mover(&wg)

	wg.Wait()
}

func (g *Game) GameOver() {
	g.Config.Logger.Info("GameOver")
	g.Ableton.FireScene(7)
	close(g.QuitChan)
}

func (g *Game) Pause() {
	// max duration of 270 years or something
	g.MoverTicker.Reset(math.MaxInt64 * time.Nanosecond)
	g.Paused = true
}

func (g *Game) Resume() {
	g.MoverTicker.Reset(MovementFrequency)
	g.Paused = false
}

// Mover calls Eel.Move() on an interval and steps the breathing brightness
// before each move. When the eel dies it hands over to GameOverAnimation.
func (g *Game) Mover(wg *sync.WaitGroup) {
	defer wg.Done()
	g.MoverTicker = time.NewTicker(MovementFrequency)
	defer g.MoverTicker.Stop()

	for {
		select {
		case <-g.MoverTicker.C:
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

		case <-g.QuitChan:
			g.Config.Logger.Info("Mover received quit")
			return
		}
	}
}

func (g *Game) GameOverAnimation(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(MovementFrequency * 2)
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

func (g *Game) HandleControllerState(state controller.ControllerState) {
	g.Config.Logger.Info("HandleControllerState", "joystick", state.Direction.String(), "buttons", state.ButtonStatus.String())

	if g.DemoMode {
		// we have a contender!
		g.Config.Logger.Info("HandleControllerState setting DemoMode = false")
		g.DemoMode = false
	}

	g.Eel.ControlDir = state.Direction

	// special case for all buttons held (black is drawn as a rainbow)
	if state.ButtonStatus&controller.Btn_Red > 0 &&
		state.ButtonStatus&controller.Btn_Green > 0 &&
		state.ButtonStatus&controller.Btn_Blue > 0 &&
		state.ButtonStatus&controller.Btn_Yellow > 0 &&
		state.ButtonStatus&controller.Btn_White > 0 {
		g.PrimaryColor = Black
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

	buttonIndex += 1
	if buttonIndex >= 2 {
		buttonIndex = 0
	}

	for bs, addrs := range buttonToOSCAddress {
		for idx, a := range addrs {
			val := float32(0.0)
			if bs == status && (idx == buttonIndex || status == controller.Btn_White) {
				val = 1.0
			}

			err := g.Chromatik.OscSend(a, val)
			if err != nil {
				g.Config.Logger.Error("error sending channel OSC:", "error", err)
			}
		}

	}

	if status == controller.Btn_White {
		if g.Paused {
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
