package game

import (
	"fmt"

	"github.com/hypebeast/go-osc/osc"
	"github.com/thommahoney/dsk-eel/config"
)

type Ableton struct {
	OscClient *osc.Client
}

func InitAbleton(c *config.Config) (*Ableton, error) {
	oscClient := osc.NewClient("192.168.1.104", 11000)

	return &Ableton{OscClient: oscClient}, nil
}

func (c *Ableton) FireScene(scene int32) {
	c.OscSend("/live/scene/fire", scene)
}

func (c *Ableton) FireClip(clip, track int32) {
	c.OscSend("/live/clip/fire", clip, track)
}

func (c *Ableton) OscSend(address string, values ...int32) error {
	if address == "" {
		return nil
	}

	msg := osc.NewMessage(address)
	for v := range values {
		msg.Append(v)
	}

	err := c.OscClient.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send OSC message: %s", err)
	}

	return nil
}
