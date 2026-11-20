package game

import (
	"errors"
	"net"
	"strings"
	"syscall"
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
// the Art-Net socket, which is bound to the fixed port 6454, and the OSC
// client's socket.
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

// expectSendToSelf sends a datagram with ArtNetSend and reads it back from
// the same socket. That works when the socket is bound to the address and
// port that frames are sent to, as in the loopback tests below.
func expectSendToSelf(t *testing.T, chromatik *Chromatik) {
	t.Helper()
	if _, err := chromatik.ArtNetSend([]byte("frame")); err != nil {
		t.Fatalf("ArtNetSend: %v", err)
	}
	buf := make([]byte, 16)
	_ = chromatik.ArtNetConnection.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := chromatik.ArtNetConnection.ReadFromUDP(buf)
	if err != nil || string(buf[:n]) != "frame" {
		t.Errorf("the datagram did not arrive: got %q, err %v", buf[:n], err)
	}
}

// skipIfPortBusy skips the test when the Art-Net port is already bound by
// something else on this machine, which says nothing about the code.
func skipIfPortBusy(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, syscall.EADDRINUSE) {
		t.Skipf("port %d is in use: %v", testArtNetPort, err)
	}
}

func TestInitChromatikRejectsInvalidSubnet(t *testing.T) {
	for _, subnet := range []string{"", "banana", "127.0.0.1", "127.0.0.1/33"} {
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

// With the default subnet the socket is bound to 127.0.0.1:6454 and frames
// go to port 6454 of the destination.
func TestInitChromatikLoopback(t *testing.T) {
	chromatik, err := initChromatikForTest(t, chromatikConfig("127.0.0.1/24", net.IPv4(127, 0, 0, 1)))
	skipIfPortBusy(t, err)
	if err != nil {
		t.Fatalf("InitChromatik: %v", err)
	}

	local := chromatik.ArtNetConnection.LocalAddr().(*net.UDPAddr)
	if !local.IP.Equal(net.IPv4(127, 0, 0, 1)) || local.Port != testArtNetPort {
		t.Errorf("socket is bound to %v, want 127.0.0.1:%d", local, testArtNetPort)
	}
	node := chromatik.ArtNetNode
	if !node.IP.Equal(net.IPv4(127, 0, 0, 1)) || node.Port != testArtNetPort {
		t.Errorf("destination is %v, want 127.0.0.1:%d", node, testArtNetPort)
	}
	expectSendToSelf(t, chromatik)
}

// Frames go to the --chromatik address, which is usually another machine,
// while the socket stays bound to the local address in the subnet. Nothing
// is sent here; only the addresses are checked.
func TestInitChromatikSendsToTheConfiguredDestination(t *testing.T) {
	dest := net.IPv4(10, 0, 0, 5)
	chromatik, err := initChromatikForTest(t, chromatikConfig("127.0.0.1/24", dest))
	skipIfPortBusy(t, err)
	if err != nil {
		t.Fatalf("InitChromatik: %v", err)
	}

	local := chromatik.ArtNetConnection.LocalAddr().(*net.UDPAddr)
	if !local.IP.Equal(net.IPv4(127, 0, 0, 1)) || local.Port != testArtNetPort {
		t.Errorf("socket is bound to %v, want 127.0.0.1:%d", local, testArtNetPort)
	}
	node := chromatik.ArtNetNode
	if !node.IP.Equal(dest) || node.Port != testArtNetPort {
		t.Errorf("destination is %v, want %v:%d", node, dest, testArtNetPort)
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
// the other way round, and failed sends are not reported during play. So a
// --chromatik address of one kind with a --listen subnet of the other must
// be refused at startup, with a message that names both flags.
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
			skipIfPortBusy(t, err)
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
	chromatik, err := initChromatikForTest(t, chromatikConfig("127.0.0.1/24", net.ParseIP("::ffff:127.0.0.1")))
	skipIfPortBusy(t, err)
	if err != nil {
		t.Fatalf("InitChromatik: %v", err)
	}
	expectSendToSelf(t, chromatik)
}

// The same over IPv6, on machines whose loopback interface has ::1.
func TestInitChromatikIPv6Loopback(t *testing.T) {
	if !hasIPv6Loopback(t) {
		t.Skip("this machine has no ::1 address")
	}

	chromatik, err := initChromatikForTest(t, chromatikConfig("::1/128", net.IPv6loopback))
	skipIfPortBusy(t, err)
	if err != nil {
		t.Fatalf("InitChromatik: %v", err)
	}

	local := chromatik.ArtNetConnection.LocalAddr().(*net.UDPAddr)
	if !local.IP.Equal(net.IPv6loopback) || local.Port != testArtNetPort {
		t.Errorf("socket is bound to %v, want [::1]:%d", local, testArtNetPort)
	}
	node := chromatik.ArtNetNode
	if !node.IP.Equal(net.IPv6loopback) || node.Port != testArtNetPort {
		t.Errorf("destination is %v, want [::1]:%d", node, testArtNetPort)
	}
	expectSendToSelf(t, chromatik)
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
