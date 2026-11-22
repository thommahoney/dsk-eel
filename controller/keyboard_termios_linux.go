package controller

import "golang.org/x/sys/unix"

// How Linux names the terminal calls unbufferTerminal makes.
const (
	ioctlGetTermios = unix.TCGETS
	ioctlSetTermios = unix.TCSETS

	// A control character set to this value is switched off.
	keyDisabled = 0
)

// The control characters that stop the program: ctrl-Z.
var suspendKeys = []int{unix.VSUSP}

// discardInput throws away what has been typed on the terminal and not yet
// read.
func discardInput(fd int) {
	_ = unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH)
}
