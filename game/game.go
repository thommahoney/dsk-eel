package game

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"sync"

	"github.com/thommahoney/dsk-eel/controller"
)

// Tracks game state
type Game struct {
	logger *slog.Logger

	Segments []Segment
	Controller *controller.Controller
	IP net.IP
}

func NewGame(logger *slog.Logger, ip net.IP) *Game {
	game := &Game{
		logger: logger,
		IP: ip,
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
	g.logger.Info("Starting game")

	var wg sync.WaitGroup

	wg.Add(1)

	// draw

	wg.Wait()
}

func (g *Game) HandleControllerState(state controller.ControllerState) {
	fmt.Println("joystick:", state.Direction, "buttons:", state.ButtonStatus)
}
