package game

const (
	// artnetPort is the UDP port Art-Net is sent to. The game sends from a
	// port of its own: see InitChromatik.
	artnetPort = 0x1936 // 6454

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
// The layout is the ArtDmx packet of the Art-Net specification.
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
	for univ := range universeCount {
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

// Draw sends the current g.frame as 7 ArtDMX packets. While the sends
// work it does not allocate: it writes the pixels into the per-universe
// buffers and hands each buffer to ArtNetSend, which has written it to the
// socket by the time it returns.
//
// Payload bytes past a universe's last pixel are never written, so they
// keep the zero they start with: the last 2 bytes of universes 0..5 and
// everything after pixel 58 in universe 6.
//
// A send that fails stops neither the frame nor the game: the universes
// after it are still sent, and the next frame is tried like any other.
// A frame counts as failed if even one of its seven packets could not be
// sent, because part of the dome then did not get it, and as sent only if
// all seven were. artnetFrameResult says in the log when that changes.
func (g *Game) Draw() {
	// The error of the first packet of this frame that could not be sent.
	var sendErr error
	for univ := range universeCount {
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

		if _, err := g.Chromatik.ArtNetSend(buf[:]); err != nil && sendErr == nil {
			sendErr = err
		}
	}

	// Nearly every frame is sent after a frame that was sent, and then
	// there is nothing to say or to remember.
	if sendErr != nil || g.artnetFailedFrames != 0 {
		g.artnetFrameResult(sendErr)
	}
}

// artnetFrameResult takes the result of a frame: the error that makes it
// a failed one, or nil for the first frame sent after a failed one. Draw
// does not call it for any other frame. It logs when frames stop getting
// out and when they get out again: once each, and nothing for the failed
// frames in between, however many there are. A line for each would be 14
// lines a second.
//
// Both lines are warnings, the second one too although nothing is wrong
// any more: warnings are what the log shows without -v, and a log that
// says frames stopped has to say when they came back.
//
// It never opens the socket again. The socket is bound to the address this
// machine had at the start, so sends that fail because that address is gone
// work again when the same address is back, and not with a different one.
func (g *Game) artnetFrameResult(err error) {
	if err != nil {
		if g.artnetFailedFrames == 0 {
			g.Config.Logger.Warn("cannot send Art-Net frames, will keep trying", "error", err)
		}
		g.artnetFailedFrames++
		return
	}
	g.Config.Logger.Warn("sending Art-Net frames again", "failed_frames", g.artnetFailedFrames)
	g.artnetFailedFrames = 0
}
