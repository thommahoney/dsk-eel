package game

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

// Helpers shared by the pin_*_test.go files. Their names all start with
// pin, and so do those of the helpers in the files themselves.

// pinMark is a datagram of the test's own. Everything a game sends has
// been sent by the time Run returns, so a mark sent after that arrives
// behind all of it, and a test can read up to the mark without waiting to
// see whether more is coming.
var pinMark = []byte("pin: nothing more was sent")

// pinSendMark sends the sink a mark.
func pinSendMark(t *testing.T, sink *artnetSink) {
	t.Helper()
	if _, err := listenLoopback(t).WriteToUDP(pinMark, sink.conn.LocalAddr().(*net.UDPAddr)); err != nil {
		t.Fatalf("sending the mark: %v", err)
	}
}

// pinExpectNothingMore fails if the sink holds a datagram that has not
// been read. Unlike expectNoFrame it does not wait: it is for when whatever
// was sending has finished.
func pinExpectNothingMore(t *testing.T, sink *artnetSink) {
	t.Helper()
	pinSendMark(t, sink)
	d := sink.datagram(time.Second)
	if d == nil {
		t.Fatal("the mark did not arrive")
	}
	if !bytes.Equal(d, pinMark) {
		t.Fatalf("unexpected datagram of %d bytes", len(d))
	}
}

// pinWait fails the test if the goroutines of the wait group have not
// ended after ten seconds.
func pinWait(t *testing.T, wg *sync.WaitGroup, what string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Errorf("%s did not end", what)
	}
}
