package controller

import (
	"log/slog"

	"github.com/eiannone/keyboard"
	"github.com/thommahoney/dsk-eel/config"
)

type Keyboard struct {
	Handler func(ControllerState)
	logger  *slog.Logger
}

func NewKeyboard(config *config.Config) (*Keyboard, error) {
	k := &Keyboard{
		logger: config.Logger,
	}

	if err := keyboard.Open(); err != nil {
		return nil, err
	}
	defer func() {
		_ = keyboard.Close()
	}()

	k.logger.Info("connected to keyboard")

	keysEvents, err := keyboard.GetKeys(10) // Buffer size of 10
	if err != nil {
		k.logger.Error("error in keyboard.GetKeys", "error", err)
	}

	go func() {
		for {
			changed := false

			event := <-keysEvents
			if event.Err != nil {
				k.logger.Error("error in keyboard channel", "error", event.Err)
			}

			state := &ControllerState{}

			switch event.Key {
			case keyboard.KeyArrowUp:
				changed = true
				state.Direction = Dir_North
			case keyboard.KeyArrowDown:
				changed = true
				state.Direction = Dir_South
			case keyboard.KeyArrowLeft:
				changed = true
				state.Direction = Dir_West
			case keyboard.KeyArrowRight:
				changed = true
				state.Direction = Dir_East
			}

			switch event.Rune {
			case rune('w'):
				changed = true
				state.ButtonStatus = Btn_White
			case rune('y'):
				changed = true
				state.ButtonStatus = Btn_Yellow
			case rune('b'):
				changed = true
				state.ButtonStatus = Btn_Blue
			case rune('r'):
				changed = true
				state.ButtonStatus = Btn_Red
			}

			if changed && k.Handler != nil {
				k.Handler(*state)
			}
		}
	}()

	return k, nil
}

func (k *Keyboard) SetHandleFunc(handleFunc func(ControllerState)) {
	k.Handler = handleFunc
}
