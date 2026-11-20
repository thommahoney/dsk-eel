package game

import (
	"net"
	"strings"
	"testing"

	"github.com/thommahoney/dsk-eel/config"
)

// loopbackChromatik calls InitChromatik with every address on loopback and
// OSC addressed to oscPort. InitChromatik binds the fixed Art-Net port, so
// its sockets are closed again when the test ends. If something else on
// this machine already has that port, which says nothing about the code,
// the test is skipped.
func loopbackChromatik(t *testing.T, oscPort int) *Chromatik {
	t.Helper()
	c, err := InitChromatik(&config.Config{
		ArtNetDest:   net.IPv4(127, 0, 0, 1),
		ListenSubnet: "127.0.0.1/8",
		OscDest:      "127.0.0.1",
		OscPort:      oscPort,
	})
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			t.Skipf("the Art-Net port is in use on this machine: %v", err)
		}
		t.Fatalf("InitChromatik: %v", err)
	}
	t.Cleanup(func() {
		c.ArtNetConnection.Close()
		c.OscClient.Close()
	})
	return c
}

// Art-Net's UDP port is 6454. InitChromatik binds its socket to that port
// and addresses frames to that port of the destination. Nothing is sent
// here; only the addresses are checked.
func TestInitChromatikUsesArtNetPort(t *testing.T) {
	oscPort := listenLoopback(t).LocalAddr().(*net.UDPAddr).Port
	c := loopbackChromatik(t, oscPort)

	local := c.ArtNetConnection.LocalAddr().(*net.UDPAddr)
	if !local.IP.Equal(net.IPv4(127, 0, 0, 1)) || local.Port != 6454 {
		t.Errorf("the Art-Net socket is bound to %v, want 127.0.0.1:6454", local)
	}
	if node := c.ArtNetNode; !node.IP.Equal(net.IPv4(127, 0, 0, 1)) || node.Port != 6454 {
		t.Errorf("frames are addressed to %v, want 127.0.0.1:6454", node)
	}
}
