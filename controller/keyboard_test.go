package controller

import (
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

// typist is a keyboard under test, reading from a pipe that the test types
// into.
type typist struct {
	t      *testing.T
	k      *Keyboard
	in     *io.PipeWriter
	log    *logRecorder
	states chan ControllerState
	quits  atomic.Int32
}

func startTypist(t *testing.T) *typist {
	t.Helper()
	r, w := io.Pipe()
	ty := &typist{t: t, in: w, log: &logRecorder{}, states: make(chan ControllerState, 64)}
	ty.k = StartKeyboard(slog.New(ty.log), r,
		func(s ControllerState) { ty.states <- s },
		func() { ty.quits.Add(1) })
	t.Cleanup(func() {
		_ = w.Close()
		ty.waitFinished()
	})
	return ty
}

// keys types the keys. A write to the pipe returns when the reader has
// taken it, which is before the reader has acted on it.
func (ty *typist) keys(s string) {
	ty.t.Helper()
	if _, err := ty.in.Write([]byte(s)); err != nil {
		ty.t.Fatalf("typing %q: %v", s, err)
	}
}

// want fails unless the next state reported is the given one.
func (ty *typist) want(direction Direction, buttons ButtonStatus) {
	ty.t.Helper()
	select {
	case got := <-ty.states:
		if got.Direction != direction || got.ButtonStatus != buttons {
			ty.t.Fatalf("handler got %v/%v, want %v/%v", got.Direction, got.ButtonStatus, direction, buttons)
		}
	case <-time.After(5 * time.Second):
		ty.t.Fatalf("handler was not called; want %v/%v", direction, buttons)
	}
}

// waitFinished waits for the reader goroutine to end.
func (ty *typist) waitFinished() {
	ty.t.Helper()
	select {
	case <-ty.k.finished:
	case <-time.After(5 * time.Second):
		ty.t.Fatal("the keyboard reader did not end")
	}
}

// wantNothingMore ends the input and fails if a state was reported that no
// want has taken.
func (ty *typist) wantNothingMore() {
	ty.t.Helper()
	_ = ty.in.Close()
	ty.waitFinished()
	select {
	case got := <-ty.states:
		ty.t.Fatalf("handler got an unexpected %v/%v", got.Direction, got.ButtonStatus)
	default:
	}
}

// The arrow keys as a terminal sends them.
const (
	keyUp    = "\x1b[A"
	keyDown  = "\x1b[B"
	keyRight = "\x1b[C"
	keyLeft  = "\x1b[D"
)

// TestKeyboardArrows: an arrow pushes the stick that way and it stays
// there; space centres it. A direction is reported when it changes, so an
// arrow the terminal repeats while it is held down is reported once.
func TestKeyboardArrows(t *testing.T) {
	ty := startTypist(t)

	ty.keys(keyUp)
	ty.want(Dir_North, Btn_None)
	ty.keys(keyDown)
	ty.want(Dir_South, Btn_None)
	ty.keys(keyRight)
	ty.want(Dir_East, Btn_None)
	ty.keys(keyLeft)
	ty.want(Dir_West, Btn_None)
	ty.keys(" ")
	ty.want(Dir_Neutral, Btn_None)

	ty.keys(keyUp + keyUp + keyUp + keyUp + keyLeft + keyLeft + " " + " ")
	ty.want(Dir_North, Btn_None)
	ty.want(Dir_West, Btn_None)
	ty.want(Dir_Neutral, Btn_None)
	ty.wantNothingMore()
}

// TestKeyboardStartsCentred: until an arrow is pressed the stick is in the
// middle, so space reports nothing and a button is reported with a neutral
// stick, not with the zero Direction, which is south-east.
func TestKeyboardStartsCentred(t *testing.T) {
	ty := startTypist(t)

	ty.keys(" r")
	ty.want(Dir_Neutral, Btn_Red)
	ty.want(Dir_Neutral, Btn_None)
	ty.wantNothingMore()
}

// TestKeyboardButtons: a button key is a press followed by a release, so
// that pressing the same key again is a second press. The stick stays where
// the arrows left it.
func TestKeyboardButtons(t *testing.T) {
	ty := startTypist(t)

	ty.keys(keyUp)
	ty.want(Dir_North, Btn_None)

	for _, c := range []struct {
		keys   string
		button ButtonStatus
	}{
		{"r", Btn_Red}, {"g", Btn_Green}, {"b", Btn_Blue}, {"y", Btn_Yellow}, {"w", Btn_White},
		{"R", Btn_Red}, {"G", Btn_Green}, {"B", Btn_Blue}, {"Y", Btn_Yellow}, {"W", Btn_White},
	} {
		ty.keys(c.keys)
		ty.want(Dir_North, c.button)
		ty.want(Dir_North, Btn_None)
	}

	ty.keys("rr")
	ty.want(Dir_North, Btn_Red)
	ty.want(Dir_North, Btn_None)
	ty.want(Dir_North, Btn_Red)
	ty.want(Dir_North, Btn_None)

	ty.keys(keyLeft + "w")
	ty.want(Dir_West, Btn_None)
	ty.want(Dir_West, Btn_White)
	ty.want(Dir_West, Btn_None)
	ty.wantNothingMore()
}

// TestKeyboardEscapeSequences covers the other ways a terminal can send an
// arrow: ESC O A in application mode, with numbers when a modifier is held,
// and in pieces when the bytes do not arrive in one read.
func TestKeyboardEscapeSequences(t *testing.T) {
	ty := startTypist(t)

	ty.keys("\x1bOA")
	ty.want(Dir_North, Btn_None)
	ty.keys("\x1b[1;5C") // ctrl-right
	ty.want(Dir_East, Btn_None)
	ty.keys("\x1b[1;2B") // shift-down
	ty.want(Dir_South, Btn_None)

	for _, piece := range []string{"\x1b", "[", "D"} {
		ty.keys(piece)
	}
	ty.want(Dir_West, Btn_None)
	for _, piece := range []string{"\x1b", "[", "1", ";", "5", "A"} {
		ty.keys(piece)
	}
	ty.want(Dir_North, Btn_None)
	ty.wantNothingMore()
}

// TestKeyboardIgnoresOtherKeys types keys that mean nothing here, among
// them ones that arrive as escape sequences, and then one that does mean
// something: the reader must have reported nothing before it and must not
// be left in the middle of a sequence. The letters that end an arrow's
// sequence are not arrows by themselves.
func TestKeyboardIgnoresOtherKeys(t *testing.T) {
	ty := startTypist(t)

	for _, ignored := range []string{
		"x", "1", "\r", "\n", "\t", "A", "C", "D", "[", "O", "~",
		"\x1b[3~",    // delete
		"\x1b[5~",    // page up
		"\x1bOP",     // F1
		"\x1b[15;2~", // shift-F5
		"\x1b[H",     // home
		"\x04",       // ctrl-D
	} {
		ty.keys(ignored)
		ty.keys("g")
		ty.want(Dir_Neutral, Btn_Green)
		ty.want(Dir_Neutral, Btn_None)
	}

	// Alt held with a key arrives as ESC and then the key, and is not that
	// key: alt-left is ESC b in some terminals and must not press blue,
	// and alt-q must not quit. The reader is back to normal afterwards: a
	// bracket or an O typed next does not start a sequence.
	for _, ignored := range []string{"\x1bb", "\x1bq", "\x1by", "\x1b\x03"} {
		ty.keys(ignored)
		ty.keys("[g")
		ty.want(Dir_Neutral, Btn_Green)
		ty.want(Dir_Neutral, Btn_None)
		ty.keys("OAr")
		ty.want(Dir_Neutral, Btn_Red)
		ty.want(Dir_Neutral, Btn_None)
	}

	// The Linux console sends ESC [ [ A to ESC [ [ E for F1 to F5. The
	// letter belongs to the sequence: F2 must not press blue.
	for _, ignored := range []string{"\x1b[[A", "\x1b[[B", "\x1b[[C", "\x1b[[D", "\x1b[[E"} {
		ty.keys(ignored)
		ty.keys("g")
		ty.want(Dir_Neutral, Btn_Green)
		ty.want(Dir_Neutral, Btn_None)
	}

	// ESC twice and then an arrow's sequence is still the arrow.
	ty.keys("\x1b\x1b[A")
	ty.want(Dir_North, Btn_None)
	ty.wantNothingMore()

	if n := ty.quits.Load(); n != 0 {
		t.Errorf("quit was called %d times, want never", n)
	}
}

// TestKeyboardQuit: q, Q and ctrl-C call the quit function once and end the
// reader, and nothing typed after them is acted on.
func TestKeyboardQuit(t *testing.T) {
	for _, quit := range []string{"q", "Q", "\x03"} {
		ty := startTypist(t)

		ty.keys(keyUp)
		ty.want(Dir_North, Btn_None)
		// From a goroutine: the reader may stop before it has taken
		// everything, and the write then waits for the pipe to close.
		go func() { _, _ = ty.in.Write([]byte(quit + "r" + keyDown)) }()
		ty.waitFinished()

		if n := ty.quits.Load(); n != 1 {
			t.Errorf("%q: quit was called %d times, want once", quit, n)
		}
		select {
		case got := <-ty.states:
			t.Errorf("%q: handler got %v/%v for keys typed after it", quit, got.Direction, got.ButtonStatus)
		default:
		}
	}
}

// TestKeyboardInputEnds: when the terminal goes away the reader says so and
// stops. That is not a request to quit.
func TestKeyboardInputEnds(t *testing.T) {
	ty := startTypist(t)

	ty.keys(keyRight)
	ty.want(Dir_East, Btn_None)
	_ = ty.in.Close()
	ty.waitFinished()

	if n := ty.log.count(slog.LevelWarn, "keyboard"); n != 1 {
		t.Errorf("%d warnings about the keyboard, want 1", n)
	}
	if n := ty.quits.Load(); n != 0 {
		t.Errorf("quit was called %d times, want never", n)
	}
}

// TestKeyboardCallbacksAreOptional runs a keyboard with no handler and no
// quit function through a direction, a button and a quit.
func TestKeyboardCallbacksAreOptional(t *testing.T) {
	r, w := io.Pipe()
	k := StartKeyboard(slog.New(&logRecorder{}), r, nil, nil)
	if _, err := w.Write([]byte(keyUp + "r" + "q")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-k.finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the keyboard reader did not end")
	}
	_ = w.Close()
}
