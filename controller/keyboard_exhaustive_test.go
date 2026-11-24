package controller

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The tests in this file do not pick examples. The key reader remembers two
// things from one byte to the next, how far into an escape sequence it is
// and where the stick is held, so there are 5 x 5 x 256 = 6,400 different
// steps it can take. The first two tests take every one of them, and the
// third types an arrow at each of the 25 things the reader can remember.
// What they establish therefore holds for input of any length. The last
// test types every short string of bytes at a fresh reader instead, and so
// does not lean on those two things being all there is to remember.

// Everything the reader can remember.
var (
	escapeStates   = []int{escapeNone, escapeStarted, escapeOpened, escapeBody, escapeLastOne}
	heldDirections = []Direction{Dir_Neutral, Dir_North, Dir_South, Dir_East, Dir_West}
)

func escapeName(escape int) string {
	switch escape {
	case escapeNone:
		return "outside a sequence"
	case escapeStarted:
		return "after ESC"
	case escapeOpened:
		return "after ESC [ or ESC O"
	case escapeBody:
		return "further into a sequence"
	case escapeLastOne:
		return "after ESC [ ["
	}
	return fmt.Sprintf("in the unknown escape state %d", escape)
}

// The keys that mean something, written out again here and not taken from
// the reader.
var (
	buttonKeys = map[byte]ButtonStatus{
		'r': Btn_Red, 'g': Btn_Green, 'b': Btn_Blue, 'y': Btn_Yellow, 'w': Btn_White,
		'R': Btn_Red, 'G': Btn_Green, 'B': Btn_Blue, 'Y': Btn_Yellow, 'W': Btn_White,
	}
	arrowLetters = map[byte]Direction{'A': Dir_North, 'B': Dir_South, 'C': Dir_East, 'D': Dir_West}
)

func isQuitKey(b byte) bool { return b == 'q' || b == 'Q' || b == 0x03 }

// keyStep is one byte arriving at a reader that remembers the given things.
type keyStep struct {
	escape    int
	direction Direction
	b         byte
}

func (s keyStep) String() string {
	return fmt.Sprintf("byte %q %s with the stick held %v", s.b, escapeName(s.escape), s.direction)
}

// keyOutcome is what the step left behind and what it reported.
type keyOutcome struct {
	escape    int
	direction Direction
	reports   []ControllerState
	quit      bool
}

func (o keyOutcome) String() string {
	reports := "nothing"
	if len(o.reports) > 0 {
		var parts []string
		for _, r := range o.reports {
			parts = append(parts, fmt.Sprintf("%v/%v", r.Direction, r.ButtonStatus))
		}
		reports = strings.Join(parts, ", ")
	}
	quit := ""
	if o.quit {
		quit = ", asks to quit"
	}
	return fmt.Sprintf("%s with the stick held %v, reports %s%s", escapeName(o.escape), o.direction, reports, quit)
}

func (o keyOutcome) equal(other keyOutcome) bool {
	return o.escape == other.escape && o.direction == other.direction &&
		o.quit == other.quit && slices.Equal(o.reports, other.reports)
}

// take gives the real reader the step.
func (s keyStep) take() keyOutcome {
	var out keyOutcome
	k := Keyboard{escape: s.escape, direction: s.direction}
	k.handle = func(state ControllerState) { out.reports = append(out.reports, state) }
	out.quit = k.key(s.b)
	out.escape, out.direction = k.escape, k.direction
	return out
}

// everyKeyStep calls check with each of the 6,400 steps and what the reader
// made of it. check returns what is wrong, or "". The first few failures
// are printed and the rest counted.
func everyKeyStep(t *testing.T, check func(s keyStep, got keyOutcome) string) {
	t.Helper()
	steps, failures := 0, 0
	for _, escape := range escapeStates {
		for _, direction := range heldDirections {
			for b := 0; b < 256; b++ {
				s := keyStep{escape: escape, direction: direction, b: byte(b)}
				got := s.take()
				steps++
				if wrong := check(s, got); wrong != "" {
					failures++
					if failures <= 10 {
						t.Errorf("%v: %s\n\tthe reader is left %v", s, wrong, got)
					}
				}
			}
		}
	}
	if failures > 0 {
		t.Errorf("%d of %d steps are wrong", failures, steps)
	}
}

// TestKeyboardEveryStepKeepsTheRules holds every step of the reader to the
// few rules that must never be broken, whatever else a key does:
//
//   - the reader stays within the escape states and stick directions that
//     the other tests in this file cover;
//   - it asks to quit for q, Q and ctrl-C outside an escape sequence, and
//     for nothing else;
//   - a byte inside an escape sequence never presses a button and never
//     quits: it belongs to another key;
//   - a button that goes down comes up again in the same step, and what is
//     reported last is where the stick is and no button;
//   - the stick moves only for space outside a sequence, which centres it,
//     and for the A, B, C or D that ends an arrow's sequence;
//   - ESC starts a sequence from everywhere and does nothing else.
func TestKeyboardEveryStepKeepsTheRules(t *testing.T) {
	everyKeyStep(t, func(s keyStep, got keyOutcome) string {
		inside := s.escape != escapeNone

		if !slices.Contains(escapeStates, got.escape) {
			return "left the reader in an escape state that is not one of the five"
		}
		if !slices.Contains(heldDirections, got.direction) {
			return "left the stick in a direction the keyboard does not have"
		}

		if want := !inside && isQuitKey(s.b); got.quit != want {
			if want {
				return "want it to ask to quit"
			}
			return "want it not to ask to quit: only q, Q and ctrl-C outside a sequence do"
		}

		pressed := false
		for i, r := range got.reports {
			if r.ButtonStatus == Btn_None {
				continue
			}
			pressed = true
			if inside {
				return "want no button pressed by a byte inside an escape sequence"
			}
			if i+1 >= len(got.reports) || got.reports[i+1].ButtonStatus != Btn_None {
				return "want a button that is pressed to be released straight after"
			}
		}
		if n := len(got.reports); n > 0 {
			if last := got.reports[n-1]; last.Direction != got.direction || last.ButtonStatus != Btn_None {
				return "want the last report to say where the stick is held, with no button down"
			}
		}

		arrow, isArrow := arrowLetters[s.b]
		switch {
		case !inside && s.b == ' ':
			if got.direction != Dir_Neutral {
				return "want space to centre the stick"
			}
		case isArrow && (s.escape == escapeOpened || s.escape == escapeBody):
			if got.direction != arrow {
				return fmt.Sprintf("want the stick held %v", arrow)
			}
		default:
			if got.direction != s.direction {
				return "want the stick left where it was"
			}
		}
		if moved := got.direction != s.direction; moved != (len(got.reports) > 0) && !pressed {
			return "want a report when the stick moves and none when it does not"
		}

		if s.b == 0x1b && (got.escape != escapeStarted || len(got.reports) > 0) {
			return "want ESC to start a new sequence and report nothing"
		}
		return ""
	})
}

// keyRules says what every step does: the reader's whole behaviour, written
// by kind of key and not, as the reader has it, by escape state. Each step
// must fall under exactly one rule and come out as that rule says.
var keyRules = []struct {
	name    string
	applies func(s keyStep) bool
	want    func(s keyStep) keyOutcome
}{
	{
		name:    "ESC starts a sequence, also inside one that has not ended",
		applies: func(s keyStep) bool { return s.b == 0x1b },
		want:    func(s keyStep) keyOutcome { return s.leaves(escapeStarted) },
	},
	{
		name:    "outside a sequence q, Q and ctrl-C quit",
		applies: func(s keyStep) bool { return s.escape == escapeNone && isQuitKey(s.b) },
		want: func(s keyStep) keyOutcome {
			out := s.leaves(escapeNone)
			out.quit = true
			return out
		},
	},
	{
		name: "outside a sequence a button key is a press and a release",
		applies: func(s keyStep) bool {
			_, isButton := buttonKeys[s.b]
			return s.escape == escapeNone && isButton
		},
		want: func(s keyStep) keyOutcome {
			out := s.leaves(escapeNone)
			out.reports = []ControllerState{{s.direction, buttonKeys[s.b]}, {s.direction, Btn_None}}
			return out
		},
	},
	{
		name:    "outside a sequence space centres the stick",
		applies: func(s keyStep) bool { return s.escape == escapeNone && s.b == ' ' },
		want:    func(s keyStep) keyOutcome { return s.steers(Dir_Neutral) },
	},
	{
		name: "outside a sequence no other key does anything",
		applies: func(s keyStep) bool {
			_, isButton := buttonKeys[s.b]
			return s.escape == escapeNone && s.b != 0x1b && s.b != ' ' && !isButton && !isQuitKey(s.b)
		},
		want: func(s keyStep) keyOutcome { return s.leaves(escapeNone) },
	},
	{
		name:    "[ or O after ESC opens a sequence",
		applies: func(s keyStep) bool { return s.escape == escapeStarted && (s.b == '[' || s.b == 'O') },
		want:    func(s keyStep) keyOutcome { return s.leaves(escapeOpened) },
	},
	{
		name: "any other key after ESC is that key with alt held, and is dropped",
		applies: func(s keyStep) bool {
			return s.escape == escapeStarted && s.b != 0x1b && s.b != '[' && s.b != 'O'
		},
		want: func(s keyStep) keyOutcome { return s.leaves(escapeNone) },
	},
	{
		name:    "a second [ straight after ESC [ or ESC O takes one more byte with it (the linux console's F1 to F5)",
		applies: func(s keyStep) bool { return s.escape == escapeOpened && s.b == '[' },
		want:    func(s keyStep) keyOutcome { return s.leaves(escapeLastOne) },
	},
	{
		name:    "the byte after ESC [ [ ends the sequence and does nothing",
		applies: func(s keyStep) bool { return s.escape == escapeLastOne && s.b != 0x1b },
		want:    func(s keyStep) keyOutcome { return s.leaves(escapeNone) },
	},
	{
		name: "A, B, C and D end an open sequence and steer",
		applies: func(s keyStep) bool {
			_, isArrow := arrowLetters[s.b]
			return s.open() && isArrow
		},
		want: func(s keyStep) keyOutcome { return s.steers(arrowLetters[s.b]) },
	},
	{
		name: "any other byte from @ to ~, the letters among them, ends an open sequence and does nothing",
		applies: func(s keyStep) bool {
			_, isArrow := arrowLetters[s.b]
			secondBracket := s.escape == escapeOpened && s.b == '['
			return s.open() && s.b >= '@' && s.b <= '~' && !isArrow && !secondBracket
		},
		want: func(s keyStep) keyOutcome { return s.leaves(escapeNone) },
	},
	{
		name: "digits, semicolons and every remaining byte keep an open sequence open",
		applies: func(s keyStep) bool {
			return s.open() && s.b != 0x1b && (s.b < '@' || s.b > '~')
		},
		want: func(s keyStep) keyOutcome { return s.leaves(escapeBody) },
	},
}

// open says whether the step comes after ESC [ or ESC O, with the sequence
// not yet ended.
func (s keyStep) open() bool { return s.escape == escapeOpened || s.escape == escapeBody }

// leaves is the outcome of a step that only changes the escape state.
func (s keyStep) leaves(escape int) keyOutcome {
	return keyOutcome{escape: escape, direction: s.direction}
}

// steers is the outcome of a step that ends up outside a sequence with the
// stick held in a direction. That is reported only if it is a change.
func (s keyStep) steers(direction Direction) keyOutcome {
	out := keyOutcome{escape: escapeNone, direction: direction}
	if direction != s.direction {
		out.reports = []ControllerState{{direction, Btn_None}}
	}
	return out
}

// TestKeyboardEveryStepFollowsItsRule checks all 6,400 steps against
// keyRules.
func TestKeyboardEveryStepFollowsItsRule(t *testing.T) {
	used := make([]int, len(keyRules))
	everyKeyStep(t, func(s keyStep, got keyOutcome) string {
		rule := -1
		for i, r := range keyRules {
			if !r.applies(s) {
				continue
			}
			if rule >= 0 {
				return fmt.Sprintf("the test's rules %q and %q both claim this step", keyRules[rule].name, r.name)
			}
			rule = i
		}
		if rule < 0 {
			return "none of the test's rules covers this step"
		}
		used[rule]++
		if want := keyRules[rule].want(s); !got.equal(want) {
			return fmt.Sprintf("rule %q\n\twant the reader left %v", keyRules[rule].name, want)
		}
		return ""
	})
	for i, n := range used {
		if n == 0 {
			t.Errorf("rule %q was never used: it covers no step", keyRules[i].name)
		}
	}
}

// arrowSequence is one way a terminal sends an arrow.
type arrowSequence struct {
	keys      string
	direction Direction
}

// arrowSequences are the ways a terminal sends an arrow that the reader
// understands: ESC [ A, ESC O A in application mode, and with numbers when
// a modifier is held.
func arrowSequences(beforeLetter ...string) (sequences []arrowSequence) {
	for _, letter := range []byte("ABCD") {
		for _, before := range beforeLetter {
			sequences = append(sequences, arrowSequence{before + string(letter), arrowLetters[letter]})
		}
	}
	return sequences
}

// TestKeyboardArrowSteersFromEveryState: whatever was typed before, and
// however far into a sequence it left the reader, an arrow key typed next
// steers that way and leaves the reader outside a sequence. As the reader
// can be in no state but these 25, no input of any length can make it miss
// an arrow.
func TestKeyboardArrowSteersFromEveryState(t *testing.T) {
	typed, failures := 0, 0
	for _, escape := range escapeStates {
		for _, direction := range heldDirections {
			for _, arrow := range arrowSequences("\x1b[", "\x1bO", "\x1b[1;5", "\x1b[1;2") {
				var reports []ControllerState
				k := Keyboard{escape: escape, direction: direction}
				k.handle = func(state ControllerState) { reports = append(reports, state) }
				quit := false
				for _, b := range []byte(arrow.keys) {
					quit = k.key(b) || quit
				}

				var want []ControllerState
				if arrow.direction != direction {
					want = []ControllerState{{arrow.direction, Btn_None}}
				}
				typed++
				if !quit && k.direction == arrow.direction && k.escape == escapeNone && slices.Equal(reports, want) {
					continue
				}
				failures++
				if failures <= 10 {
					t.Errorf("%q typed %s with the stick held %v: the reader is left %s with the stick held %v, "+
						"reported %v and quit=%v; want it outside a sequence with the stick held %v, reporting %v and not quitting",
						arrow.keys, escapeName(escape), direction,
						escapeName(k.escape), k.direction, reports, quit, arrow.direction, want)
				}
			}
		}
	}
	if failures > 0 {
		t.Errorf("%d of %d arrows were misread", failures, typed)
	}
}

// prefixTyper types every string of up to a number of bytes, and after each
// of them every arrow.
type prefixTyper struct {
	t       *testing.T
	longest int
	arrows  []arrowSequence
	typed   [3]byte

	// What the reader reported for the arrow now being typed.
	reports int
	last    ControllerState
	button  bool

	// How many strings were typed, after how many of them an arrow went
	// wrong, and how many failures have been printed.
	strings, misread, printed int
}

func (p *prefixTyper) handle(state ControllerState) {
	p.reports++
	p.last = state
	if state.ButtonStatus != Btn_None {
		p.button = true
	}
}

// after carries on from a reader that has read the first n bytes of
// p.typed. The reader is passed by value, so every string starts from a
// copy of the reader as its first bytes left it.
func (p *prefixTyper) after(k Keyboard, n int) {
	p.strings++
	misread := false
	for i := range p.arrows {
		if !p.arrow(k, n, &p.arrows[i]) {
			misread = true
		}
	}
	if misread {
		p.misread++
	}
	if n == p.longest {
		return
	}
	for b := 0; b < 256; b++ {
		next := k
		if next.key(byte(b)) {
			// The reader ends when it is asked to quit. Nothing typed
			// after that is read.
			continue
		}
		p.typed[n] = byte(b)
		p.after(next, n+1)
	}
}

// arrow types one arrow at a copy of the reader and reports whether it
// steered and did nothing else. The first few failures are printed.
func (p *prefixTyper) arrow(k Keyboard, n int, arrow *arrowSequence) bool {
	before := k.direction
	p.reports, p.button = 0, false
	quit := false
	for i := 0; i < len(arrow.keys); i++ {
		quit = k.key(arrow.keys[i]) || quit
	}

	ok := !quit && !p.button && k.direction == arrow.direction && k.escape == escapeNone
	if arrow.direction == before {
		ok = ok && p.reports == 0
	} else {
		ok = ok && p.reports == 1 && p.last == ControllerState{arrow.direction, Btn_None}
	}
	if ok {
		return true
	}
	p.printed++
	if p.printed > 10 {
		return false
	}
	reported := "reported nothing"
	if p.reports > 0 {
		reported = fmt.Sprintf("made %d reports, the last %v/%v", p.reports, p.last.Direction, p.last.ButtonStatus)
	}
	p.t.Errorf("%q typed after %q: the reader is left %s with the stick held %v, %s, pressed a button=%v and quit=%v; "+
		"want it outside a sequence with the stick held %v, having reported that once if it is a change, and nothing else",
		arrow.keys, p.typed[:n], escapeName(k.escape), k.direction, reported, p.button, quit, arrow.direction)
	return false
}

// TestKeyboardArrowSteersAfterAnyKeys types every string of up to three
// bytes at a reader that has just started, leaving out what comes after a
// key that quits, and then each arrow in each of its forms: the arrow must
// steer, and do nothing else. It goes through the reader's own entry point
// with nothing set up by hand, so it would also notice the reader starting
// to remember something that TestKeyboardArrowSteersFromEveryState does not
// know about.
//
// There are 16.3 million such strings, which take about two seconds. Under
// the race detector they take about twenty times as long, so there the
// strings are of up to two bytes: 64 thousand of them, among them the two
// that used to go wrong, ESC [ and ESC O. The three-byte strings are only
// typed when the tests are run without -race.
func TestKeyboardArrowSteersAfterAnyKeys(t *testing.T) {
	p := &prefixTyper{t: t, longest: 3, arrows: arrowSequences("\x1b[", "\x1bO")}
	if raceDetector {
		p.longest = 2
		t.Log("the race detector is on: typing the strings of up to 2 bytes only, the 3-byte ones need a run without -race")
	}
	p.after(Keyboard{direction: Dir_Neutral, handle: p.handle}, 0)

	if p.misread > 0 {
		t.Errorf("an arrow was misread after %d of the %d strings of up to %d bytes", p.misread, p.strings, p.longest)
	}
	// A walk that was cut short would pass without having tried much. The
	// strings that do not quit, by length: 1 empty, 253 of one byte,
	// 64,012 of two and 16,195,801 of three.
	if want := map[int]int{2: 64266, 3: 16260067}[p.longest]; p.strings != want {
		t.Errorf("typed %d strings of up to %d bytes, want %d: every string in which no key quits",
			p.strings, p.longest, want)
	}
	t.Logf("%d strings of up to %d bytes, each followed by %d arrow sequences", p.strings, p.longest, len(p.arrows))
}
