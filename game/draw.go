package game

const (
	// universeCount is the number of Art-Net DMX universes sent per frame.
	// Universes 0..5 carry 170 RGB pixels each and universe 6 carries the
	// remaining 58 (49 segments * 22 pixels = 1078 = 6*170 + 58).
	universeCount     = 7
	pixelsPerFullUniv = 170
	pixelsInLastUniv  = SegmentCount*SegmentLength - (universeCount-1)*pixelsPerFullUniv

	// An ArtDMX packet is an 18-byte header followed by a 512-byte DMX
	// payload, 530 bytes in all.
	artnetHeaderLen     = 18
	artnetDMXPayloadLen = 512
	artnetPacketLen     = artnetHeaderLen + artnetDMXPayloadLen

	// Offset of the one header byte that differs between universes.
	artnetSubUniByte = 14
)

// initDrawBuffers fills in the header of each universe's ArtDMX packet.
// The header never changes after this, so Draw only rewrites pixel bytes.
// The layout is the one jsimonetti/go-artnet's ArtDMXPacket.MarshalBinary
// produces, which the tests compare against.
func (g *Game) initDrawBuffers() {
	header := [artnetHeaderLen]byte{
		0x41, 0x72, 0x74, 0x2d, 0x4e, 0x65, 0x74, 0x00, // ID "Art-Net\0"
		0x00, 0x50, // OpCode 0x5000 (ArtDmx), low byte first
		0x00, 0x0e, // protocol version 14, high byte first
		0x00,       // Sequence: 0 tells the receiver sequencing is not in use
		0x00,       // Physical
		0x00,       // SubUni (set per universe below)
		0x00,       // Net
		0x02, 0x00, // payload length 512, high byte first
	}
	for univ := 0; univ < universeCount; univ++ {
		copy(g.artnetBufs[univ][:artnetHeaderLen], header[:])
		g.artnetBufs[univ][artnetSubUniByte] = uint8(univ)
	}
}

// backgroundColor is what a pixel shows when nothing is drawn on it.
var backgroundColor = Color{0x69, 0x69, 0x69}

// resetFrame paints the frame buffer with the background color and clears
// the eel-body occupancy bitmap. Called once per frame before rendering.
func (g *Game) resetFrame() {
	for i := range g.frame {
		g.frame[i] = backgroundColor
		g.frameSet[i] = false
	}
}

// Draw sends the current g.frame as 7 ArtDMX packets. It does not
// allocate: it writes the pixels into the per-universe buffers and hands
// each buffer to ArtNetSend, which has written it to the socket by the
// time it returns.
//
// Payload bytes past a universe's last pixel are never written, so they
// keep the zero they start with: the last 2 bytes of universes 0..5 and
// everything after pixel 58 in universe 6.
func (g *Game) Draw() {
	for univ := 0; univ < universeCount; univ++ {
		buf := &g.artnetBufs[univ]

		count := pixelsPerFullUniv
		if univ == universeCount-1 {
			count = pixelsInLastUniv
		}
		base := univ * pixelsPerFullUniv
		for i := 0; i < count; i++ {
			c := g.frame[base+i]
			off := artnetHeaderLen + i*RGB
			buf[off+0] = c[0]
			buf[off+1] = c[1]
			buf[off+2] = c[2]
		}

		_, _ = g.Chromatik.ArtNetSend(buf[:])
	}
}
