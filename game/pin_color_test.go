package game

import (
	"math"
	"testing"
)

// pinHsvToRGBUnlimited is hsvToRGB as it was before it limited the value to
// 0..1, kept to compare against.
func pinHsvToRGBUnlimited(hue, saturation, value float64) Color {
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

// TestHsvToRGBKeepsValueInRange asks for values outside 0..1. The color
// must be the one for the nearest value there is: above 1 the color at full
// brightness, below 0 black. Without the limits a little over full came out
// nearly black: on arm64, grey at 1.05 was level 11.
func TestHsvToRGBKeepsValueInRange(t *testing.T) {
	tests := []struct {
		name                   string
		hue, saturation, value float64
		want                   Color
	}{
		{"grey a step over full", 0, 0, 1.05, White},
		{"grey well over full", 0, 0, 2, White},
		{"grey at more than any number", 0, 0, math.Inf(1), White},
		{"grey a step under nothing", 0, 0, -0.05, Black},
		{"grey well under nothing", 0, 0, -0.5, Black},
		{"grey at less than any number", 0, 0, math.Inf(-1), Black},
		{"red over full", 0, 1, 1.05, Red},
		{"red under nothing", 0, 1, -0.05, Black},
		{"yellow over full", 60, 1, 1.3, Yellow},
		{"yellow under nothing", 60, 1, -0.2, Black},
		{"blue over full", 240, 1, 1.0000001, Blue},
		{"a pale color over full", 120, 0.5, 1.5, Color{127, 255, 127}},
		// The ends of the range are in it.
		{"grey at full", 0, 0, 1, White},
		{"grey at nothing", 0, 0, 0, Black},
	}
	for _, tc := range tests {
		if got := hsvToRGB(tc.hue, tc.saturation, tc.value); got != tc.want {
			t.Errorf("%s: hsvToRGB(%v, %v, %v) is %v, want %v", tc.name, tc.hue, tc.saturation, tc.value, got, tc.want)
		}
	}
}

// TestHsvToRGBUnchangedInRange shows that the limits change no color the
// game draws: for values from 0 to 1 hsvToRGB gives, byte for byte, what it
// gave without them.
//
// First the values the game asks for. They are each brightness of a breath
// as Mover's own additions arrive at it, which is not always the round
// number (the eighth is 0.5999999999999996), and each of those less the
// tenths by which the five pixels of a turn signal fade. They are tried
// with every whole degree of hue, which takes in the signals and the food,
// with the hue of every point of every eel of up to 60 points, and with
// the two saturations the game uses, 0 and 1.
//
// Then the range as a whole: every thousandth from 0 to 1, with a hue
// every five degrees and a saturation in between as well.
func TestHsvToRGBUnchangedInRange(t *testing.T) {
	tried := 0
	compare := func(values, hues, saturations []float64) {
		t.Helper()
		for _, value := range values {
			if value < 0 || value > 1 {
				t.Fatalf("the test asks for the value %v, which is not one from 0 to 1", value)
			}
			for _, saturation := range saturations {
				for _, hue := range hues {
					tried++
					got, want := hsvToRGB(hue, saturation, value), pinHsvToRGBUnlimited(hue, saturation, value)
					if got != want {
						t.Fatalf("hsvToRGB(%v, %v, %v) is %v, and was %v before the value was limited", hue, saturation, value, got, want)
					}
				}
			}
		}
	}

	var asked []float64
	brightness, step := 1.0, -0.05
	for range len(pinBreath) {
		brightness += step
		if brightness <= MinBrightness || brightness >= MaxBrightness {
			step = -step
		}
		for i := range 5 {
			asked = append(asked, brightness-float64(i)*.1)
		}
	}
	var hues []float64
	for degree := range 360 {
		hues = append(hues, float64(degree))
	}
	for length := 1; length <= 60; length++ {
		for i := range length {
			hues = append(hues, float64(i)*360.0/float64(length))
		}
	}
	compare(asked, hues, []float64{0, 1})

	var thousandths, fiveDegrees []float64
	for k := range 1001 {
		thousandths = append(thousandths, float64(k)/1000)
	}
	for degree := 0; degree < 360; degree += 5 {
		fiveDegrees = append(fiveDegrees, float64(degree))
	}
	compare(thousandths, fiveDegrees, []float64{0, 0.5, 1})

	t.Logf("%d colors compared", tried)
}
