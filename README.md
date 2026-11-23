# DSK Eel

This is the classic game of snake but displayed on the inside of a geodesic dome struture.

## Features

- Support for a custom joystick device
- Keyboard control for development without the joystick (`--keyboard`), on Linux, macOS and Windows
- Game Mode displays the Eel game
- Proxy Mode allows for passthrough of ArtNet data from other sources

## Installing a release

The machine that runs the game needs neither git nor Go. A merge to `main` publishes a binary on the [releases page](https://github.com/thommahoney/dsk-eel/releases), built with the pinned Go release and the pinned module versions. To install the newest one, or to update to it:

```bash
cd "$(mktemp -d)" &&
curl -fLO https://github.com/thommahoney/dsk-eel/releases/latest/download/dsk-eel-linux-arm64 &&
curl -fLO https://github.com/thommahoney/dsk-eel/releases/latest/download/dsk-eel-linux-arm64.sha256 &&
sha256sum -c dsk-eel-linux-arm64.sha256 &&
sudo install -m 755 dsk-eel-linux-arm64 /usr/local/bin/dsk-eel &&
dsk-eel --version
```

`--version` says which build is installed. A game that is running keeps running the old build until it is started again.

A release also carries `dsk-eel-windows-amd64.exe`, which plays from the keyboard only and needs nothing installed, and its `.sha256`. It is not signed, so Windows may ask before running it.

A release also carries `dsk-eel-darwin-arm64`, for a Mac with Apple silicon, and its `.sha256` (check it with `shasum -a 256 -c`). It is not signed. Downloaded with `curl` as above it runs; downloaded with a browser, macOS refuses to open it until `xattr -d com.apple.quarantine dsk-eel-darwin-arm64` has been run on it.

What the machine needs:

- **A 64-bit system.** `dpkg --print-architecture` must say `arm64`. A 32-bit Raspberry Pi OS does not work, even on a board with a 64-bit processor; there the shell says the file is "not found".
- **glibc 2.34 or newer**, which Raspberry Pi OS 12 (bookworm) and later have. Raspberry Pi OS 11 is too old and says so (`GLIBC_2.34 not found`).
- **Permission to read the joystick.** `/dev/hidraw*` usually belongs to root, so run the game with `sudo`, or add a udev rule that gives your user the device: the rule under [The joystick](#the-joystick) does that too.

### The joystick

The game opens the joystick by its path, `/dev/hidraw3` unless `--joystick` names another, and takes whatever device is there for the joystick: it does not check. The number in `hidrawN` is given out by the system in the order it finds the devices, so the joystick can be at another number after a restart, or after something else has been plugged in. The game then reads a device that is not the joystick: the stick does nothing, and what that device sends can be taken for moves of the stick.

So that this can be seen, the log says which device was opened, when the game starts and when a lost joystick comes back:

```
time=2026-11-22T20:15:04.512Z level=WARN msg="connected to joystick" path=/dev/hidraw3 manufacturer="<who made it>" product="<what it is>"
```

Nothing is wrong when this line appears; it is a warning only because warnings are what the log shows without `-v`. If the manufacturer and the product are not the joystick's, the path is the wrong one.

A device that can be opened but fails as soon as it is read comes back every second. The log does not name it every time, only about once a minute, as with its losses. A device with another manufacturer or product than the one the log named last is always named, so the last of these lines names the device that is open, or the last one that was.

**Finding the path.** With the joystick plugged in, this lists every HID device with its name:

```bash
grep -H -e HID_ID -e HID_NAME /sys/class/hidraw/*/device/uevent
```

```
/sys/class/hidraw/hidraw3/device/uevent:HID_ID=0003:0000VVVV:0000PPPP
/sys/class/hidraw/hidraw3/device/uevent:HID_NAME=<who made it> <what it is>
```

The joystick is the one that is missing from the list when it is unplugged. One listed under `hidraw3` is at `/dev/hidraw3`, which is what `--joystick` takes.

**Keeping the path the same.** In the `HID_ID` line, `VVVV` and `PPPP` are the joystick's vendor and product ID, which do not change. A udev rule can use them to give the joystick a path of its own, and to let your user read it. Put this line in `/etc/udev/rules.d/99-dsk-eel-joystick.rules`, with the two IDs in place of `VVVV` and `PPPP` and their letters in lower case (`0A1B` becomes `0a1b`):

```
SUBSYSTEM=="hidraw", ATTRS{idVendor}=="VVVV", ATTRS{idProduct}=="PPPP", SYMLINK+="dsk-eel-joystick", GROUP="input", MODE="0660"
```

`GROUP` and `MODE` give the device to the members of a group, so that the game runs without `sudo`: `id` shows the groups your user is in, and on Raspberry Pi OS `input` is usually one of them. Then load the rule and start the game with the new path:

```bash
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw    # or unplug the joystick and plug it in again
ls -l /dev/dsk-eel-joystick                      # shows which hidraw device it stands for
dsk-eel --joystick /dev/dsk-eel-joystick
```

The rule is for a USB joystick (its `HID_ID` starts with `0003`). It has not been tried with the joystick yet. If the list shows two devices with the joystick's `HID_ID`, the rule matches both and the new path stands for only one of them, so check with the log line that it is the one that works.

### The network

The game sends each frame to Chromatik as seven Art-Net packets. If the system does not take one of them, the log says so once, with the system's error, and once more when a whole frame gets out again:

```
time=2026-11-22T20:15:04.512Z level=WARN msg="cannot send Art-Net frames, will keep trying" error="write udp 10.0.0.7:41234->10.0.0.5:6454: sendto: network is unreachable"
time=2026-11-22T20:15:19.352Z level=WARN msg="sending Art-Net frames again" failed_frames=212
```

In between the game plays on, tries every frame and logs nothing more about it, so the first line without the second means that frames are still not getting out. The second line is a warning for the same reason as the joystick's.

The game sends from the address this machine had on the network when the game started. If the machine loses that address, frames get out again when it has the same address back. If it comes back with a different address they do not, and the game has to be started again.

These lines are only about sending. With Chromatik not running, or out of reach on a network that is up, every packet is sent without an error and the log shows nothing.

## Development

### Dev container

The only thing needed on the host is a container runtime with a `docker` CLI. The Go toolchain, libudev and the lint tools live in the image defined by [dev/Dockerfile](./dev/Dockerfile).

```bash
dev/run check            # build, go.mod tidiness, vet, staticcheck, gofmt, tests with and without -race
dev/run smoke            # race-instrumented demo run of the game loop
dev/run go test ./...    # any command, run in the container
dev/run                  # interactive shell
```

`dev/run` builds the image on first use and again whenever the Dockerfile changes. The checkout is mounted at `/src`, and the Go caches are kept in the `dsk-eel-dev-cache` volume. The image's Go version and the `go` line in `go.mod` name the same release; bump them together. Set `GO_VERSION` to try a newer one first, for example `GO_VERSION=1.28 dev/run check`.

Every pull request and every push to `main` runs `dev/run check` and `dev/run smoke` in GitHub Actions ([.github/workflows/checks.yml](./.github/workflows/checks.yml)), in the same image, on an arm64 runner. The code is also built and tested on macOS and on Windows, in the `check (macos)` and `check (windows)` jobs. `main` is protected: a pull request can only be merged once `check`, `check (macos)`, `check (windows)` and `release build` have passed.

Merging a pull request also publishes a build: once the checks have passed on `main`, the same workflow builds the binary with `dev/run release` and puts it on the [releases page](https://github.com/thommahoney/dsk-eel/releases), named after the day and the commit, like `2026-11-22-a77b7e7`. Merges made within a few minutes of each other can end up in one build. A pull request builds the same binary without publishing it. See [Installing a release](#installing-a-release).

Dependabot ([.github/dependabot.yml](./.github/dependabot.yml)) opens a pull request each month for any Go module or GitHub Action with a new release. To update by hand: `dev/run go get -u ./...`, then `dev/run go mod tidy`. Go itself and the lint tools are pinned in [dev/Dockerfile](./dev/Dockerfile).

Things to know:

- The checkout, and any git worktree of it, must be somewhere the container runtime shares with its VM. With Colima's defaults that is only `$HOME`, so not `/tmp`.
- `dev/run check` runs the tests twice, with the race detector and without. The detector makes them many times slower, so the longest tests do less work under it; the run without it is the one where they run at full length.
- `dev/run smoke` runs the game with `--nojoy --demo` for five seconds and fails on a data race or a crash. It runs without a network, with `--ableton` pointed at loopback so the Ableton code is exercised. It does not exercise the joystick goroutine.
- Builds are for the VM's architecture (linux/arm64 on an Apple silicon Mac). The image cannot cross-compile, because go-hid needs cgo.
- Every container shares the VM's CPUs, so a build running in another checkout shifts benchmark results by 10–20%. Take before and after numbers with nothing else running, use `-count 8` or more, and compare with `benchstat old.txt new.txt`.
- `go tool pprof -top` and `-list` work; graph output (`-svg`, `-png`) needs graphviz, which is not in the image.
- Git stays on the host; the container does not see a usable repository.
- Each Dockerfile revision gets its own image tag. `docker images dsk-eel-dev` lists them, and old ones can be removed with `docker rmi`.

### Playing without the joystick

`--keyboard` reads the keyboard of the terminal the game runs in, in place of the joystick:

```bash
dev/run go run . --keyboard --chromatik <address of the machine running Chromatik>
```

| Key | Does |
|---|---|
| arrow keys | push the stick that way; it stays there until another arrow or space |
| space | centre the stick |
| `r` `g` `b` `y` `w` | the red, green, blue, yellow and white buttons |
| `q` or ctrl-C | quit |

A terminal reports that a key was pressed but not that it was let go, so this is not quite the joystick: a direction stays held, there are no diagonals, a button is a press and release in one, two buttons cannot be held together, and holding a button key down repeats the press.

- With the stick centred the eel dies at its first junction, as with the joystick, so press an arrow soon after starting. With an arrow held it never dies there, so space is also how to end a game, and with `--demo` how to hand the eel back to the demo.
- It needs a terminal on standard input, so it cannot be combined with `dev/run smoke` or run from a service. `dev/run` only passes a terminal on when its output is one too: do not pipe or redirect its standard output.
- Ctrl-Z is switched off while the game runs. If the game crashes, the terminal can be left not showing what is typed; `stty sane` puts it right.

### Developing on a Mac

The game builds and runs on macOS without the container. It needs the Go release named on the `go` line of [go.mod](./go.mod); the installer at <https://go.dev/dl/> is the simplest way to get it.

```bash
go test ./...
go run . --keyboard                      # Chromatik on this Mac
go run . --keyboard --chromatik 10.0.0.5  # Chromatik somewhere else
```

- **The joystick** needs a C compiler, because the library that reads it is C code. With Apple's command line tools installed (`xcode-select --install`) it is built in. Without them, `CGO_ENABLED=0 go build` gives a build that plays from the keyboard only; `--version` says `keyboard only` for such a build. On macOS the joystick is not at `/dev/hidraw3`, and nobody has tried one there yet.
- **Chromatik on the same Mac** has the Art-Net port to listen on, so the game leaves it alone and sends from another one, whichever of the two is started first.
- **Tests and the release for the dome** are still Linux: GitHub Actions runs `dev/run check` in the dev image for every pull request, and also builds and tests the code on macOS. `dev/run` itself still works on a Mac, with Docker.

### Developing on Windows

The game builds and runs on Windows too, from a console window (Windows Terminal, PowerShell or cmd). It needs the Go release named on the `go` line of [go.mod](./go.mod), from <https://go.dev/dl/>.

```
go test ./...
go run . --keyboard --chromatik <address of the machine running Chromatik>
```

- With Go alone the build plays from the keyboard only. The joystick needs a C compiler as well (mingw-w64, for example from MSYS2), because the library that reads it is C code.
- The shell scripts in `dev/` are for the Linux container; on Windows use the `go` commands directly.
- Git checks the files out with Unix line ends on Windows as well ([.gitattributes](./.gitattributes)).

### Native build

This game was developed using a basic Ubuntu virtual machine with go installed.

It needs the Go release named on the `go` line of [go.mod](./go.mod), or a newer one. `go version` shows what is installed. A distribution's own package is often older than that; the installers at <https://go.dev/dl/> are always current.

```bash
sudo apt-get install libudev-dev
# Go from the distribution, if it is new enough:
# sudo add-apt-repository ppa:longsleep/golang-backports
# sudo apt update
sudo apt install golang-go
go build
```

## Design Notes

Drawn below is a top-down diagram of the dome structure with segments labeled by lengths A, B, and C.

![Dome Diagram](./docs/dome-diagram.png)


The following diagram is the above diagram with only the "B" length segments drawn. The colors are changed to help conceptualize how the Eel game is modeled.

Note: This modeling matches the way the lights are wired up.

Each of the 5 dome sections are colored separately (Blue, Red, Purple, Yellow, Green). All sections have 10 segments each except for the Blue door section which has only 9 segments.

Note that these colors are just for conceptualization and modeling but aren't otherwise meaningful.

![Lights Diagram](./docs/lights-diagram.png)

## Author

@thommahoney for [Deep Sea Kingdom](https://deepseakingdom.com)
