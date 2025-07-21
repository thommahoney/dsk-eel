package game

import (
	"math"
	"sync"
	"time"

	"github.com/jsimonetti/go-artnet/packet"
)

const pixelCount = 170

func (g *Game) Draw(wg *sync.WaitGroup, quit <-chan struct{}) {
	defer wg.Done()

	var sequence uint8 = 0
	prevColor := RandomColor()

	for {
		select {
		case <-quit:
			g.Config.Logger.Info("Draw received quit")
			return
		default:
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

			for univ := uint8(0); univ < 7; univ++ {
				sequence++
				p := &packet.ArtDMXPacket{
					Sequence: sequence,
					SubUni:   univ,
					Net:      0,
					Data:     data,
				}

				b, _ := p.MarshalBinary()
				_, _ = g.Chromatik.Send(b)
			}
		}
	}
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
