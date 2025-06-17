package game

import (
	"fmt"
	"log"
	"math/rand/v2"
	"sync"

	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

// Tracks game state
type Game struct {
	Config       *config.Config
	Controller   *controller.Controller
	PrimaryColor [3]byte
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

	g.Draw()

	wg.Wait()
}

var Yellow [3]byte = [...]byte{0xff, 0xff, 0x00} // #FFFF00
var Blue [3]byte = [...]byte{0x00, 0x00, 0xff}   // #0000FF
var Red [3]byte = [...]byte{0xff, 0x00, 0x00}    // #FF0000
var Black [3]byte = [...]byte{0x00, 0x00, 0x00}  // #000000
var White [3]byte = [...]byte{0xff, 0xff, 0xff}  // #ffffff

func (g *Game) HandleControllerState(state controller.ControllerState) {
	fmt.Println("joystick:", state.Direction, "buttons:", state.ButtonStatus)
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

func RandomColor() [3]byte {
	return [3]byte{byte(rand.IntN(255)), byte(rand.IntN(255)), byte(rand.IntN(255))}
}
