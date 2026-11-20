package controller

import "github.com/sstallion/go-hid"

// hidDevice is a joystick opened through go-hid.
type hidDevice struct {
	device *hid.Device
}

// initHID prepares the HID library. NewController calls it once, before the
// first OpenHID.
func initHID() error {
	return hid.Init()
}

// OpenHID opens the HID device at a filesystem path such as /dev/hidraw3.
func OpenHID(path string) (Device, error) {
	d, err := hid.OpenPath(path)
	if err != nil {
		return nil, err
	}
	return hidDevice{device: d}, nil
}

// Read blocks until the device sends a report. It fails when the device has
// been unplugged.
func (h hidDevice) Read(p []byte) (int, error) { return h.device.Read(p) }

func (h hidDevice) Close() error { return h.device.Close() }

func (h hidDevice) Manufacturer() (string, error) { return h.device.GetMfrStr() }

func (h hidDevice) Product() (string, error) { return h.device.GetProductStr() }
