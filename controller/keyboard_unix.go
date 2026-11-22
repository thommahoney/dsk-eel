//go:build linux || darwin

package controller

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/thommahoney/dsk-eel/config"
	"golang.org/x/sys/unix"
)

// The signals that end the program and that it can catch. Each of them puts
// the terminal back before it does.
var keyboardSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}

// NewKeyboard starts reading the keyboard from standard input, which has to
// be a terminal. The terminal is set to hand over each key as it is pressed
// and not to show it. It is put back when the program is ended with q, with
// ctrl-C or ctrl-\, or by SIGTERM or SIGHUP. It is not put back if the
// program panics or is killed outright; `stty sane` does it then.
func NewKeyboard(config *config.Config, handleFunc func(ControllerState)) (*Keyboard, error) {
	restore, err := unbufferTerminal(int(os.Stdin.Fd()))
	if err != nil {
		return nil, fmt.Errorf("--keyboard needs a terminal on standard input: %w", err)
	}

	// Ctrl-C still arrives as a signal: only line buffering and echo are
	// turned off. A signal that ends the program has to put the terminal
	// back first. It is then sent again with nothing in its way, so that
	// the program ends the way it would have without the keyboard, which
	// is what a shell looks at to stop a loop on ctrl-C.
	//
	// A program started with one of the signals ignored, as a shell
	// starts a background job, would not be ended by sending it again.
	// That has to be asked before the signals are caught.
	startedIgnored := map[os.Signal]bool{}
	for _, sig := range keyboardSignals {
		startedIgnored[sig] = signal.Ignored(sig)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, keyboardSignals...)
	go func() {
		sig := (<-signals).(syscall.Signal)
		restore()
		if !startedIgnored[sig] {
			signal.Reset(sig)
			_ = syscall.Kill(syscall.Getpid(), sig)
			// The signal reaches some thread of the program, not
			// necessarily this one, so give it time to do its work.
			time.Sleep(5 * time.Second)
		}
		// The signal was ignored, or did not end the program: exit with
		// the status a shell gives the signal.
		os.Exit(128 + int(sig))
	}()

	fmt.Fprintln(os.Stderr, KeyboardHelp)

	quit := func() {
		restore()
		os.Exit(0)
	}
	return StartKeyboard(config.Logger, os.Stdin, handleFunc, quit), nil
}

// unbufferTerminal makes the terminal on fd deliver every key at once and
// stop echoing what is typed, and takes the suspend keys away (ctrl-Z, and
// on macOS ctrl-Y as well): a stopped game would come back to a terminal
// its shell has reset, and inside a container nothing could wake it at
// all. Everything else stays as it was, so ctrl-C
// still interrupts and log lines still end in a proper new line. Keys typed
// before the switch are thrown away.
//
// It returns a function that puts the terminal back, again throwing away
// what has been typed and not read; calling it more than once is harmless.
// It fails if fd is not a terminal.
func unbufferTerminal(fd int) (restore func(), err error) {
	old, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return nil, err
	}

	unbuffered := *old
	unbuffered.Lflag &^= unix.ICANON | unix.ECHO
	unbuffered.Cc[unix.VMIN] = 1
	unbuffered.Cc[unix.VTIME] = 0
	for _, key := range suspendKeys {
		unbuffered.Cc[key] = keyDisabled
	}
	if err := unix.IoctlSetTermios(fd, ioctlSetTermios, &unbuffered); err != nil {
		return nil, err
	}
	discardInput(fd)

	var once sync.Once
	return func() {
		once.Do(func() {
			_ = unix.IoctlSetTermios(fd, ioctlSetTermios, old)
			discardInput(fd)
		})
	}, nil
}
