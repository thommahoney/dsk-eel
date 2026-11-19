package game

import (
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/hypebeast/go-osc/osc"
	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

// The tests spell out the wire layout in their own literals instead of
// using the package constants, so a wrong constant cannot hide itself.
const (
	testPixels     = 1078
	testUniverses  = 7
	testPacketLen  = 530
	testHeaderLen  = 18
	testFullUniv   = 170
	testLastUniv   = 58
	testBackground = 0x69
)

// artnetSink is a loopback UDP listener standing in for Chromatik.
type artnetSink struct {
	t    *testing.T
	conn *net.UDPConn
	buf  []byte
}

func listenLoopback(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// newTestGame returns a game wired to loopback sockets only: Art-Net goes
// to the returned sink, and OSC for Chromatik and Ableton goes to a socket
// nobody reads. Nothing leaves the machine.
func newTestGame(t *testing.T) (*Game, *artnetSink) {
	t.Helper()
	sink := &artnetSink{t: t, conn: listenLoopback(t), buf: make([]byte, 2048)}
	// Room for every frame of a whole game, so a test can read them
	// after the game has finished.
	_ = sink.conn.SetReadBuffer(4 << 20)
	oscPort := listenLoopback(t).LocalAddr().(*net.UDPAddr).Port

	g := &Game{
		Brightness: 1.0,
		Config:     &config.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
	}
	g.Chromatik = &Chromatik{
		ArtNetConnection: listenLoopback(t),
		ArtNetNode:       sink.conn.LocalAddr().(*net.UDPAddr),
		OscClient:        osc.NewClient("127.0.0.1", oscPort),
	}
	g.Ableton = &Ableton{OscClient: osc.NewClient("127.0.0.1", oscPort)}
	g.Segments = InitSegments()
	g.initDrawBuffers()
	return g, sink
}

// datagram returns the next datagram, or nil if none arrives in time.
func (s *artnetSink) datagram(wait time.Duration) []byte {
	_ = s.conn.SetReadDeadline(time.Now().Add(wait))
	n, _, err := s.conn.ReadFromUDP(s.buf)
	if err != nil {
		return nil
	}
	return append([]byte(nil), s.buf[:n]...)
}

// frame reads one frame (7 datagrams, universes 0..6 in order) and
// returns its pixels. ok is false if no frame is waiting.
func (s *artnetSink) frame(wait time.Duration) (pixels [testPixels]Color, ok bool) {
	s.t.Helper()
	for univ := 0; univ < testUniverses; univ++ {
		d := s.datagram(wait)
		if d == nil {
			if univ == 0 {
				return pixels, false
			}
			s.t.Fatalf("frame stopped after %d of %d datagrams", univ, testUniverses)
		}
		if len(d) != testPacketLen {
			s.t.Fatalf("universe %d: datagram is %d bytes, want %d", univ, len(d), testPacketLen)
		}
		if d[14] != byte(univ) {
			s.t.Fatalf("datagram %d of the frame is for universe %d", univ, d[14])
		}
		count := testFullUniv
		if univ == testUniverses-1 {
			count = testLastUniv
		}
		for i := 0; i < count; i++ {
			off := testHeaderLen + i*3
			pixels[univ*testFullUniv+i] = Color{d[off], d[off+1], d[off+2]}
		}
	}
	return pixels, true
}

// mustFrame is frame for when a frame has to be there.
func (s *artnetSink) mustFrame() [testPixels]Color {
	s.t.Helper()
	pixels, ok := s.frame(time.Second)
	if !ok {
		s.t.Fatal("no frame was sent")
	}
	return pixels
}

// expectNoFrame fails if anything was sent.
func (s *artnetSink) expectNoFrame() {
	s.t.Helper()
	if d := s.datagram(50 * time.Millisecond); d != nil {
		s.t.Fatalf("unexpected datagram of %d bytes", len(d))
	}
}

// lit returns how many pixels are not the background color.
func lit(pixels [testPixels]Color) int {
	n := 0
	for _, c := range pixels {
		if c != (Color{testBackground, testBackground, testBackground}) {
			n++
		}
	}
	return n
}

// checkFrame compares a received frame with the expected non-background
// pixels. Every pixel not listed in want must be the background color.
func checkFrame(t *testing.T, name string, got [testPixels]Color, want map[int]Color) {
	t.Helper()
	for i, c := range got {
		w, listed := want[i]
		if !listed {
			w = Color{testBackground, testBackground, testBackground}
		}
		if c != w {
			t.Errorf("%s: pixel %d is %v, want %v", name, i, c, w)
		}
	}
}

// line returns points on seg from position `from` to position `to`,
// inclusive, in that order.
func line(seg *Segment, from, to int) []*Point {
	var points []*Point
	step := 1
	if from > to {
		step = -1
	}
	for p := from; ; p += step {
		points = append(points, &Point{Segment: seg, Position: p})
		if p == to {
			return points
		}
	}
}

// place puts an eel of n points on segment seg, head at headPos and
// travelling towards higher positions, with a neutral joystick; and a
// fresh food on segment foodSeg covering positions foodFrom..foodTo.
func place(g *Game, seg, headPos, n int, brightness float64, foodSeg, foodFrom, foodTo int) {
	s := g.Segments[seg]
	g.Eel = &Eel{
		Body:       line(s, headPos, headPos-n+1),
		ControlDir: controller.Dir_Neutral,
		Game:       g,
		Segment:    s,
		TravelDir:  Greater,
	}
	g.Food = &Food{Body: line(g.Segments[foodSeg], foodFrom, foodTo), Game: g, Fresh: true}
	g.Brightness = brightness
}
