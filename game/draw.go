package game

import (
	"fmt"
	"math"
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
		// c_idx := 0

		// var data = [512]byte{}
		// for i := 0; i < 510; i++ {
		// 	if i == 0 {
		// 		data[0] = 0xff
		// 		data[1] = 0x00
		// 		data[2] = 0x00
		// 		i = 2
		// 		continue
		// 	}
		// 	if i == 507 {
		// 		data[507] = 0xff
		// 		data[508] = 0x00
		// 		data[509] = 0x00
		// 		i = 509
		// 		continue
		// 	}
		// 	data[i] = color[c_idx]
		// 	c_idx = (c_idx + 1) % 3
		// }

		const pixelCount = 170
		data := [512]byte{}

		for i := 0; i < pixelCount; i++ {
			hue := float64(i) * 360.0 / float64(pixelCount)
			r, g, b := hsvToRGB(hue, 1.0, 1.0)
			data[i*3+0] = r
			data[i*3+1] = g
			data[i*3+2] = b
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

// hsvToRGB converts hue (0-360), saturation (0-1), value (0-1) to RGB (0-255).
func hsvToRGB(h, s, v float64) (r, g, b byte) {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c

	var rf, gf, bf float64

	switch {
	case h < 60:
		rf, gf, bf = c, x, 0
	case h < 120:
		rf, gf, bf = x, c, 0
	case h < 180:
		rf, gf, bf = 0, c, x
	case h < 240:
		rf, gf, bf = 0, x, c
	case h < 300:
		rf, gf, bf = x, 0, c
	default:
		rf, gf, bf = c, 0, x
	}

	r = byte((rf + m) * 255)
	g = byte((gf + m) * 255)
	b = byte((bf + m) * 255)
	return
}
