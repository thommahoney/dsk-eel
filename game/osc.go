package game

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// oscMaxMessage is the largest OSC message the game can send, in bytes.
// The longest one it does send, a palette swatch recall, is 40 bytes.
const oscMaxMessage = 128

// OscClient sends OSC 1.0 messages over UDP to one destination. It knows
// the two kinds of message the game sends: one float32 argument, or a few
// int32 arguments.
//
// The destination is resolved and the socket opened once, in NewOscClient.
// A send builds its message in a buffer on its own stack, so it does not
// allocate and any number of goroutines may send at the same time.
type OscClient struct {
	conn *net.UDPConn
	dest netip.AddrPort
}

// NewOscClient returns a client that sends to hostport, a "host:port" with
// a port from 1 to 65535. A host name is looked up here and never again.
func NewOscClient(hostport string) (*OscClient, error) {
	host, portText, err := net.SplitHostPort(hostport)
	if err != nil {
		return nil, fmt.Errorf("want host:port: %s", err)
	}
	if host == "" {
		return nil, fmt.Errorf("want host:port: the host is empty")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("want host:port: the port %q is not a number from 1 to 65535", portText)
	}

	addr, err := net.ResolveUDPAddr("udp", hostport)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve the host: %s", err)
	}
	ap := addr.AddrPort()
	dest := netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
	if !dest.IsValid() {
		return nil, fmt.Errorf("the host resolved to no usable address")
	}

	// The socket is not connected to the destination. A connected UDP
	// socket remembers that the destination refused an earlier packet and
	// fails a later write, so sends would report errors whenever the
	// receiving program is not running. Nothing ever reads this socket.
	network := "udp4"
	if !dest.Addr().Is4() {
		network = "udp6"
	}
	conn, err := net.ListenUDP(network, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot open a UDP socket: %s", err)
	}

	return &OscClient{conn: conn, dest: dest}, nil
}

// Close closes the socket. Sends fail after that.
func (c *OscClient) Close() error {
	return c.conn.Close()
}

// SendFloat sends the message "address ,f value".
func (c *OscClient) SendFloat(address string, value float32) error {
	var buf [oscMaxMessage]byte
	n, err := oscHeader(&buf, address, 'f', 1)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint32(buf[n:], math.Float32bits(value))
	_, err = c.conn.WriteToUDPAddrPort(buf[:n+4], c.dest)
	return err
}

// SendInts sends the message "address ,ii... values...": one int32
// argument per value, in order.
func (c *OscClient) SendInts(address string, values ...int32) error {
	var buf [oscMaxMessage]byte
	n, err := oscHeader(&buf, address, 'i', len(values))
	if err != nil {
		return err
	}
	for _, v := range values {
		binary.BigEndian.PutUint32(buf[n:], uint32(v))
		n += 4
	}
	_, err = c.conn.WriteToUDPAddrPort(buf[:n], c.dest)
	return err
}

// oscHeader writes the part of an OSC message that comes before the
// arguments and returns its length, which is where the first argument goes.
// That part is the address, then the type tag string: a comma followed by
// one tag character per argument. Each of the two strings is ended by a
// NUL and then padded with NULs to a multiple of 4 bytes.
//
// The message has count arguments, all of type tag ('i' for int32, 'f' for
// float32) and so 4 bytes each. A message that would not fit in buf is an
// error; it is never cut short.
func oscHeader(buf *[oscMaxMessage]byte, address string, tag byte, count int) (int, error) {
	if !strings.HasPrefix(address, "/") {
		return 0, fmt.Errorf("OSC address %q does not start with a slash", address)
	}
	if strings.IndexByte(address, 0) >= 0 {
		return 0, fmt.Errorf("OSC address %q contains a NUL", address)
	}

	addressLen := oscPadded(len(address) + 1)
	tagsLen := oscPadded(1 + count + 1)
	if total := addressLen + tagsLen + 4*count; total > len(buf) {
		return 0, fmt.Errorf("OSC message to %q with %d arguments would be %d bytes; the most that can be sent is %d",
			address, count, total, len(buf))
	}

	clear(buf[:addressLen+tagsLen])
	copy(buf[:], address)
	tags := buf[addressLen:]
	tags[0] = ','
	for i := 1; i <= count; i++ {
		tags[i] = tag
	}
	return addressLen + tagsLen, nil
}

// oscPadded rounds n up to a multiple of 4.
func oscPadded(n int) int {
	return (n + 3) &^ 3
}
