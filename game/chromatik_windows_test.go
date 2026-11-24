package game

import (
	"net"
	"strings"
	"testing"
)

// On Windows the unspecified address is refused as a destination when the
// game starts, by the program as it is built here: for --chromatik, with
// and without --listen, and for --osc-dest. The message says what to give
// for a Chromatik on this machine.
func TestInitChromatikRefusesUnspecifiedDestinationOnWindows(t *testing.T) {
	loopback := net.IPv4(127, 0, 0, 1)
	cases := []struct {
		name         string
		listenSubnet string
		dest         net.IP
		oscDest      string
		wantAdvice   string
	}{
		{"--chromatik 0.0.0.0", "", net.IPv4zero, "", "--chromatik 127.0.0.1"},
		{"--chromatik 0.0.0.0 with a loopback --listen", "127.0.0.1/24", net.IPv4zero, "", "--chromatik 127.0.0.1"},
		{"--chromatik ::", "", net.IPv6unspecified, "", "--chromatik ::1"},
		{"--osc-dest 0.0.0.0", "", loopback, "0.0.0.0", "--osc-dest 127.0.0.1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := chromatikConfig(c.listenSubnet, c.dest)
			cfg.OscDest = c.oscDest
			chromatik, err := initChromatikForTest(t, cfg)
			if err == nil {
				t.Fatalf("the game started; Art-Net goes to %v", chromatik.ArtNetNode)
			}
			if chromatik != nil {
				t.Errorf("a Chromatik was returned along with the error")
			}
			for _, want := range []string{"unspecified", c.wantAdvice} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the error does not say %q: %v", want, err)
				}
			}
		})
	}
}

// This is why the address is refused. Started as a system that takes the
// unspecified address for this machine would start it, the game comes up
// on Windows too, and then every Art-Net frame and every OSC message
// fails. If a send here ever succeeds, Windows has changed and
// unspecifiedReachesThisMachine is to be looked at again.
//
// Without IPv6 on the machine the game does not come up for ::, and there
// is nothing to send.
func TestSendsToTheUnspecifiedAddressFailOnWindows(t *testing.T) {
	cases := []struct {
		name         string
		listenSubnet string
		dest         net.IP
	}{
		{"to 0.0.0.0", "", net.IPv4zero},
		{"to 0.0.0.0 with a loopback --listen", "127.0.0.1/24", net.IPv4zero},
		{"to ::", "", net.IPv6unspecified},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// With no --osc-dest, OSC goes to the same address.
			cfg := chromatikConfig(c.listenSubnet, c.dest)
			cfg.OscDest = ""
			chromatik, err := initChromatikOn(t, cfg, true)
			if err != nil {
				if c.dest.To4() == nil {
					t.Skipf("the game does not start for %v on this machine: %v", c.dest, err)
				}
				t.Fatalf("the game does not start for %v even with the refusal left out: %v", c.dest, err)
			}

			if _, err := chromatik.ArtNetSend([]byte("frame")); err == nil {
				t.Errorf("an Art-Net frame from %v to %v was sent without an error", chromatik.ArtNetConnection.LocalAddr(), chromatik.ArtNetNode)
			} else {
				t.Logf("Art-Net from %v to %v: %v", chromatik.ArtNetConnection.LocalAddr(), chromatik.ArtNetNode, err)
			}
			if err := chromatik.OscSend("/lx/mixer/channel/1/fader", 1); err == nil {
				t.Errorf("an OSC message to %v was sent without an error", c.dest)
			} else {
				t.Logf("OSC to %v: %v", c.dest, err)
			}
		})
	}
}
