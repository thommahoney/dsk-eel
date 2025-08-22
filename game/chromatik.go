package game

import (
	"fmt"
	"net"

	"github.com/hypebeast/go-osc/osc"
	"github.com/jsimonetti/go-artnet/packet"
	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

type Chromatik struct {
	ArtNetConnection *net.UDPConn
	ArtNetNode       *net.UDPAddr
	OscClient        *osc.Client
}

func InitChromatik(c *config.Config) (*Chromatik, error) {
	_, cidrnet, _ := net.ParseCIDR(c.ListenSubnet)

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Printf("error getting ips: %s\n", err)
	}

	var ip net.IP

	for _, addr := range addrs {
		ip = addr.(*net.IPNet).IP
		if cidrnet.Contains(ip) {
			break
		}
	}

	dst := fmt.Sprintf("%s:%d", c.ArtNetDest, packet.ArtNetPort)
	node, err := net.ResolveUDPAddr("udp", dst)
	if err != nil {
		return nil, fmt.Errorf("error resolving udp dst: %s", err)
	}
	src := fmt.Sprintf("%s:%d", ip, packet.ArtNetPort)
	localAddr, err := net.ResolveUDPAddr("udp", src)
	if err != nil {
		return nil, fmt.Errorf("error resolving udp src: %s", err)
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("error opening udp: %s", err)
	}

	oscClient := osc.NewClient(c.OscDest, c.OscPort)

	return &Chromatik{ArtNetConnection: conn, ArtNetNode: node, OscClient: oscClient}, nil
}

func (c *Chromatik) ArtNetSend(bytes []byte) (int, error) {
	return c.ArtNetConnection.WriteTo(bytes, c.ArtNetNode)
}

var buttonIndex = 0;
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

	msg := osc.NewMessage(address)
	msg.Append(value)

	err := c.OscClient.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send OSC message: %s", err)
	}

	return nil
}

func (c *Chromatik) OscSend_swatch(swatch int) error {
	msg := osc.NewMessage(fmt.Sprintf("/lx/palette/swatches/%d/recall", swatch))
	msg.Append(float32(1.0))

	err := c.OscClient.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send OSC swatch message: %s", err)
	}

	return nil
}
