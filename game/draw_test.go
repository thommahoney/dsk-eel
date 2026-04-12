package game

import (
	"bytes"
	"testing"

	"github.com/jsimonetti/go-artnet/packet"
)

// TestArtDMXWireFormatMatches ensures the hand-built ArtDMX buffer produced
// by initDrawBuffers + Draw emits byte-for-byte identical wire output to
// what the pre-refactor code did via go-artnet's reflection-based
// ArtDMXPacket.MarshalBinary. This pins the wire contract so the
// allocation-avoidance refactor can't silently change what goes out over
// UDP to Chromatik.
func TestArtDMXWireFormatMatches(t *testing.T) {
	// Build a deterministic frame: index i -> Color{i, i^0x55, i^0xAA}.
	var g Game
	g.initDrawBuffers()
	for i := range g.frame {
		g.frame[i] = Color{byte(i), byte(i ^ 0x55), byte(i ^ 0xAA)}
	}

	for univ := 0; univ < universeCount; univ++ {
		max := pixelsPerFullUniv
		if univ == universeCount-1 {
			max = pixelsInLastUniv
		}

		// What the pre-refactor code produced:
		var data [512]byte
		for i := 0; i < max; i++ {
			p := univ*pixelsPerFullUniv + i
			data[i*RGB+0] = g.frame[p][0]
			data[i*RGB+1] = g.frame[p][1]
			data[i*RGB+2] = g.frame[p][2]
		}
		pkt := &packet.ArtDMXPacket{
			Sequence: uint8(univ), // matches original local `sequence := 0; sequence++`
			SubUni:   uint8(univ),
			Net:      0,
			Data:     data,
		}
		want, err := pkt.MarshalBinary()
		if err != nil {
			t.Fatalf("univ %d MarshalBinary: %v", univ, err)
		}

		// What the new code produces for this universe.
		buf := &g.artnetBufs[univ]
		base := univ * pixelsPerFullUniv
		for i := 0; i < max; i++ {
			c := g.frame[base+i]
			off := artnetDataOffset + i*RGB
			buf[off+0] = c[0]
			buf[off+1] = c[1]
			buf[off+2] = c[2]
		}
		if max < pixelsPerFullUniv {
			tailStart := artnetDataOffset + max*RGB
			tailEnd := artnetDataOffset + artnetDMXPayloadLen
			for i := tailStart; i < tailEnd; i++ {
				buf[i] = 0
			}
		}
		got := buf[:]

		if !bytes.Equal(got, want) {
			// Find the first differing offset for a readable failure.
			var diffAt = -1
			for i := 0; i < len(want) && i < len(got); i++ {
				if got[i] != want[i] {
					diffAt = i
					break
				}
			}
			t.Errorf("univ %d: wire bytes differ at offset %d (len got=%d, want=%d)\n got=%x\nwant=%x",
				univ, diffAt, len(got), len(want), got[:min(32, len(got))], want[:min(32, len(want))])
		}
	}
}
