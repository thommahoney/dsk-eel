package main

import (
	"log/slog"
	"net"
	"os"

	"github.com/spf13/pflag"
	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/game"
)

func main() {
	config := config.Config{}

	pflag.CountVarP(&config.Verbosity, "verbose", "v", "Verbosity level eg. -v, -vv, -vvv etc.")
	pflag.IPVar(&config.ArtNetDest, "chromatik", net.IP("127.0.0.1"), "IP address of Chromatik")
	pflag.StringVar(&config.ListenSubnet, "listen", "127.0.0.1/24", "Network subnet for Art-Net comms")
	pflag.BoolVar(&config.NoJoy, "nojoy", false, "Run without a joystick connected")

	pflag.Parse()

	config.Logger = NewLogger(config.Verbosity)

	game := game.NewGame(&config)
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
