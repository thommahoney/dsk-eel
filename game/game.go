package game

import (
	"log"
	"math/rand/v2"
	"sync"

	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

type Color [3]byte

var Yellow = Color{0xff, 0xff, 0x00} // #FFFF00
var Blue = Color{0x00, 0x00, 0xff}   // #0000FF
var Red = Color{0xff, 0x00, 0x00}    // #FF0000
var Black = Color{0x00, 0x00, 0x00}  // #000000
var White = Color{0xff, 0xff, 0xff}  // #ffffff

// Tracks game state
type Game struct {
	Config       *config.Config
	Controller   *controller.Controller
	PrimaryColor Color
	Segments     [49]Segment
}

func NewGame(config *config.Config) *Game {
	game := &Game{
		Config:       config,
		PrimaryColor: RandomColor(),
	}

	game.Init()

	if !game.Config.NoJoy {
		c, err := controller.NewController(config, game.HandleControllerState)
		if err != nil {
			log.Fatal(err)
		}

		game.Controller = c
	}

	return game
}

func (g *Game) Run() {
	g.Config.Logger.Info("Starting game")

	var wg sync.WaitGroup

	wg.Add(1)

	go g.Draw(&wg)

	wg.Wait()
}

func (g *Game) HandleControllerState(state controller.ControllerState) {
	g.Config.Logger.Info("HandleControllerState", "joystick", state.Direction.String(), "buttons", state.ButtonStatus.String())

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
