package game

import (
	"math"

	"github.com/jsimonetti/go-artnet/packet"
)

func hasCommonKeys(m1, m2 map[int]Color) bool {
	for k := range m1 {
		if _, found := m2[k]; found {
			return true
		}
	}
	return false
}

func (g *Game) Draw(colorMap map[int]Color) {
	pixels := make([]Color, SegmentCount*SegmentLength)
	for i := 0; i < len(pixels); i++ {
		if c, key := colorMap[i]; key {
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
			data[i*RGB+0] = pixels[p][0]
			data[i*RGB+1] = pixels[p][1]
			data[i*RGB+2] = pixels[p][2]
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

// hueToRGB converts hue (0-360), RGB (0-255). Assumes saturation and value of 1.0
func hueToRGB(hue float64) Color {
	x := 1 - math.Abs(math.Mod(hue/60, 2)-1)

	var rf, gf, bf float64

	switch {
	case hue < 60:
		rf, gf, bf = 1.0, x, 0
	case hue < 120:
		rf, gf, bf = x, 1.0, 0
	case hue < 180:
		rf, gf, bf = 0, 1.0, x
	case hue < 240:
		rf, gf, bf = 0, x, 1.0
	case hue < 300:
		rf, gf, bf = x, 0, 1.0
	default:
		rf, gf, bf = 1.0, 0, x
	}

	return Color{
		byte(rf * 255),
		byte(gf * 255),
		byte(bf * 255),
	}
}
