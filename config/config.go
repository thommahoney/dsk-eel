package config

import (
	"log/slog"
	"net"
)

type Config struct {
	IP net.IP
	Logger slog.Logger
	Verbosity int
}
