package main

import (
	"context"
	"log/slog"
	"runtime"
	"strings"
	"testing"
)

// TestNewLogger: without -v only warnings and errors are logged, -v adds
// information, and -vv or more adds everything.
func TestNewLogger(t *testing.T) {
	for _, c := range []struct {
		verbosity int
		lowest    slog.Level
	}{
		{-1, slog.LevelWarn},
		{0, slog.LevelWarn},
		{1, slog.LevelInfo},
		{2, slog.LevelDebug},
		{3, slog.LevelDebug},
		{17, slog.LevelDebug},
	} {
		logger := NewLogger(c.verbosity)
		for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
			want := level >= c.lowest
			if got := logger.Enabled(context.Background(), level); got != want {
				t.Errorf("verbosity %d: logging at %v is %v, want %v", c.verbosity, level, got, want)
			}
		}
	}
}

// TestVersionLine: the line --version prints names the build, the Go
// release and the system.
func TestVersionLine(t *testing.T) {
	// A build that was not given a name says "dev".
	want := "dsk-eel dev (" + runtime.Version() + ", " + runtime.GOOS + "/" + runtime.GOARCH + ")"
	if got := versionLine(); got != want {
		t.Errorf("versionLine() = %q, want %q", got, want)
	}

	// A release build's name comes through.
	old := version
	defer func() { version = old }()
	version = "2026-11-22-abc1234"
	if got := versionLine(); !strings.HasPrefix(got, "dsk-eel 2026-11-22-abc1234 (go") {
		t.Errorf("versionLine() = %q, want it to start with the build's name", got)
	}
}
