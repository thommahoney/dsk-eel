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

	// reset demo mode to configured value
	g.DemoMode = g.Config.DemoMode

	var wg sync.WaitGroup
	g.QuitChan = make(chan struct{})

	g.Ableton.FireScene(0)

	wg.Add(1)
	go g.Mover(&wg)

	wg.Add(1)
	go g.BrightnessOscillator(&wg)

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

// calls Eel.Move() on an interval
func (g *Game) Mover(wg *sync.WaitGroup) {
	defer wg.Done()
	g.MoverTicker = time.NewTicker(MovementFrequency)
	defer g.MoverTicker.Stop()

	for {
		select {
		case <-g.MoverTicker.C:
			err := g.Eel.Move()
			if err != nil {
				g.Config.Logger.Info("Mover sent game over", "err", err)
				g.GameOver()
				return
			}

		case <-g.QuitChan:
			g.Config.Logger.Info("Mover received quit")
			return
		}
	}
}

func (g *Game) BrightnessOscillator(wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(MovementFrequency)
	defer ticker.Stop()

	increment := -0.05

	for {
		select {
		case <-ticker.C:
			g.Brightness += increment
			if g.Brightness <= MinBrightness || g.Brightness >= MaxBrightness {
				increment = increment * -1
			}

		case <-g.QuitChan:
			g.Config.Logger.Info("BrightnessOscillator received quit")
			g.Brightness = 1.0
			wg.Add(1)
			go g.GameOverAnimation(wg)
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
		eelBody, _ := g.Eel.Pixels(g.Brightness, true)
		if len(eelBody) == 0 {
			g.Brightness = 1.0
			return
		}
		g.Draw(eelBody)
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
				g.Config.Logger.Error("error handling button press", "error", err)
			}
		}
	}

	if status == controller.Btn_White {
		if g.Paused {
			g.Resume()
		} else {
			g.Pause()
		}
	} else {
		g.Pause()
	}
}
