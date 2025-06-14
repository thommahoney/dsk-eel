package game

import (
	"fmt"
	"log"
	"log/slog"
	"sync"

	"github.com/thommahoney/dsk-eel/controller"
)

// Tracks game state
type Game struct {
	logger *slog.Logger

	Segments []Segment
	Controller *controller.Controller
}

func NewGame(logger *slog.Logger) *Game {
	game := &Game{
		logger: logger,
	}

	game.Init()

	c, err := controller.NewController(logger, game.HandleControllerState)
	if err != nil {
		log.Fatal(err)
	}

	game.Controller = c

	return game
}

func (g *Game) Run() {
	fmt.Println("Starting game")

	var wg sync.WaitGroup

	wg.Add(1)

	// draw

	wg.Wait()
}

func (g *Game) HandleControllerState(state controller.ControllerState) {
	fmt.Println("joystick:", state.Direction, "buttons:", state.ButtonStatus)
}
