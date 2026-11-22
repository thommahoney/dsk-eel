package main

import (
	"net"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// TestDefaultChromatikAddress starts from an empty command line, as when
// the program is run without --chromatik, and checks that the default
// Art-Net destination is the loopback address and that a datagram can
// really be sent to it. Only loopback sockets on ports the kernel picks
// are used.
func TestDefaultChromatikAddress(t *testing.T) {
	flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err := parseFlags(flags, nil)
	if err != nil {
		t.Fatalf("parseFlags with no arguments: %v", err)
	}

	loopback := net.IPv4(127, 0, 0, 1)
	if !cfg.ArtNetDest.Equal(loopback) {
		t.Errorf("default --chromatik is %q (% x), want 127.0.0.1", cfg.ArtNetDest, []byte(cfg.ArtNetDest))
	}
	if got := flags.Lookup("chromatik").DefValue; got != "127.0.0.1" {
		t.Errorf("--help shows the --chromatik default as %q, want 127.0.0.1", got)
	}

	// The address must be usable as a UDP destination.
	if _, err := net.ResolveUDPAddr("udp", net.JoinHostPort(cfg.ArtNetDest.String(), "6454")); err != nil {
		t.Errorf("default --chromatik does not resolve: %v", err)
	}

	sink, err := net.ListenUDP("udp", &net.UDPAddr{IP: loopback})
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	defer sink.Close()
	src, err := net.ListenUDP("udp", &net.UDPAddr{IP: loopback})
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	defer src.Close()

	dst := &net.UDPAddr{IP: cfg.ArtNetDest, Port: sink.LocalAddr().(*net.UDPAddr).Port}
	if _, err := src.WriteTo([]byte("frame"), dst); err != nil {
		t.Fatalf("send to the default --chromatik address: %v", err)
	}
	buf := make([]byte, 16)
	_ = sink.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := sink.ReadFromUDP(buf)
	if err != nil || string(buf[:n]) != "frame" {
		t.Errorf("datagram sent to the default --chromatik address did not arrive: got %q, err %v", buf[:n], err)
	}
}

// TestChromatikFlag checks that an address given on the command line
// replaces the default.
func TestChromatikFlag(t *testing.T) {
	flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err := parseFlags(flags, []string{"--chromatik", "10.0.0.5"})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if !cfg.ArtNetDest.Equal(net.IPv4(10, 0, 0, 5)) {
		t.Errorf("--chromatik 10.0.0.5 gave %q", cfg.ArtNetDest)
	}
}

// TestListenFlag checks that --listen is empty unless it is given. Empty
// tells the game to pick the local Art-Net address from the route to
// --chromatik.
func TestListenFlag(t *testing.T) {
	flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err := parseFlags(flags, nil)
	if err != nil {
		t.Fatalf("parseFlags with no arguments: %v", err)
	}
	if cfg.ListenSubnet != "" {
		t.Errorf("default --listen is %q, want it empty", cfg.ListenSubnet)
	}

	flags = pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err = parseFlags(flags, []string{"--listen", "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("parseFlags: %v", err)
	}
	if cfg.ListenSubnet != "10.0.0.0/24" {
		t.Errorf("--listen 10.0.0.0/24 gave %q", cfg.ListenSubnet)
	}
}

// TestAbletonFlag checks the three forms of --ableton: left out or given
// empty, the address is empty, which leaves Ableton off; given, it is the
// address to send to.
func TestAbletonFlag(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"not given", nil, ""},
		{"given", []string{"--ableton", "10.0.0.7:9000"}, "10.0.0.7:9000"},
		{"empty", []string{"--ableton", ""}, ""},
	} {
		flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
		cfg, err := parseFlags(flags, c.args)
		if err != nil {
			t.Fatalf("%s: parseFlags: %v", c.name, err)
		}
		if cfg.AbletonDest != c.want {
			t.Errorf("%s: the Ableton address is %q, want %q", c.name, cfg.AbletonDest, c.want)
		}
	}
}

// TestOscDestFlag checks that --osc-dest is empty unless it is given. Empty
// tells the game to send OSC to the --chromatik address.
func TestOscDestFlag(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"not given", nil, ""},
		{"given", []string{"--osc-dest", "10.0.0.7"}, "10.0.0.7"},
	} {
		flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
		cfg, err := parseFlags(flags, c.args)
		if err != nil {
			t.Fatalf("%s: parseFlags: %v", c.name, err)
		}
		if cfg.OscDest != c.want {
			t.Errorf("%s: the OSC host is %q, want %q", c.name, cfg.OscDest, c.want)
		}
	}
}

// TestKeyboardFlag: the keyboard is off unless asked for.
func TestKeyboardFlag(t *testing.T) {
	cfg, err := parseFlags(pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError), nil)
	if err != nil {
		t.Fatalf("parseFlags with no arguments: %v", err)
	}
	if cfg.Keyboard {
		t.Error("the keyboard is on with no --keyboard flag")
	}

	cfg, err = parseFlags(pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError), []string{"--keyboard"})
	if err != nil {
		t.Fatalf("parseFlags --keyboard: %v", err)
	}
	if !cfg.Keyboard {
		t.Error("--keyboard did not turn the keyboard on")
	}
}
