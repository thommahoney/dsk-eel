package game

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

// A joystick state says which buttons are held, not which one was pressed.
// HandleControllerState works the press out from the state before it, and
// between two calls it remembers four things: the buttons that were held,
// whether the game is paused, which fader of a color's pair the last press
// went to, and the input state, that is the stick and whether the eel
// steers itself.
//
// The joystick reports the buttons as a byte, so there are 256 values for
// the buttons held before and 256 for the buttons held now. The test in
// this file calls the real handler for every pair of them, in a game that
// is paused and one that is not, after a press that went to the first
// fader of a pair and after one that went to the second, with the eel
// steering itself and with a player steering: 256 x 256 x 2 x 2 x 2 =
// 524,288 calls. Each is compared with the rule written out below, in what
// it sends and in what the handler remembers afterwards, so what holds for
// one call holds for any number of calls in a row.
//
// The rest of the input state is not multiplied in, but none of it is left
// as it happens to be. The stick a call reports moves on from one call to
// the next through all 16 values. In half of the calls the game held that
// same stick before, which is a button pressed with the stick still, the
// ordinary press at the dome, and in the other half it held a different
// one. The byte has three bits that are no button, and one of them in the
// byte before, buttonRuleStickMoved, says which half a call is in, so every
// pair of the five buttons is tried both ways in each of the eight kinds of
// game. An eel that steers itself does so by turns because of a demo,
// because the joystick is lost, and both.
//
// The rule, as the comments in game.go and the tests in buttons_test.go
// state it:
//
//   - a state is a press if one of the five buttons is held now that was
//     not held before, unless all five are held now. A button let go, a
//     button still held, a stick that moved and a bit of the byte that is
//     none of the five buttons are not a press;
//   - the press is of one button: the first that went down, in the order
//     red, green, blue, yellow, white;
//   - a press sets all nine channel faders, one of them to 1 and the rest
//     to 0. A color has two faders and white has one. Every press, of any
//     button, moves on to the other fader of a pair, and the first press
//     of all goes to the second;
//   - a color recalls its palette swatch and pauses the game. White
//     recalls nothing, and pauses a game that is running and resumes one
//     that is paused;
//   - a state that is not a press sends nothing, and leaves pause and
//     which fader is next alone;
//   - every state, press or not, is input: the stick it reports is held
//     from then on, and a demo and a lost joystick are over.
//
// What the handler sends is read from a loopback socket in Chromatik's
// place. What this cannot show is that the rule is the right one, or that
// Chromatik does with a fader or a swatch what the dome needs.

// buttonRuleButtons is the five buttons in the order in which a press of
// several at once is a press of the first, each with the mixer channels of
// its faders and the palette swatch it recalls, 0 for none.
var buttonRuleButtons = [...]struct {
	name   string
	button controller.ButtonStatus
	faders []int
	swatch int
}{
	{"red", controller.Btn_Red, []int{1, 2}, 1},
	{"green", controller.Btn_Green, []int{3, 4}, 2},
	{"blue", controller.Btn_Blue, []int{5, 6}, 3},
	{"yellow", controller.Btn_Yellow, []int{7, 8}, 4},
	{"white", controller.Btn_White, []int{9}, 0},
}

// buttonRuleStickMoved is the bit of the byte that is no button which the
// test uses to say whether the stick moved. Where the buttons held before
// have it clear, the stick the call reports is the one the game held
// already; where they have it set, the game held a different one. It is the
// lowest bit above the five buttons, so the 64 x 64 pairs tried under the
// race detector have both as well.
const buttonRuleStickMoved controller.ButtonStatus = 0x20

// buttonRulePress is which button a joystick state is a press of, as a
// number in buttonRuleButtons, or -1 if it is not a press: the first of
// the five that is held now and was not held before, and none at all when
// all five are held now.
func buttonRulePress(before, now controller.ButtonStatus) int {
	held, press := 0, -1
	for i, b := range buttonRuleButtons {
		if now&b.button == 0 {
			continue
		}
		held++
		if press < 0 && before&b.button == 0 {
			press = i
		}
	}
	if held == len(buttonRuleButtons) {
		return -1
	}
	return press
}

// The datagrams a press can send, by number: each of the nine faders at 0
// and at 1, the recall of each of the four swatches, and after them the
// one the test sends itself to mark the end of what a call sent.
const (
	buttonRuleFaders    = 9
	buttonRuleSwatches  = 4
	buttonRuleEnd       = 2*buttonRuleFaders + buttonRuleSwatches
	buttonRuleDatagrams = buttonRuleEnd + 1

	buttonRuleEndAddress = "/test/button-rule/end"
)

// buttonRuleFader is the number of the datagram that sets the fader of a
// mixer channel, 1 to 9, to 1 if up and to 0 if not.
func buttonRuleFader(channel int, up bool) int {
	if up {
		return 2*(channel-1) + 1
	}
	return 2 * (channel - 1)
}

// buttonRuleSwatch is the number of the datagram that recalls a palette
// swatch, 1 to 4.
func buttonRuleSwatch(swatch int) int {
	return 2*buttonRuleFaders + swatch - 1
}

// buttonRuleSent is how many times each datagram was sent.
type buttonRuleSent [buttonRuleDatagrams]uint8

// buttonRuleWant is what the rule says a call does: whether the game is
// paused afterwards, which fader of a pair the last press went to, and
// what is sent. press is what buttonRulePress says of the state, and
// paused and last are how the game was before the call. last is 0 for the
// first fader of a pair, which is also what a new game has, and 1 for the
// second.
func buttonRuleWant(press int, paused bool, last int) (pausedAfter bool, lastAfter int, sent buttonRuleSent) {
	if press < 0 {
		return paused, last, sent
	}
	b := buttonRuleButtons[press]
	lastAfter = 1 - last
	// White has the one fader, whichever of a pair is next.
	up := b.faders[lastAfter%len(b.faders)]
	for channel := 1; channel <= buttonRuleFaders; channel++ {
		sent[buttonRuleFader(channel, channel == up)]++
	}
	if b.swatch == 0 {
		// White.
		return !paused, lastAfter, sent
	}
	sent[buttonRuleSwatch(b.swatch)]++
	return true, lastAfter, sent
}

// buttonRuleMessage is the datagram of an OSC message with one float
// argument, built here and not by the game's own code: the address and
// the type tags ",f", each ended by a NUL and padded with NULs to a
// multiple of four bytes, and then the float, high byte first.
func buttonRuleMessage(address string, value float32) string {
	d := append([]byte(address), make([]byte, 4-len(address)%4)...)
	d = append(d, ',', 'f', 0, 0)
	return string(binary.BigEndian.AppendUint32(d, math.Float32bits(value)))
}

// buttonRuleMessages returns the datagrams a press can send and the end
// mark: the number of each, by its bytes.
func buttonRuleMessages(t *testing.T) map[string]int {
	t.Helper()
	numbers := map[string]int{}
	add := func(number int, address string, value float32) {
		datagram := buttonRuleMessage(address, value)
		// decodeOSC is checked against datagrams written out byte by byte
		// (TestOSCDecode), so this holds the datagrams built here to them.
		if got, err := decodeOSC([]byte(datagram)); err != nil || got != (oscMessage{Address: address, Value: value}) {
			t.Fatalf("the test's own datagram for %s at %v reads back as %v, %v", address, value, got, err)
		}
		numbers[datagram] = number
	}
	for channel := 1; channel <= buttonRuleFaders; channel++ {
		address := fmt.Sprintf("/lx/mixer/channel/%d/fader", channel)
		add(buttonRuleFader(channel, false), address, 0)
		add(buttonRuleFader(channel, true), address, 1)
	}
	for swatch := 1; swatch <= buttonRuleSwatches; swatch++ {
		add(buttonRuleSwatch(swatch), fmt.Sprintf("/lx/palette/swatches/%d/recall", swatch), 1)
	}
	add(buttonRuleEnd, buttonRuleEndAddress, 0)
	if len(numbers) != buttonRuleDatagrams {
		t.Fatalf("the test knows %d different datagrams, want %d", len(numbers), buttonRuleDatagrams)
	}
	return numbers
}

// buttonRuleHeld says in words which buttons a status has held.
func buttonRuleHeld(status controller.ButtonStatus) string {
	var held []string
	rest := status
	for _, b := range buttonRuleButtons {
		if status&b.button != 0 {
			held = append(held, b.name)
			rest &^= b.button
		}
	}
	if rest != 0 {
		held = append(held, fmt.Sprintf("the bits %08b that are no button", int(rest)))
	}
	switch len(held) {
	case 0:
		return "nothing"
	case 1:
		return held[0]
	}
	return strings.Join(held[:len(held)-1], ", ") + " and " + held[len(held)-1]
}

// buttonRulePaused says in words whether a game is paused.
func buttonRulePaused(paused bool) string {
	if paused {
		return "paused"
	}
	return "running"
}

// buttonRuleList says in words what was sent: which faders were set to 1,
// which to 0 and which swatches were recalled.
func buttonRuleList(sent buttonRuleSent) string {
	times := func(what string, number int, n uint8) string {
		if n > 1 {
			return fmt.Sprintf("%s %d (%d times)", what, number, n)
		}
		return fmt.Sprintf("%s %d", what, number)
	}
	var up, down, recalled, list []string
	for channel := 1; channel <= buttonRuleFaders; channel++ {
		if n := sent[buttonRuleFader(channel, true)]; n > 0 {
			up = append(up, times("fader", channel, n))
		}
		if n := sent[buttonRuleFader(channel, false)]; n > 0 {
			down = append(down, times("fader", channel, n))
		}
	}
	for swatch := 1; swatch <= buttonRuleSwatches; swatch++ {
		if n := sent[buttonRuleSwatch(swatch)]; n > 0 {
			recalled = append(recalled, times("swatch", swatch, n))
		}
	}
	if len(up) > 0 {
		list = append(list, strings.Join(up, ", ")+" at 1")
	}
	if len(down) > 0 {
		list = append(list, strings.Join(down, ", ")+" at 0")
	}
	if len(recalled) > 0 {
		list = append(list, "the recall of "+strings.Join(recalled, ", "))
	}
	if len(list) == 0 {
		return "nothing"
	}
	return strings.Join(list, "; ")
}

// buttonRuleSweep calls the handler for every pair of buttons held before
// and buttons held now, out of the first `statuses` values of the byte,
// on a game that is paused or not, whose last press went to the first
// fader of a pair (0) or the second (1), and with the eel steering itself
// or not. It returns how many of the calls were a press of each button,
// first of the calls that report the stick where it was and then of those
// that report it somewhere else.
//
// The game is put in that state before every call, by writing what the
// handler remembers, so each call is tried by itself. The stick a call
// reports is a different one of its 16 values from one call to the next,
// buttonRuleStickMoved in the buttons held before says whether the game
// held that stick already, and where the eel steers itself that is by
// turns a demo, a lost joystick and both.
func buttonRuleSweep(t *testing.T, statuses int, paused bool, last int, steersItself bool) (presses [2][len(buttonRuleButtons)]int) {
	numbers := buttonRuleMessages(t)

	// The handler needs a log, Chromatik's OSC and nothing else of a game.
	// The log is one that turns a line down before it is put together:
	// the handler logs every call.
	chromatik := listenLoopback(t)
	client := testOscClient(t, chromatik.LocalAddr().(*net.UDPAddr).Port)
	g := &Game{
		Config:    &config.Config{Logger: slog.New(slog.DiscardHandler)},
		Chromatik: &Chromatik{OscClient: client},
	}

	few := reportFew(t)
	buf := make([]byte, 512)
	for b := range statuses {
		// One datagram that does not arrive must not hold the test up for
		// good.
		_ = chromatik.SetReadDeadline(time.Now().Add(30 * time.Second))
		for n := range statuses {
			before, now := controller.ButtonStatus(b), controller.ButtonStatus(n)
			stick := controller.Direction((b + n) % stickValues)
			// The game already held the stick the call reports, or it held
			// one that is never the same.
			stickBefore, moved, stickIs := stick, 0, "where it was"
			if before&buttonRuleStickMoved != 0 {
				stickBefore, moved, stickIs = controller.Direction((b+n+5)%stickValues), 1, "somewhere else"
			}
			was := inputOf(stickBefore, false, false)
			if steersItself {
				was = inputOf(stickBefore, b%3 != 1, b%3 != 0)
			}

			g.prevButtons = before
			g.paused.Store(paused)
			g.buttonIndex = last
			g.input.Swap(was)

			g.HandleControllerState(controller.ControllerState{Direction: stick, ButtonStatus: now})

			// The handler has sent what it sends by the time it returns,
			// on the socket the end mark follows it on, so everything
			// that arrives before the mark is the handler's.
			if err := client.SendFloat(buttonRuleEndAddress, 0); err != nil {
				t.Fatalf("sending the end mark: %v", err)
			}
			var sent buttonRuleSent
			var strange []string
			for {
				size, _, err := chromatik.ReadFromUDPAddrPort(buf)
				if err != nil {
					t.Fatalf("with %s held before and %s now: reading what was sent: %v", buttonRuleHeld(before), buttonRuleHeld(now), err)
				}
				number, known := numbers[string(buf[:size])]
				if !known {
					strange = append(strange, fmt.Sprintf("%q", buf[:size]))
					continue
				}
				if number == buttonRuleEnd {
					break
				}
				sent[number]++
			}

			press := buttonRulePress(before, now)
			wantPaused, wantLast, wantSent := buttonRuleWant(press, paused, last)
			pressOf := "which is no press"
			if press >= 0 {
				presses[moved][press]++
				pressOf = "which is a press of " + buttonRuleButtons[press].name
			}
			what := func() string {
				return fmt.Sprintf("with %s held before and %s now, %s, the stick reported %s, in a game that was %s, had its last press on fader %d of a pair and had as input %s",
					buttonRuleHeld(before), buttonRuleHeld(now), pressOf, stickIs, buttonRulePaused(paused), last+1, describeInput(was))
			}
			if len(strange) > 0 {
				few.Errorf("%s: the handler sent %d datagrams that are no message of a press: %s", what(), len(strange), strings.Join(strange, " "))
			}
			if sent != wantSent {
				few.Errorf("%s: the handler sent %s, want %s", what(), buttonRuleList(sent), buttonRuleList(wantSent))
			}
			if got := g.paused.Load(); got != wantPaused {
				few.Errorf("%s: the game is %s afterwards, want it %s", what(), buttonRulePaused(got), buttonRulePaused(wantPaused))
			}
			if g.buttonIndex != wantLast {
				few.Errorf("%s: the handler remembers fader %d of a pair as the last one, want fader %d", what(), g.buttonIndex+1, wantLast+1)
			}
			// What it remembers of the bits that are no button cannot
			// matter: every value of them is tried as held before, and
			// the rule does not look at them.
			if got := g.prevButtons & allFive; got != now&allFive {
				few.Errorf("%s: of the five buttons the handler remembers %s as held, want those held now", what(), buttonRuleHeld(got))
			}
			if got := g.input.Load(); got != inputOf(stick, false, false) {
				few.Errorf("%s: the input state is %s afterwards, want %s", what(), describeInput(got), describeInput(inputOf(stick, false, false)))
			}
		}
	}
	return presses
}

// TestButtonRuleForEveryStateBeforeAndAfter calls the handler for all 256
// x 256 pairs of buttons held before and buttons held now, in each of
// eight kinds of game: paused or running, the last press on the first or
// the second fader of a pair, the eel steering itself or a player
// steering. See the top of this file for the rule every call is held to.
//
// How many of the 65,536 pairs are a press of each button is pinned, so
// that a rule that calls too little a press, or too much, cannot pass.
// Looking at the five buttons only, 32 x 32 pairs: red went down in 256
// of them, green and not red in 192, blue and neither of those in 144,
// yellow and none of those in 108 and white and no color in 81, of which
// 16, 8, 4, 2 and 1 have all five held and are no press. That leaves 240,
// 184, 140, 106 and 80, 750 presses in the 1,024 pairs. The other three
// bits of the byte change nothing, and there are 8 x 8 pairs of them. One
// of those bits says whether the call reports the stick where it was, and
// the counts are pinned for each half: 32 times those numbers of presses
// with the stick still, and as many with a stick that moved.
//
// Under the race detector, which has nothing to find in a single
// goroutine and makes the run several times longer, the top two bits of
// the byte stay clear: 64 x 64 pairs, of the five buttons and the one bit
// that says whether the stick moved. The run without it is the one that
// tries all 256 x 256.
func TestButtonRuleForEveryStateBeforeAndAfter(t *testing.T) {
	statuses := 256
	if raceDetector {
		statuses = 64
		t.Log("the race detector is on: trying 64 x 64 pairs, the five buttons and one other bit of the byte; all 256 x 256 need a run without -race")
	}
	if allFive != 0b11111 {
		t.Fatalf("the five buttons are the bits %b, want the low five, which the first 32 values of the byte cover", int(allFive))
	}
	if buttonRuleStickMoved&allFive != 0 || int(buttonRuleStickMoved) >= statuses {
		t.Fatalf("the bit %08b that says whether the stick moved is one of the five buttons, or is not among the first %d values of the byte",
			int(buttonRuleStickMoved), statuses)
	}
	// The pairs of the bits that are no button, half of them with the bit
	// clear before that says whether the stick moved and half with it set.
	half := (statuses / 32) * (statuses / 32) / 2
	wantHalf := [len(buttonRuleButtons)]int{half * 240, half * 184, half * 140, half * 106, half * 80}
	want := [2][len(buttonRuleButtons)]int{wantHalf, wantHalf}

	for _, paused := range []bool{false, true} {
		for last := range 2 {
			for _, steersItself := range []bool{false, true} {
				steering := "a player steering"
				if steersItself {
					steering = "the eel steering itself"
				}
				name := fmt.Sprintf("%s, last press on fader %d of a pair, %s", buttonRulePaused(paused), last+1, steering)
				t.Run(name, func(t *testing.T) {
					// Each of the eight has a game and sockets of its own,
					// so they can run side by side.
					t.Parallel()
					presses := buttonRuleSweep(t, statuses, paused, last, steersItself)
					if presses != want {
						t.Errorf("of the %d x %d pairs, %d were a press of red, green, blue, yellow and white with the stick reported where it was and %d with it somewhere else, want %d of each",
							statuses, statuses, presses[0], presses[1], wantHalf)
					}
				})
			}
		}
	}
}
