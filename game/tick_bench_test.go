package game

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/thommahoney/dsk-eel/config"
	"github.com/thommahoney/dsk-eel/controller"
)

// newLoopbackGame is newTestGame for benchmarks as well as tests: a game
// whose Art-Net and OSC go to loopback sockets that nobody reads.
func newLoopbackGame(tb testing.TB) *Game {
	tb.Helper()
	listen := func() *net.UDPConn {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			tb.Fatalf("listen on loopback: %v", err)
		}
		tb.Cleanup(func() { conn.Close() })
		return conn
	}
	oscPort := listen().LocalAddr().(*net.UDPAddr).Port

	g := &Game{
		Brightness: 1.0,
		Config:     &config.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))},
	}
	g.Chromatik = &Chromatik{
		ArtNetConnection: listen(),
		ArtNetNode:       listen().LocalAddr().(*net.UDPAddr),
		OscClient:        testOscClient(tb, oscPort),
	}
	g.Ableton = &Ableton{OscClient: testOscClient(tb, oscPort)}
	g.Segments = InitSegments()
	g.initDrawBuffers()
	return g
}

// straightRun is how many ordinary ticks a lap of a lapEel has: the head
// goes from position 6 of segment 0 to position 21 without a junction.
const straightRun = 15

// lapEel puts an n-point eel on the dome for measuring ordinary ticks, with
// a food it never reaches, and returns a function that puts the eel back
// where it started without allocating. The head and the six points behind
// it lie on positions 6..0 of segment 0; the rest of the body fills the
// dome from segment 2 onwards, out of the head's way. The eel can then move
// straightRun times before it has to be put back.
func lapEel(g *Game, n int) (reset func()) {
	seg := g.Segments[0]
	body := make([]*Point, n)
	for i := range body {
		body[i] = &Point{}
	}
	g.Eel = &Eel{Body: body, Game: g, Segment: seg, TravelDir: Greater}
	g.controlDir.Store(controller.Dir_Neutral)
	g.Food = &Food{Body: line(g.Segments[1], 12, 6), Game: g, Fresh: true}
	g.Brightness = 0.75

	reset = func() {
		for i, p := range g.Eel.Body {
			if i < 7 {
				p.Segment, p.Position = seg, 6-i
				continue
			}
			px := 2*SegmentLength + i - 7
			p.Segment, p.Position = g.Segments[px/SegmentLength], px%SegmentLength
		}
	}
	reset()
	return reset
}

// BenchmarkMoveTick measures an ordinary tick (no junction, no growth, no
// food eaten) for eels of different lengths, with rendering and the seven
// Art-Net sends to a loopback socket included.
func BenchmarkMoveTick(b *testing.B) {
	for _, n := range []int{7, 50, 300, 1000} {
		b.Run(fmt.Sprintf("eel=%d", n), func(b *testing.B) {
			g := newLoopbackGame(b)
			reset := lapEel(g, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if i%straightRun == 0 {
					reset()
				}
				if err := g.Eel.Move(); err != nil {
					b.Fatalf("Move: %v", err)
				}
			}
		})
	}
}
