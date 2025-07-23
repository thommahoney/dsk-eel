package game

import (
	"math"

	"github.com/jsimonetti/go-artnet/packet"
)

func (g *Game) Draw() {
	eelBody := g.Eel.BodyPixels()
	pixels := make([]Color, len(g.Segments)*SegmentLength)
	for i := 0; i < len(pixels); i++ {
		if c, key := eelBody[i]; key {
			pixels[i] = c
		} else {
			pixels[i] = Color{0x69, 0x69, 0x69}
		}
	}

	var sequence uint8 = 0
	for univ := 0; univ < 7; univ++ {
		data := [512]byte{}

		max := 170
		if univ == 6 {
			max = 58
		}
		for i := 0; i < max; i++ {
			p := univ*170 + i
			data[i*3+0] = pixels[p][0]
			data[i*3+1] = pixels[p][1]
			data[i*3+2] = pixels[p][2]
		}

		p := &packet.ArtDMXPacket{
			Sequence: sequence,
			SubUni:   uint8(univ),
			Net:      0,
			Data:     data,
		}
		sequence++

		b, _ := p.MarshalBinary()
		_, _ = g.Chromatik.Send(b)
	}
}

// hsvToRGB converts hue (0-360), saturation (0-1), value (0-1) to RGB (0-255).
func hsvToRGB(hue, saturation, value float64) Color {
	chroma := value * saturation
	x := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	m := value - chroma

	var rf, gf, bf float64

	switch {
	case hue < 60:
		rf, gf, bf = chroma, x, 0
	case hue < 120:
		rf, gf, bf = x, chroma, 0
	case hue < 180:
		rf, gf, bf = 0, chroma, x
	case hue < 240:
		rf, gf, bf = 0, x, chroma
	case hue < 300:
		rf, gf, bf = x, 0, chroma
	default:
		rf, gf, bf = chroma, 0, x
	}

	return Color{
		byte((rf + m) * 255),
		byte((gf + m) * 255),
		byte((bf + m) * 255),
	}
}
