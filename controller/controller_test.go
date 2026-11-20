package controller

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The real joystick is not needed for these tests: the controller is given
// a fake device, and an open function that the test scripts.

const testRetry = 2 * time.Millisecond

var errUnplugged = errors.New("fake joystick unplugged")

// fakeDevice is a joystick whose reports come from the test.
type fakeDevice struct {
	reports chan []byte
	unplug  chan struct{}
	closed  atomic.Bool

	manufacturer, product string
	stringErr             error
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{
		reports:      make(chan []byte),
		unplug:       make(chan struct{}),
		manufacturer: " Fake Co ",
		product:      "Fake Stick\n",
	}
}

// Read blocks, as the real device does, until the test sends a report or
// pulls the plug. A report shorter than p leaves the rest of p alone.
func (f *fakeDevice) Read(p []byte) (int, error) {
	select {
	case r := <-f.reports:
		return copy(p, r), nil
	case <-f.unplug:
		return -1, errUnplugged
	}
}

func (f *fakeDevice) Close() error {
	f.closed.Store(true)
	return nil
}

func (f *fakeDevice) Manufacturer() (string, error) { return f.manufacturer, f.stringErr }
func (f *fakeDevice) Product() (string, error)      { return f.product, f.stringErr }

// send hands the controller one report. It returns when the controller has
// taken it, which is before the handler has been called for it.
func (f *fakeDevice) send(t *testing.T, report []byte) {
	t.Helper()
	select {
	case f.reports <- report:
	case <-time.After(5 * time.Second):
		t.Fatal("the controller is not reading the device")
	}
}

// report builds the 8-byte report the joystick sends for a state.
func report(dir Direction, buttons ButtonStatus) []byte {
	r := make([]byte, 8)
	r[0] = byte(dir>>2) << 6
	r[1] = byte(dir&0b11) << 6
	r[5] = byte(buttons&0b1111) << 4
	r[6] = byte(buttons >> 4)
	return r
}

// opener is an OpenFunc that fails until the test gives it a device to hand
// out, and counts how often it is called.
type opener struct {
	mu       sync.Mutex
	next     *fakeDevice
	attempts int
}

var errNoDevice = errors.New("fake joystick is not plugged in")

func (o *opener) open() (Device, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.attempts++
	if o.next == nil {
		return nil, errNoDevice
	}
	d := o.next
	o.next = nil
	return d, nil
}

// plugIn makes the next open succeed with a new device.
func (o *opener) plugIn() *fakeDevice {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.next = newFakeDevice()
	return o.next
}

func (o *opener) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.attempts
}

// logRecorder is a slog handler that keeps what was logged.
type logRecorder struct {
	mu      sync.Mutex
	records []slog.Record
}

func (l *logRecorder) Enabled(context.Context, slog.Level) bool { return true }
func (l *logRecorder) WithAttrs([]slog.Attr) slog.Handler       { return l }
func (l *logRecorder) WithGroup(string) slog.Handler            { return l }
func (l *logRecorder) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r)
	return nil
}

// count returns how many records of the level have msg in their message.
func (l *logRecorder) count(level slog.Level, msg string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, r := range l.records {
		if r.Level == level && strings.Contains(r.Message, msg) {
			n++
		}
	}
	return n
}

// rig is a controller under test with everything it talks to.
type rig struct {
	t      *testing.T
	c      *Controller
	opener *opener
	log    *logRecorder
	states chan ControllerState
	lost   atomic.Int32
}

// start starts a controller. device is what the first open returns; nil
// means the joystick is not there at startup.
func start(t *testing.T, device *fakeDevice) *rig {
	t.Helper()
	r := &rig{
		t:      t,
		opener: &opener{next: device},
		log:    &logRecorder{},
		states: make(chan ControllerState, 64),
	}
	r.c = Start(slog.New(r.log), r.opener.open, testRetry,
		func(s ControllerState) { r.states <- s },
		func() { r.lost.Add(1) })
	t.Cleanup(r.stop)
	return r
}

// stop ends the reader goroutine and waits for it. The device it may be
// reading has to have been unplugged by the test.
func (r *rig) stop() {
	r.t.Helper()
	r.c.Stop()
	select {
	case <-r.c.finished:
	case <-time.After(5 * time.Second):
		r.t.Error("the reader goroutine did not end")
	}
}

// wantState fails unless the next state the handler was given is want.
func (r *rig) wantState(want ControllerState) {
	r.t.Helper()
	select {
	case got := <-r.states:
		if got != want {
			r.t.Fatalf("handler got %v/%v, want %v/%v", got.Direction, got.ButtonStatus, want.Direction, want.ButtonStatus)
		}
	case <-time.After(5 * time.Second):
		r.t.Fatalf("handler was not called; want %v/%v", want.Direction, want.ButtonStatus)
	}
}

// wantNoMoreStates stops the controller and fails if the handler was given
// anything that has not been taken with wantState.
func (r *rig) wantNoMoreStates() {
	r.t.Helper()
	r.stop()
	select {
	case got := <-r.states:
		r.t.Fatalf("handler got an unexpected %v/%v", got.Direction, got.ButtonStatus)
	default:
	}
}

// waitFor polls until cond holds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestStatesReachHandler(t *testing.T) {
	device := newFakeDevice()
	r := start(t, device)

	if n := r.log.count(slog.LevelInfo, "connected to joystick"); n != 1 {
		t.Errorf("%d connection messages logged, want 1", n)
	}

	// Only changes are reported: not the untouched stick a new connection
	// starts from, and not a repeat of the same state.
	device.send(t, report(Dir_Neutral, Btn_None))
	device.send(t, report(Dir_North, Btn_None))
	r.wantState(ControllerState{Dir_North, Btn_None})
	device.send(t, report(Dir_North, Btn_None))
	device.send(t, report(Dir_North, Btn_Red))
	r.wantState(ControllerState{Dir_North, Btn_Red})
	device.send(t, report(Dir_SouthWest, Btn_Red|Btn_White))
	r.wantState(ControllerState{Dir_SouthWest, Btn_Red | Btn_White})
	device.send(t, report(Dir_Neutral, Btn_None))
	r.wantState(ControllerState{Dir_Neutral, Btn_None})

	for _, dir := range []Direction{Dir_North, Dir_NorthEast, Dir_East, Dir_SouthEast, Dir_South, Dir_SouthWest, Dir_West, Dir_NorthWest} {
		for _, button := range []ButtonStatus{Btn_Red, Btn_Green, Btn_Blue, Btn_Yellow, Btn_White} {
			device.send(t, report(dir, button))
			r.wantState(ControllerState{dir, button})
		}
	}

	close(device.unplug)
	r.wantNoMoreStates()
	if !device.closed.Load() {
		t.Error("the device was not closed")
	}
}

// TestReadErrorReconnects pulls the plug in the middle of a session. The
// controller must say so once, close the device, keep trying to open it and
// carry on when it is back.
func TestReadErrorReconnects(t *testing.T) {
	first := newFakeDevice()
	r := start(t, first)

	first.send(t, report(Dir_North, Btn_None))
	r.wantState(ControllerState{Dir_North, Btn_None})

	close(first.unplug)
	waitFor(t, "the lost notification", func() bool { return r.lost.Load() == 1 })
	if !first.closed.Load() {
		t.Error("the device that failed was not closed before the game was told")
	}
	if n := r.log.count(slog.LevelWarn, "read failed"); n != 1 {
		t.Errorf("the read error was logged %d times, want once", n)
	}

	// Stay unplugged for a few attempts, then come back.
	before := r.opener.count()
	waitFor(t, "three more attempts to open", func() bool { return r.opener.count() >= before+3 })
	second := r.opener.plugIn()
	waitFor(t, "the reconnect to be logged", func() bool {
		return r.log.count(slog.LevelInfo, "reconnected to joystick") == 1
	})

	// The stick is still held north. To the controller that is a change,
	// because a lost joystick counts as let go.
	second.send(t, report(Dir_North, Btn_None))
	r.wantState(ControllerState{Dir_North, Btn_None})

	if n := r.lost.Load(); n != 1 {
		t.Errorf("lost was called %d times, want once", n)
	}
	close(second.unplug)
	r.wantNoMoreStates()
}

// TestAbsentJoystickDoesNotSpin leaves the joystick unplugged for more than a
// hundred attempts. The attempts must be an interval apart, the log must
// get the first failure and then only one line in openLogEvery, and no
// goroutines may pile up.
func TestAbsentJoystickDoesNotSpin(t *testing.T) {
	const attempts = 2*openLogEvery + 5

	begin := time.Now()
	r := start(t, nil)
	if r.opener.count() != 1 || r.lost.Load() != 1 {
		t.Fatalf("after Start: %d attempts and %d lost notifications, want 1 and 1", r.opener.count(), r.lost.Load())
	}

	waitFor(t, "20 attempts", func() bool { return r.opener.count() >= 20 })
	goroutines := runtime.NumGoroutine()
	waitFor(t, "the remaining attempts", func() bool { return r.opener.count() >= attempts })
	if now := runtime.NumGoroutine(); now > goroutines {
		t.Errorf("%d goroutines after %d attempts, %d after 20", now, attempts, goroutines)
	}

	r.stop()
	elapsed := time.Since(begin)
	n := r.opener.count()

	// The first attempt is immediate and every later one waits an interval.
	if least := time.Duration(n-1) * testRetry; elapsed < least {
		t.Errorf("%d attempts in %v: they cannot have been %v apart", n, elapsed, testRetry)
	}

	wantWarnings := 1 + n/openLogEvery
	if got := r.log.count(slog.LevelWarn, "cannot open the joystick"); got != wantWarnings {
		t.Errorf("%d failed attempts logged %d warnings, want %d", n, got, wantWarnings)
	}
	if got := r.lost.Load(); got != 1 {
		t.Errorf("lost was called %d times, want once", got)
	}
}

// TestDeviceThatNeverReads has a device that can be opened but fails on the
// first read, every time. The controller must go round once per interval,
// and log the read error the first time and then only one time in
// openLogEvery.
func TestDeviceThatNeverReads(t *testing.T) {
	const rounds = openLogEvery + 5

	dead := newFakeDevice()
	close(dead.unplug)
	var opens atomic.Int32
	open := func() (Device, error) {
		opens.Add(1)
		return dead, nil
	}
	log := &logRecorder{}
	var lost atomic.Int32

	begin := time.Now()
	c := Start(slog.New(log), open, testRetry, nil, func() { lost.Add(1) })
	waitFor(t, "the device to be opened again and again", func() bool { return lost.Load() >= rounds })
	c.Stop()
	select {
	case <-c.finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the reader goroutine did not end")
	}
	elapsed := time.Since(begin)

	n := int(lost.Load())
	if least := time.Duration(n-1) * testRetry; elapsed < least {
		t.Errorf("%d rounds in %v: they cannot have been %v apart", n, elapsed, testRetry)
	}
	if got, want := log.count(slog.LevelWarn, "read failed"), 1+n/openLogEvery; got != want {
		t.Errorf("%d failed connections logged %d warnings, want %d", n, got, want)
	}
}

// TestStartWithoutJoystick starts with nothing plugged in. Start must
// succeed, having said the joystick is lost, and pick the joystick up when
// it appears.
func TestStartWithoutJoystick(t *testing.T) {
	r := start(t, nil)
	if r.c == nil {
		t.Fatal("Start returned no controller")
	}
	if r.lost.Load() != 1 {
		t.Fatalf("lost was called %d times before Start returned, want once", r.lost.Load())
	}

	device := r.opener.plugIn()
	device.send(t, report(Dir_East, Btn_Green))
	r.wantState(ControllerState{Dir_East, Btn_Green})

	// It was never connected before, so this is not a reconnect.
	if n := r.log.count(slog.LevelInfo, "connected to joystick"); n != 1 {
		t.Errorf("%d connection messages logged, want 1", n)
	}
	if n := r.log.count(slog.LevelInfo, "reconnected"); n != 0 {
		t.Errorf("a first connection was logged as a reconnect")
	}
	if r.lost.Load() != 1 {
		t.Errorf("lost was called %d times, want once", r.lost.Load())
	}
	close(device.unplug)
	r.wantNoMoreStates()
}

// TestShortReportIgnored sends reports that stop before the last byte the
// decoders use. Decoding one would combine what it has with bytes left in
// the buffer by the report before it. The lengths are written out here
// instead of being taken from minReportLen, so a wrong constant cannot
// pass: the decoders read bytes 0, 1, 5 and 6, so 6 bytes are one too few
// and 7 are enough.
func TestShortReportIgnored(t *testing.T) {
	device := newFakeDevice()
	r := start(t, device)

	// White is the button in byte 6, the last byte the decoders use.
	device.send(t, report(Dir_North, Btn_Red|Btn_White))
	r.wantState(ControllerState{Dir_North, Btn_Red | Btn_White})

	// Decoded, the 3-byte report would be south with red and white still
	// down, and the 6-byte one south with white still down.
	device.send(t, report(Dir_South, Btn_None)[:3])
	device.send(t, report(Dir_South, Btn_None)[:6])
	device.send(t, nil)

	device.send(t, report(Dir_West, Btn_Blue)[:7])
	r.wantState(ControllerState{Dir_West, Btn_Blue})

	// Short reports are worth one warning for the connection, not one each.
	if n := r.log.count(slog.LevelWarn, "too short"); n != 1 {
		t.Errorf("3 short reports logged %d warnings, want 1", n)
	}

	close(device.unplug)
	r.wantNoMoreStates()
}

// TestShortReportWarningPerConnection checks that the warning about short
// reports is given again for a new connection, and not for a connection
// that sends none.
func TestShortReportWarningPerConnection(t *testing.T) {
	first := newFakeDevice()
	r := start(t, first)
	short := report(Dir_South, Btn_None)[:6]

	first.send(t, report(Dir_North, Btn_None))
	r.wantState(ControllerState{Dir_North, Btn_None})
	if n := r.log.count(slog.LevelWarn, "too short"); n != 0 {
		t.Fatalf("%d warnings about short reports before any was sent", n)
	}
	first.send(t, short)
	first.send(t, short)
	close(first.unplug)
	waitFor(t, "the lost notification", func() bool { return r.lost.Load() == 1 })

	second := r.opener.plugIn()
	second.send(t, short)
	second.send(t, short)
	second.send(t, report(Dir_East, Btn_None))
	r.wantState(ControllerState{Dir_East, Btn_None})
	if n := r.log.count(slog.LevelWarn, "too short"); n != 2 {
		t.Errorf("short reports on two connections logged %d warnings, want 2", n)
	}

	close(second.unplug)
	r.wantNoMoreStates()
}

// TestIdentityStringErrors has a joystick that will not give its
// manufacturer and product strings. That is worth a warning and no more.
func TestIdentityStringErrors(t *testing.T) {
	device := newFakeDevice()
	device.stringErr = errors.New("no string descriptor")
	r := start(t, device)

	if n := r.log.count(slog.LevelWarn, "string"); n != 2 {
		t.Errorf("%d warnings about the identity strings, want 2", n)
	}
	device.send(t, report(Dir_South, Btn_Yellow))
	r.wantState(ControllerState{Dir_South, Btn_Yellow})

	if r.lost.Load() != 0 {
		t.Errorf("lost was called %d times, want never", r.lost.Load())
	}
	close(device.unplug)
	r.wantNoMoreStates()
}

// TestCallbacksAreOptional runs a controller with no handler and no lost
// callback through a connection, a state and a disconnection.
func TestCallbacksAreOptional(t *testing.T) {
	device := newFakeDevice()
	o := &opener{next: device}
	c := Start(slog.New(&logRecorder{}), o.open, testRetry, nil, nil)

	device.send(t, report(Dir_North, Btn_Red))
	close(device.unplug)
	waitFor(t, "an attempt to reopen", func() bool { return o.count() >= 2 })

	c.Stop()
	select {
	case <-c.finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the reader goroutine did not end")
	}
}
