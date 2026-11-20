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
	// To16 is nil for anything that is not a 4 or 16 byte address. Sends
	// to such a destination would all fail, and Draw does not report
	// failed sends, so it is refused here.
	if c.ArtNetDest.To16() == nil {
		return nil, fmt.Errorf("invalid Chromatik address %q (set with --chromatik)", c.ArtNetDest.String())
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("error listing local addresses: %w", err)
	}
	ip, err := localAddrInSubnet(c.ListenSubnet, addrs)
	if err != nil {
		return nil, err
	}

	// A socket bound to an IPv4 address cannot send to an IPv6 destination,
	// nor the other way round. Every send would fail, and Draw does not
	// report failed sends, so a mix is refused here as well.
	if (ip.To4() == nil) != (c.ArtNetDest.To4() == nil) {
		return nil, fmt.Errorf("the Chromatik address %s (set with --chromatik) and the local address %s (the one in the listen subnet %s, set with --listen) must both be IPv4 or both be IPv6",
			c.ArtNetDest, ip, c.ListenSubnet)
	}

	node := &net.UDPAddr{IP: c.ArtNetDest, Port: artnetPort}
	localAddr := &net.UDPAddr{IP: ip, Port: artnetPort}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("error opening udp: %w", err)
	}

	oscDest := net.JoinHostPort(c.OscDest, strconv.Itoa(c.OscPort))
	oscClient, err := NewOscClient(oscDest)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("invalid OSC destination %q (--osc-dest, --osc-port): %s", oscDest, err)
	}

	return &Chromatik{ArtNetConnection: conn, ArtNetNode: node, OscClient: oscClient}, nil
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
	// controller.Btn_None:   {},
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
