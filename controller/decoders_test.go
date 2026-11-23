package controller

import (
	"strings"
	"testing"
)

// The two decoders each look at two bytes of a report, so everything one of
// them can be given fits in a loop: 65,536 pairs of bytes. The tests in this
// file go through all of them. What a pair should decode to is written down
// here as a table of byte ranges for the stick and a table of single bits
// for the buttons, not worked out with the decoders' own shifts, so that a
// slip in a shift or a mask shows.
//
// The tables agree with the direction and button constants and with the
// reports the other tests build (report, in controller_test.go). Whether
// they are what the joystick really sends is something only the joystick
// can say.

// complaints reports the first few failures of a test that goes through
// tens of thousands of reports, and counts the rest: a broken decoder is
// wrong for thousands of them, and a line for each would bury the first.
type complaints struct {
	t *testing.T
	n int
}

const complaintsShown = 5

func (c *complaints) add(format string, args ...any) {
	c.t.Helper()
	c.n++
	if c.n <= complaintsShown {
		c.t.Errorf(format, args...)
	}
}

func (c *complaints) done() {
	c.t.Helper()
	if c.n > complaintsShown {
		c.t.Errorf("and %d more wrong reports like these", c.n-complaintsShown)
	}
}

// otherFillings are what the six bytes of a report that a decoder does not
// look at are filled with: every pair is decoded with them all clear, with
// them all set, and with each of them set while the other five are clear,
// and must decode the same every time. One byte set alone shows a decoder
// that mixes other bytes in so that they cancel out when they are all
// equal. That is eight fillings out of far too many to go through, and a
// decoder that looked for one particular value in another byte would get
// past them; that no other value matters rests on reading the decoders.
var otherFillings = [...][6]byte{
	{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	{0xff, 0x00, 0x00, 0x00, 0x00, 0x00},
	{0x00, 0xff, 0x00, 0x00, 0x00, 0x00},
	{0x00, 0x00, 0xff, 0x00, 0x00, 0x00},
	{0x00, 0x00, 0x00, 0xff, 0x00, 0x00},
	{0x00, 0x00, 0x00, 0x00, 0xff, 0x00},
	{0x00, 0x00, 0x00, 0x00, 0x00, 0xff},
}

// byteRange is the values lo to hi of one byte, both included.
type byteRange struct{ lo, hi byte }

func (r byteRange) has(b byte) bool { return r.lo <= b && b <= r.hi }

// An axis byte is in one of four ranges, a quarter of its values each: only
// its top two bits count. The stick has three positions on an axis, and the
// nine directions with a name use the low, middle and high ranges. The odd
// range, between middle and high, is no position of the stick.
//
// Byte 0 is the axis that tells east from west and byte 1 the one that
// tells north from south, going by the names of the directions, which are
// what the game steers by.
var (
	axisLow    = byteRange{0x00, 0x3f} // east on byte 0, south on byte 1
	axisMiddle = byteRange{0x40, 0x7f} // centred
	axisOdd    = byteRange{0x80, 0xbf}
	axisHigh   = byteRange{0xc0, 0xff} // west on byte 0, north on byte 1
)

// stickTable says what every pair of axis bytes decodes to. The first nine
// rows are the directions with a name: the pairs in which neither byte is
// in the odd range. The other seven are the pairs in which one is. Their
// values have no name, and String calls each of them "Unknown".
var stickTable = []struct {
	eastWest, northSouth byteRange // bytes 0 and 1
	want                 Direction
	name                 string
}{
	{axisMiddle, axisMiddle, Dir_Neutral, "Neutral"},
	{axisMiddle, axisHigh, Dir_North, "North"},
	{axisLow, axisHigh, Dir_NorthEast, "NorthEast"},
	{axisLow, axisMiddle, Dir_East, "East"},
	{axisLow, axisLow, Dir_SouthEast, "SouthEast"},
	{axisMiddle, axisLow, Dir_South, "South"},
	{axisHigh, axisLow, Dir_SouthWest, "SouthWest"},
	{axisHigh, axisMiddle, Dir_West, "West"},
	{axisHigh, axisHigh, Dir_NorthWest, "NorthWest"},

	{axisLow, axisOdd, Direction(0b0010), "Unknown"},
	{axisMiddle, axisOdd, Direction(0b0110), "Unknown"},
	{axisOdd, axisLow, Direction(0b1000), "Unknown"},
	{axisOdd, axisMiddle, Direction(0b1001), "Unknown"},
	{axisOdd, axisOdd, Direction(0b1010), "Unknown"},
	{axisOdd, axisHigh, Direction(0b1011), "Unknown"},
	{axisHigh, axisOdd, Direction(0b1110), "Unknown"},
}

// TestEveryStickReport decodes all 65,536 pairs of axis bytes, once for
// every filling of bytes 2 to 7, and checks each against stickTable, the
// name String gives it included. Then it counts: the stick decodes to 16
// different values, each from a sixteenth of the pairs, and nine of them
// have a name.
func TestEveryStickReport(t *testing.T) {
	j := NewJoystick(quietLogger())
	bad := complaints{t: t}

	const pairsPerValue = 64 * 64
	pairs := map[Direction]int{}
	for i, other := range otherFillings {
		data := []byte{0, 0, other[0], other[1], other[2], other[3], other[4], other[5]}
		for b0 := 0; b0 < 256; b0++ {
			for b1 := 0; b1 < 256; b1++ {
				data[0], data[1] = byte(b0), byte(b1)
				got := j.GetDirection(data)
				if i == 0 {
					pairs[got]++
				}

				rows := 0
				for _, row := range stickTable {
					if !row.eastWest.has(data[0]) || !row.northSouth.has(data[1]) {
						continue
					}
					rows++
					if got != row.want {
						bad.add("axis bytes 0x%02x 0x%02x with % x in bytes 2 to 7 decode to %04b (%s), want %04b (%s)",
							b0, b1, other[:], int(got), got, int(row.want), row.name)
					} else if name := got.String(); name != row.name {
						bad.add("axis bytes 0x%02x 0x%02x decode to %04b, which String calls %q, want %q",
							b0, b1, int(got), name, row.name)
					}
				}
				if rows != 1 {
					t.Fatalf("the test's own table has %d rows for axis bytes 0x%02x 0x%02x, want exactly 1", rows, b0, b1)
				}
			}
		}
	}
	bad.done()

	if len(pairs) != 16 {
		t.Errorf("the stick decodes to %d different values, want 16", len(pairs))
	}
	named := 0
	for d, n := range pairs {
		if d < 0 || d > 15 {
			t.Errorf("the stick decodes to %d, want only values from 0 to 15", int(d))
		}
		if n != pairsPerValue {
			t.Errorf("%d pairs of axis bytes decode to %04b, want %d", n, int(d), pairsPerValue)
		}
		if d.String() != "Unknown" {
			named++
		}
	}
	if named != 9 {
		t.Errorf("%d of the values the stick decodes to have a name, want 9", named)
	}
}

// TestDirectionStringForAnyValue checks that String has an answer for
// values well outside the 16 the decoder produces too, negative ones
// included: a name for the nine directions and "Unknown" for anything else.
func TestDirectionStringForAnyValue(t *testing.T) {
	names := map[Direction]string{}
	for _, row := range stickTable {
		if row.name != "Unknown" {
			names[row.want] = row.name
		}
	}
	if len(names) != 9 {
		t.Fatalf("the test's own table names %d directions, want 9", len(names))
	}

	for d := Direction(-300); d <= 300; d++ {
		want, ok := names[d]
		if !ok {
			want = "Unknown"
		}
		if got := d.String(); got != want {
			t.Errorf("Direction(%d).String() = %q, want %q", int(d), got, want)
		}
	}
}

// fiveButtons is every button the joystick has.
const fiveButtons = Btn_Red | Btn_Green | Btn_Blue | Btn_Yellow | Btn_White

// buttonBits says what each of the 16 bits of bytes 5 and 6 does to the
// button status. Five are the buttons. Three more are passed on as status
// bits that are no button: to the game they are no press, though a change
// in one still counts as input to it and ends demo mode. The other eight do
// nothing at all.
var buttonBits = []struct {
	index  int          // which byte of the report
	bit    byte         // which bit of that byte
	sets   ButtonStatus // what it sets in the status
	button string       // the button it is, if it is one
}{
	{5, 0x01, 0, ""},
	{5, 0x02, 0, ""},
	{5, 0x04, 0, ""},
	{5, 0x08, 0, ""},
	{5, 0x10, Btn_Red, "red"},
	{5, 0x20, Btn_Green, "green"},
	{5, 0x40, Btn_Blue, "blue"},
	{5, 0x80, Btn_Yellow, "yellow"},

	{6, 0x01, Btn_White, "white"},
	{6, 0x02, ButtonStatus(0b0010_0000), ""},
	{6, 0x04, ButtonStatus(0b0100_0000), ""},
	{6, 0x08, ButtonStatus(0b1000_0000), ""},
	{6, 0x10, 0, ""},
	{6, 0x20, 0, ""},
	{6, 0x40, 0, ""},
	{6, 0x80, 0, ""},
}

// TestEveryButtonReport decodes all 65,536 pairs of button bytes, once for
// every filling of bytes 0 to 4 and 7, and checks each against buttonBits,
// in two parts: which of the five buttons are reported, which only the five
// button bits may change, and which status bits that are no button come
// with them.
func TestEveryButtonReport(t *testing.T) {
	b := NewButtons(quietLogger())
	bad := complaints{t: t}

	for _, other := range otherFillings {
		data := []byte{other[0], other[1], other[2], other[3], other[4], 0, 0, other[5]}
		for b5 := 0; b5 < 256; b5++ {
			for b6 := 0; b6 < 256; b6++ {
				data[5], data[6] = byte(b5), byte(b6)
				got := b.GetStatus(data)

				var held, rest ButtonStatus
				for _, row := range buttonBits {
					if data[row.index]&row.bit == 0 {
						continue
					}
					if row.button != "" {
						held |= row.sets
					} else {
						rest |= row.sets
					}
				}

				if got&fiveButtons != held {
					bad.add("button bytes 0x%02x 0x%02x with % x in bytes 0 to 4 and 7 report the buttons %05b, want %05b (white, yellow, blue, green, red)",
						b5, b6, other[:], int(got&fiveButtons), int(held))
				}
				if got&^fiveButtons != rest {
					bad.add("button bytes 0x%02x 0x%02x with % x in bytes 0 to 4 and 7 give the status %#b, whose bits that are no button should be %#b",
						b5, b6, other[:], int(got), int(rest))
				}
			}
		}
	}
	bad.done()
}

// TestButtonStatusStringForEveryStatus checks String for all 256 values the
// decoder can give: "None" for no bits at all, "Unknown" for bits of which
// none is a button, and otherwise the mark of every button held, once each
// and in the order red, green, blue, yellow, white, whatever other bits are
// set.
//
// The marks are colored discs. The red and white marks also carry an
// invisible character, U+FE0F, that picks how the disc is drawn. Where and
// how often it comes is not pinned here, but nothing else may be in the
// string: no text before, between or after the marks.
func TestButtonStatusStringForEveryStatus(t *testing.T) {
	marks := []struct {
		button ButtonStatus
		name   string
		disc   string
	}{
		{Btn_Red, "red", "\U0001F534"},
		{Btn_Green, "green", "\U0001F7E2"},
		{Btn_Blue, "blue", "\U0001F535"},
		{Btn_Yellow, "yellow", "\U0001F7E1"},
		{Btn_White, "white", "⚪"},
	}

	const drawnAs = "\uFE0F"

	for v := 0; v < 256; v++ {
		status := ButtonStatus(v)
		got := status.String()

		switch {
		case status == Btn_None:
			if got != "None" {
				t.Errorf("ButtonStatus(%#b).String() = %q, want %q", v, got, "None")
			}
			continue
		case status&fiveButtons == 0:
			if got != "Unknown" {
				t.Errorf("ButtonStatus(%#b).String() = %q, want %q: none of its bits is a button", v, got, "Unknown")
			}
			continue
		}

		after := 0
		left := strings.ReplaceAll(got, drawnAs, "")
		for _, m := range marks {
			left = strings.ReplaceAll(left, m.disc, "")
			held := status&m.button != 0
			at := strings.Index(got, m.disc)
			switch {
			case held && strings.Count(got, m.disc) != 1:
				t.Errorf("ButtonStatus(%#b).String() = %q, want the %s mark in it once", v, got, m.name)
			case held && at < after:
				t.Errorf("ButtonStatus(%#b).String() = %q, want the %s mark after the marks before it", v, got, m.name)
			case !held && at >= 0:
				t.Errorf("ButtonStatus(%#b).String() = %q, want no %s mark in it", v, got, m.name)
			}
			if held {
				after = at
			}
		}
		if left != "" {
			t.Errorf("ButtonStatus(%#b).String() = %q, want nothing in it but the marks of the buttons held, and it also has %q", v, got, left)
		}
		if only := (status & fiveButtons).String(); got != only {
			t.Errorf("ButtonStatus(%#b).String() = %q, want %q: bits that are no button do not show", v, got, only)
		}
	}
}
