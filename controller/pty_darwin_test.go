package controller

import (
	"bytes"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctlUnread asks a terminal how many bytes are waiting to be read. It is
// FIONREAD, which the unix package does not name on macOS.
const ioctlUnread = 0x4004667f

// What unbufferTerminal must set the suspend keys to on macOS: ctrl-Z and
// ctrl-Y, off, which is written 0xff here.
var wantKeysOff = map[int]uint8{unix.VSUSP: 0xff, unix.VDSUSP: 0xff}

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

	// What grantpt, unlockpt and ptsname do.
	fd := master.Fd()
	name := make([]byte, 128)
	for _, call := range []struct {
		what    string
		request uintptr
		arg     uintptr
	}{
		{"granting", unix.TIOCPTYGRANT, 0},
		{"unlocking", unix.TIOCPTYUNLK, 0},
		{"naming", unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))},
	} {
		if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, call.request, call.arg); errno != 0 {
			t.Fatalf("%s the pseudo-terminal: %v", call.what, errno)
		}
	}
	if end := bytes.IndexByte(name, 0); end >= 0 {
		name = name[:end]
	}

	slave, err = os.OpenFile(string(name), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("opening the terminal end %q: %v", name, err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}
