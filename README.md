# SerTerm

[![CI](https://github.com/eliachiarucci/serterm/actions/workflows/ci.yml/badge.svg)](https://github.com/eliachiarucci/serterm/actions/workflows/ci.yml)

A simple serial terminal for the command line.
With AI agent-friendly CLI commands, talk to anything that speaks serial.

![SerTerm](images/demo.gif)

## Install

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/eliachiarucci/serterm/main/install.sh | sh
```

**Windows**

Download the zip for your architecture from the
[latest release](https://github.com/eliachiarucci/serterm/releases/latest),
extract `serterm.exe`, and put it on your `PATH`.

**Packages**

`.deb`, `.rpm`, and `.apk` packages are attached to each
[release](https://github.com/eliachiarucci/serterm/releases).

**With Go**

```sh
go install github.com/eliachiarucci/serterm@latest
```

Or build from source (see [Build](#build) below).

## Usage

```sh
serterm
```

Running `serterm` with no arguments starts the interactive device picker.
There are also a few subcommands:

| Command | Action |
|---------|--------|
| `serterm list` | list connected serial devices (device, tab, description) |
| `serterm open [--baud N] <device>` | connect and go straight to the terminal screen |
| `serterm open [--baud N] <device> <secs>` | stream logs to stdout for `secs` seconds (max 60), then exit |
| `serterm open --send "text" <device> <secs>` | send a line to the device, then stream the response |
| `serterm update` | update serterm to the latest release (runs the install script) |
| `serterm help` | show usage |

The timed form of `open` is designed for scripts and AI agents: it needs no
TTY, prints raw device output, and always releases the port when it exits.
Without a time limit, `open` refuses to run if stdout is not a terminal.

```sh
serterm open --baud 9600 /dev/cu.usbmodem1101 5   # capture 5 seconds of logs
serterm open --send "status" /dev/cu.usbmodem1101 3   # send a command, capture the reply
```

`serterm --version` prints the version.

**Device picker**

| Key | Action |
|-----|--------|
| `↑`/`↓` | select device |
| `←`/`→` | change baud rate (default 115200) |
| `enter` | connect |
| `r` | refresh device list |
| `q` | quit |

**Terminal**

| Key | Action |
|-----|--------|
| `enter` | send the input line (a `\n` is appended) |
| `ctrl+l` | clear the output stream |
| `ctrl+d` | back to the device picker |
| `pgup`/`pgdn` | scroll the output |
| `ctrl+c` | quit |

Sent messages are echoed in the stream prefixed with `→`. If the device is
unplugged, a notice appears and `ctrl+d` returns to the picker.

## Build

```sh
go build -ldflags="-s -w" -o serterm .
```

Produces a single self-contained binary, no runtime dependencies.

## Test

```sh
go test ./...
```

## Code layout

- `main.go` — root model; switches between the two screens
- `cli.go` — `list`, `open`, `update`, and `help` subcommands
- `ports.go` — device discovery (hides macOS `/dev/tty.*` duplicates, USB first)
- `picker.go` — device selection screen
- `terminal.go` — streaming view, input line, serial reader goroutine
- `styles.go` — shared lipgloss styles

## License

[MIT](LICENSE)
