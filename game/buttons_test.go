package game

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/controller"
)

// oscSink is a loopback UDP listener standing in for Chromatik's OSC port.
type oscSink struct {
	t    *testing.T
	conn *net.UDPConn
}

// oscMessage is an OSC message with one float argument, which is all the
// game sends to Chromatik.
type oscMessage struct {
	Address string
	Value   float32
}

// newOSCTestGame is newTestGame with Chromatik's OSC client pointed at a
// listener the test can read.
func newOSCTestGame(t *testing.T) (*Game, *oscSink) {
	t.Helper()
	g, _ := newTestGame(t)
	sink := &oscSink{t: t, conn: listenLoopback(t)}
	g.Chromatik.OscClient = testOscClient(t, sink.conn.LocalAddr().(*net.UDPAddr).Port)
	return g, sink
}

// decodeOSC reads the address and the single float32 argument out of an OSC
// message datagram: the address and the type tags ",f" are NUL-terminated
// strings padded to a multiple of four bytes, and the float is big-endian.
func decodeOSC(d []byte) (oscMessage, error) {
	end := bytes.IndexByte(d, 0)
	if end < 0 {
		return oscMessage{}, fmt.Errorf("no address terminator in % x", d)
	}
	msg := oscMessage{Address: string(d[:end])}
	rest := d[(end/4+1)*4:]
	if len(rest) != 8 || string(rest[:4]) != ",f\x00\x00" {
		return msg, fmt.Errorf("%s: arguments are % x, want one float", msg.Address, rest)
	}
	msg.Value = math.Float32frombits(binary.BigEndian.Uint32(rest[4:]))
	return msg, nil
}

// messages returns every OSC message sent since the last call. The handler
// sends on the caller's goroutine, so by the time it has returned everything
// is already waiting in the socket; the short wait only detects the end.
func (s *oscSink) messages() []oscMessage {
	s.t.Helper()
	var out []oscMessage
	buf := make([]byte, 512)
	for {
		_ = s.conn.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
		n, _, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			return out
		}
		msg, err := decodeOSC(buf[:n])
		if err != nil {
			s.t.Fatalf("bad OSC datagram: %v", err)
		}
		out = append(out, msg)
	}
}

// expectNone fails if a state change sent any OSC at all.
func (s *oscSink) expectNone(what string) {
	s.t.Helper()
	if got := s.messages(); len(got) != 0 {
		s.t.Fatalf("%s sent %d OSC messages, want none: %v", what, len(got), got)
	}
}

// expectPress fails unless the messages are exactly what one button press
// sends: every one of the nine channel faders once, at 1 for the fader named
// and 0 for the rest, plus one recall of the given palette swatch, or no
// recall if swatch is 0.
func (s *oscSink) expectPress(what, faderOn string, swatch int) {
	s.t.Helper()
	got := s.messages()

	want := map[string]float32{}
	for ch := 1; ch <= 9; ch++ {
		want[fmt.Sprintf("/lx/mixer/channel/%d/fader", ch)] = 0
	}
	if _, ok := want[faderOn]; !ok {
		s.t.Fatalf("%s: %q is not a fader address", what, faderOn)
	}
	want[faderOn] = 1
	if swatch != 0 {
		want[fmt.Sprintf("/lx/palette/swatches/%d/recall", swatch)] = 1
	}

	seen := map[string]int{}
	for _, m := range got {
		seen[m.Address]++
		v, ok := want[m.Address]
		if !ok {
			s.t.Errorf("%s: unexpected message to %s", what, m.Address)
		} else if m.Value != v {
			s.t.Errorf("%s: %s was sent %v, want %v", what, m.Address, m.Value, v)
		}
	}
	for addr := range want {
		if seen[addr] != 1 {
			s.t.Errorf("%s: %s was sent %d times, want once", what, addr, seen[addr])
		}
	}
	if s.t.Failed() {
		s.t.Fatalf("%s: got %d messages: %v", what, len(got), got)
	}
}

const (
	fader1 = "/lx/mixer/channel/1/fader"
	fader2 = "/lx/mixer/channel/2/fader"
	fader3 = "/lx/mixer/channel/3/fader"
	fader4 = "/lx/mixer/channel/4/fader"
	fader5 = "/lx/mixer/channel/5/fader"
	fader6 = "/lx/mixer/channel/6/fader"
	fader7 = "/lx/mixer/channel/7/fader"
	fader8 = "/lx/mixer/channel/8/fader"
	fader9 = "/lx/mixer/channel/9/fader"
)

// allFive is every button held at once.
const allFive = controller.Btn_Red | controller.Btn_Green | controller.Btn_Blue | controller.Btn_Yellow | controller.Btn_White

// stickTour is a walk around the stick that ends back in the middle.
var stickTour = []controller.Direction{
	controller.Dir_North, controller.Dir_NorthEast, controller.Dir_East,
	controller.Dir_South, controller.Dir_Neutral,
}

// TestButtonHeldWhileStickMoves holds red and moves the stick around. That
// is one press of red, however many states the controller reports.
func TestButtonHeldWhileStickMoves(t *testing.T) {
	g, sink := newOSCTestGame(t)

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red))
	sink.expectPress("pressing red", fader2, 1)
	if !g.paused.Load() {
		t.Error("red did not pause")
	}

	for _, dir := range stickTour {
		g.HandleControllerState(input(dir, controller.Btn_Red))
		sink.expectNone(fmt.Sprintf("moving the stick to %v with red held", dir))
		if got := g.input.Load().direction(); got != dir {
			t.Errorf("held direction is %v, want %v", got, dir)
		}
	}

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of red")
}

// TestButtonPressedTwice presses red, lets go and presses it again. Each
// press acts, and the second one selects red's other fader.
func TestButtonPressedTwice(t *testing.T) {
	g, sink := newOSCTestGame(t)

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red))
	sink.expectPress("first press of red", fader2, 1)
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of red")
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red))
	sink.expectPress("second press of red", fader1, 1)
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of red again")
}

// TestWhiteHeldWhileStickMoves holds white and moves the stick. Pause must
// flip once, when white goes down, and not again until the next press.
func TestWhiteHeldWhileStickMoves(t *testing.T) {
	g, sink := newOSCTestGame(t)

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_White))
	sink.expectPress("pressing white", fader9, 0)
	if !g.paused.Load() {
		t.Fatal("white did not pause")
	}

	for _, dir := range stickTour {
		g.HandleControllerState(input(dir, controller.Btn_White))
		sink.expectNone(fmt.Sprintf("moving the stick to %v with white held", dir))
		if !g.paused.Load() {
			t.Fatalf("moving the stick to %v with white held resumed the game", dir)
		}
	}

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of white")
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_White))
	sink.expectPress("pressing white again", fader9, 0)
	if g.paused.Load() {
		t.Fatal("the second press of white did not resume")
	}
}

// TestSecondButtonThenRelease holds red, adds green and lets go of red.
// Green going down is a press of green by itself, although red is still
// held: green's fader and swatch. Red coming up is not a press of anything.
func TestSecondButtonThenRelease(t *testing.T) {
	g, sink := newOSCTestGame(t)

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red))
	sink.expectPress("pressing red", fader2, 1)
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red|controller.Btn_Green))
	sink.expectPress("adding green", fader3, 2)
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Green))
	sink.expectNone("letting go of red with green held")
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of green")

	// The two presses above each moved on to the other fader of a pair,
	// so the next press of green is on its second fader.
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Green))
	sink.expectPress("pressing green by itself", fader4, 2)
}

// TestRolledPress presses a second button before the first is let go, in
// each order of letting go. The second button must act as a press of that
// button alone, so that the last button pressed is the one whose fader is
// left up. Every press sets all nine faders, so the messages of the last
// press are the state Chromatik is left in: here always one fader at 1,
// never all of them at 0.
func TestRolledPress(t *testing.T) {
	const (
		red   = controller.Btn_Red
		blue  = controller.Btn_Blue
		white = controller.Btn_White
	)
	tests := []struct {
		name          string
		first, second controller.ButtonStatus
		firstFader    string
		firstSwatch   int
		secondFader   string
		secondSwatch  int
		// Which of the two is let go first.
		releaseFirst controller.ButtonStatus
		// Whether the game is paused in the end. A colored button pauses
		// and white toggles.
		paused bool
	}{
		{"red then white, red let go first", red, white, fader2, 1, fader9, 0, red, false},
		{"red then white, white let go first", red, white, fader2, 1, fader9, 0, white, false},
		{"red then blue, red let go first", red, blue, fader2, 1, fader5, 3, red, true},
		{"red then blue, blue let go first", red, blue, fader2, 1, fader5, 3, blue, true},
		{"white then red, white let go first", white, red, fader9, 0, fader1, 1, white, true},
		{"white then red, red let go first", white, red, fader9, 0, fader1, 1, red, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, sink := newOSCTestGame(t)

			g.HandleControllerState(input(controller.Dir_Neutral, tc.first))
			sink.expectPress("the first button", tc.firstFader, tc.firstSwatch)
			g.HandleControllerState(input(controller.Dir_Neutral, tc.first|tc.second))
			sink.expectPress("the second button, with the first still held", tc.secondFader, tc.secondSwatch)
			if got := g.paused.Load(); got != tc.paused {
				t.Errorf("paused is %v after the second button, want %v", got, tc.paused)
			}

			g.HandleControllerState(input(controller.Dir_Neutral, (tc.first|tc.second)&^tc.releaseFirst))
			sink.expectNone("letting go of one button")
			g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
			sink.expectNone("letting go of the other")
			if got := g.paused.Load(); got != tc.paused {
				t.Errorf("paused is %v in the end, want %v", got, tc.paused)
			}
		})
	}
}

// TestButtonsDownTogether has several buttons go down in the same report.
// That is a press of one of them (see firstButton), so one fader is left
// up. It is one of the buttons that went down, whatever else is held. All
// five held together does nothing.
func TestButtonsDownTogether(t *testing.T) {
	g, sink := newOSCTestGame(t)

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red|controller.Btn_Blue))
	sink.expectPress("pressing red and blue together", fader2, 1)
	if !g.paused.Load() {
		t.Error("two buttons together did not pause")
	}
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Blue))
	sink.expectNone("letting go of red")
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of blue")

	g.HandleControllerState(input(controller.Dir_Neutral, allFive))
	sink.expectNone("pressing all five buttons")
	g.HandleControllerState(input(controller.Dir_Neutral, allFive&^controller.Btn_White))
	sink.expectNone("letting go of white with the other four held")
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of the rest")

	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red))
	sink.expectPress("pressing red by itself", fader1, 1)

	// Two more going down in one report while red is held are a press of
	// one of those two. Red comes first in the order, but it did not go
	// down, so this is not a press of red again.
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_Red|controller.Btn_Green|controller.Btn_Blue))
	sink.expectPress("adding green and blue together with red held", fader4, 2)
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of all three")

	// A color and white together are a press of the color, so the game
	// stays paused: white does not get to toggle it.
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_White|controller.Btn_Yellow))
	sink.expectPress("pressing yellow and white together", fader7, 4)
	if !g.paused.Load() {
		t.Error("yellow and white together resumed the game")
	}
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("letting go of yellow and white")

	// Four held and the fifth added is all five held, which does nothing:
	// it is not a press of the fifth, which for white would resume.
	g.HandleControllerState(input(controller.Dir_Neutral, allFive&^controller.Btn_White))
	sink.expectPress("pressing four together", fader2, 1)
	g.HandleControllerState(input(controller.Dir_Neutral, allFive))
	sink.expectNone("adding white with the other four held")
	if !g.paused.Load() {
		t.Error("adding white with the other four held resumed the game")
	}
}

// TestEveryButtonCombinationLeavesOneFaderUp presses every combination of
// buttons short of all five, each as the first press of a new game. Every
// one must leave exactly one fader up, that of the first button in the
// order red, green, blue, yellow, white that is part of the combination,
// and never all nine at 0. The first press of a game uses the second fader
// of a color's pair.
func TestEveryButtonCombinationLeavesOneFaderUp(t *testing.T) {
	order := []struct {
		button controller.ButtonStatus
		fader  string
		swatch int
	}{
		{controller.Btn_Red, fader2, 1},
		{controller.Btn_Green, fader4, 2},
		{controller.Btn_Blue, fader6, 3},
		{controller.Btn_Yellow, fader8, 4},
		{controller.Btn_White, fader9, 0},
	}
	for buttons := controller.Btn_Red; buttons < allFive; buttons++ {
		t.Run(fmt.Sprintf("%05b", buttons), func(t *testing.T) {
			g, sink := newOSCTestGame(t)
			g.HandleControllerState(input(controller.Dir_Neutral, buttons))
			for _, o := range order {
				if buttons&o.button != 0 {
					sink.expectPress("pressing "+buttons.String(), o.fader, o.swatch)
					break
				}
			}
			if !g.paused.Load() {
				t.Error("the press did not pause the game")
			}
		})
	}
}

// TestUnknownButtonBit sets a bit of the button byte that is none of the
// five buttons. The controller passes the whole byte on, so the game can
// be handed one. Alone it is not a press; with a button it is a press of
// that button.
func TestUnknownButtonBit(t *testing.T) {
	const unknown = controller.ButtonStatus(0b100000)
	g, sink := newOSCTestGame(t)

	g.HandleControllerState(input(controller.Dir_Neutral, unknown))
	sink.expectNone("an unknown button bit")
	if g.paused.Load() {
		t.Error("an unknown button bit paused the game")
	}
	g.HandleControllerState(input(controller.Dir_Neutral, controller.Btn_None))
	sink.expectNone("the unknown bit clearing")

	g.HandleControllerState(input(controller.Dir_Neutral, unknown|controller.Btn_Green))
	sink.expectPress("green together with an unknown bit", fader4, 2)
	if !g.paused.Load() {
		t.Error("green together with an unknown bit did not pause")
	}
}

// TestOSCDecode checks the test's own decoder against OSC messages written
// out byte by byte: the address and the type tags ",f" are each ended by a
// NUL and padded with NULs to a multiple of 4 bytes, and 0.25 as a
// big-endian float32 is 3e 80 00 00.
func TestOSCDecode(t *testing.T) {
	for _, c := range []struct {
		addr     string
		datagram string
	}{
		{"/lx/mixer/channel/3/fader", "/lx/mixer/channel/3/fader\x00\x00\x00,f\x00\x00\x3e\x80\x00\x00"},
		{"/lx/palette/swatches/4/recall", "/lx/palette/swatches/4/recall\x00\x00\x00,f\x00\x00\x3e\x80\x00\x00"},
		{"/a", "/a\x00\x00,f\x00\x00\x3e\x80\x00\x00"},
		{"/abc", "/abc\x00\x00\x00\x00,f\x00\x00\x3e\x80\x00\x00"},
	} {
		got, err := decodeOSC([]byte(c.datagram))
		if err != nil || got != (oscMessage{Address: c.addr, Value: 0.25}) {
			t.Errorf("decodeOSC(%q) = %v, %v", c.addr, got, err)
		}
	}
}
