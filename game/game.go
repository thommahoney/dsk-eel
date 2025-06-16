package game

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"sync"

	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

// Tracks game state
type Game struct {
	logger *slog.Logger

	Segments []Segment
	Controller *controller.Controller
	IP net.IP
	NoJoy bool
}

func NewGame(config *config.Config) *Game {
	game := &Game{
		logger: config.Logger,
		IP: config.IP,
		NoJoy: config.NoJoy,
	}

	game.Init()

	if !game.NoJoy {
		c, err := controller.NewController(config.Logger, game.HandleControllerState)
		if err != nil {
			log.Fatal(err)
		}

		game.Controller = c
	}

	return game
}

func (g *Game) Run() {
	g.logger.Info("Starting game")

	var wg sync.WaitGroup

	wg.Add(1)

	g.Draw()

	wg.Wait()
}

func (g *Game) HandleControllerState(state controller.ControllerState) {
	fmt.Println("joystick:", state.Direction, "buttons:", state.ButtonStatus)
}
