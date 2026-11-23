package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestGoVersionsAgree: dev/Dockerfile and go.mod name the same Go release.
// The Linux and Windows release binaries are built with the image's Go. The
// macOS one, and every test on macOS and Windows, uses the Go that go.mod
// names. A go.mod that is ahead of the image refuses to build there, but
// nothing else notices when the image is ahead of go.mod: every check passes,
// and the next release carries binaries from two Go releases.
//
// It compares what the two files say, not the Go that runs the test.
// GO_VERSION=<version> dev/run ... tries another release without changing
// either file, and has to pass here.
func TestGoVersionsAgree(t *testing.T) {
	dockerfile := filepath.Join("dev", "Dockerfile")
	image, from := "", ""
	for _, line := range readLines(t, dockerfile) {
		if v, ok := strings.CutPrefix(line, "ARG GO_VERSION="); ok {
			// Docker takes the value with or without quotes.
			image = strings.Trim(v, `"'`)
		}
		if strings.HasPrefix(line, "FROM ") {
			from = line
		}
	}
	if image == "" {
		t.Fatalf("%s has no line that starts with ARG GO_VERSION=, which is where this test reads the image's Go release from", dockerfile)
	}
	// ARG GO_VERSION only says which Go the image has while FROM uses it.
	// It is also what GO_VERSION=<version> dev/run sets.
	if want := "FROM golang:${GO_VERSION}-"; !strings.HasPrefix(from, want) {
		t.Fatalf("the FROM line of %s is %q, want it to start with %q: this test reads the image's Go release from ARG GO_VERSION, which is only right while FROM uses it",
			dockerfile, from, want)
	}

	// The go line is the oldest release that may build the module. A
	// toolchain line, when go.mod has one, is the release that setup-go
	// installs on the macOS and Windows runners in place of it. Either line
	// may end in a comment.
	goLine, toolchain := "", ""
	for _, line := range readLines(t, "go.mod") {
		words := strings.Fields(line)
		if len(words) < 2 || (len(words) > 2 && !strings.HasPrefix(words[2], "//")) {
			continue
		}
		switch words[0] {
		case "go":
			goLine = words[1]
		case "toolchain":
			toolchain = strings.TrimPrefix(words[1], "go")
		}
	}
	if goLine == "" {
		t.Fatalf("go.mod has no go line")
	}

	if image != goLine {
		t.Errorf("%s builds with Go %s (ARG GO_VERSION) and go.mod names Go %s (its go line); release binaries come from both, so they must name the same release: bump the two together",
			dockerfile, image, goLine)
	}
	if toolchain != "" && toolchain != image {
		t.Errorf("%s builds with Go %s (ARG GO_VERSION) and go.mod names Go %s (its toolchain line, which the macOS and Windows runners install); remove the toolchain line, or bump the go line and ARG GO_VERSION to that release, after which go mod tidy removes the line",
			dockerfile, image, toolchain)
	}
}

// readLines returns the lines of a file of this checkout, named from the
// directory the test runs in, which is the top of the checkout. The lines
// come without the spaces around them and without a Windows line end.
func readLines(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("cannot read %s: %v", name, err)
	}
	lines := strings.Split(string(data), "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return lines
}

// fakeDocker stands in for the docker command that dev/run calls. It writes
// down what it was asked to do, and with which image, and does none of it. A
// build that is told to pull the base image and to use nothing from the
// cache is written down as a rebuild. FAKE_BUILD says whether a build works
// (ok, fail), and FAKE_IMAGE whether the image exists (present, absent, or
// after-build: only once a build has been tried, which is what a build that
// lost the race to another dev/run sees).
const fakeDocker = `#!/bin/sh
image=none pull=0 nocache=0
for arg; do
	case $arg in
	--pull) pull=1 ;;
	--no-cache) nocache=1 ;;
	dsk-eel-dev:*) [ "$image" != none ] || image=$arg ;;
	esac
done
case "$1 ${2:-}" in
"build "*)
	if [ "$pull$nocache" = 11 ]; then
		echo "rebuild $image" >>"$FAKE_DIR/calls"
	else
		echo "build $image" >>"$FAKE_DIR/calls"
	fi
	[ "$FAKE_BUILD" = ok ] && exit 0
	echo "fake docker: the build failed" >&2
	exit 1
	;;
"image inspect")
	echo "inspect $image" >>"$FAKE_DIR/calls"
	case $FAKE_IMAGE in
	present) exit 0 ;;
	after-build) grep -q "build " "$FAKE_DIR/calls" && exit 0 ;;
	esac
	echo "fake docker: no such image" >&2
	exit 1
	;;
"run "*)
	echo "run $image" >>"$FAKE_DIR/calls"
	exit 0
	;;
esac
echo "fake docker: unexpected call: $*" >&2
exit 64
`

// TestDevRunImageBuild: dev/run only runs the command when it has the image
// it was asked for. A --rebuild builds from scratch, and when that fails
// dev/run stops there with a failing status, although the image from before
// still exists; a first build whose image is there afterwards, because
// another dev/run built it at the same time, carries on. The image that is
// looked for, built and run is the same one throughout.
func TestDevRunImageBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dev/run is a script for a Unix shell; on Windows the go commands are used directly")
	}
	// Looking at the script here also makes a change to it count as a
	// change to this test's input, so that go test does not answer from its
	// cache of results.
	script := filepath.Join("dev", "run")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("cannot find the script to test: %v", err)
	}
	for _, c := range []struct {
		name    string
		rebuild bool
		build   string // FAKE_BUILD
		image   string // FAKE_IMAGE
		calls   string // what docker is asked to do, in order
		runs    bool   // whether the command is run, and dev/run succeeds
	}{
		{"a rebuild that fails stops, though the old image is there", true, "fail", "present", "rebuild", false},
		{"a rebuild that works runs the command", true, "ok", "present", "rebuild run", true},
		{"a first build that fails stops", false, "fail", "absent", "inspect build inspect", false},
		{"a first build that works runs the command", false, "ok", "absent", "inspect build run", true},
		{"a first build that lost the race to another runs the command", false, "fail", "after-build", "inspect build inspect run", true},
		{"an image that is there is not built again", false, "fail", "present", "inspect run", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			docker := filepath.Join(dir, "docker")
			if err := os.WriteFile(docker, []byte(fakeDocker), 0o755); err != nil {
				t.Fatal(err)
			}
			args := []string{script}
			if c.rebuild {
				args = append(args, "--rebuild")
			}
			cmd := exec.Command("sh", append(args, "true")...)
			cmd.Env = append(os.Environ(),
				"DOCKER="+docker, "FAKE_DIR="+dir, "FAKE_BUILD="+c.build, "FAKE_IMAGE="+c.image)
			out, err := cmd.CombinedOutput()
			var failed *exec.ExitError
			if err != nil && !errors.As(err, &failed) {
				t.Fatalf("dev/run could not be started: %v", err)
			}

			// Each line of the log is what docker was asked to do and
			// the image it was given.
			log, _ := os.ReadFile(filepath.Join(dir, "calls"))
			var calls, images []string
			for _, line := range strings.Split(strings.TrimSpace(string(log)), "\n") {
				what, image, _ := strings.Cut(line, " ")
				calls = append(calls, what)
				images = append(images, image)
			}
			if got := strings.Join(calls, " "); got != c.calls {
				t.Errorf("docker was asked to %q, want %q\ndev/run printed:\n%s", got, c.calls, out)
			}
			for _, image := range images {
				if image == "none" || image != images[0] {
					t.Errorf("docker was given the images %q, want one image, the same in every call", images)
					break
				}
			}
			if c.runs && err != nil {
				t.Errorf("dev/run failed (%v), want it to run the command and succeed\ndev/run printed:\n%s", err, out)
			}
			if !c.runs && err == nil {
				t.Errorf("dev/run succeeded, want a failing exit status because it has no image to run the command in\ndev/run printed:\n%s", out)
			}
		})
	}
}
