package main

import (
	"log/slog"
	"os"

	"github.com/spf13/pflag"
	"github.com/thommahoney/dsk-eel/game"
)

func main() {
	var verbosity int
	pflag.CountVarP(&verbosity, "verbose", "v", "set verbosity level eg. -v, -vv, -vvv etc.")
	pflag.Parse()

	logger := NewLogger(verbosity)

	game := game.NewGame(logger)
	game.Run()
}

func NewLogger(verbosity int) *slog.Logger {
	logLevel := slog.LevelWarn
	if verbosity > 2 {
		verbosity = 2
	}
	switch verbosity {
	case 0:
		logLevel = slog.LevelWarn
	case 1:
		logLevel = slog.LevelInfo // -v
	case 2:
		logLevel = slog.LevelDebug // -vv
	}

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
}
