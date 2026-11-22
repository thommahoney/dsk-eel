package game

import (
	"net"
	"testing"

	"github.com/thommahoney/dsk-eel/config"
)

// loopbackChromatik calls InitChromatik with every address on loopback and
// OSC addressed to oscPort, and closes its sockets again when the test
// ends.
func loopbackChromatik(t *testing.T, oscPort int) *Chromatik {
	t.Helper()
	c, err := InitChromatik(&config.Config{
		ArtNetDest:   net.IPv4(127, 0, 0, 1),
		ListenSubnet: "127.0.0.1/8",
		OscDest:      "127.0.0.1",
		OscPort:      oscPort,
	})
	if err != nil {
		t.Fatalf("InitChromatik: %v", err)
	}
	t.Cleanup(func() {
		c.ArtNetConnection.Close()
		c.OscClient.Close()
	})
	return c
}

// Art-Net's UDP port is 6454. InitChromatik addresses frames to that port
// of the destination. With the destination on this machine the port is
// Chromatik's, and the socket frames are sent from is bound to another
// one. Nothing is sent here; only the addresses are checked.
func TestInitChromatikAddressesFramesToArtNetPort(t *testing.T) {
	oscPort := listenLoopback(t).LocalAddr().(*net.UDPAddr).Port
	c := loopbackChromatik(t, oscPort)

	local := c.ArtNetConnection.LocalAddr().(*net.UDPAddr)
	if !local.IP.Equal(net.IPv4(127, 0, 0, 1)) || local.Port == 0 || local.Port == 6454 {
		t.Errorf("the Art-Net socket is bound to %v, want 127.0.0.1 and a port other than 6454", local)
	}
	if node := c.ArtNetNode; !node.IP.Equal(net.IPv4(127, 0, 0, 1)) || node.Port != 6454 {
		t.Errorf("frames are addressed to %v, want 127.0.0.1:6454", node)
	}
}
