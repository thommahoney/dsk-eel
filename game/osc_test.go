package game

import (
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/config"
)

// testOscClient returns a client that sends to the given loopback port
// and is closed when the test ends.
func testOscClient(t testing.TB, port int) *OscClient {
	t.Helper()
	c, err := NewOscClient(fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("NewOscClient for loopback port %d: %v", port, err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// closedOscClient returns a client whose socket is already closed, so
// that every send fails without anything reaching the network.
func closedOscClient(t testing.TB) *OscClient {
	t.Helper()
	c := testOscClient(t, 9)
	c.Close()
	return c
}

// oscPair returns a client and the loopback socket it sends to.
func oscPair(t *testing.T) (*OscClient, *net.UDPConn) {
	t.Helper()
	conn := listenLoopback(t)
	return testOscClient(t, conn.LocalAddr().(*net.UDPAddr).Port), conn
}

// oscCase is one message and the datagram it must become. A message has
// either one float32 argument or the int32 arguments in ints.
type oscCase struct {
	address string
	isFloat bool
	float   float32
	ints    []int32
	want    string
}

func (c oscCase) String() string {
	if c.isFloat {
		return fmt.Sprintf("%s %v", c.address, c.float)
	}
	return fmt.Sprintf("%s %v", c.address, c.ints)
}

func (c oscCase) send(client *OscClient) error {
	if c.isFloat {
		return client.SendFloat(c.address, c.float)
	}
	return client.SendInts(c.address, c.ints...)
}

func floatCase(address string, value float32, want string) oscCase {
	return oscCase{address: address, isFloat: true, float: value, want: want}
}

func intCase(address string, want string, values ...int32) oscCase {
	return oscCase{address: address, ints: values, want: want}
}

// oscCases lists every message the game sends, and others that exercise
// the padding rules and argument values. The expected datagrams are
// written out from the OSC 1.0 message layout: the address and then the
// type tag string, each ended by a NUL and padded with NULs to a multiple
// of 4 bytes, then 4 big-endian bytes per argument. The float32 1.0 is
// 3f 80 00 00.
var oscCases = []oscCase{
	// Chromatik mixer faders, off and on. The address is 25 characters.
	floatCase("/lx/mixer/channel/1/fader", 0, "/lx/mixer/channel/1/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/2/fader", 0, "/lx/mixer/channel/2/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/3/fader", 0, "/lx/mixer/channel/3/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/4/fader", 0, "/lx/mixer/channel/4/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/5/fader", 0, "/lx/mixer/channel/5/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/6/fader", 0, "/lx/mixer/channel/6/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/7/fader", 0, "/lx/mixer/channel/7/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/8/fader", 0, "/lx/mixer/channel/8/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/9/fader", 0, "/lx/mixer/channel/9/fader\x00\x00\x00,f\x00\x00\x00\x00\x00\x00"),
	floatCase("/lx/mixer/channel/1/fader", 1, "/lx/mixer/channel/1/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/mixer/channel/2/fader", 1, "/lx/mixer/channel/2/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/mixer/channel/3/fader", 1, "/lx/mixer/channel/3/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/mixer/channel/4/fader", 1, "/lx/mixer/channel/4/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/mixer/channel/5/fader", 1, "/lx/mixer/channel/5/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/mixer/channel/6/fader", 1, "/lx/mixer/channel/6/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/mixer/channel/7/fader", 1, "/lx/mixer/channel/7/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/mixer/channel/8/fader", 1, "/lx/mixer/channel/8/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/mixer/channel/9/fader", 1, "/lx/mixer/channel/9/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),

	// Chromatik palette swatch recall. The address is 29 characters.
	floatCase("/lx/palette/swatches/1/recall", 1, "/lx/palette/swatches/1/recall\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/palette/swatches/2/recall", 1, "/lx/palette/swatches/2/recall\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/palette/swatches/3/recall", 1, "/lx/palette/swatches/3/recall\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),
	floatCase("/lx/palette/swatches/4/recall", 1, "/lx/palette/swatches/4/recall\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"),

	// Ableton: scenes 0 and 7, and clip 6 with each second argument the
	// game can pick. The addresses are 16 and 15 characters.
	intCase("/live/scene/fire", "/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x00\x00\x00\x00", 0),
	intCase("/live/scene/fire", "/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x00\x00\x00\x07", 7),
	intCase("/live/clip/fire", "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x01", 6, 1),
	intCase("/live/clip/fire", "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x02", 6, 2),
	intCase("/live/clip/fire", "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x03", 6, 3),
	intCase("/live/clip/fire", "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x04", 6, 4),
	intCase("/live/clip/fire", "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x05", 6, 5),
	intCase("/live/clip/fire", "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x06", 6, 6),
	intCase("/live/clip/fire", "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x07", 6, 7),
	intCase("/live/clip/fire", "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x08", 6, 8),

	// Other float values.
	floatCase("/lx/mixer/channel/1/fader", 0.5, "/lx/mixer/channel/1/fader\x00\x00\x00,f\x00\x00\x3f\x00\x00\x00"),
	floatCase("/lx/mixer/channel/1/fader", -1, "/lx/mixer/channel/1/fader\x00\x00\x00,f\x00\x00\xbf\x80\x00\x00"),
	floatCase("/lx/mixer/channel/1/fader", 0.05, "/lx/mixer/channel/1/fader\x00\x00\x00,f\x00\x00\x3d\x4c\xcc\xcd"),
	floatCase("/lx/mixer/channel/1/fader", math.MaxFloat32, "/lx/mixer/channel/1/fader\x00\x00\x00,f\x00\x00\x7f\x7f\xff\xff"),

	// Other int values.
	intCase("/live/scene/fire", "/live/scene/fire\x00\x00\x00\x00,i\x00\x00\xff\xff\xff\xff", -1),
	intCase("/live/scene/fire", "/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x7f\xff\xff\xff", math.MaxInt32),
	intCase("/live/scene/fire", "/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x80\x00\x00\x00", math.MinInt32),
	intCase("/live/scene/fire", "/live/scene/fire\x00\x00\x00\x00,i\x00\x00\x01\x02\x03\x04", 0x01020304),

	// Address lengths on each side of a multiple of 4. A string whose
	// length is already a multiple of 4 still gets its NUL, and so 4 of them.
	intCase("/a", "/a\x00\x00,i\x00\x00\x00\x00\x00\x01", 1),
	intCase("/ab", "/ab\x00,i\x00\x00\x00\x00\x00\x01", 1),
	intCase("/abc", "/abc\x00\x00\x00\x00,i\x00\x00\x00\x00\x00\x01", 1),
	intCase("/abcd", "/abcd\x00\x00\x00,i\x00\x00\x00\x00\x00\x01", 1),

	// The type tag string is padded the same way: with 0, 1, 2 and 3
	// arguments it is 1, 2, 3 and 4 characters long.
	intCase("/n", "/n\x00\x00,\x00\x00\x00"),
	intCase("/n", "/n\x00\x00,i\x00\x00\x00\x00\x00\x01", 1),
	intCase("/n", "/n\x00\x00,ii\x00\x00\x00\x00\x01\x00\x00\x00\x02", 1, 2),
	intCase("/n", "/n\x00\x00,iii\x00\x00\x00\x00\x00\x00\x00\x01\x00\x00\x00\x02\x00\x00\x00\x03", 1, 2, 3),
}

// TestOscMessagesOnTheWire sends every case through a client to a
// loopback socket and compares the datagram that arrives.
func TestOscMessagesOnTheWire(t *testing.T) {
	client, conn := oscPair(t)
	for _, c := range oscCases {
		if err := c.send(client); err != nil {
			t.Errorf("%v: %v", c, err)
			continue
		}
		got := nextDatagram(conn, time.Second)
		if got == nil {
			t.Fatalf("%v: no datagram was sent", c)
		}
		if string(got) != c.want {
			t.Errorf("%v: datagram is\n  %q\nwant\n  %q", c, got, c.want)
		}
	}
	if extra := nextDatagram(conn, 50*time.Millisecond); extra != nil {
		t.Errorf("an extra datagram was sent: %q", extra)
	}
}

// The game reaches the client through these methods of Chromatik.
func TestChromatikOscSend(t *testing.T) {
	client, conn := oscPair(t)
	c := &Chromatik{OscClient: client}

	if err := c.OscSend("/lx/mixer/channel/9/fader", 1.0); err != nil {
		t.Fatalf("OscSend: %v", err)
	}
	expectOSC(t, conn, []byte("/lx/mixer/channel/9/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"))

	if err := c.OscSend_swatch(3); err != nil {
		t.Fatalf("OscSend_swatch: %v", err)
	}
	expectOSC(t, conn, []byte("/lx/palette/swatches/3/recall\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"))

	// An empty address means there is nothing to send.
	if err := c.OscSend("", 1.0); err != nil {
		t.Errorf("OscSend with an empty address: %v", err)
	}
	if extra := nextDatagram(conn, 50*time.Millisecond); extra != nil {
		t.Errorf("OscSend with an empty address sent %q", extra)
	}
}

// InitChromatik points the OSC client at the host and port given by
// --osc-dest and --osc-port.
func TestInitChromatikSendsOscToConfiguredDestination(t *testing.T) {
	conn := listenLoopback(t)
	c := loopbackChromatik(t, conn.LocalAddr().(*net.UDPAddr).Port)

	if err := c.OscSend("/lx/mixer/channel/9/fader", 1.0); err != nil {
		t.Fatalf("OscSend: %v", err)
	}
	expectOSC(t, conn, []byte("/lx/mixer/channel/9/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"))
}

// An OSC destination that cannot be used stops InitChromatik with an error
// that names the flags, and the Art-Net socket it had already opened is
// closed again.
func TestInitChromatikRejectsBadOscDestination(t *testing.T) {
	c, err := InitChromatik(&config.Config{
		ArtNetDest:   net.IPv4(127, 0, 0, 1),
		ListenSubnet: "127.0.0.1/8",
		OscDest:      "127.0.0.1",
		OscPort:      0,
	})
	if err == nil {
		c.ArtNetConnection.Close()
		c.OscClient.Close()
		t.Fatal("InitChromatik accepted OSC port 0")
	}
	if strings.Contains(err.Error(), "address already in use") {
		t.Skipf("the Art-Net port is in use on this machine: %v", err)
	}
	for _, want := range []string{"--osc-dest", "--osc-port", `"127.0.0.1:0"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %s: %v", want, err)
		}
	}

	// The Art-Net port can be bound only if InitChromatik let go of it.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 6454})
	if err != nil {
		t.Fatalf("the Art-Net port is still bound after the failed start: %v", err)
	}
	conn.Close()
}

// A message that does not fit in the send buffer is refused whole.
func TestOscRejectsMessageThatDoesNotFit(t *testing.T) {
	client, conn := oscPair(t)

	// With one argument the address may take 120 of the 128 bytes,
	// which is 119 characters and the NUL.
	longest := "/" + strings.Repeat("a", 118)
	if err := client.SendFloat(longest, 1); err != nil {
		t.Fatalf("SendFloat with a %d-character address: %v", len(longest), err)
	}
	expectOSC(t, conn, []byte(longest+"\x00,f\x00\x00\x3f\x80\x00\x00"))
	if err := client.SendInts(longest, 1); err != nil {
		t.Fatalf("SendInts with a %d-character address: %v", len(longest), err)
	}
	expectOSC(t, conn, []byte(longest+"\x00,i\x00\x00\x00\x00\x00\x01"))

	tooLong := longest + "a"
	if err := client.SendFloat(tooLong, 1); err == nil {
		t.Errorf("SendFloat with a %d-character address succeeded", len(tooLong))
	}
	if err := client.SendInts(tooLong, 1); err == nil {
		t.Errorf("SendInts with a %d-character address succeeded", len(tooLong))
	}
	// Two arguments leave 4 bytes less for the address.
	if err := client.SendInts(longest, 1, 2); err == nil {
		t.Errorf("SendInts with a %d-character address and two arguments succeeded", len(longest))
	}
	if err := client.SendInts("/n", make([]int32, 40)...); err == nil {
		t.Error("SendInts with 40 arguments succeeded")
	}
	if got := nextDatagram(conn, 50*time.Millisecond); got != nil {
		t.Errorf("a message that does not fit was sent anyway, as %d bytes: %q", len(got), got)
	}
}

func TestOscRejectsBadAddress(t *testing.T) {
	client, conn := oscPair(t)
	for _, address := range []string{"", "lx/mixer", "/lx\x00/mixer"} {
		if err := client.SendFloat(address, 1); err == nil {
			t.Errorf("SendFloat(%q) succeeded", address)
		}
		if err := client.SendInts(address, 1); err == nil {
			t.Errorf("SendInts(%q) succeeded", address)
		}
	}
	if got := nextDatagram(conn, 50*time.Millisecond); got != nil {
		t.Errorf("a message with a bad address was sent anyway: %q", got)
	}
}

func TestNewOscClientRejectsMalformedDestination(t *testing.T) {
	for _, dest := range []string{
		"",
		"127.0.0.1",       // no port
		"127.0.0.1:",      // empty port
		":3030",           // empty host
		"127.0.0.1:osc",   // port is not a number
		"127.0.0.1:0",     // port out of range
		"127.0.0.1:65536", // port out of range
		"127.0.0.1:-1",    // port out of range
		"127.0.0.1:1:2",   // too many colons
		"[::1]",           // IPv6 address without a port
	} {
		c, err := NewOscClient(dest)
		if err == nil {
			c.Close()
			t.Errorf("NewOscClient(%q) succeeded, want an error", dest)
		}
	}
}

// Sending to a port nobody listens on must keep succeeding, as it does
// when Chromatik or Ableton is not running. A socket connected to the
// destination would fail the second send with "connection refused".
func TestOscSendWithNobodyListening(t *testing.T) {
	conn := listenLoopback(t)
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	client := testOscClient(t, port)
	for i := 0; i < 5; i++ {
		if err := client.SendFloat("/lx/mixer/channel/1/fader", 1); err != nil {
			t.Fatalf("send %d: %v", i+1, err)
		}
		// Give the refusal of this send time to come back.
		time.Sleep(2 * time.Millisecond)
	}
}

// The joystick goroutine and the game loop may send at the same time.
// Under the race detector this also shows the client shares no buffer.
func TestOscConcurrentSends(t *testing.T) {
	client, conn := oscPair(t)
	_ = conn.SetReadBuffer(1 << 20)
	const each = 100
	fader := "/lx/mixer/channel/1/fader\x00\x00\x00,f\x00\x00\x3f\x80\x00\x00"
	clip := "/live/clip/fire\x00,ii\x00\x00\x00\x00\x06\x00\x00\x00\x03"

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < each; i++ {
			_ = client.SendFloat("/lx/mixer/channel/1/fader", 1)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < each; i++ {
			_ = client.SendInts("/live/clip/fire", 6, 3)
		}
	}()

	counts := map[string]int{}
	for i := 0; i < 2*each; i++ {
		got := nextDatagram(conn, 2*time.Second)
		if got == nil {
			t.Fatalf("only %d of %d datagrams arrived", i, 2*each)
		}
		counts[string(got)]++
	}
	wg.Wait()
	if counts[fader] != each || counts[clip] != each {
		t.Errorf("received %d fader and %d clip messages intact, want %d of each", counts[fader], counts[clip], each)
	}
}

// A send must not allocate: the game loop sends to Ableton, and the
// joystick goroutine sends ten messages to Chromatik per button press.
func TestOscSendDoesNotAllocate(t *testing.T) {
	g, _ := newTestGame(t)
	client := g.Chromatik.OscClient
	for _, tc := range []struct {
		name string
		send func()
	}{
		{"OscClient.SendFloat", func() { _ = client.SendFloat("/lx/mixer/channel/1/fader", 1) }},
		{"OscClient.SendInts with 1 value", func() { _ = client.SendInts("/live/scene/fire", 7) }},
		{"OscClient.SendInts with 2 values", func() { _ = client.SendInts("/live/clip/fire", 6, 3) }},
		{"Chromatik.OscSend", func() { _ = g.Chromatik.OscSend("/lx/mixer/channel/1/fader", 1) }},
		{"Ableton.FireScene", func() { g.Ableton.FireScene(7) }},
		{"Ableton.FireClip", func() { g.Ableton.FireClip(6, 3) }},
	} {
		if allocs := testing.AllocsPerRun(200, tc.send); allocs != 0 {
			t.Errorf("%s allocates %v times per send, want 0", tc.name, allocs)
		}
	}
}

func BenchmarkChromatikOscSend(b *testing.B) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()
	c := &Chromatik{OscClient: testOscClient(b, conn.LocalAddr().(*net.UDPAddr).Port)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = c.OscSend("/lx/mixer/channel/1/fader", 1.0)
	}
}

func BenchmarkAbletonFireClip(b *testing.B) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()
	a := &Ableton{OscClient: testOscClient(b, conn.LocalAddr().(*net.UDPAddr).Port)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.FireClip(6, 3)
	}
}
