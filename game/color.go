package game

import (
	"math"
	"math/rand/v2"
)

type Color [RGB]byte

var Black = Color{0x00, 0x00, 0x00}  // #000000
var Blue = Color{0x00, 0x00, 0xff}   // #0000FF
var Green = Color{0x00, 0xff, 0x00}  // #00ff00
var Purple = Color{0xae, 0x00, 0xff} // #ae00ff
var Red = Color{0xff, 0x00, 0x00}    // #FF0000
var White = Color{0xff, 0xff, 0xff}  // #ffffff
var Yellow = Color{0xff, 0xff, 0x00} // #FFFF00

func RandomColor() Color {
	return Color{byte(rand.IntN(255)), byte(rand.IntN(255)), byte(rand.IntN(255))}
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
