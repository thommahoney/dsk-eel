package config

import (
	"log/slog"
	"net"
)

type Config struct {
	AbletonDest    string // host:port of Ableton's OSC server; empty disables Ableton
	ArtNetDest     net.IP
	ControllerPath string
	DemoMode       bool
	Keyboard       bool // steer with the keyboard instead of the joystick
	ListenSubnet   string
	Logger         *slog.Logger
	NoJoy          bool
	OscDest        string
	OscPort        int
	PrintVersion   bool // print the version and exit
	Verbosity      int
}
