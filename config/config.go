package config

import (
	"log/slog"
	"net"
)

type Config struct {
	ArtNetDest     net.IP
	ControllerPath string
	ListenSubnet   string
	Logger         *slog.Logger
	NoJoy          bool
	Verbosity      int
}
