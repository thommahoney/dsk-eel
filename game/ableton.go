package game

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/hypebeast/go-osc/osc"
	"github.com/thommahoney/dsk-eel/config"
)

// Ableton sends OSC messages to Ableton Live (the AbletonOSC remote script).
type Ableton struct {
	// OscClient is nil when Ableton is disabled; sends then do nothing.
	OscClient *osc.Client
	// Logger receives send errors at debug level. It may be nil.
	Logger *slog.Logger
}

// InitAbleton sets up the OSC destination from c.AbletonDest, a host:port.
// An empty AbletonDest disables Ableton.
func InitAbleton(c *config.Config) (*Ableton, error) {
	if c.AbletonDest == "" {
		return &Ableton{Logger: c.Logger}, nil
	}

	host, port, err := splitHostPort(c.AbletonDest)
	if err != nil {
		return nil, fmt.Errorf("invalid Ableton address %q (--ableton): %s", c.AbletonDest, err)
	}

	// The OSC client joins host and port with a colon itself, so an IPv6
	// address needs its brackets back.
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}

	return &Ableton{OscClient: osc.NewClient(host, port), Logger: c.Logger}, nil
}

// splitHostPort splits "host:port" and checks that both parts are usable:
// a host that is not empty and a port number from 1 to 65535.
func splitHostPort(hostport string) (string, int, error) {
	host, portText, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", 0, fmt.Errorf("want host:port: %s", err)
	}
	if host == "" {
		return "", 0, fmt.Errorf("want host:port: the host is empty")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("want host:port: the port %q is not a number from 1 to 65535", portText)
	}
	return host, port, nil
}

// FireScene triggers a scene. AbletonOSC counts scenes from 0.
func (c *Ableton) FireScene(scene int32) {
	c.fire("/live/scene/fire", scene)
}

// FireClip starts a clip playing. AbletonOSC takes the track first and then
// the clip slot on that track, both counted from 0.
func (c *Ableton) FireClip(track, clip int32) {
	c.fire("/live/clip/fire", track, clip)
}

// fire sends a message for callers that carry on whether or not Ableton
// got it. A failure is logged at debug level, so that playing without
// Ableton reachable does not fill the log at the default level.
func (c *Ableton) fire(address string, values ...int32) {
	if err := c.OscSend(address, values...); err != nil && c.Logger != nil {
		c.Logger.Debug("Ableton OSC send failed", "address", address, "error", err)
	}
}

// OscSend sends one OSC message whose arguments are the given int32
// values, in order. It does nothing when Ableton is disabled.
func (c *Ableton) OscSend(address string, values ...int32) error {
	if address == "" || c.OscClient == nil {
		return nil
	}

	msg := osc.NewMessage(address)
	for _, v := range values {
		msg.Append(v)
	}

	err := c.OscClient.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send OSC message: %s", err)
	}

	return nil
}
