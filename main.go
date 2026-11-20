package main

import (
	"log"
	"log/slog"
	"net"
	"os"

	"github.com/spf13/pflag"
	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/game"
)

// parseFlags registers the command line flags on the given flag set, parses
// args with it and returns the resulting configuration. main passes
// pflag.CommandLine; tests pass a flag set of their own.
func parseFlags(flags *pflag.FlagSet, args []string) (config.Config, error) {
	config := config.Config{}

	flags.BoolVar(&config.DemoMode, "demo", false, "Randomly move eel")
	flags.BoolVar(&config.NoJoy, "nojoy", false, "Run without a joystick connected")
	flags.CountVarP(&config.Verbosity, "verbose", "v", "Verbosity level eg. -v, -vv, -vvv etc.")
	flags.IPVar(&config.ArtNetDest, "chromatik", net.IPv4(127, 0, 0, 1), "IP address of Chromatik")
	flags.StringVar(&config.ControllerPath, "joystick", "/dev/hidraw3", "Joystick device filesystem path")
	flags.StringVar(&config.ListenSubnet, "listen", "", "Subnet of the local address to send Art-Net from, e.g. 10.0.0.0/24. Empty picks the address that reaches --chromatik; give it on a machine with more than one network")
	flags.StringVar(&config.OscDest, "osc-dest", "127.0.0.1", "IP address of OSC server. Usually the same as --chromatik")
	flags.IntVar(&config.OscPort, "osc-port", 3030, "Port of OSC server")
	flags.StringVar(&config.AbletonDest, "ableton", "192.168.1.104:11000", "host:port of Ableton Live's OSC server (AbletonOSC). Empty disables Ableton")

	err := flags.Parse(args)
	return config, err
}

func main() {
	config, err := parseFlags(pflag.CommandLine, os.Args[1:])
	if err != nil {
		log.Fatalf("invalid arguments: %s", err)
	}

	config.Logger = NewLogger(config.Verbosity)

	game, err := game.NewGame(&config)
	if err != nil {
		log.Fatalf("NewGame failure: %s", err)
	}

	for {
		game.Run()
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
