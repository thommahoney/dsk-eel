package controller

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thommahoney/dsk-eel/config"
	"golang.org/x/sys/windows"
)

// TestUnbufferConsoleNeedsAConsole: a file is refused. Tests have no
// console window to try the rest on; what is typed in one is covered by
// the tests of the reader, which are the same on every system.
func TestUnbufferConsoleNeedsAConsole(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "not-a-console"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if restore, err := unbufferConsole(windows.Handle(f.Fd())); err == nil {
		restore()
		t.Error("unbufferConsole accepted a file")
	}
}

// TestNewKeyboardNeedsAConsole: with standard input that is not a console,
// as it is under `go test`, the keyboard must say what it needs and not
// start.
func TestNewKeyboardNeedsAConsole(t *testing.T) {
	var mode uint32
	if windows.GetConsoleMode(windows.Handle(os.Stdin.Fd()), &mode) == nil {
		t.Skip("standard input is a console here")
	}

	cfg := &config.Config{Logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	k, err := NewKeyboard(cfg, func(ControllerState) {})
	if err == nil {
		t.Fatalf("NewKeyboard started with no console: %v", k)
	}
	if !strings.Contains(err.Error(), "--keyboard needs a console window") {
		t.Errorf("the error does not say what is needed: %v", err)
	}
}
