package game

import (
	"bytes"
	"fmt"
	"net"
	"slices"
	"testing"
	"time"
)

// pinSent reads everything that was sent to the sink, up to a mark, and
// says what it was, in the order of sending: frames (seven Art-Net
// datagrams each, universes 0 to 6) of play or of the game-over animation,
// and Ableton messages. Frames of play that follow one another are counted
// and named once.
//
// A frame of play shows the eel, the food and the turn signals, 19 pixels
// or more. One of the animation shows only what is left of the eel, which
// for an eel that has not eaten is 5 pixels, then 3, then 1.
func pinSent(t *testing.T, sink *artnetSink) []string {
	t.Helper()
	pinSendMark(t, sink)

	var sent []string
	play := 0 // frames of play since the last thing that was not one
	add := func(what ...string) {
		if play > 0 {
			sent = append(sent, fmt.Sprintf("%d frames of play", play))
			play = 0
		}
		sent = append(sent, what...)
	}

	var frame [testPixels]Color
	univ := 0
	for {
		d := sink.datagram(time.Second)
		switch {
		case d == nil:
			add()
			t.Fatalf("the mark did not arrive; read so far: %q", sent)
		case len(d) == testPacketLen && bytes.HasPrefix(d, []byte("Art-Net\x00")):
			if d[14] != byte(univ) {
				add()
				t.Fatalf("after %q: a datagram for universe %d, want universe %d", sent, d[14], univ)
			}
			count := testFullUniv
			if univ == testUniverses-1 {
				count = testLastUniv
			}
			for i := range count {
				off := testHeaderLen + i*3
				frame[univ*testFullUniv+i] = Color{d[off], d[off+1], d[off+2]}
			}
			if univ++; univ == testUniverses {
				univ = 0
				if n := lit(frame); n > 7 {
					play++
				} else {
					add(fmt.Sprintf("animation frame, %d lit", n))
				}
			}
		case univ != 0:
			add()
			t.Fatalf("after %q: a frame was cut short after %d of %d datagrams by %q", sent, univ, testUniverses, d)
		case bytes.Equal(d, pinMark):
			add()
			return sent
		case bytes.Equal(d, []byte("/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x00\x00\x00\x00")):
			add("Ableton scene 0")
		case bytes.Equal(d, []byte("/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x00\x00\x00\x07")):
			add("Ableton scene 7")
		default:
			add(fmt.Sprintf("%q", d))
		}
	}
}

// TestRunFiresTheAbletonScenes plays two whole games through Run and reads
// what they send. Ableton's messages go to the socket the frames go to, so
// that they arrive in one line with the frames and the order of sending can
// be seen.
//
// A game is: scene 0 before anything is drawn, the frames of play, scene 7
// when the eel dies, and only then the game-over animation. With nobody at
// the stick the eel moves 15 times, dies at its first junction without
// having eaten, and its animation is three frames, so that is everything a
// game sends, and each game sends it again.
func TestRunFiresTheAbletonScenes(t *testing.T) {
	g, sink := newTestGame(t)
	g.tick = fastTick
	g.Ableton = &Ableton{OscClient: testOscClient(t, sink.conn.LocalAddr().(*net.UDPAddr).Port)}

	want := []string{
		"Ableton scene 0",
		"15 frames of play",
		"Ableton scene 7",
		"animation frame, 5 lit", "animation frame, 3 lit", "animation frame, 1 lit",
	}
	for game := range 2 {
		waitGameOver(t, startGame(g))
		if got := pinSent(t, sink); !slices.Equal(got, want) {
			t.Errorf("game %d sent, in this order,\n  %q\nwant\n  %q", game, got, want)
		}
	}
}
