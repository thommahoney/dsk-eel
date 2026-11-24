package controller

import (
	"log/slog"
	"testing"
)

// The controller does not log every loss of the joystick and every failed
// attempt to open it: a device that can be opened but not read, or a
// joystick that stays away, would fill the log. It counts, and logs the
// first of a run and then one in openLogEvery. The tests in this file are
// about when the counting starts again, which is what makes the next real
// loss or absence show in the log, and about Stop, which is not a loss.
//
// Those about the counting hand the controller its devices one after the
// other and wait for a state from the next device before they look at the
// log. A state from a device means that the controller has finished with
// the one before it: logged its loss, called lost and opened this one.

// pinStart starts a controller that is handed the devices in turn, one for
// every time it opens the joystick: see inTurn. When the test ends, the
// devices that are still plugged in are unplugged and the controller is
// stopped.
func pinStart(t *testing.T, devices ...*fakeDevice) *rig {
	t.Helper()
	r := &rig{t: t, log: &logRecorder{}, states: make(chan ControllerState, 64)}
	c, err := Start(slog.New(r.log), inTurn(devices...), testRetry,
		func(s ControllerState) { r.states <- s },
		func() { r.lost.Add(1) })
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.c = c
	t.Cleanup(r.stop)
	t.Cleanup(func() {
		for _, d := range devices {
			if d != nil {
				pinUnplug(d)
			}
		}
	})
	return r
}

// pinUnplug pulls the plug of a device, unless it has been pulled already.
// Only the test's goroutine pulls plugs.
func pinUnplug(d *fakeDevice) {
	select {
	case <-d.unplug:
	default:
		close(d.unplug)
	}
}

// pinDead returns a device that can be opened but fails on its first read.
func pinDead() *fakeDevice {
	d := newFakeDevice()
	close(d.unplug)
	return d
}

// pinUse has the controller read one report from the device, a push of the
// stick to the north, and waits for the handler to be given it. To the
// controller that is always a change: a joystick that has just been opened
// counts as untouched.
func (r *rig) pinUse(d *fakeDevice) {
	r.t.Helper()
	d.send(r.t, report(Dir_North, Btn_None))
	r.wantState(ControllerState{Dir_North, Btn_None})
}

// pinFirstFailedOpens returns how many warnings say that the joystick could
// not be opened and that this was the first attempt since it was last open.
func (l *logRecorder) pinFirstFailedOpens() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, r := range l.records {
		if r.Level != slog.LevelWarn || r.Message != "cannot open the joystick, will keep trying" {
			continue
		}
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "attempts" && a.Value.Kind() == slog.KindInt64 && a.Value.Int64() == 1 {
				n++
			}
			return true
		})
	}
	return n
}

// TestQuickLossesOfAWorkingJoystickAreAllLogged loses three joysticks in a
// row within a few milliseconds of opening them. Each had sent a report, so
// none of them is a device that cannot be read, however soon it went: every
// loss and every return is a warning. Only a connection that fails before
// its first report is counted towards holding the warnings back.
func TestQuickLossesOfAWorkingJoystickAreAllLogged(t *testing.T) {
	devices := []*fakeDevice{newFakeDevice(), newFakeDevice(), newFakeDevice(), newFakeDevice()}
	r := pinStart(t, devices...)

	const losses = 3
	for i := range losses {
		r.pinUse(devices[i])
		close(devices[i].unplug)
	}
	r.pinUse(devices[losses])

	if n := r.log.count(slog.LevelWarn, "read failed"); n != losses {
		t.Errorf("%d losses of a joystick that had sent a report logged %d warnings, want %d: one each", losses, n, losses)
	}
	if n := r.log.count(slog.LevelWarn, "reconnected to joystick"); n != losses {
		t.Errorf("%d returns after those losses logged %d warnings, want %d: one each", losses, n, losses)
	}
	if n := r.lost.Load(); n != losses {
		t.Errorf("lost was called %d times, want %d", n, losses)
	}
}

// TestLossAfterDeadConnectionsIsLogged has a run of connections that fail
// on their first read, whose losses are held back after the first, and
// then a joystick that works. That ends the run: when the working joystick
// is lost in its turn, the loss is a warning, and so is the return of what
// comes after it. If the count of dead connections were not started again,
// it would stay where the run left it, and no loss of a working joystick
// would be logged from then on.
func TestLossAfterDeadConnectionsIsLogged(t *testing.T) {
	first, working, last := newFakeDevice(), newFakeDevice(), newFakeDevice()
	r := pinStart(t, first, pinDead(), pinDead(), pinDead(), working, last)

	r.pinUse(first)
	close(first.unplug)
	// The three dead connections go by, one retry interval each.
	r.pinUse(working)
	lossesBefore := r.log.count(slog.LevelWarn, "read failed")
	returnsBefore := r.log.count(slog.LevelWarn, "reconnected to joystick")
	if n := r.lost.Load(); n != 4 {
		t.Fatalf("lost was called %d times before the working joystick was reached, want 4: the first joystick and three dead connections", n)
	}

	close(working.unplug)
	r.pinUse(last)

	if n := r.log.count(slog.LevelWarn, "read failed"); n != lossesBefore+1 {
		t.Errorf("the loss of a working joystick after three dead connections logged %d warnings, want 1", n-lossesBefore)
	}
	if n := r.log.count(slog.LevelWarn, "reconnected to joystick"); n != returnsBefore+1 {
		t.Errorf("the return after that loss logged %d warnings, want 1", n-returnsBefore)
	}
}

// TestEveryAbsenceLogsItsFirstFailedOpen unplugs the joystick twice, and
// both times three attempts to open it fail before it is back. The first
// failed attempt of each absence is a warning, and the two after it are
// not. If the count of failed attempts were not started again when the
// joystick is opened, the second absence would begin in the middle of the
// count and say nothing.
func TestEveryAbsenceLogsItsFirstFailedOpen(t *testing.T) {
	first, second, third := newFakeDevice(), newFakeDevice(), newFakeDevice()
	r := pinStart(t, first, nil, nil, nil, second, nil, nil, nil, third)

	r.pinUse(first)
	close(first.unplug)
	r.pinUse(second)
	if n := r.log.pinFirstFailedOpens(); n != 1 {
		t.Errorf("after one absence %d warnings say that a first attempt to open failed, want 1", n)
	}

	close(second.unplug)
	r.pinUse(third)
	if n := r.log.pinFirstFailedOpens(); n != 2 {
		t.Errorf("after two absences %d warnings say that a first attempt to open failed, want 2: one for each absence", n)
	}
	if n := r.log.count(slog.LevelWarn, "cannot open the joystick"); n != 2 {
		t.Errorf("six failed attempts to open in two absences logged %d warnings, want 2: the first of each", n)
	}
}

// TestStopIsNotALoss stops the controller while it waits for a report. The
// read it is in cannot be interrupted; whatever that read comes back with
// afterwards, an error or a report, the goroutine ends and that is all: no
// loss is logged, lost is not called and the handler is not given the
// report. The game never stops its controller, but if it did it would not
// want to be told that the joystick had gone, and fall into demo mode.
func TestStopIsNotALoss(t *testing.T) {
	tests := []struct {
		name string
		// finish makes the read that was in progress return.
		finish func(t *testing.T, d *fakeDevice)
	}{
		{"the read fails", func(t *testing.T, d *fakeDevice) { close(d.unplug) }},
		{"the read returns a report", func(t *testing.T, d *fakeDevice) {
			d.send(t, report(Dir_South, Btn_Red))
			// For a controller that reads on regardless: its next read
			// fails, so the test does not have to wait for it.
			close(d.unplug)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			device := newFakeDevice()
			r := pinStart(t, device)
			r.pinUse(device)

			r.c.Stop()
			tc.finish(t, device)
			r.wantNoMoreStates()

			if n := r.lost.Load(); n != 0 {
				t.Errorf("lost was called %d times for a controller that was stopped, want never", n)
			}
			if n := r.log.count(slog.LevelWarn, "read failed"); n != 0 {
				t.Errorf("stopping the controller logged %d losses of the joystick, want none", n)
			}
			if !device.closed.Load() {
				t.Error("the device was not closed")
			}
		})
	}
}
