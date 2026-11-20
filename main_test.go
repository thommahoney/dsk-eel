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

	// The game resolves "<address>:<port>" to find where to send frames.
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
