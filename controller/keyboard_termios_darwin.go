package controller

import "golang.org/x/sys/unix"

// How macOS names the terminal calls unbufferTerminal makes.
const (
	ioctlGetTermios = unix.TIOCGETA
	ioctlSetTermios = unix.TIOCSETA

	// A control character set to this value is switched off.
	keyDisabled = 0xff
)

// The control characters that stop the program: ctrl-Z, and ctrl-Y, which
// stops it when the program reads the key.
var suspendKeys = []int{unix.VSUSP, unix.VDSUSP}

// discardInput throws away what has been typed on the terminal and not yet
// read.
func discardInput(fd int) {
	const read = 0x1 // FREAD: the input queue
	_ = unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, read)
}
