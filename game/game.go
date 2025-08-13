package game

import (
	"fmt"
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
	Brightness   float64
	Chromatik    *Chromatik
	Config       *config.Config
	Controller   *controller.Controller
	Eel          *Eel
	Food         *Food
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

	var wg sync.WaitGroup
	g.QuitChan = make(chan struct{})

	wg.Add(1)
	go g.Mover(&wg)

	wg.Add(1)
	go g.BrightnessOscillator(&wg)

	wg.Wait()
}

func (g *Game) GameOver() {
	g.Config.Logger.Info("GameOver")
	// @todo trigger sound!
	close(g.QuitChan)
}

// calls Eel.Move() on an interval
func (g *Game) Mover(wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(MovementFrequency)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
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

	switch state.ButtonStatus {
	case controller.Btn_Red:
		g.PrimaryColor = Red
	case controller.Btn_Green:
		g.PrimaryColor = Green
	case controller.Btn_Blue:
		g.PrimaryColor = Blue
	case controller.Btn_Yellow:
		g.PrimaryColor = Yellow
	case controller.Btn_White:
		g.PrimaryColor = White
	case controller.Btn_None:
		// No-Op
	default:
		g.PrimaryColor = RandomColor()
	}
}
