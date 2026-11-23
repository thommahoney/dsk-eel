package controller

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"unicode/utf16"

	"github.com/thommahoney/dsk-eel/config"
	"golang.org/x/sys/windows"
)

// The exit status Windows gives a program that ctrl-C ended, 0xC000013A.
// It is written as the negative number with the same 32 bits, which is what
// Windows is handed either way, because the positive one does not fit an
// int where an int is 32 bits.
const statusControlCExit = 0xC000013A - 1<<32

// NewKeyboard starts reading the keyboard from standard input, which has to
// be a console window. The console is set to hand over each key as it is
// pressed and not to show it, and to send the arrow keys the way a terminal
// does (ESC [ A and so on), which is what the reader understands. It is put
// back when the program is ended with q or ctrl-C, or by closing the window.
func NewKeyboard(config *config.Config, handleFunc func(ControllerState)) (*Keyboard, error) {
	console := windows.Handle(os.Stdin.Fd())
	restore, err := unbufferConsole(console)
	if err != nil {
		return nil, fmt.Errorf("--keyboard needs a console window on standard input: %w", err)
	}

	// Ctrl-C and ctrl-Break still arrive as an interrupt, and closing the
	// window as a request to end: line buffering and echo are all that is
	// turned off. Either one puts the console back before the program ends.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		restore()
		os.Exit(statusControlCExit)
	}()

	fmt.Fprintln(os.Stderr, KeyboardHelp)

	quit := func() {
		restore()
		os.Exit(0)
	}
	return StartKeyboard(config.Logger, &consoleReader{console: console}, handleFunc, quit), nil
}

// unbufferConsole makes the console deliver every key at once, stop echoing
// what is typed and report the arrow keys as escape sequences. Ctrl-C is
// left to interrupt the program. Keys typed before the switch are thrown
// away.
//
// It returns a function that puts the console back, again throwing away
// what has been typed and not read; calling it more than once is harmless.
// It fails if the handle is not a console.
func unbufferConsole(console windows.Handle) (restore func(), err error) {
	var old uint32
	if err := windows.GetConsoleMode(console, &old); err != nil {
		return nil, err
	}

	unbuffered := old &^ (windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT)
	unbuffered |= windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	if err := windows.SetConsoleMode(console, unbuffered); err != nil {
		return nil, err
	}
	_ = windows.FlushConsoleInputBuffer(console)

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = windows.SetConsoleMode(console, old)
			_ = windows.FlushConsoleInputBuffer(console)
		})
	}, nil
}

// consoleReader reads what is typed in a console window as bytes. Reading
// os.Stdin would do nearly the same, but it takes ctrl-Z for the end of the
// input, as Windows programs do when they read lines, and here ctrl-Z is
// only a key that means nothing.
type consoleReader struct {
	console windows.Handle
	units   [16]uint16
}

func (c *consoleReader) Read(p []byte) (int, error) {
	// A character the console hands over can need three bytes here, so
	// do not take more of them than p has room for.
	want := min(len(c.units), len(p)/3)
	if want == 0 {
		return 0, nil
	}
	var got uint32
	if err := windows.ReadConsole(c.console, &c.units[0], uint32(want), &got, nil); err != nil {
		return 0, err
	}
	return copy(p, string(utf16.Decode(c.units[:got]))), nil
}
