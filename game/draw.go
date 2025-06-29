package game

import (
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/jsimonetti/go-artnet/packet"
)

const pixelCount = 170

func (g *Game) Draw(wg *sync.WaitGroup) {
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
	prevColor := RandomColor()

	for {
		color := g.PrimaryColor
		if prevColor == color {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		prevColor = color

		data := [512]byte{}
		for i := 0; i < pixelCount; i++ {
			if color == Black {
				hue := float64(i) * 360.0 / float64(pixelCount)
				c := hsvToRGB(hue, 1.0, 1.0)
				data[i*3+0] = c[0]
				data[i*3+1] = c[1]
				data[i*3+2] = c[2]
			} else {
				data[i*3+0] = color[0]
				data[i*3+1] = color[1]
				data[i*3+2] = color[2]
			}
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

	wg.Done()
}

// hsvToRGB converts hue (0-360), saturation (0-1), value (0-1) to RGB (0-255).
func hsvToRGB(h, s, v float64) Color {
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

	return Color{
		byte((rf + m) * 255),
		byte((gf + m) * 255),
		byte((bf + m) * 255),
	}
}
