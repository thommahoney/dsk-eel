package game

import (
	"bytes"
	"testing"
	"time"
)

// TestDrawSendsArtDMX sends frames through Draw to a loopback socket and
// compares each datagram with the ArtDmx packet for the same pixels,
// written out here from the Art-Net packet layout.
//
// The second frame has different content in every pixel, so bytes left
// over from the first frame in the reused buffers would show up.
func TestDrawSendsArtDMX(t *testing.T) {
	g, sink := newTestGame(t)

	for round := range 2 {
		for i := range g.frame {
			g.frame[i] = Color{byte(i + round*3), byte(i>>8) | byte(0x80>>round), byte(i*31 + 7 + round)}
		}
		g.Draw()

		for univ := range testUniverses {
			got := sink.datagram(time.Second)
			if got == nil {
				t.Fatalf("frame %d: datagram %d of %d was not sent", round, univ, testUniverses)
			}

			var data [512]byte
			count := testFullUniv
			if univ == testUniverses-1 {
				count = testLastUniv
			}
			for i := 0; i < count; i++ {
				c := g.frame[univ*testFullUniv+i]
				data[i*3+0], data[i*3+1], data[i*3+2] = c[0], c[1], c[2]
			}
			want := wantArtDMX(univ, data)
			if len(want) != testPacketLen {
				t.Fatalf("expected packet is %d bytes, want %d", len(want), testPacketLen)
			}

			if len(got) > 12 && got[12] != 0 {
				t.Errorf("frame %d universe %d: sequence byte is %d, want 0 (sequencing disabled)", round, univ, got[12])
			}
			if !bytes.Equal(got, want) {
				at := 0
				for at < len(got) && at < len(want) && got[at] == want[at] {
					at++
				}
				t.Fatalf("frame %d universe %d: datagram differs from the expected packet at byte %d (%d bytes sent, want %d)",
					round, univ, at, len(got), len(want))
			}
		}
		sink.expectNoFrame()
	}
}

// wantArtDMX returns the ArtDmx packet Draw must send for one universe:
// the 18-byte header from the Art-Net specification followed by the 512
// DMX data bytes.
func wantArtDMX(univ int, data [512]byte) []byte {
	want := []byte{
		'A', 'r', 't', '-', 'N', 'e', 't', 0, // ID
		0x00, 0x50, // OpCode 0x5000 (ArtDmx), low byte first
		0, 14, // protocol version 14, high byte first
		0,          // Sequence: 0 means the sender does not number its packets
		0,          // Physical
		byte(univ), // SubUni: low 8 bits of the universe address
		0,          // Net: high 7 bits of the universe address
		0x02, 0x00, // data length 512, high byte first
	}
	return append(want, data[:]...)
}
