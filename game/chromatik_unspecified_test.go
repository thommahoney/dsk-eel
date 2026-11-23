package game

import (
	"net"
	"strings"
	"testing"

	"github.com/thommahoney/dsk-eel/config"
)

// initChromatikOn calls initChromatik as a system would on which what is
// sent to the unspecified address arrives on this machine, or does not, and
// closes the sockets of what it returns when the test ends.
func initChromatikOn(t *testing.T, c *config.Config, unspecifiedReaches bool) (*Chromatik, error) {
	t.Helper()
	chromatik, err := initChromatik(c, unspecifiedReaches)
	if chromatik != nil && chromatik.ArtNetConnection != nil {
		t.Cleanup(func() { chromatik.ArtNetConnection.Close() })
	}
	if chromatik != nil && chromatik.OscClient != nil {
		t.Cleanup(func() { chromatik.OscClient.Close() })
	}
	return chromatik, err
}

// Only the unspecified address is refused, and only on a system where
// nothing can be sent to it. The message names the flag it was given the
// destination by and what to give that flag for a Chromatik on this
// machine: the loopback address of the same kind, IPv4 or IPv6.
func TestUnspecifiedDestError(t *testing.T) {
	cases := []struct {
		name               string
		dest               net.IP
		unspecifiedReaches bool
		// wantAdvice is what the error must tell the user to give; empty
		// means the destination must be accepted.
		wantAdvice string
	}{
		{"0.0.0.0 where nothing can be sent to it", net.IPv4zero, false, "--flag 127.0.0.1"},
		{"0.0.0.0 held in four bytes", net.IPv4zero.To4(), false, "--flag 127.0.0.1"},
		{"0.0.0.0 written as an IPv6 address", net.ParseIP("::ffff:0.0.0.0"), false, "--flag 127.0.0.1"},
		{":: where nothing can be sent to it", net.IPv6unspecified, false, "--flag ::1"},
		{"0.0.0.0 where it is this machine", net.IPv4zero, true, ""},
		{":: where it is this machine", net.IPv6unspecified, true, ""},
		{"loopback", net.IPv4(127, 0, 0, 1), false, ""},
		{"IPv6 loopback", net.IPv6loopback, false, ""},
		{"another machine", net.IPv4(10, 0, 0, 5), false, ""},
		{"another machine over IPv6", net.ParseIP("fd00::5"), false, ""},
		{"broadcast", net.IPv4bcast, false, ""},
		{"an address that only starts with zeros", net.IPv4(0, 0, 0, 1), false, ""},
	}
	for _, c := range cases {
		err := unspecifiedDestError("Some address", "--flag", c.dest, c.unspecifiedReaches)
		if c.wantAdvice == "" {
			if err != nil {
				t.Errorf("%s: %v was refused, want it accepted: %v", c.name, c.dest, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: %v was accepted, want an error", c.name, c.dest)
			continue
		}
		for _, want := range []string{"Some address", "unspecified", "(set with --flag)", c.wantAdvice} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the error does not say %q: %v", c.name, want, err)
			}
		}
	}
}

// On a system where nothing can be sent to the unspecified address, which
// Windows is, the game must not start with it as a destination: every
// Art-Net frame and OSC message would be lost, the frames without a word.
// That goes for --chromatik, with or without --listen, and for --osc-dest.
// The error names the one flag that has to change and what to give it.
//
// The same command lines start as before on a system where the address is
// this machine, and other destinations are not touched on either kind.
func TestInitChromatikRefusesUnspecifiedDestination(t *testing.T) {
	loopback := net.IPv4(127, 0, 0, 1)
	cases := []struct {
		name         string
		listenSubnet string
		dest         net.IP
		oscDest      string
		// wantAdvice is what the error must tell the user to give where
		// the unspecified address cannot be sent to; empty means the game
		// must start there too.
		wantAdvice string
		// notFlag is a flag the error must leave out, because that flag
		// is not the one to change.
		notFlag string
		// needsIPv6 marks a case that opens an IPv6 socket on the way.
		needsIPv6 bool
		// onlyRefusal marks a case that is only tried where the address
		// is refused: where it is not, starting needs a route this
		// machine may not have.
		onlyRefusal bool
	}{
		{name: "--chromatik 0.0.0.0", dest: net.IPv4zero, oscDest: "127.0.0.1", wantAdvice: "--chromatik 127.0.0.1", notFlag: "--osc-dest"},
		{name: "--chromatik 0.0.0.0 and no --osc-dest", dest: net.IPv4zero, wantAdvice: "--chromatik 127.0.0.1", notFlag: "--osc-dest"},
		{name: "--chromatik 0.0.0.0 with a loopback --listen", listenSubnet: "127.0.0.1/24", dest: net.IPv4zero, oscDest: "127.0.0.1", wantAdvice: "--chromatik 127.0.0.1", notFlag: "--osc-dest"},
		{name: "--chromatik ::", dest: net.IPv6unspecified, oscDest: "127.0.0.1", wantAdvice: "--chromatik ::1", notFlag: "--osc-dest", onlyRefusal: true},
		{name: "--osc-dest 0.0.0.0", dest: loopback, oscDest: "0.0.0.0", wantAdvice: "--osc-dest 127.0.0.1", notFlag: "--chromatik"},
		{name: "--osc-dest ::", dest: loopback, oscDest: "::", wantAdvice: "--osc-dest ::1", notFlag: "--chromatik", needsIPv6: true},
		{name: "--chromatik 127.0.0.1", dest: loopback, oscDest: "127.0.0.1"},
		{name: "--chromatik 127.0.0.1 and no --osc-dest", dest: loopback},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.needsIPv6 {
				probe, err := net.ListenUDP("udp6", nil)
				if err != nil {
					t.Skipf("this machine opens no IPv6 socket: %v", err)
				}
				probe.Close()
			}
			cfg := chromatikConfig(c.listenSubnet, c.dest)
			cfg.OscDest = c.oscDest

			if !c.onlyRefusal {
				if _, err := initChromatikOn(t, cfg, true); err != nil {
					t.Errorf("where the unspecified address is this machine, the game did not start: %v", err)
				}
			}

			chromatik, err := initChromatikOn(t, cfg, false)
			if c.wantAdvice == "" {
				if err != nil {
					t.Errorf("where nothing can be sent to the unspecified address, the game did not start although no destination is that address: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("where nothing can be sent to the unspecified address, the game started; Art-Net goes to %v", chromatik.ArtNetNode)
			}
			if chromatik != nil {
				t.Errorf("a Chromatik was returned along with the error")
			}
			for _, want := range []string{"unspecified", c.wantAdvice} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the error does not say %q: %v", want, err)
				}
			}
			if strings.Contains(err.Error(), c.notFlag) {
				t.Errorf("the error names %s, which is not the flag to change: %v", c.notFlag, err)
			}
		})
	}
}
