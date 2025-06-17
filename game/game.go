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
	Segments     []Segment
}

func NewGame(config *config.Config) *Game {
	game := &Game{
		Config: config,
	}

	game.Init()

	if !game.Config.NoJoy {
		c, err := controller.NewController(config.Logger, game.HandleControllerState)
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

var yellow [3]byte = [...]byte{0xff, 0xff, 0x00} // #FFFF00
var blue [3]byte = [...]byte{0x00, 0x00, 0xff}   // #0000FF
var red [3]byte = [...]byte{0xff, 0x00, 0x00}    // #FF0000
var black [3]byte = [...]byte{0x00, 0x00, 0x00}  // #000000
var white [3]byte = [...]byte{0xff, 0xff, 0xff}  // #ffffff

func (g *Game) HandleControllerState(state controller.ControllerState) {
	fmt.Println("joystick:", state.Direction, "buttons:", state.ButtonStatus)
	switch state.ButtonStatus {
	case controller.Btn_White:
		g.PrimaryColor = white
	case controller.Btn_Red:
		g.PrimaryColor = red
	case controller.Btn_Yellow:
		g.PrimaryColor = yellow
	case controller.Btn_Blue:
		g.PrimaryColor = blue
	case controller.Btn_None:
		g.PrimaryColor = black
	default:
		g.PrimaryColor = [3]byte{byte(rand.IntN(255)), byte(rand.IntN(255)), byte(rand.IntN(255))}
	}
}
