package controller

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// ioctlUnread asks a terminal how many bytes are waiting to be read.
const ioctlUnread = unix.TIOCINQ

// What unbufferTerminal must set the suspend keys to on Linux: ctrl-Z, off.
var wantKeysOff = map[int]uint8{unix.VSUSP: 0}

// openPty opens a new pseudo-terminal and returns its two ends: what is
// written to the master is what somebody typed, and the slave is the
// terminal a program reads.
func openPty(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no pseudo-terminals here: %v", err)
	}
	t.Cleanup(func() { _ = master.Close() })

	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("unlocking the pseudo-terminal: %v", err)
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("asking for the pseudo-terminal's number: %v", err)
	}

	slave, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("opening the terminal end: %v", err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}
