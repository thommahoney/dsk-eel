package game

import (
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/config"
)

// The Art-Net port, spelled out like the other wire constants in
// helpers_test.go.
const testArtNetPort = 6454

// chromatikConfig returns just enough configuration for InitChromatik. No
// test here sends OSC; the OSC destination only has to be well formed.
func chromatikConfig(listenSubnet string, dest net.IP) *config.Config {
	return &config.Config{
		ArtNetDest:   dest,
		ListenSubnet: listenSubnet,
		OscDest:      "127.0.0.1",
		OscPort:      3030,
	}
}

// initChromatikForTest calls InitChromatik and makes sure its sockets are
// closed again when the test ends, whether or not the test expected any:
// the Art-Net socket and the OSC client's socket.
func initChromatikForTest(t *testing.T, c *config.Config) (*Chromatik, error) {
	t.Helper()
	chromatik, err := InitChromatik(c)
	if chromatik != nil && chromatik.ArtNetConnection != nil {
		t.Cleanup(func() { chromatik.ArtNetConnection.Close() })
	}
	if chromatik != nil && chromatik.OscClient != nil {
		t.Cleanup(func() { chromatik.OscClient.Close() })
	}
	return chromatik, err
}

// listenAsChromatik binds the Art-Net port on every address of dest's kind,
// IPv4 or IPv6, as a Chromatik on this machine does.
func listenAsChromatik(dest net.IP) (*net.UDPConn, error) {
	network := "udp4"
	if dest.To4() == nil {
		network = "udp6"
	}
	return net.ListenUDP(network, &net.UDPAddr{Port: testArtNetPort})
}

// ownAddresses returns this machine's addresses that a frame sent on this
// machine is sure to reach: those of interfaces that are up, other than
// loopback, link-local ones and the two ends of a tunnel. A VPN's tunnel
// has an address of this machine too, but a frame sent to it from here
// does not arrive on every system.
func ownAddresses(t *testing.T) []net.IP {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("Interfaces: %v", err)
	}
	var own []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			t.Fatalf("addresses of %s: %v", iface.Name, err)
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsLinkLocalUnicast() {
				own = append(own, ipnet.IP)
			}
		}
	}
	return own
}

// hasAddress reports whether this machine answers on ip, found out by
// binding a socket to it. Linux answers on every address in 127.0.0.0/8.
// Other systems only have 127.0.0.1 unless somebody adds more.
func hasAddress(ip net.IP) bool {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: ip})
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// expectSameMachine pins what InitChromatik does when the destination is on
// this machine, where the Art-Net port is Chromatik's to listen on and the
// game must keep off it, as it does everywhere: the socket is bound
// to wantLocal (any address, if that is nil: for the unspecified address
// systems differ) and a port the system picked, never the Art-Net port, frames
// are addressed to the Art-Net port of dest, and none of it is worth a
// warning. A frame sent with ArtNetSend must then arrive at a stand-in for
// Chromatik, a socket that has the Art-Net port on every address.
//
// Both start orders are tried. With Chromatik first, the stand-in has the
// port before InitChromatik runs. With the game first, the stand-in asks for
// the port afterwards and must still get it.
//
// If something else on this machine really has the port, the test is
// skipped: no stand-in can be put there.
func expectSameMachine(t *testing.T, listenSubnet string, dest, wantLocal net.IP) {
	t.Helper()
	for _, gameFirst := range []bool{false, true} {
		order := "Chromatik first"
		if gameFirst {
			order = "the game first"
		}
		listen := "no --listen"
		if listenSubnet != "" {
			listen = "--listen " + listenSubnet
		}
		t.Run(fmt.Sprintf("%s, to %v, %s", listen, dest, order), func(t *testing.T) {
			standIn, err := listenAsChromatik(dest)
			if err != nil {
				t.Skipf("cannot take port %d to stand in for Chromatik: %v", testArtNetPort, err)
			}
			if gameFirst {
				// The port is free. Whether it still is once the game
				// has started is checked below.
				standIn.Close()
			} else {
				t.Cleanup(func() { standIn.Close() })
			}

			var log bytes.Buffer
			cfg := chromatikConfig(listenSubnet, dest)
			cfg.Logger = slog.New(slog.NewTextHandler(&log, nil))
			chromatik, err := initChromatikForTest(t, cfg)
			if err != nil {
				t.Fatalf("InitChromatik: %v", err)
			}

			local := chromatik.ArtNetConnection.LocalAddr().(*net.UDPAddr)
			if (wantLocal != nil && !local.IP.Equal(wantLocal)) || local.Port == 0 || local.Port == testArtNetPort {
				t.Errorf("the socket is bound to %v, want %v and a port of its own, not %d", local, wantLocal, testArtNetPort)
			}
			if node := chromatik.ArtNetNode; !node.IP.Equal(dest) || node.Port != testArtNetPort {
				t.Errorf("frames are addressed to %v, want %v port %d", node, dest, testArtNetPort)
			}
			if strings.Contains(log.String(), "level=WARN") {
				t.Errorf("a warning was logged for the normal case; the log is %q", log.String())
			}
			if !strings.Contains(log.String(), "from="+local.String()) {
				t.Errorf("the log does not show the socket's address %v; the log is %q", local, log.String())
			}

			if gameFirst {
				standIn, err = listenAsChromatik(dest)
				if err != nil {
					t.Fatalf("with the game running, Chromatik cannot take port %d: %v", testArtNetPort, err)
				}
				t.Cleanup(func() { standIn.Close() })
			}

			// What a send to the unspecified address does differs between
			// systems, and a send to an address this machine does not
			// have arrives nowhere. For those only the addresses are
			// checked.
			if dest.IsUnspecified() || !hasAddress(dest) {
				return
			}
			if _, err := chromatik.ArtNetSend([]byte("frame")); err != nil {
				t.Fatalf("ArtNetSend: %v", err)
			}
			buf := make([]byte, 16)
			_ = standIn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, from, err := standIn.ReadFromUDP(buf)
			if err != nil || string(buf[:n]) != "frame" {
				t.Fatalf("the frame did not reach the holder of the Art-Net port: got %q, err %v", buf[:n], err)
			}
			if from.Port != local.Port {
				t.Errorf("the frame came from port %d, want the socket's port %d", from.Port, local.Port)
			}
		})
	}
}

// The program's defaults are the game on the same machine as Chromatik:
// no --listen and frames to 127.0.0.1. The game leaves the Art-Net port to
// Chromatik whichever of the two starts first, sends from a port of its
// own, and its frames reach whoever has the Art-Net port.
func TestInitChromatikLeavesTheArtNetPortToChromatik(t *testing.T) {
	expectSameMachine(t, "", net.IPv4(127, 0, 0, 1), net.IPv4(127, 0, 0, 1))
}

func TestInitChromatikRejectsInvalidSubnet(t *testing.T) {
	for _, subnet := range []string{" ", "banana", "127.0.0.1", "127.0.0.1/33"} {
		chromatik, err := initChromatikForTest(t, chromatikConfig(subnet, net.IPv4(127, 0, 0, 1)))
		if err == nil {
			t.Errorf("listen subnet %q: no error", subnet)
			continue
		}
		if chromatik != nil {
			t.Errorf("listen subnet %q: a Chromatik was returned along with the error", subnet)
		}
		if !strings.Contains(err.Error(), "--listen") {
			t.Errorf("listen subnet %q: error does not name the --listen flag: %v", subnet, err)
		}
	}
}

// 192.0.2.0/24 is reserved for documentation, so no interface has an
// address in it. InitChromatik must say so instead of binding to some
// other address.
func TestInitChromatikNoLocalAddressInSubnet(t *testing.T) {
	chromatik, err := initChromatikForTest(t, chromatikConfig("192.0.2.0/24", net.IPv4(127, 0, 0, 1)))
	if err == nil {
		t.Fatalf("no error; the socket was bound to %v", chromatik.ArtNetConnection.LocalAddr())
	}
	for _, want := range []string{"192.0.2.0/24", "127.0.0.1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
	}
}

func TestInitChromatikRejectsInvalidDestination(t *testing.T) {
	for _, dest := range []net.IP{nil, net.IP("127.0.0.1")} {
		chromatik, err := initChromatikForTest(t, chromatikConfig("127.0.0.1/24", dest))
		if err == nil {
			t.Errorf("destination % x: no error", []byte(dest))
		}
		if chromatik != nil {
			t.Errorf("destination % x: a Chromatik was returned", []byte(dest))
		}
	}
}

// With a subnet that holds 127.0.0.1 the socket is bound to 127.0.0.1, and
// frames go to port 6454 of the destination from a port of the game's own.
func TestInitChromatikLoopback(t *testing.T) {
	expectSameMachine(t, "127.0.0.1/24", net.IPv4(127, 0, 0, 1), net.IPv4(127, 0, 0, 1))
}

// Frames go to the --chromatik address while the socket stays bound to the
// local address in the subnet. The two are different loopback addresses
// here, so the test can tell them apart on any machine. The frame is only
// sent where the system has 127.0.0.2.
func TestInitChromatikSendsToTheConfiguredDestination(t *testing.T) {
	expectSameMachine(t, "127.0.0.1/24", net.IPv4(127, 0, 0, 2), net.IPv4(127, 0, 0, 1))
}

// A socket bound to loopback cannot send to another machine: every send
// fails, and during play that is only a warning in the log. So a loopback
// --listen subnet (the default before --listen became optional) with a
// --chromatik address that is not on this machine must be refused at
// startup, with a message that names both flags. Broadcast and multicast
// destinations are refused too: from loopback they never leave the machine.
func TestInitChromatikRejectsLoopbackListenWithRemoteDestination(t *testing.T) {
	for _, dest := range []net.IP{
		net.IPv4(10, 0, 0, 5),
		net.IPv4(192, 0, 2, 1),
		net.IPv4(255, 255, 255, 255),
		net.IPv4(239, 255, 0, 1),
	} {
		chromatik, err := initChromatikForTest(t, chromatikConfig("127.0.0.1/24", dest))
		if err == nil {
			t.Errorf("destination %v: no error; the socket was bound to %v", dest, chromatik.ArtNetConnection.LocalAddr())
			continue
		}
		if chromatik != nil {
			t.Errorf("destination %v: a Chromatik was returned along with the error", dest)
		}
		for _, want := range []string{"--listen", "--chromatik", dest.String(), "loopback"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("destination %v: error does not mention %s: %v", dest, want, err)
			}
		}
	}
}

// A loopback --listen subnet is fine when Chromatik is on this machine,
// whichever of the machine's addresses it is given by: loopback, the
// unspecified address or one of the machine's other addresses. Each of
// them leaves the Art-Net port to Chromatik. On Windows the unspecified
// address is refused whatever --listen is: see
// TestInitChromatikRefusesUnspecifiedDestination.
func TestInitChromatikAcceptsLoopbackListenWithLocalDestination(t *testing.T) {
	dests := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv4(127, 0, 0, 2)}
	if unspecifiedReachesThisMachine {
		dests = append(dests, net.IPv4zero)
	}
	for _, own := range ownAddresses(t) {
		if own.To4() == nil {
			continue
		}
		if loopbackReachesOwnAddresses {
			dests = append(dests, own)
			continue
		}
		// Windows: from loopback every send to the address would fail,
		// so the pairing is refused, with a way out.
		chromatik, err := initChromatikForTest(t, chromatikConfig("127.0.0.1/24", own))
		if err == nil {
			t.Errorf("destination %v with a loopback --listen: no error; the socket was bound to %v", own, chromatik.ArtNetConnection.LocalAddr())
		} else if !strings.Contains(err.Error(), "leave --listen out") {
			t.Errorf("destination %v with a loopback --listen: the error does not say what to do: %v", own, err)
		}
	}
	for _, dest := range dests {
		expectSameMachine(t, "127.0.0.1/24", dest, net.IPv4(127, 0, 0, 1))
	}
}

// hasIPv6Loopback reports whether this machine has the address ::1.
func hasIPv6Loopback(t *testing.T) bool {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatalf("InterfaceAddrs: %v", err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.Equal(net.IPv6loopback) {
			return true
		}
	}
	return false
}

// A socket bound to an IPv4 address cannot send to an IPv6 destination, or
// the other way round, and during play that is only a warning in the log.
// So a --chromatik address of one kind with a --listen subnet of the other
// must be refused at startup, with a message that names both flags.
func TestInitChromatikRejectsMixedIPv4AndIPv6(t *testing.T) {
	cases := []struct {
		name, listenSubnet string
		dest               net.IP
		needsIPv6Loopback  bool
	}{
		{"IPv4 subnet, IPv6 destination", "127.0.0.1/24", net.IPv6loopback, false},
		{"IPv6 subnet, IPv4 destination", "::1/128", net.IPv4(127, 0, 0, 1), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.needsIPv6Loopback && !hasIPv6Loopback(t) {
				t.Skip("this machine has no ::1 address")
			}
			chromatik, err := initChromatikForTest(t, chromatikConfig(c.listenSubnet, c.dest))
			if err == nil {
				_, sendErr := chromatik.ArtNetSend([]byte("frame"))
				t.Fatalf("no error; the socket was bound to %v with destination %v, and sending gives: %v",
					chromatik.ArtNetConnection.LocalAddr(), chromatik.ArtNetNode, sendErr)
			}
			if chromatik != nil {
				t.Errorf("a Chromatik was returned along with the error")
			}
			for _, want := range []string{"--chromatik", "--listen", c.dest.String(), c.listenSubnet} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error does not mention %s: %v", want, err)
				}
			}
		})
	}
}

// An IPv4 destination written in its IPv6 form (::ffff:a.b.c.d) is still an
// IPv4 destination and goes with an IPv4 subnet.
func TestInitChromatikAcceptsIPv4MappedDestination(t *testing.T) {
	expectSameMachine(t, "127.0.0.1/24", net.ParseIP("::ffff:127.0.0.1"), net.IPv4(127, 0, 0, 1))
}

// The same over IPv6, on machines whose loopback interface has ::1.
func TestInitChromatikIPv6Loopback(t *testing.T) {
	if !hasIPv6Loopback(t) {
		t.Skip("this machine has no ::1 address")
	}
	expectSameMachine(t, "::1/128", net.IPv6loopback, net.IPv6loopback)
}

// Without --listen the socket is bound to the local address that reaches
// the destination. For a loopback destination that is the destination's
// own address, however the destination is written, with a port of the
// game's own.
func TestInitChromatikWithoutListenSubnet(t *testing.T) {
	expectSameMachine(t, "", net.ParseIP("::ffff:127.0.0.1"), net.IPv4(127, 0, 0, 1))
	// The unspecified address is taken for this machine as well, except
	// on Windows, where it is refused. Which address the system sends
	// from to reach it is its own business.
	if unspecifiedReachesThisMachine {
		expectSameMachine(t, "", net.IPv4zero, nil)
	}
	if hasIPv6Loopback(t) {
		expectSameMachine(t, "", net.IPv6loopback, net.IPv6loopback)
	}
}

// Without --listen and with Chromatik on another machine, the socket must
// be bound to the address of the interface that reaches it. This is the
// case the automatic choice exists for: a socket bound to 127.0.0.1 cannot
// send to another machine at all. Here too the game sends from a port of
// its own and leaves the Art-Net port alone: whether another program on
// this machine has that port makes no difference, and is tried both ways.
// 192.0.2.1 is a documentation address that no machine has; nothing is
// sent, the route is only looked up. On a machine with no route to it,
// such as one with only loopback, the test is skipped.
//
// With the port free, the test also shows that it can still be taken once
// the game runs. On a machine where another program has the port already,
// that one thing cannot be tried: the test checks the rest, says so and is
// skipped.
func TestInitChromatikWithoutListenSubnetRemoteDestination(t *testing.T) {
	dest := net.IPv4(192, 0, 2, 1)
	from, err := localAddrFor(dest)
	if err != nil {
		t.Skipf("this machine has no route to %v: %v", dest, err)
	}

	for _, portTaken := range []bool{false, true} {
		name := "the Art-Net port free"
		if portTaken {
			name = "the Art-Net port taken"
		}
		t.Run(name, func(t *testing.T) {
			// Whether the port is free is found out before the game
			// starts, by taking it: only then can the test tell, further
			// down, a port that the game keeps others from taking from
			// one that another program on this machine has had all along,
			// a Chromatik that is running for example.
			var heldBefore error
			if portTaken {
				holder, err := net.ListenUDP("udp", &net.UDPAddr{IP: from, Port: testArtNetPort})
				if err != nil {
					t.Skipf("cannot take port %d to stand in for another Art-Net program: %v", testArtNetPort, err)
				}
				defer holder.Close()
			} else if probe, err := net.ListenUDP("udp", &net.UDPAddr{IP: from, Port: testArtNetPort}); err != nil {
				heldBefore = err
			} else {
				probe.Close()
			}

			var log bytes.Buffer
			cfg := chromatikConfig("", dest)
			cfg.Logger = slog.New(slog.NewTextHandler(&log, nil))
			chromatik, err := initChromatikForTest(t, cfg)
			if err != nil {
				t.Fatalf("InitChromatik: %v", err)
			}

			local := chromatik.ArtNetConnection.LocalAddr().(*net.UDPAddr)
			if !local.IP.Equal(from) || local.IP.IsLoopback() || !isOwnAddress(t, local.IP) {
				t.Errorf("the socket is bound to %v, want %v, one of this machine's addresses other than loopback", local, from)
			}
			if local.Port == 0 || local.Port == testArtNetPort {
				t.Errorf("the socket is bound to port %d, want a port of its own, not %d", local.Port, testArtNetPort)
			}
			if node := chromatik.ArtNetNode; !node.IP.Equal(dest) || node.Port != testArtNetPort {
				t.Errorf("frames are addressed to %v, want %v port %d", node, dest, testArtNetPort)
			}
			if strings.Contains(log.String(), "level=WARN") {
				t.Errorf("a warning was logged; the log is %q", log.String())
			}
			if !strings.Contains(log.String(), "from="+local.String()) {
				t.Errorf("the log does not show the socket's address %v; the log is %q", local, log.String())
			}

			// With the game running, another program can still take the
			// Art-Net port on the address the game sends from. That can
			// only be tried with a port that was free to begin with. If
			// it was not, everything above has been checked all the same,
			// with a real program holding the port.
			if !portTaken {
				if heldBefore != nil {
					t.Skipf("port %d on %v was taken before the game started, by another program on this machine (a running Chromatik?), so whether the game leaves it free cannot be checked here: %v",
						testArtNetPort, from, heldBefore)
				}
				later, err := net.ListenUDP("udp", &net.UDPAddr{IP: from, Port: testArtNetPort})
				if err != nil {
					t.Errorf("with the game running, port %d cannot be taken: %v", testArtNetPort, err)
				} else {
					later.Close()
				}
			}
		})
	}
}

// A --chromatik address that is one of this machine's own, other than
// loopback, means Chromatik runs here and listens on that address. The
// game sends from that same address, without --listen because it is the
// address the system reaches itself from, and with a --listen subnet that
// holds it. Either way the Art-Net port is left to Chromatik. On a machine
// with only loopback there is nothing to try.
func TestInitChromatikOwnAddress(t *testing.T) {
	for _, own := range ownAddresses(t) {
		subnet := own.String() + "/32"
		if own.To4() == nil {
			subnet = own.String() + "/128"
		}
		expectSameMachine(t, "", own, own)
		expectSameMachine(t, subnet, own, own)
	}
}

// isOwnAddress reports whether ip is one of this machine's addresses.
func isOwnAddress(t *testing.T, ip net.IP) bool {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatalf("InterfaceAddrs: %v", err)
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.Equal(ip) {
			return true
		}
	}
	return false
}

// localAddrFor gives the address this machine sends from to reach a
// destination. For each of this machine's own addresses that is the
// address itself, which lets the test run without knowing the network.
func TestLocalAddrForOwnAddresses(t *testing.T) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatalf("InterfaceAddrs: %v", err)
	}
	tried := 0
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLinkLocalUnicast() {
			// A link-local address cannot be used without naming its
			// interface.
			continue
		}
		tried++
		got, err := localAddrFor(ipnet.IP)
		if err != nil {
			t.Errorf("localAddrFor(%v): %v", ipnet.IP, err)
			continue
		}
		if !got.Equal(ipnet.IP) {
			t.Errorf("localAddrFor(%v) = %v, want the address itself", ipnet.IP, got)
		}
	}
	if tried == 0 {
		t.Fatal("this machine has no address to try, not even loopback")
	}
}

// An address that cannot be reached is an error that names the address,
// the flag it came from and the flag that overrides the choice. fe80::1 is
// a link-local address, which cannot be used without naming an interface,
// so it fails on every machine whatever its routes are.
func TestLocalAddrForUnreachable(t *testing.T) {
	// Windows finds a way to a link-local address with no interface named.
	if runtime.GOOS == "windows" {
		t.Skip("fe80::1 is not unreachable on Windows")
	}
	_, err := localAddrFor(net.ParseIP("fe80::1"))
	if err == nil {
		t.Fatal("localAddrFor(fe80::1) succeeded")
	}
	for _, want := range []string{"fe80::1", "--chromatik", "--listen"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %s: %v", want, err)
		}
	}
}

// cidrAddr returns the address and mask in s, such as "10.0.0.7/24", in
// the form net.InterfaceAddrs reports an interface address.
func cidrAddr(t *testing.T, s string) *net.IPNet {
	t.Helper()
	ip, network, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("bad test address %q: %v", s, err)
	}
	return &net.IPNet{IP: ip, Mask: network.Mask}
}

func TestLocalAddrInSubnetPicksTheMatchingAddress(t *testing.T) {
	// What a machine with a loopback, a wired and a wireless interface
	// reports, plus one entry of a type other than *net.IPNet.
	addrs := []net.Addr{
		&net.IPAddr{IP: net.IPv4(127, 0, 0, 9)},
		cidrAddr(t, "127.0.0.1/8"),
		cidrAddr(t, "::1/128"),
		cidrAddr(t, "10.0.0.7/24"),
		cidrAddr(t, "10.0.0.8/24"),
		cidrAddr(t, "fd00:1::5/64"),
		cidrAddr(t, "fe80::1/64"),
		cidrAddr(t, "192.168.1.20/24"),
	}
	cases := []struct {
		name, subnet, want string
	}{
		{"the default subnet picks loopback, skipping the entry that is not an IPNet", "127.0.0.1/24", "127.0.0.1"},
		{"a later interface", "192.168.1.0/24", "192.168.1.20"},
		{"the first of two addresses in the subnet", "10.0.0.0/24", "10.0.0.7"},
		{"a subnet written with host bits set", "10.0.0.99/24", "10.0.0.7"},
		{"a subnet wider than the interface's own mask", "192.168.0.0/16", "192.168.1.20"},
		{"an IPv6 subnet", "fd00:1::/64", "fd00:1::5"},
		{"IPv6 loopback", "::1/128", "::1"},
	}
	for _, c := range cases {
		got, err := localAddrInSubnet(c.subnet, addrs)
		if err != nil {
			t.Errorf("%s: subnet %s: %v", c.name, c.subnet, err)
			continue
		}
		if !got.Equal(net.ParseIP(c.want)) {
			t.Errorf("%s: subnet %s picked %v, want %s", c.name, c.subnet, got, c.want)
		}
	}
}

func TestLocalAddrInSubnetInvalidSubnet(t *testing.T) {
	addrs := []net.Addr{cidrAddr(t, "127.0.0.1/8")}
	for _, subnet := range []string{"", "banana", "127.0.0.1", "127.0.0.1/33", "fd00::/129"} {
		ip, err := localAddrInSubnet(subnet, addrs)
		if err == nil {
			t.Errorf("subnet %q: no error, picked %v", subnet, ip)
			continue
		}
		if ip != nil {
			t.Errorf("subnet %q: an address was returned along with the error: %v", subnet, ip)
		}
		if subnet != "" && !strings.Contains(err.Error(), subnet) {
			t.Errorf("subnet %q: error does not show the value: %v", subnet, err)
		}
	}
}

func TestLocalAddrInSubnetNoMatch(t *testing.T) {
	cases := []struct {
		name   string
		subnet string
		addrs  []net.Addr
	}{
		{"other subnets only", "10.0.0.0/24", []net.Addr{cidrAddr(t, "127.0.0.1/8"), cidrAddr(t, "192.168.1.20/24"), cidrAddr(t, "::1/128")}},
		{"a subnet inside the interface's network that leaves its address out", "192.168.1.128/25", []net.Addr{cidrAddr(t, "192.168.1.20/24")}},
		{"IPv6 addresses only, IPv4 subnet", "127.0.0.1/24", []net.Addr{cidrAddr(t, "::1/128"), cidrAddr(t, "fe80::1/64")}},
		{"IPv4 addresses only, IPv6 subnet", "fd00:1::/64", []net.Addr{cidrAddr(t, "127.0.0.1/8")}},
		{"the only address in the subnet is not an IPNet", "127.0.0.1/24", []net.Addr{&net.IPAddr{IP: net.IPv4(127, 0, 0, 1)}}},
		{"no addresses at all", "127.0.0.1/24", nil},
	}
	for _, c := range cases {
		ip, err := localAddrInSubnet(c.subnet, c.addrs)
		if err == nil {
			t.Errorf("%s: no error, picked %v", c.name, ip)
			continue
		}
		if ip != nil {
			t.Errorf("%s: an address was returned along with the error: %v", c.name, ip)
		}
		if !strings.Contains(err.Error(), c.subnet) {
			t.Errorf("%s: error does not name the subnet %s: %v", c.name, c.subnet, err)
		}
		for _, addr := range c.addrs {
			if !strings.Contains(err.Error(), addr.String()) {
				t.Errorf("%s: error does not list %s: %v", c.name, addr, err)
			}
		}
	}
}
