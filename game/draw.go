package game

const (
	// artnetHeaderLen is the size of the ArtDMX header that precedes the
	// 512-byte DMX payload on the wire. The full packet is 530 bytes.
	artnetHeaderLen = 18

	// universeCount is the number of ArtNet DMX universes we emit per frame.
	// Universes 0..5 carry 170 RGB pixels each and universe 6 carries the
	// remaining 58 pixels (49 segments * 22 pixels = 1078; 6*170 + 58 = 1078).
	universeCount       = 7
	pixelsPerFullUniv   = 170
	pixelsInLastUniv    = 58
	artnetSequenceByte  = 12
	artnetSubUniByte    = 14
	artnetDataOffset    = artnetHeaderLen
	artnetDMXPayloadLen = 512
)

// initDrawBuffers pre-populates the static bytes of each universe's ArtDMX
// packet so Draw() only has to touch the sequence byte and the DMX payload
// each frame. The wire layout matches what
// jsimonetti/go-artnet's ArtDMXPacket.MarshalBinary() produces.
func (g *Game) initDrawBuffers() {
	// "Art-Net\0"
	header := [artnetHeaderLen]byte{
		0x41, 0x72, 0x74, 0x2d, 0x4e, 0x65, 0x74, 0x00,
		0x00, 0x50, // OpDMX (0x5000) after byte-swap, written big-endian on the wire
		0x00, 0x0e, // Protocol version 14
		0x00,       // Sequence (rewritten per frame)
		0x00,       // Physical
		0x00,       // SubUni (set per universe below)
		0x00,       // Net
		0x02, 0x00, // Length 512 big-endian
	}
	for univ := 0; univ < universeCount; univ++ {
		copy(g.artnetBufs[univ][:artnetHeaderLen], header[:])
		// Match the pre-refactor behavior: sequence is a fixed 0..6 per
		// universe, static across frames. (The original loop reset a local
		// `sequence := 0` every frame and incremented it per universe.)
		g.artnetBufs[univ][artnetSequenceByte] = uint8(univ)
		g.artnetBufs[univ][artnetSubUniByte] = uint8(univ)
	}
}

// backgroundColor is the "nothing drawn here" color filled in by resetFrame.
// Matches the 0x696969 dim gray that the pre-refactor Draw() used.
var backgroundColor = Color{0x69, 0x69, 0x69}

// resetFrame paints the frame buffer with the default background color and
// clears the eel-body occupancy bitmap. Called once per frame before
// rendering the scene.
func (g *Game) resetFrame() {
	for i := range g.frame {
		g.frame[i] = backgroundColor
		g.frameSet[i] = false
	}
}

// Draw emits the current g.frame as 7 ArtDMX packets. Allocation-free:
// it writes directly into the pre-allocated per-universe buffers and
// passes a slice into them to ArtNetSend.
func (g *Game) Draw() {
	for univ := 0; univ < universeCount; univ++ {
		buf := &g.artnetBufs[univ]

		max := pixelsPerFullUniv
		if univ == universeCount-1 {
			max = pixelsInLastUniv
		}
		base := univ * pixelsPerFullUniv
		for i := 0; i < max; i++ {
			c := g.frame[base+i]
			off := artnetDataOffset + i*RGB
			buf[off+0] = c[0]
			buf[off+1] = c[1]
			buf[off+2] = c[2]
		}
		// Zero the unused tail of the short final universe.
		if max < pixelsPerFullUniv {
			tailStart := artnetDataOffset + max*RGB
			tailEnd := artnetDataOffset + artnetDMXPayloadLen
			for i := tailStart; i < tailEnd; i++ {
				buf[i] = 0
			}
		}

		_, _ = g.Chromatik.ArtNetSend(buf[:])
	}
}
