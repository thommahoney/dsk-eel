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
//
// A value below 0 is taken as 0 and one above 1 as 1. The game never asks
// for one: the breathing brightness stays between MinBrightness and
// MaxBrightness, and the dimmest turn-signal pixel is 0.4 below it. But the
// bytes below are made by converting a float, and what that gives for a
// number that is not from 0 to 255 is up to the compiler. On arm64 a value
// of 1.05 came out as level 11, nearly black, where full brightness was
// meant. With the limits a brightness that strays shows as the nearest
// level there is.
func hsvToRGB(hue, saturation, value float64) Color {
	value = min(max(value, 0), 1)
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
