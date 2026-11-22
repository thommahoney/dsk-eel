//go:build !linux && !darwin

package controller

import (
	"errors"

	"github.com/thommahoney/dsk-eel/config"
)

// NewKeyboard is implemented for Linux and macOS.
func NewKeyboard(config *config.Config, handleFunc func(ControllerState)) (*Keyboard, error) {
	return nil, errors.New("--keyboard is not supported on this system yet")
}
