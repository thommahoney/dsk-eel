//go:build !cgo

package controller

import "errors"

// HasJoystick says whether this build can open the joystick. This one
// cannot: it was built without cgo, and the HID library is C code. It
// still plays from the keyboard.
const HasJoystick = false

var errNoJoystickSupport = errors.New("this build has no joystick support (it was built without a C compiler); use --keyboard, or --nojoy --demo")

// initHID has nothing to prepare.
func initHID() error {
	return nil
}

// OpenHID always fails in this build.
func OpenHID(path string) (Device, error) {
	return nil, errNoJoystickSupport
}
