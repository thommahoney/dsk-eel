package controller

import (
	"io"
	"log/slog"
)

// KeyboardHelp says which keys the keyboard reader understands.
const KeyboardHelp = "keyboard: arrow keys steer and the direction stays held, space centres the stick, " +
	"r g b y w are the buttons, q quits"

// Keyboard stands in for the joystick during development. It reads keys
// from a terminal and reports them to the same handler, as joystick states.
//
// A terminal only says that a key was pressed, never that it was let go,
// and repeats a key that is held down. So the keyboard cannot be a
// joystick exactly:
//
//   - An arrow key pushes the stick that way and it stays there, as if
//     held, until another arrow is pressed or space centres it. There are
//     no diagonals.
//   - A button key is a press followed at once by a release. Holding the
//     key down repeats the press for as long as the terminal repeats the
//     key, and two buttons cannot be down together.
type Keyboard struct {
	logger *slog.Logger
	handle func(ControllerState)
	quit   func()

	// Used only by the reader goroutine. direction is where the stick is
	// held. escape is how far into a terminal escape sequence the reader
	// is: an arrow key arrives as ESC [ A, or ESC O A, not as one byte.
	// A key that is not understood still has to be read to its end, or
	// its last letter would be taken for a key of its own.
	direction Direction
	escape    int

	finished chan struct{}
}

// How far into an escape sequence the reader is.
const (
	escapeNone    = iota
	escapeStarted // after ESC
	escapeOpened  // after ESC [ or ESC O
	escapeBody    // further into the sequence
	escapeLastOne // one more byte belongs to the sequence
)

// StartKeyboard starts a goroutine that reads keys from in, which is a
// terminal that hands over each key as it is pressed, or a test's stand-in
// for one. States go to handleFunc, on that goroutine and one at a time,
// like a joystick's. When q or ctrl-C is read the goroutine calls quitFunc
// and ends. It also ends when in fails or runs out, with a warning; the
// game then carries on with the stick where it was.
func StartKeyboard(logger *slog.Logger, in io.Reader, handleFunc func(ControllerState), quitFunc func()) *Keyboard {
	k := &Keyboard{
		logger:    logger,
		handle:    handleFunc,
		quit:      quitFunc,
		direction: Dir_Neutral,
		finished:  make(chan struct{}),
	}
	go k.run(in)
	return k
}

// run is the reader goroutine.
func (k *Keyboard) run(in io.Reader) {
	defer close(k.finished)

	buf := make([]byte, 64)
	for {
		n, err := in.Read(buf)
		for _, b := range buf[:n] {
			if k.key(b) {
				if k.quit != nil {
					k.quit()
				}
				return
			}
		}
		if err != nil {
			k.logger.Warn("the keyboard cannot be read any more", "error", err)
			return
		}
	}
}

// key takes the next byte from the terminal. It reports whether the byte
// asks to quit.
func (k *Keyboard) key(b byte) (quit bool) {
	switch k.escape {
	case escapeStarted:
		switch b {
		case '[', 'O':
			k.escape = escapeOpened
		case 0x1b:
			// Still at the start of a sequence.
		default:
			// Alt held with a key sends ESC and then the key. That is
			// not the key by itself, so it is dropped: alt-left must
			// not press the blue button because it arrives as ESC b.
			k.escape = escapeNone
		}
		return false
	case escapeLastOne:
		k.escape = escapeNone
		return false
	case escapeOpened, escapeBody:
		// The Linux console sends ESC [ [ A for F1, and so on: there
		// the letter after the second bracket ends the sequence.
		if k.escape == escapeOpened && b == '[' {
			k.escape = escapeLastOne
			return false
		}
		// Digits and semicolons come first when a modifier is held, as
		// in ESC [ 1 ; 5 A for ctrl-up. A letter or ~ ends the sequence.
		if b < 0x40 || b > 0x7e {
			k.escape = escapeBody
			return false
		}
		k.escape = escapeNone
		switch b {
		case 'A':
			k.steer(Dir_North)
		case 'B':
			k.steer(Dir_South)
		case 'C':
			k.steer(Dir_East)
		case 'D':
			k.steer(Dir_West)
		}
		return false
	}

	switch b {
	case 0x1b:
		k.escape = escapeStarted
	case ' ':
		k.steer(Dir_Neutral)
	case 'r', 'R':
		k.press(Btn_Red)
	case 'g', 'G':
		k.press(Btn_Green)
	case 'b', 'B':
		k.press(Btn_Blue)
	case 'y', 'Y':
		k.press(Btn_Yellow)
	case 'w', 'W':
		k.press(Btn_White)
	case 'q', 'Q', 0x03:
		// 0x03 is ctrl-C on a terminal that passes it on as a key.
		return true
	}
	return false
}

// steer holds the stick in a direction. Like the joystick, it reports only
// a change, so a key the terminal repeats is reported once.
func (k *Keyboard) steer(direction Direction) {
	if direction == k.direction {
		return
	}
	k.direction = direction
	k.report(Btn_None)
}

// press reports a button going down and coming up again.
func (k *Keyboard) press(button ButtonStatus) {
	k.report(button)
	k.report(Btn_None)
}

func (k *Keyboard) report(buttons ButtonStatus) {
	if k.handle != nil {
		k.handle(ControllerState{Direction: k.direction, ButtonStatus: buttons})
	}
}
