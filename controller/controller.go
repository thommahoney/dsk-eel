package controller

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/thommahoney/dsk-eel/config"
)

const (
	// DefaultRetryInterval is how long the controller waits between
	// attempts to open a joystick that is not there.
	DefaultRetryInterval = time.Second

	// reportBufLen is the size of the buffer reports are read into.
	// minReportLen is the shortest report that can be decoded: the decoders
	// look at bytes 0, 1, 5 and 6.
	reportBufLen = 8
	minReportLen = 7

	// While the joystick stays away, the first failed attempt to open it is
	// logged and then one in every openLogEvery: about one line a minute at
	// the default interval. The same goes for a device that can be opened
	// but fails on its first read every time.
	openLogEvery = 60
)

// Device is what the controller needs from the joystick's HID device. The
// real one is opened by OpenHID; tests substitute a fake.
type Device interface {
	// Read waits for the next input report, copies it into p and returns
	// its length.
	Read(p []byte) (int, error)
	Close() error
	Manufacturer() (string, error)
	Product() (string, error)
}

// OpenFunc opens the joystick.
type OpenFunc func() (Device, error)

// Controller reads the joystick on a goroutine of its own and reports what
// it sees through two callbacks, which are never called at the same time:
//
//   - Handler gets the new state whenever the direction or the buttons
//     change. It is called on the reader goroutine.
//   - Lost is called when the joystick cannot be opened or stops answering.
//     From then on the stick counts as neutral with no buttons down, and the
//     controller tries to open the joystick again once every retry interval.
//     Nothing is called when that succeeds: states simply start arriving
//     again. It is called on the reader goroutine, except when the joystick
//     cannot be opened at startup: then Start calls it, on its caller's
//     goroutine, before the reader goroutine exists.
type Controller struct {
	data          []byte
	logger        *slog.Logger
	open          OpenFunc
	retryInterval time.Duration

	// Used only by the reader goroutine, and by Start before the goroutine
	// exists. openFailures counts the failed attempts to open the joystick
	// since it was last open. deadConnections counts the connections in a
	// row that failed before a single report was read. wasConnected is
	// whether the joystick has ever been open. shortReportLogged is whether
	// the warning about a report too short to decode has been logged for
	// the connection that is open now.
	openFailures      int
	deadConnections   int
	wasConnected      bool
	shortReportLogged bool

	stop     chan struct{}
	stopOnce sync.Once
	finished chan struct{}

	Buttons  *Buttons
	Handler  func(ControllerState)
	Joystick *Joystick
	Lost     func()
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
	Dir_North     = Direction(0b0111)
	Dir_NorthEast = Direction(0b0011)
	Dir_East      = Direction(0b0001)
	Dir_SouthEast = Direction(0b0000)
	Dir_South     = Direction(0b0100)
	Dir_SouthWest = Direction(0b1100)
	Dir_West      = Direction(0b1101)
	Dir_NorthWest = Direction(0b1111)
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
	Btn_None   = ButtonStatus(0b00000)
	Btn_Red    = ButtonStatus(0b00001)
	Btn_Green  = ButtonStatus(0b00010)
	Btn_Blue   = ButtonStatus(0b00100)
	Btn_Yellow = ButtonStatus(0b01000)
	Btn_White  = ButtonStatus(0b10000)
)

func (bs ButtonStatus) String() string {
	if bs == Btn_None {
		return "None"
	}

	s := ""
	if bs&Btn_Red > 0 {
		s += "️🔴"
	}
	if bs&Btn_Green > 0 {
		s += "🟢"
	}
	if bs&Btn_Blue > 0 {
		s += "🔵"
	}
	if bs&Btn_Yellow > 0 {
		s += "🟡"
	}
	if bs&Btn_White > 0 {
		s += "⚪️"
	}

	if s == "" {
		return "Unknown"
	}

	return s
}

// NewController starts reading the joystick at config.ControllerPath. It
// does not fail if the joystick is not there: see Start.
func NewController(config *config.Config, handleFunc func(ControllerState), lostFunc func()) (*Controller, error) {
	if err := initHID(); err != nil {
		return nil, err
	}

	path := config.ControllerPath
	open := func() (Device, error) { return OpenHID(path) }
	return Start(config.Logger, open, DefaultRetryInterval, handleFunc, lostFunc), nil
}

// Start opens the joystick with open and starts the reader goroutine. If the
// joystick cannot be opened, Start logs a warning, calls lostFunc before it
// returns and leaves the goroutine retrying, exactly as if the joystick had
// been there and then been unplugged.
func Start(logger *slog.Logger, open OpenFunc, retryInterval time.Duration, handleFunc func(ControllerState), lostFunc func()) *Controller {
	c := newController(logger, open, retryInterval, handleFunc, lostFunc)

	device := c.connect()
	if device == nil {
		c.notifyLost()
	}
	go c.run(device)

	return c
}

// newController returns a controller that has not opened the joystick or
// started its goroutine yet.
func newController(logger *slog.Logger, open OpenFunc, retryInterval time.Duration, handleFunc func(ControllerState), lostFunc func()) *Controller {
	return &Controller{
		Buttons:       NewButtons(logger),
		Handler:       handleFunc,
		Joystick:      NewJoystick(logger),
		Lost:          lostFunc,
		data:          make([]byte, reportBufLen),
		finished:      make(chan struct{}),
		logger:        logger,
		open:          open,
		retryInterval: retryInterval,
		stop:          make(chan struct{}),
	}
}

// Stop ends the reader goroutine. It cannot interrupt a read from the
// device: the goroutine ends when the read in progress returns, or at once
// if it is waiting to retry. The game reads the joystick for as long as the
// program runs and never calls this; tests do.
func (c *Controller) Stop() {
	c.stopOnce.Do(func() { close(c.stop) })
}

func (c *Controller) stopped() bool {
	select {
	case <-c.stop:
		return true
	default:
		return false
	}
}

func (c *Controller) notifyLost() {
	if c.Lost != nil {
		c.Lost()
	}
}

// run is the reader goroutine. device is the open joystick, or nil if it
// could not be opened.
func (c *Controller) run(device Device) {
	defer close(c.finished)

	for {
		if device == nil {
			device = c.reconnect()
			if device == nil {
				return
			}
		}

		gotReport, err := c.readStates(device)
		if err != nil {
			if gotReport {
				c.deadConnections = 0
			} else {
				c.deadConnections++
			}
			if c.deadConnections <= 1 || c.deadConnections%openLogEvery == 0 {
				c.logger.Warn("lost the joystick: read failed", "error", err)
			}
		}
		if closeErr := device.Close(); closeErr != nil {
			c.logger.Debug("closing the joystick failed", "error", closeErr)
		}
		device = nil
		if err == nil {
			return
		}
		c.notifyLost()
	}
}

// connect tries once to open the joystick and returns it, or nil. It logs
// the joystick's identity on success, and on failure a warning for the first
// failed attempt in a row and for an occasional one after that.
func (c *Controller) connect() Device {
	device, err := c.open()
	if err != nil {
		c.openFailures++
		if c.openFailures == 1 || c.openFailures%openLogEvery == 0 {
			c.logger.Warn("cannot open the joystick, will keep trying",
				"error", err, "attempts", c.openFailures, "retry", c.retryInterval)
		}
		return nil
	}
	c.openFailures = 0

	// The identity strings are only for the log.
	mfr, err := device.Manufacturer()
	if err != nil {
		c.logger.Warn("cannot read the joystick's manufacturer string", "error", err)
	}
	product, err := device.Product()
	if err != nil {
		c.logger.Warn("cannot read the joystick's product string", "error", err)
	}

	msg := "connected to joystick"
	if c.wasConnected {
		msg = "reconnected to joystick"
	}
	c.wasConnected = true
	c.logger.Info(msg, "manufacturer", strings.TrimSpace(mfr), "product", strings.TrimSpace(product))

	return device
}

// reconnect tries to open the joystick once every retry interval until it
// succeeds. It returns nil if the controller is stopped first.
func (c *Controller) reconnect() Device {
	timer := time.NewTimer(c.retryInterval)
	defer timer.Stop()

	for {
		select {
		case <-c.stop:
			return nil
		case <-timer.C:
		}

		if device := c.connect(); device != nil {
			return device
		}
		timer.Reset(c.retryInterval)
	}
}

// readStates reads reports from the device and passes every change of state
// to the handler. It returns the error of the read that failed, or nil if
// the controller was stopped, and whether any report was read before that.
func (c *Controller) readStates(device Device) (gotReport bool, err error) {
	// A joystick that has just been opened counts as untouched, which is
	// also what Lost told the handler's owner about the one before it.
	prevDirection := Dir_Neutral
	prevButtons := Btn_None
	c.shortReportLogged = false
	for {
		state, ok, err := c.readState(device)
		if c.stopped() {
			return gotReport, nil
		}
		if err != nil {
			return gotReport, err
		}
		gotReport = true
		if !ok {
			continue
		}

		changed := false
		if state.Direction != prevDirection {
			prevDirection = state.Direction
			changed = true
		}

		if state.ButtonStatus != prevButtons {
			prevButtons = state.ButtonStatus
			changed = true
		}

		if changed && c.Handler != nil {
			c.Handler(state)
		}
	}
}

// readState reads one report and decodes it. ok is false, with no error,
// for a report too short to decode: the rest of the buffer would still hold
// bytes of an earlier report. The first short report of a connection is
// logged, because a device that only ever sends short ones (the wrong
// device at the path, say) would otherwise be ignored without a word.
//
// A joystick can send a hundred or more reports a second, whether or not
// anything has changed. So that reading one does not allocate, the state is
// returned by value, and here and in the decoders the arguments of a debug
// log line are only built when debug logging is on.
func (c *Controller) readState(device Device) (state ControllerState, ok bool, err error) {
	l, err := device.Read(c.data)
	if err != nil {
		return ControllerState{}, false, err
	}

	if c.logger.Enabled(context.Background(), slog.LevelDebug) {
		c.logger.Debug("read from device", "length", l, "bytes", c.data)
	}

	if l < minReportLen {
		if !c.shortReportLogged {
			c.shortReportLogged = true
			c.logger.Warn("ignoring joystick reports that are too short to decode",
				"length", l, "want", minReportLen)
		}
		return ControllerState{}, false, nil
	}

	state = ControllerState{
		Direction:    c.Joystick.GetDirection(c.data),
		ButtonStatus: c.Buttons.GetStatus(c.data),
	}
	return state, true, nil
}

func NewJoystick(logger *slog.Logger) *Joystick {
	return &Joystick{
		logger: logger,
	}
}

func (j *Joystick) GetDirection(data []byte) Direction {
	updown, leftright := data[0]>>6, data[1]>>6

	if j.logger.Enabled(context.Background(), slog.LevelDebug) {
		j.logger.Debug("GetDirection", "updown", updown, "leftright", leftright)
	}

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
	buttons := data[5]>>4 + data[6]<<4

	if b.logger.Enabled(context.Background(), slog.LevelDebug) {
		b.logger.Debug("GetStatus", "buttons", buttons)
	}

	return ButtonStatus(buttons)
}
