package game

import (
	"math"

	"github.com/jsimonetti/go-artnet/packet"
)

// func (g *Game) Draw(wg *sync.WaitGroup, quit <-chan struct{}) {
func (g *Game) Draw() {
	// defer wg.Done()

	var sequence uint8 = 0
	// prevColor := RandomColor()
	// ticker := time.NewTicker(MovementFrequency)
	// defer ticker.Stop()

	// for {
	// 	select {
	// 	case <-quit:
	// 		g.Config.Logger.Info("Draw received quit")
	// 		return
	// 	case <-ticker.C:
			// color := g.PrimaryColor

			eelBody := g.Eel.BodyPixels()	
			pixels := make([]Color, len(g.Segments) * SegmentLength)
			for i := 0; i < len(pixels); i++ {
				if eelBody[i] {
					pixels[i] = Red
				} else {
					pixels[i] = Black
				}
			}

			for univ := 0; univ < 7; univ++ {
				data := [512]byte{}

				max := 170
				if univ == 6 {
					max = 58
				}
				for i := 0; i < max; i++ {
					p := univ*170+i
					data[i*3+0] = pixels[p][0]
					data[i*3+1] = pixels[p][1]
					data[i*3+2] = pixels[p][2]
				}

				sequence++
				p := &packet.ArtDMXPacket{
					Sequence: sequence,
					SubUni:   uint8(univ),
					Net:      0,
					Data:     data,
				}

				b, _ := p.MarshalBinary()
				_, _ = g.Chromatik.Send(b)
			}
	// 	}
	// }
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
