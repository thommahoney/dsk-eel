package controller

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/config"
)

// The joystick is opened by its path, and nothing checks what is there.
// These tests are about the one thing that tells someone the path has gone
// to another device: the line in the log that says what was opened.

// connection is one line of the log that says a device was opened.
type connection struct {
	msg, manufacturer, product string
}

// connections returns the lines that say a device was opened, of those the
// program shows without -v: warnings and above.
func (l *logRecorder) connections() []connection {
	l.mu.Lock()
	defer l.mu.Unlock()
	var found []connection
	for _, r := range l.records {
		if r.Level < slog.LevelWarn || (r.Message != "connected to joystick" && r.Message != "reconnected to joystick") {
			continue
		}
		c := connection{msg: r.Message}
		r.Attrs(func(a slog.Attr) bool {
			switch a.Key {
			case "manufacturer":
				c.manufacturer = a.Value.String()
			case "product":
				c.product = a.Value.String()
			}
			return true
		})
		found = append(found, c)
	}
	return found
}

// TestEveryConnectionNamesTheDevice follows a joystick through a start, a
// loss and the return of something else at its path. Without -v the log
// must name each device that was opened, once, and nothing in between: not
// for a report, and not for an attempt to open that failed.
func TestEveryConnectionNamesTheDevice(t *testing.T) {
	first := newFakeDevice()
	r := start(t, first)

	// The strings are logged without the space around them.
	want := []connection{{"connected to joystick", "Fake Co", "Fake Stick"}}
	check := func(when string) {
		t.Helper()
		if got := r.log.connections(); !slices.Equal(got, want) {
			t.Errorf("%s the log shown without -v names these opened devices: %q, want %q", when, got, want)
		}
	}
	check("after the start")

	first.send(t, report(Dir_North, Btn_None))
	r.wantState(ControllerState{Dir_North, Btn_None})
	first.send(t, report(Dir_Neutral, Btn_Red))
	r.wantState(ControllerState{Dir_Neutral, Btn_Red})
	check("after two reports")

	close(first.unplug)
	waitFor(t, "the lost notification", func() bool { return r.lost.Load() == 1 })
	before := r.opener.count()
	waitFor(t, "three failed attempts to open", func() bool { return r.opener.count() >= before+3 })
	check("with the joystick away")

	// What comes back at the path is not the joystick.
	second := newFakeDevice()
	second.manufacturer, second.product = "Other Co", "Some Keyboard"
	r.opener.plug(second)
	second.send(t, report(Dir_East, Btn_None))
	r.wantState(ControllerState{Dir_East, Btn_None})
	want = append(want, connection{"reconnected to joystick", "Other Co", "Some Keyboard"})
	check("after another device came back")

	close(second.unplug)
	r.wantNoMoreStates()
	check("at the end")
}

// inTurn returns an OpenFunc that hands out the devices one after the other.
// A nil in the list is an attempt to open that fails, and so is every
// attempt after the last device.
func inTurn(devices ...*fakeDevice) OpenFunc {
	var mu sync.Mutex
	return func() (Device, error) {
		mu.Lock()
		defer mu.Unlock()
		if len(devices) == 0 {
			return nil, errNoDevice
		}
		d := devices[0]
		devices = devices[1:]
		if d == nil {
			return nil, errNoDevice
		}
		return d, nil
	}
}

// TestWorkingDeviceIsNamedAfterDeadOnes loses the joystick to a device that
// can be opened but fails on its first read. The log shown without -v names
// that device for its first two returns and then holds the line back, as it
// does the losses. What is opened after that is another device, and one
// that works: its line must not be held back as well, or the log would go
// on naming a device that is no longer the one the game reads. With -v the
// line that was held back is there too.
func TestWorkingDeviceIsNamedAfterDeadOnes(t *testing.T) {
	deadDevice := func() *fakeDevice {
		d := newFakeDevice()
		d.manufacturer, d.product = "Dead Co", "Dead Thing"
		close(d.unplug)
		return d
	}

	tests := []struct {
		name string
		// between is what the open function does between two devices.
		between []*fakeDevice
	}{
		{"one device straight after the other", nil},
		{"with a failed attempt to open before each device", []*fakeDevice{nil}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := newFakeDevice()
			other := newFakeDevice()
			other.manufacturer, other.product = "Other Co", "Some Keyboard"

			devices := []*fakeDevice{first}
			for _, d := range []*fakeDevice{deadDevice(), deadDevice(), deadDevice(), other} {
				devices = append(append(devices, tc.between...), d)
			}

			r := &rig{t: t, log: &logRecorder{}, states: make(chan ControllerState, 64)}
			c, err := Start(slog.New(r.log), inTurn(devices...), testRetry,
				func(s ControllerState) { r.states <- s },
				func() { r.lost.Add(1) })
			if err != nil {
				t.Fatalf("Start: %v", err)
			}
			r.c = c
			t.Cleanup(r.stop)
			t.Cleanup(func() { close(other.unplug) })

			first.send(t, report(Dir_North, Btn_None))
			r.wantState(ControllerState{Dir_North, Btn_None})
			close(first.unplug)

			// The device that works is being read: the game gets its states.
			other.send(t, report(Dir_East, Btn_None))
			r.wantState(ControllerState{Dir_East, Btn_None})
			other.send(t, report(Dir_West, Btn_None))
			r.wantState(ControllerState{Dir_West, Btn_None})

			want := []connection{
				{"connected to joystick", "Fake Co", "Fake Stick"},
				{"reconnected to joystick", "Dead Co", "Dead Thing"},
				{"reconnected to joystick", "Dead Co", "Dead Thing"},
				{"reconnected to joystick", "Other Co", "Some Keyboard"},
			}
			if got := r.log.connections(); !slices.Equal(got, want) {
				t.Errorf("the log shown without -v names these opened devices: %q, want %q, the last being the one the game reads", got, want)
			}
			if n := r.log.count(slog.LevelInfo, "reconnected to joystick"); n != 1 {
				t.Errorf("%d returns are in the log only with -v, want 1: the third of the device that cannot be read", n)
			}
		})
	}
}

// TestNewControllerLogsThePath starts a controller the way the game does,
// with the program's logger as it is without -v, and a fake device in place
// of the one the HID library would open. The log must say which path was
// opened along with what the device there calls itself.
func TestNewControllerLogsThePath(t *testing.T) {
	const path = "/dev/fake-joystick"

	device := newFakeDevice()
	var paths []string
	realOpenHID := openHID
	t.Cleanup(func() { openHID = realOpenHID })
	openHID = func(p string) (Device, error) {
		paths = append(paths, p)
		if len(paths) > 1 {
			return nil, errNoDevice
		}
		return device, nil
	}

	var log bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelWarn}))
	c, err := NewController(&config.Config{ControllerPath: path, Logger: logger}, nil, nil)
	if err != nil {
		t.Fatalf("NewController: %v", err)
	}

	// The log and the paths are read once the reader goroutine has ended.
	close(device.unplug)
	c.Stop()
	select {
	case <-c.finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the reader goroutine did not end")
	}

	if len(paths) == 0 || paths[0] != path {
		t.Errorf("the paths opened were %q, want the configured %q first", paths, path)
	}
	lines := strings.Split(strings.TrimRight(log.String(), "\n"), "\n")
	first := lines[0]
	for _, want := range []string{"level=WARN", `msg="connected to joystick"`, "path=" + path, `manufacturer="Fake Co"`, `product="Fake Stick"`} {
		if !strings.Contains(first, want) {
			t.Errorf("the first line of the log does not have %s: %s", want, first)
		}
	}
	if n := strings.Count(log.String(), "connected to joystick"); n != 1 {
		t.Errorf("one connection was logged %d times, want once:\n%s", n, log.String())
	}
}

// TestDeviceThatNeverReadsNorNamesItself is about a device that can be
// opened but fails on its first read, every time, and will not give its
// strings either. It is a new connection every interval. Without -v the log
// must have a connection, and the two complaints about its strings, only
// for the start and for a return whose loss was logged: the first loss and
// then one in openLogEvery.
func TestDeviceThatNeverReadsNorNamesItself(t *testing.T) {
	const rounds = openLogEvery + 5

	dead := newFakeDevice()
	dead.stringErr = errors.New("no string descriptor")
	close(dead.unplug)
	open := func() (Device, error) { return dead, nil }
	log := &logRecorder{}
	var lost atomic.Int32

	c, err := Start(slog.New(log), open, testRetry, nil, func() { lost.Add(1) })
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, "the device to be opened again and again", func() bool { return lost.Load() >= rounds })
	c.Stop()
	select {
	case <-c.finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the reader goroutine did not end")
	}

	// The start, and a return for every loss that was logged. Only if the
	// last loss of all was a logged one can the controller have been
	// stopped before the return that follows it.
	n := int(lost.Load())
	losses := log.count(slog.LevelWarn, "read failed")
	if want := 1 + n/openLogEvery; losses != want {
		t.Fatalf("%d failed connections logged %d warnings, want %d", n, losses, want)
	}
	named := len(log.connections())
	if named != losses+1 && (named != losses || n%openLogEvery != 0) {
		t.Errorf("%d connections are in the log shown without -v after %d failed ones, want the start and one for each of the %d logged losses", named, n, losses)
	}
	if n := log.count(slog.LevelWarn, "string"); n != 2*named {
		t.Errorf("%d warnings about the identity strings for %d logged connections, want two for each", n, named)
	}
	if all := log.count(slog.LevelWarn, ""); all != losses+3*named {
		t.Errorf("%d warnings in all, want only the %d losses and three for each of the %d logged connections", all, losses, named)
	}
}

// TestReadingAReportWithThePathDoesNotAllocate is the check of
// TestReadingAReportDoesNotAllocate with the logger that NewController
// makes for the game, which carries the path. The controller NewController
// returns has a reader goroutine of its own, so the reports are read by a
// second controller that is given its logger.
func TestReadingAReportWithThePathDoesNotAllocate(t *testing.T) {
	levels := []struct {
		name  string
		level slog.Level
	}{
		{"without -v", slog.LevelWarn},
		{"with -v", slog.LevelInfo},
	}
	for _, tc := range levels {
		t.Run(tc.name, func(t *testing.T) {
			realOpenHID := openHID
			t.Cleanup(func() { openHID = realOpenHID })
			// The first device has no report to give, and there is no second.
			opens := 0
			openHID = func(string) (Device, error) {
				if opens++; opens > 1 {
					return nil, errNoDevice
				}
				return &endlessDevice{}, nil
			}

			made, err := NewController(&config.Config{ControllerPath: "/dev/hidraw3", Logger: loggerAt(tc.level)}, nil, nil)
			if err != nil {
				t.Fatalf("NewController: %v", err)
			}
			made.Stop()
			select {
			case <-made.finished:
			case <-time.After(5 * time.Second):
				t.Fatal("the reader goroutine did not end")
			}

			handled := 0
			c := newController(made.logger, nil, testRetry, func(ControllerState) { handled++ }, nil)
			device := newEndlessDevice()
			decode := testing.AllocsPerRun(1000, func() {
				if _, ok, err := c.readState(device); err != nil || !ok {
					t.Fatalf("readState: ok=%v err=%v", ok, err)
				}
			})
			if decode != 0 {
				t.Errorf("decoding a report allocates %v times, want 0", decode)
			}

			const reports = 1000
			loop := testing.AllocsPerRun(10, func() {
				device.left = reports
				if _, err := c.readStates(device); err != errNoMoreReports {
					t.Fatalf("readStates: %v", err)
				}
			})
			if loop != 0 {
				t.Errorf("reading %d reports allocates %v times, want 0", reports, loop)
			}
			// AllocsPerRun calls the function once more than it is asked to.
			if want := 11 * reports; handled != want {
				t.Errorf("the handler was called %d times, want %d", handled, want)
			}
		})
	}
}
