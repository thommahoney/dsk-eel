package game

import (
	"fmt"
	"log/slog"
)

const (
	SegmentLength = 22
)

// Tracks game state
type Game struct {
	logger *slog.Logger

	Segments []Segment
}

func NewGame(logger *slog.Logger) *Game {
	game := &Game{
		logger: logger,
	}

	game.Init()

	return game
}

func (g *Game) Start() {
	fmt.Println("Starting game")
}
