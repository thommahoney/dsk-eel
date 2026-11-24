package game

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

type Chromatik struct {
	ArtNetConnection *net.UDPConn
	ArtNetNode       *net.UDPAddr
	OscClient        *OscClient
}

func InitChromatik(c *config.Config) (*Chromatik, error) {
	return initChromatik(c, unspecifiedReachesThisMachine)
}

// initChromatik is InitChromatik for a system on which what is sent to the
// unspecified address arrives on this machine or does not, as
// unspecifiedReaches says. The system's own answer is the constant
// unspecifiedReachesThisMachine; a test can give the other one.
func initChromatik(c *config.Config, unspecifiedReaches bool) (*Chromatik, error) {
	// To16 is nil for anything that is not a 4 or 16 byte address. Sends
	// to such a destination would all fail, and Draw only logs a warning
	// and plays on, so it is refused here.
	if c.ArtNetDest.To16() == nil {
		return nil, fmt.Errorf("invalid Chromatik address %q (set with --chromatik)", c.ArtNetDest.String())
	}
	// So is the unspecified address where nothing can be sent to it. That
	// is asked first: the choice of a local address below would not
	// notice, because connecting a socket to the address works there.
	if err := unspecifiedDestError("Chromatik address", "--chromatik", c.ArtNetDest, unspecifiedReaches); err != nil {
		return nil, err
	}

	// The local address the Art-Net socket is bound to decides which
	// interface frames leave from. Without --listen it is the address the
	// system would use to reach Chromatik anyway.
	var ip net.IP
	if c.ListenSubnet == "" {
		var err error
		ip, err = localAddrFor(c.ArtNetDest)
		if err != nil {
			return nil, err
		}
	} else {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return nil, fmt.Errorf("error listing local addresses: %w", err)
		}
		ip, err = localAddrInSubnet(c.ListenSubnet, addrs)
		if err != nil {
			return nil, err
		}
		// A socket bound to a loopback address can only send to this
		// machine. To anywhere else every send would fail, and Draw only
		// logs a warning and plays on, so that pairing is refused here.
		if ip.IsLoopback() && !onThisMachine(c.ArtNetDest, addrs) {
			return nil, fmt.Errorf("the local address %s (the one in the listen subnet %s, set with --listen) is a loopback address and cannot send to the Chromatik address %s (set with --chromatik), which is not on this machine; leave --listen out, or give the subnet of the network Chromatik is on",
				ip, c.ListenSubnet, c.ArtNetDest)
		}
		// On Windows it cannot send to this machine's other addresses
		// either, and that is refused for the same reason.
		if ip.IsLoopback() && !loopbackReachesOwnAddresses && !c.ArtNetDest.IsLoopback() && !c.ArtNetDest.IsUnspecified() {
			return nil, fmt.Errorf("the local address %s (the one in the listen subnet %s, set with --listen) is a loopback address, and on this system it cannot send to the Chromatik address %s (set with --chromatik) although that is this machine's own; leave --listen out",
				ip, c.ListenSubnet, c.ArtNetDest)
		}
	}

	// A socket bound to an IPv4 address cannot send to an IPv6 destination,
	// nor the other way round. Every send would fail, and Draw only logs
	// a warning and plays on, so a mix is refused here as well. It can only
	// happen with --listen: an address chosen from the route to the
	// destination is of the destination's kind.
	if (ip.To4() == nil) != (c.ArtNetDest.To4() == nil) {
		return nil, fmt.Errorf("the Chromatik address %s (set with --chromatik) and the local address %s (the one in the listen subnet %s, set with --listen) must both be IPv4 or both be IPv6",
			c.ArtNetDest, ip, c.ListenSubnet)
	}

	node := &net.UDPAddr{IP: c.ArtNetDest, Port: artnetPort}

	// Frames go to the Art-Net port and come from a port the system
	// picks. The Art-Net port belongs to whoever listens for Art-Net, and
	// on Chromatik's own machine that is Chromatik: a game that took it
	// first kept Chromatik from starting its Art-Net input, and on Windows
	// a game that took it second got its own frames back. Chromatik does
	// not look at the port a frame comes from.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: ip})
	if err != nil {
		return nil, fmt.Errorf("error opening udp: %w", err)
	}
	if c.Logger != nil {
		c.Logger.Info("Art-Net socket open", "from", conn.LocalAddr().String(), "to", node.String())
	}

	// Chromatik takes OSC on the machine it takes Art-Net on, unless
	// --osc-dest says otherwise.
	oscHost := c.OscDest
	if oscHost == "" {
		oscHost = c.ArtNetDest.String()
	}
	oscDest := net.JoinHostPort(oscHost, strconv.Itoa(c.OscPort))
	oscClient, err := NewOscClient(oscDest)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("invalid OSC destination %q (--osc-dest, --osc-port): %s", oscDest, err)
	}
	// --osc-dest can name the unspecified address too, as an address or
	// as a host name that stands for it, so the address the client sends
	// to is what is asked about. Without --osc-dest it is the Chromatik
	// address, which got this far.
	if err := unspecifiedDestError("OSC address", "--osc-dest", oscClient.dest.Addr().AsSlice(), unspecifiedReaches); err != nil {
		conn.Close()
		oscClient.Close()
		return nil, err
	}

	return &Chromatik{ArtNetConnection: conn, ArtNetNode: node, OscClient: oscClient}, nil
}

// unspecifiedDestError returns the error for a destination that is the
// unspecified address, 0.0.0.0 or ::, on a system where nothing can be
// sent to it: every send would fail, and Draw only logs a warning and
// plays on. On a system where what is sent there arrives on this machine,
// which unspecifiedReaches says, and for every other destination it
// returns nil. what and flag name the destination in the message, which
// says what to give for a Chromatik on this machine.
func unspecifiedDestError(what, flag string, dest net.IP, unspecifiedReaches bool) error {
	if unspecifiedReaches || !dest.IsUnspecified() {
		return nil
	}
	thisMachine := "127.0.0.1"
	if dest.To4() == nil {
		thisMachine = "::1"
	}
	return fmt.Errorf("the %s %s (set with %s) is the unspecified address, and on this system nothing can be sent to it; for a Chromatik on this machine give %s %s",
		what, dest, flag, flag, thisMachine)
}

// localAddrFor returns the local address this machine sends from to reach
// dest: the address of the interface its routes choose for dest, or dest
// itself when dest is one of this machine's own addresses. It finds out by
// connecting a UDP socket, which looks up the route and sends nothing.
func localAddrFor(dest net.IP) (net.IP, error) {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: dest, Port: artnetPort})
	if err != nil {
		return nil, fmt.Errorf("cannot find a local address to reach the Chromatik address %s (set with --chromatik); if the network is up, name the local subnet with --listen: %w", dest, err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP, nil
}

// onThisMachine reports whether a datagram sent to dest stays on this
// machine: dest is a loopback address, the unspecified address (on a
// system where that cannot be sent to, InitChromatik has refused it before
// it asks here) or one of addrs, the machine's own addresses as
// net.InterfaceAddrs returns them.
func onThisMachine(dest net.IP, addrs []net.Addr) bool {
	if dest.IsLoopback() || dest.IsUnspecified() {
		return true
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.Equal(dest) {
			return true
		}
	}
	return false
}

// localAddrInSubnet returns the first of addrs that lies inside subnet,
// which is in CIDR notation such as "10.0.0.0/24" or "fd00::/64". This is
// the address the Art-Net socket is bound to, so that frames leave through
// the interface on the lighting network. addrs is what net.InterfaceAddrs
// returns; entries that are not a *net.IPNet are skipped. If nothing
// matches, the error lists every address that was offered.
func localAddrInSubnet(subnet string, addrs []net.Addr) (net.IP, error) {
	_, network, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil, fmt.Errorf("invalid listen subnet %q (set with --listen): %w", subnet, err)
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && network.Contains(ipnet.IP) {
			return ipnet.IP, nil
		}
	}

	offered := make([]string, len(addrs))
	for i, addr := range addrs {
		offered[i] = addr.String()
	}
	return nil, fmt.Errorf("no local address is in the listen subnet %s (set with --listen); local addresses: [%s]",
		subnet, strings.Join(offered, ", "))
}

func (c *Chromatik) ArtNetSend(bytes []byte) (int, error) {
	return c.ArtNetConnection.WriteTo(bytes, c.ArtNetNode)
}

var buttonToOSCAddress = map[controller.ButtonStatus][]string{
	controller.Btn_Red:    {"/lx/mixer/channel/1/fader", "/lx/mixer/channel/2/fader"},
	controller.Btn_Green:  {"/lx/mixer/channel/3/fader", "/lx/mixer/channel/4/fader"},
	controller.Btn_Blue:   {"/lx/mixer/channel/5/fader", "/lx/mixer/channel/6/fader"},
	controller.Btn_Yellow: {"/lx/mixer/channel/7/fader", "/lx/mixer/channel/8/fader"},
	controller.Btn_White:  {"/lx/mixer/channel/9/fader"},
}

func (c *Chromatik) OscSend(address string, value float32) error {
	if address == "" {
		return nil
	}

	err := c.OscClient.SendFloat(address, value)
	if err != nil {
		return fmt.Errorf("failed to send OSC message: %s", err)
	}

	return nil
}

func (c *Chromatik) OscSend_swatch(swatch int) error {
	err := c.OscClient.SendFloat(fmt.Sprintf("/lx/palette/swatches/%d/recall", swatch), 1.0)
	if err != nil {
		return fmt.Errorf("failed to send OSC swatch message: %s", err)
	}

	return nil
}
