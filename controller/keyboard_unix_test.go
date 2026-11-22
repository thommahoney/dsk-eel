//go:build linux || darwin

package controller

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/config"
	"golang.org/x/sys/unix"
)

func termiosOf(t *testing.T, f *os.File) unix.Termios {
	t.Helper()
	termios, err := unix.IoctlGetTermios(int(f.Fd()), ioctlGetTermios)
	if err != nil {
		t.Fatalf("reading the terminal's settings: %v", err)
	}
	return *termios
}

// unread returns how many bytes have been typed on the terminal and not yet
// read.
func unread(t *testing.T, f *os.File) int {
	t.Helper()
	n, err := unix.IoctlGetInt(int(f.Fd()), ioctlUnread)
	if err != nil {
		t.Fatalf("asking the terminal how much is unread: %v", err)
	}
	return n
}

// waitForUnread waits until something typed has reached the terminal: the
// kernel passes it on a moment after the write.
func waitForUnread(t *testing.T, f *os.File) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for unread(t, f) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("what was typed did not reach the terminal")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestUnbufferTerminal checks the terminal setup on a real pseudo-terminal.
// Only what is meant to change may change: line buffering and echo go off,
// a read waits for one key, and the suspend keys are switched off (which
// ones, and how "off" is written, is in the file for each system). Ctrl-C and output
// processing are left alone. What was typed before is thrown away, a key
// typed afterwards is delivered without waiting for return, and restore
// puts everything back.
func TestUnbufferTerminal(t *testing.T) {
	master, slave := openPty(t)

	// Start from settings that would make reads return at once with
	// nothing, so that the test sees the ones unbufferTerminal sets.
	before := termiosOf(t, slave)
	before.Cc[unix.VMIN], before.Cc[unix.VTIME] = 0, 1
	if err := unix.IoctlSetTermios(int(slave.Fd()), ioctlSetTermios, &before); err != nil {
		t.Fatal(err)
	}
	if before.Lflag&unix.ICANON == 0 || before.Lflag&unix.ECHO == 0 {
		t.Fatalf("a new terminal should buffer lines and echo: %+v", before)
	}
	for key, off := range wantKeysOff {
		if before.Cc[key] == off {
			t.Fatalf("a new terminal should have its suspend keys: %+v", before)
		}
	}

	// A line typed before the game starts.
	if _, err := master.Write([]byte("q\n")); err != nil {
		t.Fatal(err)
	}
	waitForUnread(t, slave)

	restore, err := unbufferTerminal(int(slave.Fd()))
	if err != nil {
		t.Fatalf("unbufferTerminal: %v", err)
	}

	want := before
	want.Lflag &^= unix.ICANON | unix.ECHO
	want.Cc[unix.VMIN], want.Cc[unix.VTIME] = 1, 0
	for key, off := range wantKeysOff {
		want.Cc[key] = off
	}
	if during := termiosOf(t, slave); during != want {
		t.Errorf("the terminal is set to\n%+v, want\n%+v", during, want)
	}
	if want.Lflag&unix.ISIG == 0 || want.Oflag&unix.OPOST == 0 {
		t.Errorf("the test expects ctrl-C and output processing to stay on: %+v", want)
	}
	if n := unread(t, slave); n != 0 {
		t.Errorf("%d bytes typed before the switch are still waiting to be read", n)
	}

	// A read waits for a key, and one key with no return after it is
	// enough.
	got := make(chan string, 1)
	go func() {
		buf := make([]byte, 8)
		n, _ := slave.Read(buf)
		got <- string(buf[:n])
	}()
	select {
	case key := <-got:
		t.Fatalf("a read returned %q with nothing typed", key)
	case <-time.After(300 * time.Millisecond):
	}
	if _, err := master.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	select {
	case key := <-got:
		if key != "x" {
			t.Errorf("read %q from the terminal, want %q", key, "x")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a key typed without return was not delivered")
	}

	// Typed and not read when the game ends: not left for the shell.
	if _, err := master.Write([]byte("rm")); err != nil {
		t.Fatal(err)
	}
	waitForUnread(t, slave)
	restore()
	restore() // a second call is harmless
	if after := termiosOf(t, slave); after != before {
		t.Errorf("restore left the terminal as\n%+v, want\n%+v", after, before)
	}
	if n := unread(t, slave); n != 0 {
		t.Errorf("%d bytes typed before the restore are still waiting to be read", n)
	}
}

// TestUnbufferTerminalNeedsATerminal: a file or a pipe is refused.
func TestUnbufferTerminalNeedsATerminal(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	if restore, err := unbufferTerminal(int(null.Fd())); err == nil {
		restore()
		t.Error("unbufferTerminal accepted /dev/null")
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if restore, err := unbufferTerminal(int(r.Fd())); err == nil {
		restore()
		t.Error("unbufferTerminal accepted a pipe")
	}
}

// TestKeyboardHelperProcess is not a test. TestNewKeyboardInAProcess runs
// this test binary again with the variable below set, and this is then the
// program whose terminal, signals and exit it looks at: it starts the real
// keyboard on its standard input and waits.
func TestKeyboardHelperProcess(t *testing.T) {
	if os.Getenv("DSK_EEL_KEYBOARD_HELPER") != "1" {
		return
	}
	cfg := &config.Config{Logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	_, err := NewKeyboard(cfg, func(s ControllerState) {
		fmt.Fprintf(os.Stderr, "state %v/%v\n", s.Direction, s.ButtonStatus)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "NewKeyboard failed:", err)
		os.Exit(3)
	}
	select {}
}

// TestNewKeyboardInAProcess starts a program that reads the keyboard from a
// pseudo-terminal and ends it in each of the ways a person can. The
// terminal must be back as it was every time, and the program must end the
// way it would have without the keyboard: status 0 for q, and killed by
// the signal for a signal, which is what a shell looks at to stop a loop
// on ctrl-C. SIGQUIT is the exception in how it ends: Go prints the
// goroutines and exits with status 2.
func TestNewKeyboardInAProcess(t *testing.T) {
	for _, c := range []struct {
		name string
		// end ends the program; master is where its keys are typed.
		end        func(t *testing.T, cmd *exec.Cmd, master *os.File)
		wantSignal syscall.Signal
		wantExit   int
	}{
		{name: "q", end: func(t *testing.T, cmd *exec.Cmd, master *os.File) { typeKeys(t, master, "q") }},
		{name: "ctrl-C typed", wantSignal: syscall.SIGINT,
			end: func(t *testing.T, cmd *exec.Cmd, master *os.File) { typeKeys(t, master, "\x03") }},
		{name: "ctrl-backslash typed", wantExit: 2,
			end: func(t *testing.T, cmd *exec.Cmd, master *os.File) { typeKeys(t, master, "\x1c") }},
		{name: "SIGINT", wantSignal: syscall.SIGINT,
			end: func(t *testing.T, cmd *exec.Cmd, master *os.File) { _ = cmd.Process.Signal(syscall.SIGINT) }},
		{name: "SIGTERM", wantSignal: syscall.SIGTERM,
			end: func(t *testing.T, cmd *exec.Cmd, master *os.File) { _ = cmd.Process.Signal(syscall.SIGTERM) }},
		{name: "SIGHUP", wantSignal: syscall.SIGHUP,
			end: func(t *testing.T, cmd *exec.Cmd, master *os.File) { _ = cmd.Process.Signal(syscall.SIGHUP) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			master, slave := openPty(t)
			before := termiosOf(t, slave)

			cmd, stderr := startKeyboardHelper(t, slave, catchingInterrupt)
			waitForLine(t, stderr, "keyboard:")
			if during := termiosOf(t, slave); during.Lflag&(unix.ICANON|unix.ECHO) != 0 {
				t.Fatalf("the program did not take the terminal over: flags %#x", during.Lflag)
			}

			// The keys reach the program, and ctrl-Z is one of them now
			// and does not stop it.
			typeKeys(t, master, "\x1a\x1b[A")
			waitForLine(t, stderr, "state North/None")

			c.end(t, cmd, master)
			status := waitForExit(t, cmd)
			switch {
			case c.wantSignal != 0:
				if !status.Signaled() || status.Signal() != c.wantSignal {
					t.Errorf("the program ended with %s, want killed by %v", describe(status), c.wantSignal)
				}
			default:
				if !status.Exited() || status.ExitStatus() != c.wantExit {
					t.Errorf("the program ended with %s, want exit status %d", describe(status), c.wantExit)
				}
			}
			// Read through the master end: on macOS the terminal end
			// is taken away from everybody when the program that had it
			// as its controlling terminal exits.
			if after := termiosOf(t, master); after != before {
				t.Errorf("the terminal was left as\n%+v, want\n%+v", after, before)
			}
		})
	}
}

// TestNewKeyboardWithSignalIgnored starts the program the way a shell
// starts a background job, with SIGINT ignored. Sending the signal on to
// itself would then do nothing, so it has to exit by itself, with the
// status a shell gives the signal, and still put the terminal back.
func TestNewKeyboardWithSignalIgnored(t *testing.T) {
	master, slave := openPty(t)
	before := termiosOf(t, slave)

	cmd, stderr := startKeyboardHelper(t, slave, ignoringInterrupt)
	waitForLine(t, stderr, "keyboard:")

	_ = cmd.Process.Signal(syscall.SIGINT)
	status := waitForExit(t, cmd)
	if !status.Exited() || status.ExitStatus() != 128+int(syscall.SIGINT) {
		t.Errorf("the program ended with %s, want exit status %d", describe(status), 128+int(syscall.SIGINT))
	}
	if after := termiosOf(t, master); after != before {
		t.Errorf("the terminal was left as\n%+v, want\n%+v", after, before)
	}
}

// TestNewKeyboardNeedsATerminal: with a pipe for standard input the program
// must say what it needs and not start.
func TestNewKeyboardNeedsATerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	cmd, stderr := startKeyboardHelper(t, r, catchingInterrupt)
	waitForLine(t, stderr, "--keyboard needs a terminal on standard input")
	if status := waitForExit(t, cmd); !status.Exited() || status.ExitStatus() != 3 {
		t.Errorf("the program ended with %s, want it to report the failure of NewKeyboard", describe(status))
	}
}

// Whether the helper process is started with SIGINT ignored.
const (
	catchingInterrupt = false
	ignoringInterrupt = true
)

// startKeyboardHelper runs TestKeyboardHelperProcess in a new process with
// stdin as its standard input and, if that is a terminal, as its
// controlling terminal, so that ctrl-C typed there is a signal for it. It
// returns the lines the process writes to standard error.
//
// To start it with SIGINT ignored, a shell ignores the signal and then
// becomes the helper, which inherits that. The test process cannot ignore
// the signal itself for the moment of the start: it would stay ignored for
// every process started after it.
func startKeyboardHelper(t *testing.T, stdin *os.File, ignoreInterrupt bool) (*exec.Cmd, <-chan string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestKeyboardHelperProcess$")
	if ignoreInterrupt {
		cmd = exec.Command("/bin/sh", "-c", `trap "" INT; exec "$0" "$@"`, os.Args[0], "-test.run=^TestKeyboardHelperProcess$")
	}
	cmd.Env = append(os.Environ(), "DSK_EEL_KEYBOARD_HELPER=1")
	cmd.Stdin = stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if _, err := unix.IoctlGetTermios(int(stdin.Fd()), ioctlGetTermios); err == nil {
		cmd.SysProcAttr.Setctty = true // Ctty is 0: standard input
	}
	pipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the helper process: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	lines := make(chan string, 256)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			default: // nobody is reading any more; a stack dump is long
			}
		}
	}()
	return cmd, lines
}

func typeKeys(t *testing.T, master *os.File, keys string) {
	t.Helper()
	if _, err := master.Write([]byte(keys)); err != nil {
		t.Fatalf("typing %q: %v", keys, err)
	}
}

// waitForLine reads the helper's output until a line contains want.
func waitForLine(t *testing.T, lines <-chan string, want string) {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("the helper process ended without printing %q", want)
			}
			if strings.Contains(line, want) {
				return
			}
		case <-deadline:
			t.Fatalf("the helper process did not print %q", want)
		}
	}
}

// waitForExit waits for the helper process and returns how it ended.
func waitForExit(t *testing.T, cmd *exec.Cmd) syscall.WaitStatus {
	t.Helper()
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("the helper process did not end")
	}
	return cmd.ProcessState.Sys().(syscall.WaitStatus)
}

func describe(status syscall.WaitStatus) string {
	if status.Signaled() {
		return fmt.Sprintf("killed by %v", status.Signal())
	}
	return fmt.Sprintf("exit status %d", status.ExitStatus())
}
