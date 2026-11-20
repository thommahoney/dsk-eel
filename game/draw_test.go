package game

import (
	"bytes"
	"testing"
	"time"

	"github.com/jsimonetti/go-artnet/packet"
)

// TestDrawSendsArtDMX sends frames through Draw to a loopback socket and
// compares each datagram with what go-artnet's ArtDMXPacket.MarshalBinary
// produces for the same pixels, which is how packets were built before
// Draw wrote them by hand.
//
// The second frame has different content in every pixel, so bytes left
// over from the first frame in the reused buffers would show up.
func TestDrawSendsArtDMX(t *testing.T) {
	g, sink := newTestGame(t)

	for round := 0; round < 2; round++ {
		for i := range g.frame {
			g.frame[i] = Color{byte(i + round*3), byte(i>>8) | byte(0x80>>round), byte(i*31 + 7 + round)}
		}
		g.Draw()

		for univ := 0; univ < testUniverses; univ++ {
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
			// Sequence is left at 0, which in Art-Net means the sender
			// does not number its packets. Any other value that never
			// changes could make a receiver drop frames as out of order.
			want, err := (&packet.ArtDMXPacket{
				SubUni: uint8(univ),
				Data:   data,
			}).MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}
			if len(want) != testPacketLen {
				t.Fatalf("reference packet is %d bytes, want %d", len(want), testPacketLen)
			}

			if len(got) > 12 && got[12] != 0 {
				t.Errorf("frame %d universe %d: sequence byte is %d, want 0 (sequencing disabled)", round, univ, got[12])
			}
			if !bytes.Equal(got, want) {
				at := 0
				for at < len(got) && at < len(want) && got[at] == want[at] {
					at++
				}
				t.Fatalf("frame %d universe %d: datagram differs from go-artnet at byte %d (%d bytes sent, want %d)",
					round, univ, at, len(got), len(want))
			}
		}
		sink.expectNoFrame()
	}
}
