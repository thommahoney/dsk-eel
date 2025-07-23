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
	MovementFrequency = 25 * time.Millisecond
)

type Color [3]byte

var Yellow = Color{0xff, 0xff, 0x00} // #FFFF00
var Blue = Color{0x00, 0x00, 0xff}   // #0000FF
var Red = Color{0xff, 0x00, 0x00}    // #FF0000
var Black = Color{0x00, 0x00, 0x00}  // #000000
var White = Color{0xff, 0xff, 0xff}  // #ffffff
var Purple = Color{0xae, 0x00, 0xff} // #ae00ff
var Green = Color{0x00, 0xff, 0x00}  // #00ff00

// Tracks game state
type Game struct {
	Chromatik    *Chromatik
	Config       *config.Config
	Controller   *controller.Controller
	Eel          *Eel
	PrimaryColor Color
	QuitChan     chan struct{}
	Segments     [49]*Segment
}

func NewGame(config *config.Config) (*Game, error) {
	game := &Game{
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

func (g *Game) Run() {
	g.Config.Logger.Info("Starting game")

	g.Eel = NewEel(g)

	var wg sync.WaitGroup
	g.QuitChan = make(chan struct{})

	wg.Add(1)
	go g.Mover(&wg, g.QuitChan)

	wg.Wait()
}

func (g *Game) GameOver() {
	g.Config.Logger.Info("GameOver")
	close(g.QuitChan)
}

// calls Eel.Move() on an interval
func (g *Game) Mover(wg *sync.WaitGroup, quit <-chan struct{}) {
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

		case <-quit:
			g.Config.Logger.Info("Mover received quit")
			return
		}
	}
}

func (g *Game) HandleControllerState(state controller.ControllerState) {
	g.Config.Logger.Info("HandleControllerState", "joystick", state.Direction.String(), "buttons", state.ButtonStatus.String())

	g.Eel.ControlDir = state.Direction

	// special case for all buttons held (black is drawn as a rainbow)
	if state.ButtonStatus&controller.Btn_White > 0 &&
		state.ButtonStatus&controller.Btn_Red > 0 &&
		state.ButtonStatus&controller.Btn_Yellow > 0 &&
		state.ButtonStatus&controller.Btn_Blue > 0 {
		g.PrimaryColor = Black
		return
	}

	switch state.ButtonStatus {
	case controller.Btn_White:
		g.PrimaryColor = White
	case controller.Btn_Red:
		g.PrimaryColor = Red
	case controller.Btn_Yellow:
		g.PrimaryColor = Yellow
	case controller.Btn_Blue:
		g.PrimaryColor = Blue
	case controller.Btn_None:
		// No-Op
	default:
		g.PrimaryColor = RandomColor()
	}
}

func RandomColor() Color {
	return Color{byte(rand.IntN(255)), byte(rand.IntN(255)), byte(rand.IntN(255))}
}
