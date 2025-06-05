package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/spf13/pflag"
	"github.com/thommahoney/dsk-eel/controller"
)

func main() {
	var verbosity int
	pflag.CountVarP(&verbosity, "verbose", "v", "set verbosity level eg. -v, -vv, -vvv etc.")
	pflag.Parse()

	logger := NewLogger(verbosity)

	c, err := controller.NewController(logger)
	if err != nil {
		log.Fatal(err)
	}

	var prevDirection controller.Direction
	var prevButtons controller.ButtonStatus

	for {
		state, err := c.GetState()
		if err != nil {
			log.Fatal("error retrieving controller state:", err)
		}

		if state.Direction != prevDirection {
			prevDirection = state.Direction
			fmt.Println("joystick:", state.Direction)
		}

		if state.ButtonStatus != prevButtons {
			prevButtons = state.ButtonStatus
			fmt.Println("buttons:", state.ButtonStatus)
		}
	}
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
