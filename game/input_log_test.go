package game

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/thommahoney/dsk-eel/controller"
)

// TestInputLogLine pins the line the game logs for every joystick state it
// is given, as the text handler writes it: with -v it names the stick and
// the buttons, and without -v, where only warnings show, there is no line.
func TestInputLogLine(t *testing.T) {
	held := controller.Btn_Green | controller.Btn_White
	for _, tc := range []struct {
		name  string
		level slog.Level
		state controller.ControllerState
		want  string
	}{
		{"with -v, the stick alone", slog.LevelInfo, input(controller.Dir_North, controller.Btn_None),
			"level=INFO msg=HandleControllerState joystick=North buttons=None"},
		{"with -v, the stick and two buttons", slog.LevelInfo, input(controller.Dir_SouthWest, held),
			"level=INFO msg=HandleControllerState joystick=SouthWest buttons=" + held.String()},
		{"without -v", slog.LevelWarn, input(controller.Dir_North, held), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newLoopbackGame(t)
			var log bytes.Buffer
			g.Config.Logger = slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: tc.level}))

			g.HandleControllerState(tc.state)

			if tc.want == "" {
				if log.Len() != 0 {
					t.Errorf("the log has something, want nothing:\n%s", log.String())
				}
				return
			}
			lines := strings.Split(strings.TrimSuffix(log.String(), "\n"), "\n")
			if len(lines) != 1 || !strings.HasSuffix(lines[0], " "+tc.want) {
				t.Errorf("the log is\n%swant one line, which after its time is\n%s", log.String(), tc.want)
			}
		})
	}
}

// TestInputWithoutVDoesNotAllocate takes joystick states with the log at
// warnings only, as the program runs without -v. The line that names the
// stick and the buttons is not written then, and nothing may be built for
// it either: the names of the stick position and of the buttons held are
// only put together when the line is wanted.
func TestInputWithoutVDoesNotAllocate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		buttons controller.ButtonStatus
	}{
		{"the stick moves", controller.Btn_None},
		{"the stick moves with two buttons held", controller.Btn_Red | controller.Btn_Blue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newLoopbackGame(t)
			g.Config.Logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))

			// The buttons go down here, which is the press. After it only
			// the stick changes, and that sends nothing.
			g.HandleControllerState(input(controller.Dir_Neutral, tc.buttons))

			states := [2]controller.ControllerState{
				input(controller.Dir_North, tc.buttons),
				input(controller.Dir_Neutral, tc.buttons),
			}
			next := 0
			allocs := testing.AllocsPerRun(1000, func() {
				g.HandleControllerState(states[next])
				next ^= 1
			})
			if allocs != 0 {
				t.Errorf("taking a joystick state allocates %v times with the log at warnings only, want 0", allocs)
			}
		})
	}
}
