package game

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hypebeast/go-osc/osc"
	"github.com/thommahoney/dsk-eel/config"
)

// nextDatagram returns the next datagram conn receives, or nil if none
// arrives in time.
func nextDatagram(conn *net.UDPConn, wait time.Duration) []byte {
	buf := make([]byte, 2048)
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil
	}
	return buf[:n]
}

// newTestAbleton returns an Ableton set up by InitAbleton to send to a
// loopback socket, and that socket.
func newTestAbleton(t *testing.T) (*Ableton, *net.UDPConn) {
	t.Helper()
	conn := listenLoopback(t)
	a, err := InitAbleton(&config.Config{
		AbletonDest: conn.LocalAddr().String(),
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("InitAbleton(%q): %v", conn.LocalAddr(), err)
	}
	return a, conn
}

// expectOSC fails unless exactly one datagram with the bytes want arrives.
func expectOSC(t *testing.T, conn *net.UDPConn, want []byte) {
	t.Helper()
	got := nextDatagram(conn, time.Second)
	if got == nil {
		t.Fatal("no datagram was sent")
	}
	if !bytes.Equal(got, want) {
		t.Errorf("datagram is\n  %q\nwant\n  %q", got, want)
	}
	if extra := nextDatagram(conn, 50*time.Millisecond); extra != nil {
		t.Errorf("a second datagram was sent: %q", extra)
	}
}

// The expected datagrams below are written out from the OSC 1.0 message
// layout: the address and then the type tag string, each ended by a NUL and
// padded with NULs to a multiple of 4 bytes, then one big-endian int32 per
// argument.

func TestFireSceneSendsSceneNumber(t *testing.T) {
	a, conn := newTestAbleton(t)

	a.FireScene(7)

	expectOSC(t, conn, []byte(""+
		"/live/scene/fire\x00\x00\x00\x00"+ // 16 characters, so 4 NULs
		",i\x00\x00"+ // one int32 argument
		"\x00\x00\x00\x07")) // scene 7
}

// AbletonOSC documents /live/clip/fire as track_id, clip_id.
func TestFireClipSendsTrackThenClip(t *testing.T) {
	a, conn := newTestAbleton(t)

	a.FireClip(6, 3)

	expectOSC(t, conn, []byte(""+
		"/live/clip/fire\x00"+ // 15 characters, so 1 NUL
		",ii\x00"+ // two int32 arguments
		"\x00\x00\x00\x06"+ // track 6
		"\x00\x00\x00\x03")) // clip 3
}

// A scene number of 0 is what Run sends at the start of every game.
func TestFireSceneZero(t *testing.T) {
	a, conn := newTestAbleton(t)

	a.FireScene(0)

	expectOSC(t, conn, []byte("/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x00\x00\x00\x00"))
}

// GameOver is where the game fires scene 7.
func TestGameOverFiresScene7(t *testing.T) {
	g, _ := newTestGame(t)
	a, conn := newTestAbleton(t)
	g.Ableton = a
	g.QuitChan = make(chan struct{})

	g.GameOver()

	expectOSC(t, conn, []byte("/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x00\x00\x00\x07"))
}

// Eating fires a clip on track 6: one of its first eight clip slots, 0 to
// 7, at random. Over 400 meals every one of the eight must come up (a
// given slot is missed by chance less than once in 1e22 runs) and nothing
// else may.
func TestEatFiresClip(t *testing.T) {
	g, _ := newTestGame(t)
	a, conn := newTestAbleton(t)
	g.Ableton = a
	place(g, 0, 10, 7, 1.0, 1, 0, 6)

	prefix := []byte("/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00")
	var seen [8]int
	for meal := 0; meal < 400; meal++ {
		g.Eel.Eat()

		got := nextDatagram(conn, time.Second)
		if got == nil {
			t.Fatalf("meal %d: no datagram was sent", meal)
		}
		if len(got) != len(prefix)+1 || !bytes.HasPrefix(got, prefix) {
			t.Fatalf("meal %d: datagram is %q, want %q followed by one byte", meal, got, prefix)
		}
		clip := got[len(got)-1]
		if clip > 7 {
			t.Fatalf("meal %d: clip slot is %d, want 0 to 7", meal, clip)
		}
		seen[clip]++
	}
	for clip, n := range seen {
		if n == 0 {
			t.Errorf("clip slot %d was never fired in 400 meals", clip)
		}
	}
}

func TestAbletonIPv6Address(t *testing.T) {
	conn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6loopback})
	if err != nil {
		t.Skipf("no IPv6 loopback here: %v", err)
	}
	defer conn.Close()

	a, err := InitAbleton(&config.Config{AbletonDest: conn.LocalAddr().String()})
	if err != nil {
		t.Fatalf("InitAbleton(%q): %v", conn.LocalAddr(), err)
	}
	if err := a.OscSend("/live/scene/fire", 1); err != nil {
		t.Fatalf("OscSend: %v", err)
	}
	expectOSC(t, conn, []byte("/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x00\x00\x00\x01"))
}

// With an empty address Ableton is disabled: there is nothing to send
// with, and every send is a successful no-op.
func TestAbletonDisabled(t *testing.T) {
	a, err := InitAbleton(&config.Config{AbletonDest: ""})
	if err != nil {
		t.Fatalf("InitAbleton with an empty address: %v", err)
	}
	if a.OscClient != nil {
		t.Errorf("a disabled Ableton has an OSC client for %s:%d", a.OscClient.IP(), a.OscClient.Port())
	}
	a.FireScene(7)
	a.FireClip(6, 3)
	if err := a.OscSend("/live/scene/fire", 7); err != nil {
		t.Errorf("OscSend on a disabled Ableton: %v", err)
	}
}

func TestAbletonRejectsMalformedAddress(t *testing.T) {
	for _, dest := range []string{
		"192.168.1.104",       // no port
		"192.168.1.104:",      // empty port
		":11000",              // empty host
		"192.168.1.104:osc",   // port is not a number
		"192.168.1.104:0",     // port out of range
		"192.168.1.104:65536", // port out of range
		"192.168.1.104:-1",    // port out of range
		"192.168.1.104:1:2",   // too many colons
		"[::1]",               // IPv6 address without a port
		"11000",               // port only
		" ",                   // blank but not empty
	} {
		a, err := InitAbleton(&config.Config{AbletonDest: dest})
		if err == nil {
			t.Errorf("InitAbleton(%q) succeeded, want an error", dest)
			continue
		}
		if a != nil {
			t.Errorf("InitAbleton(%q) returned both an Ableton and an error", dest)
		}
		// The message has to tell the operator which flag and value are wrong.
		if msg := err.Error(); !strings.Contains(msg, "--ableton") || !strings.Contains(msg, `"`+dest+`"`) {
			t.Errorf("InitAbleton(%q): error %q does not name the flag and the value", dest, msg)
		}
	}
}

// A failed send is reported through the logger at debug level only, so a
// game played without Ableton does not fill the log at the default level.
func TestAbletonLogsSendErrorsAtDebug(t *testing.T) {
	for _, tc := range []struct {
		level   slog.Level
		wantLog bool
	}{
		{slog.LevelDebug, true},
		{slog.LevelInfo, false},
		{slog.LevelWarn, false},
	} {
		var log bytes.Buffer
		a := &Ableton{
			// Port 70000 does not exist, so every send fails before
			// anything reaches the network.
			OscClient: osc.NewClient("127.0.0.1", 70000),
			Logger:    slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: tc.level})),
		}
		if err := a.OscSend("/live/scene/fire", 7); err == nil {
			t.Fatal("OscSend to port 70000 succeeded; the test needs a send that fails")
		}

		a.FireScene(7)
		a.FireClip(6, 3)

		logged := strings.Count(log.String(), "Ableton OSC send failed")
		if tc.wantLog && logged != 2 {
			t.Errorf("at level %v: %d failures logged, want 2; log:\n%s", tc.level, logged, log.String())
		}
		if !tc.wantLog && log.Len() != 0 {
			t.Errorf("at level %v: a failed send wrote to the log:\n%s", tc.level, log.String())
		}
	}

	// The test helper builds an Ableton without a logger; a failed send
	// must not crash it.
	a := &Ableton{OscClient: osc.NewClient("127.0.0.1", 70000)}
	a.FireScene(7)
}
