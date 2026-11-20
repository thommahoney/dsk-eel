package game

import (
	"fmt"
	"log/slog"

	"github.com/thommahoney/dsk-eel/config"
)

// Ableton sends OSC messages to Ableton Live (the AbletonOSC remote script).
type Ableton struct {
	// OscClient is nil when Ableton is disabled; sends then do nothing.
	OscClient *OscClient
	// Logger receives send errors at debug level. It may be nil.
	Logger *slog.Logger
}

// InitAbleton sets up the OSC destination from c.AbletonDest, a host:port.
// An empty AbletonDest disables Ableton.
func InitAbleton(c *config.Config) (*Ableton, error) {
	if c.AbletonDest == "" {
		return &Ableton{Logger: c.Logger}, nil
	}

	client, err := NewOscClient(c.AbletonDest)
	if err != nil {
		return nil, fmt.Errorf("invalid Ableton address %q (--ableton): %s", c.AbletonDest, err)
	}

	return &Ableton{OscClient: client, Logger: c.Logger}, nil
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

	err := c.OscClient.SendInts(address, values...)
	if err != nil {
		return fmt.Errorf("failed to send OSC message: %s", err)
	}

	return nil
}
