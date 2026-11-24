package game

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The two lines Draw logs about failed sends, spelled out here so that a
// changed message is noticed: the README quotes them.
const (
	artnetErrLogCannotSend = "cannot send Art-Net frames, will keep trying"
	artnetErrLogSendAgain  = "sending Art-Net frames again"
)

// artnetErrLine is one line of the log.
type artnetErrLine struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

// String is the line much as the log shows it, for a failure message.
func (l artnetErrLine) String() string {
	s := fmt.Sprintf("%v %q", l.level, l.msg)
	for _, key := range slices.Sorted(maps.Keys(l.attrs)) {
		s += fmt.Sprintf(" %s=%v", key, l.attrs[key])
	}
	return s
}

// artnetErrLines is lines of the log. A failure message shows them one to
// a line.
type artnetErrLines []artnetErrLine

func (lines artnetErrLines) String() string {
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("\n\t" + line.String())
	}
	return b.String()
}

// artnetErrLog is a slog handler that keeps every line logged at any
// level, which is what the program run with -vv shows.
type artnetErrLog struct {
	mu    sync.Mutex
	lines artnetErrLines
}

func (l *artnetErrLog) Enabled(context.Context, slog.Level) bool { return true }
func (l *artnetErrLog) WithAttrs([]slog.Attr) slog.Handler       { return l }
func (l *artnetErrLog) WithGroup(string) slog.Handler            { return l }
func (l *artnetErrLog) Handle(_ context.Context, r slog.Record) error {
	line := artnetErrLine{level: r.Level, msg: r.Message, attrs: map[string]any{}}
	r.Attrs(func(a slog.Attr) bool {
		line.attrs[a.Key] = a.Value.Any()
		return true
	})
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, line)
	return nil
}

// all returns every line logged so far.
func (l *artnetErrLog) all() artnetErrLines {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append(artnetErrLines(nil), l.lines...)
}

// about returns the lines whose message contains text.
func (l *artnetErrLog) about(text string) artnetErrLines {
	var lines artnetErrLines
	for _, line := range l.all() {
		if strings.Contains(line.msg, text) {
			lines = append(lines, line)
		}
	}
	return lines
}

// artnetErrRecord makes g log into the returned handler.
func artnetErrRecord(g *Game) *artnetErrLog {
	log := &artnetErrLog{}
	g.Config.Logger = slog.New(log)
	return log
}

// artnetErrTimeOut makes every send on the game's Art-Net socket fail, by
// giving the socket a time for its writes that has already passed. The
// socket stays open, and artnetErrNoTimeOut makes it send again. Both may
// be called while the game is running on another goroutine.
func artnetErrTimeOut(t *testing.T, g *Game) {
	t.Helper()
	if err := g.Chromatik.ArtNetConnection.SetWriteDeadline(time.Unix(1, 0)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
}

func artnetErrNoTimeOut(t *testing.T, g *Game) {
	t.Helper()
	if err := g.Chromatik.ArtNetConnection.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
}

// artnetErrWait waits until done says yes, and fails the test, naming what
// it waited for, if that takes more than ten seconds.
func artnetErrWait(t *testing.T, what string, done func() bool) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); !done(); time.Sleep(time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("waited ten seconds for %s", what)
		}
	}
}

// artnetErrCheckCannotSend fails unless line is the warning that frames are
// not getting out, with the error of the failed send in it.
func artnetErrCheckCannotSend(t *testing.T, line artnetErrLine, wantErr error) {
	t.Helper()
	if line.level != slog.LevelWarn || line.msg != artnetErrLogCannotSend {
		t.Errorf("the line for the failed frame is %v %q, want a warning (shown without -v) that says %q",
			line.level, line.msg, artnetErrLogCannotSend)
	}
	err, _ := line.attrs["error"].(error)
	if !errors.Is(err, wantErr) {
		t.Errorf("the line for the failed frame carries the error %v, want the error of the failed send: %v", line.attrs["error"], wantErr)
	}
}

// artnetErrCheckSendAgain fails unless line is the warning that frames are
// getting out again. It returns how many failed frames the line counts.
func artnetErrCheckSendAgain(t *testing.T, line artnetErrLine) int {
	t.Helper()
	if line.level != slog.LevelWarn || line.msg != artnetErrLogSendAgain {
		t.Errorf("the line for the first frame sent again is %v %q, want a warning (shown without -v) that says %q",
			line.level, line.msg, artnetErrLogSendAgain)
	}
	frames, ok := line.attrs["failed_frames"].(int64)
	if !ok {
		t.Errorf("the line for the first frame sent again has failed_frames=%v, want the number of frames that failed", line.attrs["failed_frames"])
	}
	return int(frames)
}

// TestDrawLogsFailedSendsOnce makes the sends of a game's real socket fail
// and work again, and looks at everything Draw logs meanwhile, at any
// level. A failure is one warning with the error, on its first frame,
// however many frames it lasts: 150 here, ten seconds of play. The first
// frame that gets out again is one more warning, which counts the frames
// that failed. Frames that are sent after frames that were sent log
// nothing.
//
// The failure is tried two ways, with a socket whose sends time out and
// with a closed one, and both after frames that got out and as the first
// frame of all. A second failure is reported like the first.
func TestDrawLogsFailedSendsOnce(t *testing.T) {
	ways := []struct {
		name    string
		fail    func(t *testing.T, g *Game)
		restore func(t *testing.T, g *Game)
		wantErr error
	}{
		{
			name:    "sends time out",
			fail:    artnetErrTimeOut,
			restore: artnetErrNoTimeOut,
			wantErr: os.ErrDeadlineExceeded,
		},
		{
			name: "socket closed",
			fail: func(t *testing.T, g *Game) { g.Chromatik.ArtNetConnection.Close() },
			// Nothing but a new socket sends again after that. The game
			// does not open one; the test does, between two frames.
			restore: func(t *testing.T, g *Game) { g.Chromatik.ArtNetConnection = listenLoopback(t) },
			wantErr: net.ErrClosed,
		},
	}
	starts := []struct {
		name       string
		sentBefore int
	}{
		{"after frames that got out", 3},
		{"as the first frame", 0},
	}
	const failedFrames = 150

	for _, way := range ways {
		for _, start := range starts {
			t.Run(way.name+" "+start.name, func(t *testing.T) {
				g, sink := newTestGame(t)
				log := artnetErrRecord(g)
				g.resetFrame()

				for range start.sentBefore {
					g.Draw()
					sink.mustFrame()
				}
				if lines := log.all(); len(lines) != 0 {
					t.Fatalf("%d frames were sent and Draw logged %d lines, want none: %v", start.sentBefore, len(lines), lines)
				}

				way.fail(t, g)
				g.Draw()
				lines := log.all()
				if len(lines) != 1 {
					t.Fatalf("the first failed frame logged %d lines, want one warning: %v", len(lines), lines)
				}
				artnetErrCheckCannotSend(t, lines[0], way.wantErr)

				for range failedFrames - 1 {
					g.Draw()
				}
				if lines := log.all(); len(lines) != 1 {
					t.Fatalf("%d failed frames in a row logged %d lines, want only the warning of the first: %v", failedFrames, len(lines), lines[1:])
				}
				sink.expectNoFrame()

				way.restore(t, g)
				g.Draw()
				sink.mustFrame()
				lines = log.all()
				if len(lines) != 2 {
					t.Fatalf("the first frame sent after the failure logged %d lines, want one warning: %v", len(lines)-1, lines[1:])
				}
				if got := artnetErrCheckSendAgain(t, lines[1]); got != failedFrames {
					t.Errorf("the line for the first frame sent again counts %d failed frames, want %d", got, failedFrames)
				}

				for range failedFrames {
					g.Draw()
					sink.mustFrame()
				}
				if lines := log.all(); len(lines) != 2 {
					t.Fatalf("frames sent after the failure was over logged %d more lines, want none: %v", len(lines)-2, lines[2:])
				}

				// A second failure, of one frame, is a failure like the
				// first, and its count starts again.
				way.fail(t, g)
				g.Draw()
				way.restore(t, g)
				g.Draw()
				sink.mustFrame()
				lines = log.all()
				if len(lines) != 4 {
					t.Fatalf("a second failure and its end logged %d lines, want two: %v", len(lines)-2, lines[2:])
				}
				artnetErrCheckCannotSend(t, lines[2], way.wantErr)
				if got := artnetErrCheckSendAgain(t, lines[3]); got != 1 {
					t.Errorf("after a failure of one frame the line counts %d failed frames, want 1", got)
				}
			})
		}
	}
}

// TestDrawCountsAFramePartlySentAsFailed pins what a failed frame is when
// some of its seven packets get out and others do not: a failed one.
//
// A real socket cannot be told to fail the fourth send of a frame. What
// can be done is to let its sends begin to fail, and begin to work again,
// at a moment that has nothing to do with the frames being drawn, and that
// moment is often inside a frame. So the test does that again and again,
// and holds every frame to the rule by what arrived of it: a frame of
// which a packet is missing is a failed frame there and then, and the
// line that says frames are sent again comes only with a frame that
// arrived whole. It goes on until it has seen a frame cut short at the
// start of a failure and one at the end of a failure.
func TestDrawCountsAFramePartlySentAsFailed(t *testing.T) {
	g, sink := newTestGame(t)
	log := artnetErrRecord(g)
	g.resetFrame()
	conn := g.Chromatik.ArtNetConnection

	// arrived returns the universe of every datagram that is waiting.
	arrived := func() []int {
		var universes []int
		for d := sink.datagram(50 * time.Millisecond); d != nil; d = sink.datagram(50 * time.Millisecond) {
			universes = append(universes, int(d[14]))
		}
		return universes
	}
	whole := []int{0, 1, 2, 3, 4, 5, 6}

	const (
		maxRounds = 500
		edge      = 300 * time.Microsecond
	)
	cutAtStart, cutAtEnd := 0, 0
	for round := 0; round < maxRounds && (cutAtStart == 0 || cutAtEnd == 0); round++ {
		logged := len(log.all())

		// The sends begin to fail while frames are drawn.
		if err := conn.SetWriteDeadline(time.Now().Add(edge)); err != nil {
			t.Fatalf("SetWriteDeadline: %v", err)
		}
		for g.Draw(); len(log.all()) == logged; g.Draw() {
			for univ := range testUniverses {
				if sink.datagram(time.Second) == nil {
					t.Fatalf("round %d: of the 7 packets of a frame only %d arrived, and Draw logged nothing; a frame that is not sent whole is a failed frame",
						round, univ)
				}
			}
		}
		got := arrived()
		if len(got) >= testUniverses {
			t.Fatalf("round %d: Draw logged a failed frame, and all 7 of its packets arrived", round)
		}
		if !slices.Equal(got, whole[:len(got)]) {
			t.Fatalf("round %d: of the failed frame the universes %v arrived, want the first ones in order", round, got)
		}
		if len(got) > 0 {
			cutAtStart++
		}
		lines := log.all()
		if len(lines) != logged+1 {
			t.Fatalf("round %d: the failed frame logged %d lines, want one warning: %v", round, len(lines)-logged, lines[logged:])
		}
		artnetErrCheckCannotSend(t, lines[logged], os.ErrDeadlineExceeded)

		// The sends begin to work again while frames are drawn.
		cleared := make(chan error, 1)
		go func() {
			time.Sleep(edge)
			cleared <- conn.SetWriteDeadline(time.Time{})
		}()
		failed := 1
		end := time.Now().Add(10 * time.Second)
		for g.Draw(); len(log.all()) == logged+1; g.Draw() {
			failed++
			if time.Now().After(end) {
				t.Fatalf("round %d: frames were drawn for ten seconds after the sends began to work again, and Draw did not log it", round)
			}
		}
		if err := <-cleared; err != nil {
			t.Fatalf("SetWriteDeadline: %v", err)
		}
		lines = log.all()
		if len(lines) != logged+2 || lines[logged+1].msg != artnetErrLogSendAgain {
			t.Fatalf("round %d: after the warning for the failed frame, Draw logged: %v\nwant nothing for the failed frames after it, and then the one line that frames are sent again",
				round, lines[logged+1:])
		}
		got = arrived()
		if len(got) < testUniverses || !slices.Equal(got[len(got)-testUniverses:], whole) {
			t.Fatalf("round %d: Draw logged that frames are sent again on a frame that did not arrive whole; the universes that arrived since the failure are %v",
				round, got)
		}
		// What arrived before that frame is the end of the frame during
		// which the sends began to work: a failed frame, the last one.
		cut := got[:len(got)-testUniverses]
		if len(cut) >= testUniverses || !slices.Equal(cut, whole[testUniverses-len(cut):]) {
			t.Fatalf("round %d: before the first whole frame the universes %v arrived, want the last ones of one frame in order", round, cut)
		}
		if len(cut) > 0 {
			cutAtEnd++
		}
		if n := artnetErrCheckSendAgain(t, lines[logged+1]); n != failed {
			t.Errorf("round %d: the line for the first frame sent again counts %d failed frames, want %d", round, n, failed)
		}
	}
	t.Logf("frames cut short: %d as a failure began, %d as one ended", cutAtStart, cutAtEnd)
	if cutAtStart == 0 || cutAtEnd == 0 {
		t.Errorf("in %d rounds no frame was cut short at the start of a failure (%d seen) and at the end of one (%d seen); the test has not seen what it is for",
			maxRounds, cutAtStart, cutAtEnd)
	}
}

// TestArtNetFailureAcrossGamesIsLoggedOnce plays game after game on the
// real tickers, the way main does, and makes the sends fail under the
// running game for at least two whole games. Frames are drawn by the Mover
// of each game and by the game-over animation it starts, one after the
// other, so the failure passes through all of them: it must still be one
// warning, and one more when frames get out again, with every failed
// frame of every game counted. Under the race detector this is also the
// check that the goroutines hand the state on without sharing it.
func TestArtNetFailureAcrossGamesIsLoggedOnce(t *testing.T) {
	g, sink := newTestGame(t)
	log := artnetErrRecord(g)
	g.tick = fastTick

	var stop atomic.Bool
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for !stop.Load() {
			g.Run()
		}
	}()
	defer func() {
		stop.Store(true)
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			t.Error("the games did not stop")
		}
	}()

	// With a neutral stick and no demo, a game is 15 frames of play and
	// an animation of 3 or more, and then the next game starts.
	const framesOfAGame = 18
	games := func() int { return len(log.about("Starting game")) }
	artnetLines := func() artnetErrLines { return log.about("Art-Net") }

	artnetErrWait(t, "the second game to start", func() bool { return games() >= 2 })
	if lines := artnetLines(); len(lines) != 0 {
		t.Fatalf("a game was played with every frame sent, and the log has %d lines about Art-Net, want none: %v", len(lines), lines)
	}

	artnetErrTimeOut(t, g)
	artnetErrWait(t, "the warning for the failed frame", func() bool { return len(artnetLines()) >= 1 })
	// The game after the one that is running now, and the one after that,
	// are played from start to end with no frame sent.
	failedIn := games()
	artnetErrWait(t, "three more games to start", func() bool { return games() >= failedIn+3 })
	lines := artnetLines()
	if len(lines) != 1 {
		t.Fatalf("two games and more were played with no frame sent, and the log has %d lines about Art-Net, want only the warning of the first failed frame: %v", len(lines), lines)
	}
	artnetErrCheckCannotSend(t, lines[0], os.ErrDeadlineExceeded)

	// Nothing has arrived since the failure began; take what arrived
	// before it out of the way.
	for sink.datagram(50*time.Millisecond) != nil {
	}

	artnetErrNoTimeOut(t, g)
	artnetErrWait(t, "the line for the first frame sent again", func() bool { return len(artnetLines()) >= 2 })
	if sink.datagram(time.Second) == nil {
		t.Errorf("the log says frames are sent again, and none arrived")
	}
	sentIn := games()
	artnetErrWait(t, "two more games to start", func() bool { return games() >= sentIn+2 })

	stop.Store(true)
	<-stopped
	lines = artnetLines()
	if len(lines) != 2 {
		t.Fatalf("a failure and the games played after it logged %d lines about Art-Net, want two, its start and its end: %v", len(lines), lines)
	}
	if got := artnetErrCheckSendAgain(t, lines[1]); got < 2*framesOfAGame {
		t.Errorf("the line for the first frame sent again counts %d failed frames, want at least the %d of two whole games", got, 2*framesOfAGame)
	}
}
