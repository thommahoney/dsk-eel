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
- **Permission to read the joystick.** `/dev/hidraw*` usually belongs to root, so run the game with `sudo`, or add a udev rule that gives your user the device.

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
