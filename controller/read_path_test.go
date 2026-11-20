package controller

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"testing"
)

// The tests and benchmarks in this file measure what reading one report
// costs. A joystick can send reports many times a second whether or not
// anything has changed, so this is the controller's hot path.

// endlessDevice is a joystick that always has a report ready: it alternates
// between two, so every report is a change of state. It stops with an error
// after `left` reports; a negative `left` means never.
type endlessDevice struct {
	reports [2][8]byte
	next    int
	left    int
}

var errNoMoreReports = errors.New("no more reports")

func newEndlessDevice() *endlessDevice {
	d := &endlessDevice{left: -1}
	copy(d.reports[0][:], report(Dir_North, Btn_Red))
	copy(d.reports[1][:], report(Dir_SouthWest, Btn_White))
	return d
}

func (d *endlessDevice) Read(p []byte) (int, error) {
	if d.left == 0 {
		return -1, errNoMoreReports
	}
	d.left--
	d.next ^= 1
	return copy(p, d.reports[d.next][:]), nil
}

func (d *endlessDevice) Close() error                  { return nil }
func (d *endlessDevice) Manufacturer() (string, error) { return "", nil }
func (d *endlessDevice) Product() (string, error)      { return "", nil }

// loggerAt logs at the given level and above, and throws the output away.
func loggerAt(level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: level}))
}

// quietLogger logs warnings and errors only, as the program does when it is
// run without -v.
func quietLogger() *slog.Logger {
	return loggerAt(slog.LevelWarn)
}

// TestReadingAReportDoesNotAllocate checks, with debug logging off, both the
// decoding of one report and the whole loop around it: the read, the
// decoding, the comparison with the previous state and the call to the
// handler. Debug logging is off both without -v (warnings only) and with
// -v (info), so both are checked.
func TestReadingAReportDoesNotAllocate(t *testing.T) {
	levels := []struct {
		name  string
		level slog.Level
	}{
		{"without -v", slog.LevelWarn},
		{"with -v", slog.LevelInfo},
	}
	for _, tc := range levels {
		t.Run(tc.name, func(t *testing.T) {
			handled := 0
			c := newController(loggerAt(tc.level), nil, testRetry, func(ControllerState) { handled++ }, nil)
			device := newEndlessDevice()

			decode := testing.AllocsPerRun(1000, func() {
				if _, ok, err := c.readState(device); err != nil || !ok {
					t.Fatalf("readState: ok=%v err=%v", ok, err)
				}
			})
			if decode != 0 {
				t.Errorf("decoding a report allocates %v times, want 0", decode)
			}

			const reports = 1000
			handled = 0
			loop := testing.AllocsPerRun(10, func() {
				device.left = reports
				if _, err := c.readStates(device); err != errNoMoreReports {
					t.Fatalf("readStates: %v", err)
				}
			})
			if loop != 0 {
				t.Errorf("reading %d reports allocates %v times, want 0", reports, loop)
			}
			// AllocsPerRun calls the function once more than it is asked to.
			if want := 11 * reports; handled != want {
				t.Errorf("the handler was called %d times, want %d", handled, want)
			}
		})
	}
}

// TestDebugLogOfAReport checks that -vv still logs what it did for every
// report: the raw bytes, the two stick axes and the button bits.
func TestDebugLogOfAReport(t *testing.T) {
	log := &logRecorder{min: slog.LevelDebug}
	c := newController(slog.New(log), nil, testRetry, nil, nil)
	device := newEndlessDevice()
	device.reports[0] = [8]byte(report(Dir_North, Btn_Red|Btn_White))
	device.reports[1] = device.reports[0]

	if _, ok, err := c.readState(device); err != nil || !ok {
		t.Fatalf("readState: ok=%v err=%v", ok, err)
	}

	type line struct {
		msg   string
		attrs map[string]any
	}
	var got []line
	for _, r := range log.records {
		l := line{msg: r.Message, attrs: map[string]any{}}
		r.Attrs(func(a slog.Attr) bool {
			l.attrs[a.Key] = a.Value.Any()
			return true
		})
		got = append(got, l)
	}
	if len(got) != 3 {
		t.Fatalf("one report logged %d debug lines, want 3: %v", len(got), got)
	}

	if l := got[0]; l.msg != "read from device" || l.attrs["length"] != int64(8) ||
		!bytes.Equal(l.attrs["bytes"].([]byte), device.reports[0][:]) {
		t.Errorf("first line is %q %v", l.msg, l.attrs)
	}
	// North is the up-down axis at 1 and the left-right axis at 3.
	if l := got[1]; l.msg != "GetDirection" || l.attrs["updown"] != uint64(1) || l.attrs["leftright"] != uint64(3) {
		t.Errorf("second line is %q %v", l.msg, l.attrs)
	}
	if l := got[2]; l.msg != "GetStatus" || l.attrs["buttons"] != uint64(Btn_Red|Btn_White) {
		t.Errorf("third line is %q %v", l.msg, l.attrs)
	}
}

// BenchmarkReadState is the cost of reading and decoding one report with
// debug logging off.
func BenchmarkReadState(b *testing.B) {
	c := newController(quietLogger(), nil, testRetry, nil, nil)
	device := newEndlessDevice()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := c.readState(device); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadStates is the cost per report of the whole reader loop, with
// every report a change of state that is passed to the handler.
func BenchmarkReadStates(b *testing.B) {
	handled := 0
	c := newController(quietLogger(), nil, testRetry, func(ControllerState) { handled++ }, nil)
	device := newEndlessDevice()
	device.left = b.N
	b.ReportAllocs()
	b.ResetTimer()
	if _, err := c.readStates(device); err != errNoMoreReports {
		b.Fatal(err)
	}
}
