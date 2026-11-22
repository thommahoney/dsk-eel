package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"runtime"

	"github.com/spf13/pflag"
	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
	"github.com/thommahoney/dsk-eel/game"
)

// version names the build. A release build is given the name of its release
// when it is linked (see dev/release); any other build says "dev".
var version = "dev"

// versionLine is what --version prints: the build, the Go release and the
// system it was built with and for, and whether it is a build that cannot
// open the joystick.
func versionLine() string {
	line := fmt.Sprintf("dsk-eel %s (%s, %s/%s", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if !controller.HasJoystick {
		line += ", keyboard only"
	}
	return line + ")"
}

func main() {
	cfg, err := config.Parse(pflag.CommandLine, os.Args[1:])
	if err != nil {
		// The exit status an unknown flag gets from the flag library.
		fmt.Fprintf(os.Stderr, "invalid arguments: %s\n", err)
		os.Exit(2)
	}

	if cfg.PrintVersion {
		fmt.Println(versionLine())
		return
	}

	cfg.Logger = NewLogger(cfg.Verbosity)

	game, err := game.NewGame(&cfg)
	if err != nil {
		log.Fatalf("NewGame failure: %s", err)
	}

	for {
		game.Run()
	}
}

// NewLogger returns the program's logger: warnings and errors by default,
// information as well with -v, and everything with -vv or more.
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
