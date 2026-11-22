//go:build !linux

package controller

import (
	"errors"

	"github.com/thommahoney/dsk-eel/config"
)

// NewKeyboard is only implemented for Linux, where the game runs and is
// developed.
func NewKeyboard(config *config.Config, handleFunc func(ControllerState)) (*Keyboard, error) {
	return nil, errors.New("--keyboard is only supported on Linux")
}
