package config

import (
	"net"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// parse runs Parse on a flag set of its own that returns an error for bad
// arguments. The program's own flag set exits instead.
func parse(t *testing.T, args ...string) (Config, *pflag.FlagSet) {
	t.Helper()
	flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err := Parse(flags, args)
	if err != nil {
		t.Fatalf("Parse(%q): %v", args, err)
	}
	return cfg, flags
}

// same compares two configurations. Addresses are compared as addresses:
// 127.0.0.1 is the same whether it is held in 4 bytes or in 16.
func same(a, b Config) bool {
	a.ArtNetDest, b.ArtNetDest = a.ArtNetDest.To16(), b.ArtNetDest.To16()
	return reflect.DeepEqual(a, b)
}

// TestDefaults is the configuration of a program started with no arguments,
// written out in full: a changed default has to be changed here as well.
func TestDefaults(t *testing.T) {
	got, _ := parse(t)
	want := Config{
		AbletonDest:    "",
		ArtNetDest:     net.IPv4(127, 0, 0, 1),
		ControllerPath: "/dev/hidraw3",
		DemoMode:       false,
		Keyboard:       false,
		ListenSubnet:   "",
		Logger:         nil,
		NoJoy:          false,
		OscDest:        "",
		OscPort:        3030,
		PrintVersion:   false,
		Verbosity:      0,
	}
	if !same(got, want) {
		t.Errorf("with no arguments the configuration is\n%+v, want\n%+v", got, want)
	}
}

// TestEachFlag gives one flag at a time, in the forms a command line can
// take, and checks the one field it is for. Everything else must stay at
// its default.
func TestEachFlag(t *testing.T) {
	defaults, _ := parse(t)

	for _, c := range []struct {
		args []string
		// set makes the expected change to a configuration of defaults.
		set func(*Config)
	}{
		{[]string{"--demo"}, func(c *Config) { c.DemoMode = true }},
		{[]string{"--demo=true"}, func(c *Config) { c.DemoMode = true }},
		{[]string{"--demo=false"}, func(c *Config) {}},
		{[]string{"--nojoy"}, func(c *Config) { c.NoJoy = true }},
		{[]string{"--keyboard"}, func(c *Config) { c.Keyboard = true }},
		{[]string{"--version"}, func(c *Config) { c.PrintVersion = true }},

		{[]string{"-v"}, func(c *Config) { c.Verbosity = 1 }},
		{[]string{"-vv"}, func(c *Config) { c.Verbosity = 2 }},
		{[]string{"-vvv"}, func(c *Config) { c.Verbosity = 3 }},
		{[]string{"-v", "-v"}, func(c *Config) { c.Verbosity = 2 }},
		{[]string{"--verbose"}, func(c *Config) { c.Verbosity = 1 }},
		{[]string{"--verbose", "--verbose", "-v"}, func(c *Config) { c.Verbosity = 3 }},

		{[]string{"--chromatik", "10.0.0.5"}, func(c *Config) { c.ArtNetDest = net.IPv4(10, 0, 0, 5) }},
		{[]string{"--chromatik=10.0.0.5"}, func(c *Config) { c.ArtNetDest = net.IPv4(10, 0, 0, 5) }},
		{[]string{"--chromatik", "fd00::5"}, func(c *Config) { c.ArtNetDest = net.ParseIP("fd00::5") }},
		{[]string{"--joystick", "/dev/hidraw0"}, func(c *Config) { c.ControllerPath = "/dev/hidraw0" }},
		{[]string{"--joystick=/dev/hidraw0"}, func(c *Config) { c.ControllerPath = "/dev/hidraw0" }},
		{[]string{"--listen", "10.0.0.0/24"}, func(c *Config) { c.ListenSubnet = "10.0.0.0/24" }},
		{[]string{"--osc-dest", "chromatik.local"}, func(c *Config) { c.OscDest = "chromatik.local" }},
		{[]string{"--osc-port", "9000"}, func(c *Config) { c.OscPort = 9000 }},
		{[]string{"--osc-port=9000"}, func(c *Config) { c.OscPort = 9000 }},
		{[]string{"--ableton", "10.0.0.7:11000"}, func(c *Config) { c.AbletonDest = "10.0.0.7:11000" }},

		// An empty value is a value: it is what turns these back off.
		{[]string{"--ableton", ""}, func(c *Config) {}},
		{[]string{"--listen="}, func(c *Config) {}},

		// The last one given wins.
		{[]string{"--osc-port", "1", "--osc-port", "2"}, func(c *Config) { c.OscPort = 2 }},
		{[]string{"--demo", "--demo=false"}, func(c *Config) {}},
	} {
		got, _ := parse(t, c.args...)
		want := defaults
		c.set(&want)
		if !same(got, want) {
			t.Errorf("%q gives\n%+v, want\n%+v", c.args, got, want)
		}
	}
}

// TestAllFlagsTogether gives every flag at once, in an order unlike the one
// they are declared in.
func TestAllFlagsTogether(t *testing.T) {
	got, _ := parse(t,
		"--ableton", "10.0.0.7:11000", "-vv", "--osc-port", "9000", "--keyboard",
		"--osc-dest", "10.0.0.6", "--version", "--listen", "10.0.0.0/24",
		"--joystick", "/dev/hidraw1", "--nojoy", "--chromatik", "10.0.0.5", "--demo")
	want := Config{
		AbletonDest:    "10.0.0.7:11000",
		ArtNetDest:     net.IPv4(10, 0, 0, 5),
		ControllerPath: "/dev/hidraw1",
		DemoMode:       true,
		Keyboard:       true,
		ListenSubnet:   "10.0.0.0/24",
		NoJoy:          true,
		OscDest:        "10.0.0.6",
		OscPort:        9000,
		PrintVersion:   true,
		Verbosity:      2,
	}
	if !same(got, want) {
		t.Errorf("with every flag the configuration is\n%+v, want\n%+v", got, want)
	}
}

// TestBadArguments are command lines that must be refused, with an error
// that says which flag or value is wrong.
func TestBadArguments(t *testing.T) {
	for _, c := range []struct {
		args    []string
		mention string
	}{
		{[]string{"--nope"}, "nope"},
		{[]string{"-x"}, "x"},
		{[]string{"--chromatik", "not-an-address"}, "chromatik"},
		{[]string{"--chromatik", "10.0.0"}, "chromatik"},
		{[]string{"--chromatik", "chromatik.local"}, "chromatik"},
		{[]string{"--chromatik"}, "chromatik"},
		{[]string{"--osc-port", "abc"}, "osc-port"},
		{[]string{"--osc-port", "30.5"}, "osc-port"},
		{[]string{"--osc-port"}, "osc-port"},
		{[]string{"--demo=maybe"}, "demo"},
		{[]string{"--joystick"}, "joystick"},

		// An empty path opens nothing.
		{[]string{"--joystick", ""}, "--joystick"},
		{[]string{"--joystick="}, "--joystick"},

		// Words that are neither a flag nor a flag's value. An on/off
		// flag takes no separate value, so "false" is one of them, and
		// so is the 2 after a flag that is counted, not given a number.
		{[]string{"stray"}, `"stray"`},
		{[]string{"--demo", "false"}, `"false"`},
		{[]string{"--verbose", "2"}, `"2"`},
		{[]string{"--nojoy", "--chromatik", "10.0.0.5", "left", "over"}, `"left"`},
		{[]string{"--", "--demo"}, `"--demo"`},
		{[]string{"chromatik=10.0.0.5"}, `"chromatik=10.0.0.5"`},
	} {
		_, err := Parse(pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError), c.args)
		if err == nil {
			t.Errorf("%q was accepted", c.args)
		} else if !strings.Contains(err.Error(), c.mention) {
			t.Errorf("%q: the error does not mention %q: %v", c.args, c.mention, err)
		}
	}
}

// TestEveryFieldHasOneFlag ties the fields of Config to the flags. Setting
// any one flag to a value other than its default must change exactly one
// field, no two flags may change the same field, and every field but
// Logger must be reached. So a field added without a flag, a flag that
// forgets to store its value, and two flags sharing a field all fail here.
func TestEveryFieldHasOneFlag(t *testing.T) {
	defaults, flags := parse(t)

	// A value of each kind that no flag has as its default.
	sample := map[string]string{
		"bool":   "true",
		"count":  "3",
		"int":    "4711",
		"ip":     "192.0.2.9",
		"string": "something-else",
	}

	fieldOfFlag := map[string]string{}
	flags.VisitAll(func(f *pflag.Flag) {
		value, ok := sample[f.Value.Type()]
		if !ok {
			t.Errorf("--%s is of a kind this test has no value for: %s", f.Name, f.Value.Type())
			return
		}
		got, _ := parse(t, "--"+f.Name+"="+value)

		var changed []string
		was, is := reflect.ValueOf(defaults), reflect.ValueOf(got)
		for i := 0; i < was.NumField(); i++ {
			if !reflect.DeepEqual(was.Field(i).Interface(), is.Field(i).Interface()) {
				changed = append(changed, was.Type().Field(i).Name)
			}
		}
		if len(changed) != 1 {
			t.Errorf("--%s=%s changed the fields %v, want exactly one", f.Name, value, changed)
			return
		}
		fieldOfFlag[f.Name] = changed[0]
	})

	flagOfField := map[string]string{}
	for flag, field := range fieldOfFlag {
		if other, taken := flagOfField[field]; taken {
			t.Errorf("--%s and --%s both set %s", flag, other, field)
		}
		flagOfField[field] = flag
	}

	typ := reflect.TypeOf(defaults)
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if name == "Logger" {
			continue // made by the program from Verbosity
		}
		if flagOfField[name] == "" {
			t.Errorf("no flag sets Config.%s", name)
		}
	}
	if flagOfField["Logger"] != "" {
		t.Errorf("--%s sets Config.Logger, which the program makes itself", flagOfField["Logger"])
	}
}

// TestFlagNamesAndHelp pins the names people type: the full list of flags
// and the one short form. Every flag has to appear in --help with some
// description (the wording is free), and each default is pinned as the
// flag declares it. --help prints the three that are not empty, false or 0.
func TestFlagNamesAndHelp(t *testing.T) {
	_, flags := parse(t)

	var names []string
	flags.VisitAll(func(f *pflag.Flag) {
		names = append(names, f.Name)
		if strings.TrimSpace(f.Usage) == "" {
			t.Errorf("--%s has no description", f.Name)
		}
		if f.Shorthand != "" && f.Name != "verbose" {
			t.Errorf("--%s has the short form -%s; only --verbose has one", f.Name, f.Shorthand)
		}
	})
	slices.Sort(names)
	want := []string{"ableton", "chromatik", "demo", "joystick", "keyboard", "listen", "nojoy", "osc-dest", "osc-port", "verbose", "version"}
	if !slices.Equal(names, want) {
		t.Errorf("the flags are %v, want %v", names, want)
	}
	if got := flags.Lookup("verbose").Shorthand; got != "v" {
		t.Errorf("--verbose has the short form %q, want v", got)
	}

	// What --help prints. A hidden or deprecated flag would be missing.
	help := flags.FlagUsages()
	for _, name := range want {
		if !strings.Contains(help, "--"+name) {
			t.Errorf("--help does not list --%s", name)
		}
	}
	for _, shown := range []string{"(default 127.0.0.1)", `(default "/dev/hidraw3")`, "(default 3030)"} {
		if !strings.Contains(help, shown) {
			t.Errorf("--help does not show %s", shown)
		}
	}

	for name, def := range map[string]string{
		"ableton":   "",
		"chromatik": "127.0.0.1",
		"demo":      "false",
		"joystick":  "/dev/hidraw3",
		"keyboard":  "false",
		"listen":    "",
		"nojoy":     "false",
		"osc-dest":  "",
		"osc-port":  "3030",
		"verbose":   "0",
		"version":   "false",
	} {
		if got := flags.Lookup(name).DefValue; got != def {
			t.Errorf("the default of --%s is declared as %q, want %q", name, got, def)
		}
	}
}

// TestParseTwice: a flag set can only be given the flags once. Calling
// Parse again with the same set is a mistake in the program, and panics
// instead of quietly giving two sets of flags.
func TestParseTwice(t *testing.T) {
	flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	if _, err := Parse(flags, nil); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Error("a second Parse with the same flag set did not panic")
		}
	}()
	_, _ = Parse(flags, nil)
}

// TestDefaultChromatikAddress starts from an empty command line, as when
// the program is run without --chromatik, and checks that the default
// Art-Net destination is the loopback address and that a datagram can
// really be sent to it. Only loopback sockets on ports the kernel picks
// are used.
func TestDefaultChromatikAddress(t *testing.T) {
	flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err := Parse(flags, nil)
	if err != nil {
		t.Fatalf("Parse with no arguments: %v", err)
	}

	loopback := net.IPv4(127, 0, 0, 1)
	if !cfg.ArtNetDest.Equal(loopback) {
		t.Errorf("default --chromatik is %q (% x), want 127.0.0.1", cfg.ArtNetDest, []byte(cfg.ArtNetDest))
	}
	if got := flags.Lookup("chromatik").DefValue; got != "127.0.0.1" {
		t.Errorf("--help shows the --chromatik default as %q, want 127.0.0.1", got)
	}

	// The address must be usable as a UDP destination.
	if _, err := net.ResolveUDPAddr("udp", net.JoinHostPort(cfg.ArtNetDest.String(), "6454")); err != nil {
		t.Errorf("default --chromatik does not resolve: %v", err)
	}

	sink, err := net.ListenUDP("udp", &net.UDPAddr{IP: loopback})
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	defer sink.Close()
	src, err := net.ListenUDP("udp", &net.UDPAddr{IP: loopback})
	if err != nil {
		t.Fatalf("listen on loopback: %v", err)
	}
	defer src.Close()

	dst := &net.UDPAddr{IP: cfg.ArtNetDest, Port: sink.LocalAddr().(*net.UDPAddr).Port}
	if _, err := src.WriteTo([]byte("frame"), dst); err != nil {
		t.Fatalf("send to the default --chromatik address: %v", err)
	}
	buf := make([]byte, 16)
	_ = sink.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := sink.ReadFromUDP(buf)
	if err != nil || string(buf[:n]) != "frame" {
		t.Errorf("datagram sent to the default --chromatik address did not arrive: got %q, err %v", buf[:n], err)
	}
}

// TestChromatikFlag checks that an address given on the command line
// replaces the default.
func TestChromatikFlag(t *testing.T) {
	flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err := Parse(flags, []string{"--chromatik", "10.0.0.5"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !cfg.ArtNetDest.Equal(net.IPv4(10, 0, 0, 5)) {
		t.Errorf("--chromatik 10.0.0.5 gave %q", cfg.ArtNetDest)
	}
}

// TestListenFlag checks that --listen is empty unless it is given. Empty
// tells the game to pick the local Art-Net address from the route to
// --chromatik.
func TestListenFlag(t *testing.T) {
	flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err := Parse(flags, nil)
	if err != nil {
		t.Fatalf("Parse with no arguments: %v", err)
	}
	if cfg.ListenSubnet != "" {
		t.Errorf("default --listen is %q, want it empty", cfg.ListenSubnet)
	}

	flags = pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
	cfg, err = Parse(flags, []string{"--listen", "10.0.0.0/24"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.ListenSubnet != "10.0.0.0/24" {
		t.Errorf("--listen 10.0.0.0/24 gave %q", cfg.ListenSubnet)
	}
}

// TestAbletonFlag checks the three forms of --ableton: left out or given
// empty, the address is empty, which leaves Ableton off; given, it is the
// address to send to.
func TestAbletonFlag(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"not given", nil, ""},
		{"given", []string{"--ableton", "10.0.0.7:9000"}, "10.0.0.7:9000"},
		{"empty", []string{"--ableton", ""}, ""},
	} {
		flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
		cfg, err := Parse(flags, c.args)
		if err != nil {
			t.Fatalf("%s: Parse: %v", c.name, err)
		}
		if cfg.AbletonDest != c.want {
			t.Errorf("%s: the Ableton address is %q, want %q", c.name, cfg.AbletonDest, c.want)
		}
	}
}

// TestOscDestFlag checks that --osc-dest is empty unless it is given. Empty
// tells the game to send OSC to the --chromatik address.
func TestOscDestFlag(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"not given", nil, ""},
		{"given", []string{"--osc-dest", "10.0.0.7"}, "10.0.0.7"},
	} {
		flags := pflag.NewFlagSet("dsk-eel", pflag.ContinueOnError)
		cfg, err := Parse(flags, c.args)
		if err != nil {
			t.Fatalf("%s: Parse: %v", c.name, err)
		}
		if cfg.OscDest != c.want {
			t.Errorf("%s: the OSC host is %q, want %q", c.name, cfg.OscDest, c.want)
		}
	}
}
