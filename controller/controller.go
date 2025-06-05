package controller

import (
	"log"
	"log/slog"
	"strings"

	"github.com/sstallion/go-hid"
)

type Controller struct {
	device *hid.Device
	data   []byte
	logger *slog.Logger

	Joystick *Joystick
	Buttons  *Buttons
}

type ControllerState struct {
	Direction    Direction
	ButtonStatus ButtonStatus
}

type Joystick struct {
	logger *slog.Logger

	Manufacturer string
	Product      string
}

type Direction int

const (
	Dir_Neutral   = Direction(0b0101)
	Dir_North     = Direction(0b1101)
	Dir_NorthEast = Direction(0b1111)
	Dir_East      = Direction(0b0111)
	Dir_SouthEast = Direction(0b0011)
	Dir_South     = Direction(0b0001)
	Dir_SouthWest = Direction(0b0000)
	Dir_West      = Direction(0b0100)
	Dir_NorthWest = Direction(0b1100)
)

func (d Direction) String() string {
	switch d {
	case Dir_Neutral:
		return "Neutral"
	case Dir_North:
		return "North"
	case Dir_NorthEast:
		return "NorthEast"
	case Dir_East:
		return "East"
	case Dir_SouthEast:
		return "SouthEast"
	case Dir_South:
		return "South"
	case Dir_SouthWest:
		return "SouthWest"
	case Dir_West:
		return "West"
	case Dir_NorthWest:
		return "NorthWest"
	}
	return "Unknown"
}

type Buttons struct {
	logger *slog.Logger
}

type ButtonStatus int

const (
	Btn_None   = ButtonStatus(0b0000)
	Btn_White  = ButtonStatus(0b0001)
	Btn_Red    = ButtonStatus(0b0010)
	Btn_Yellow = ButtonStatus(0b0100)
	Btn_Blue   = ButtonStatus(0b1000)
)

func (bs ButtonStatus) String() string {
	if bs == Btn_None {
		return "None"
	}

	s := ""
	if bs&Btn_White > 0 {
		s += "⚪️"
	}
	if bs&Btn_Red > 0 {
		s += "️🔴"
	}
	if bs&Btn_Yellow > 0 {
		s += "🟡"
	}
	if bs&Btn_Blue > 0 {
		s += "🔵"
	}

	if s == "" {
		return "Unknown"
	}

	return s
}

func NewController(logger *slog.Logger) (*Controller, error) {
	c := &Controller{
		data:   make([]byte, 8),
		logger: logger,
	}

	if err := c.Init(); err != nil {
		return nil, err
	}

	d, err := hid.OpenPath("/dev/hidraw3")
	if err != nil {
		return nil, err
	}

	c.device = d

	mfr, err := c.GetManufacturer()
	if err != nil {
		log.Fatal(err)
	}
	product, err := c.GetProduct()
	if err != nil {
		log.Fatal(err)
	}
	c.logger.Info("connected to joystick", "manufacturer", strings.TrimSpace(mfr), "product", strings.TrimSpace(product))

	c.Joystick = NewJoystick(logger)
	c.Buttons = NewButtons(logger)

	return c, nil
}

func (c *Controller) Init() error {
	if err := hid.Init(); err != nil {
		return err
	}
	return nil
}

func (c *Controller) GetManufacturer() (string, error) {
	s, err := c.device.GetMfrStr()
	if err != nil {
		return "", err
	}
	return s, nil
}

func (c *Controller) GetProduct() (string, error) {
	s, err := c.device.GetProductStr()
	if err != nil {
		return "", err
	}
	return s, nil
}

func (c *Controller) GetState() (*ControllerState, error) {
	l, err := c.device.Read(c.data)
	if err != nil {
		return nil, err
	}

	slog.Debug("read from device", "length", l, "bytes", c.data)

	cs := &ControllerState{
		Direction:    c.Joystick.GetDirection(c.data),
		ButtonStatus: c.Buttons.GetStatus(c.data),
	}
	return cs, nil
}

func NewJoystick(logger *slog.Logger) *Joystick {
	return &Joystick{
		logger: logger,
	}
}

func (j *Joystick) GetDirection(data []byte) Direction {
	updown, leftright := data[0]>>6, data[1]>>6

	j.logger.Debug("GetDirection", "updown", updown, "leftright", leftright)

	direction := 0
	direction += int(updown << 2)
	direction += int(leftright)

	return Direction(direction)
}

func NewButtons(logger *slog.Logger) *Buttons {
	return &Buttons{
		logger: logger,
	}
}

func (b *Buttons) GetStatus(data []byte) ButtonStatus {
	buttons := data[5] >> 4

	b.logger.Debug("GetStatus", "buttons", buttons)

	return ButtonStatus(buttons)
}
