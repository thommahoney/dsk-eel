package game

import (
	"fmt"
	"net"
	"time"

	"github.com/jsimonetti/go-artnet/packet"
)

func (g *Game) Draw() {
	_, cidrnet, _ := net.ParseCIDR(g.Config.ListenSubnet)

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

	dst := fmt.Sprintf("%s:%d", g.Config.ArtNetDest, packet.ArtNetPort)
	node, _ := net.ResolveUDPAddr("udp", dst)
	// lie about source
	src := fmt.Sprintf("%s:%d", ip, packet.ArtNetPort)
	localAddr, _ := net.ResolveUDPAddr("udp", src)

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		fmt.Printf("error opening udp: %s\n", err)
		return
	}

	var sequence uint8 = 0
	prevColor := Black

	for {
		color := g.PrimaryColor
		if prevColor == color {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		prevColor = color
		c_idx := 0

		var data = [512]byte{}
		for i := 0; i < 510; i++ {
			data[i] = color[c_idx]
			c_idx = (c_idx + 1) % 3
		}

		p := &packet.ArtDMXPacket{
			Sequence: sequence,
			SubUni:   0,
			Net:      0,
			Data:     data,
		}

		b, _ := p.MarshalBinary()
		_, _ = conn.WriteTo(b, node)

		sequence++
		p = &packet.ArtDMXPacket{
			Sequence: sequence,
			SubUni:   1,
			Net:      0,
			Data:     data,
		}

		b, _ = p.MarshalBinary()
		_, _ = conn.WriteTo(b, node)

		sequence++
		p = &packet.ArtDMXPacket{
			Sequence: sequence,
			SubUni:   2,
			Net:      0,
			Data:     data,
		}

		b, _ = p.MarshalBinary()
		_, _ = conn.WriteTo(b, node)

		sequence++
		p = &packet.ArtDMXPacket{
			Sequence: sequence,
			SubUni:   3,
			Net:      0,
			Data:     data,
		}

		b, _ = p.MarshalBinary()
		_, _ = conn.WriteTo(b, node)

		sequence++
		p = &packet.ArtDMXPacket{
			Sequence: sequence,
			SubUni:   4,
			Net:      0,
			Data:     data,
		}

		b, _ = p.MarshalBinary()
		_, _ = conn.WriteTo(b, node)

		sequence++
		p = &packet.ArtDMXPacket{
			Sequence: sequence,
			SubUni:   5,
			Net:      0,
			Data:     data,
		}

		b, _ = p.MarshalBinary()
		_, _ = conn.WriteTo(b, node)

		sequence++
		p = &packet.ArtDMXPacket{
			Sequence: sequence,
			SubUni:   6,
			Net:      0,
			Data:     data,
		}

		b, _ = p.MarshalBinary()
		_, _ = conn.WriteTo(b, node)
	}
}
