package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/spf13/pflag"
)

// Config is what the command line asked for. Parse fills it: every field
// but Logger has a flag there, so a new field needs a new flag.
type Config struct {
	AbletonDest    string // host:port of Ableton's OSC server; empty disables Ableton
	ArtNetDest     net.IP
	ControllerPath string
	DemoMode       bool
	Keyboard       bool // steer with the keyboard instead of the joystick
	ListenSubnet   string
	Logger         *slog.Logger
	NoJoy          bool
	OscDest        string
	OscPort        int
	PrintVersion   bool // print the version and exit
	Verbosity      int
}

// Parse registers the command line flags on the given flag set, parses args
// with it and returns the resulting configuration. A flag set can be given
// to it once.
//
// Every field of Config except Logger has a flag here, and each flag sets
// one field. Logger is made by the caller from Verbosity.
//
// If the arguments are wrong, the error says how and the Config is not to
// be used. A flag the library does not know, or a value it cannot read, is
// reported by the library: the program passes pflag.CommandLine, which
// prints the usage and the error and exits by itself, where the flag set a
// test passes returns the error. What only this program knows to be wrong
// is returned as an error to both.
func Parse(flags *pflag.FlagSet, args []string) (Config, error) {
	cfg := Config{}

	flags.BoolVar(&cfg.DemoMode, "demo", false, "Randomly move eel")
	flags.BoolVar(&cfg.NoJoy, "nojoy", false, "Run without a joystick connected")
	flags.BoolVar(&cfg.Keyboard, "keyboard", false, "Steer with the keyboard of this terminal instead of the joystick: arrow keys (the direction stays held), space to centre the stick, r g b y w for the buttons, q to quit")
	flags.CountVarP(&cfg.Verbosity, "verbose", "v", "Log more: -v adds what the game is doing, -vv adds everything")
	flags.BoolVar(&cfg.PrintVersion, "version", false, "Print which build this is and exit")
	cfg.ArtNetDest = net.IPv4(127, 0, 0, 1)
	flags.Var(&hostValue{ip: &cfg.ArtNetDest}, "chromatik", "Address or host name of the machine Chromatik runs on")
	flags.StringVar(&cfg.ControllerPath, "joystick", "/dev/hidraw3", "Joystick device filesystem path")
	flags.StringVar(&cfg.ListenSubnet, "listen", "", "Subnet of the local address to send Art-Net from, e.g. 10.0.0.0/24. Empty picks the address that reaches --chromatik; give it on a machine with more than one network")
	flags.StringVar(&cfg.OscDest, "osc-dest", "", "Host of Chromatik's OSC server. Empty, the default, uses the --chromatik address")
	flags.IntVar(&cfg.OscPort, "osc-port", 3030, "Port of OSC server")
	flags.StringVar(&cfg.AbletonDest, "ableton", "", "host:port of Ableton Live's OSC server (AbletonOSC), e.g. 192.168.1.104:11000. Empty, the default, sends nothing to Ableton")

	if err := flags.Parse(args); err != nil {
		return cfg, err
	}

	// Anything that is not a flag or a flag's value used to be ignored
	// without a word. That hid mistakes: in "--demo false" the word false
	// is such a leftover, and the demo was on.
	if flags.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected argument %q: every argument has to be a flag, like --demo, or the value of one; an on/off flag is turned off as in --demo=false", flags.Arg(0))
	}
	if flags.Changed("joystick") && cfg.ControllerPath == "" {
		return cfg, errors.New("--joystick needs the path of the joystick's device; to run without the joystick use --keyboard, or --nojoy --demo")
	}
	return cfg, nil
}

// hostLookupTimeout is how long --chromatik waits for a name to be looked
// up before it gives up.
const hostLookupTimeout = 5 * time.Second

// hostValue is the value of a flag that takes an address or a host name.
// A name is looked up once, when the flag is read, and what is kept is the
// address: the program sends to that address for as long as it runs. A
// name that cannot be found is an error there and then, like any other
// value a flag cannot take.
type hostValue struct {
	ip *net.IP
}

func (h *hostValue) String() string {
	if h.ip == nil || *h.ip == nil {
		return ""
	}
	return h.ip.String()
}

func (h *hostValue) Type() string { return "host" }

func (h *hostValue) Set(value string) error {
	if ip := net.ParseIP(value); ip != nil {
		*h.ip = ip
		return nil
	}
	if value == "" {
		return errors.New("an address or a host name is needed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), hostLookupTimeout)
	defer cancel()
	// Art-Net is at home on IPv4, so that is what a name with both kinds
	// of address stands for.
	for _, network := range []string{"ip4", "ip6"} {
		if ips, err := net.DefaultResolver.LookupIP(ctx, network, value); err == nil && len(ips) > 0 {
			*h.ip = ips[0]
			return nil
		}
	}
	return fmt.Errorf("%q is not an address, and no host of that name could be found", value)
}
