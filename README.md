# SerTerm

[![CI](https://github.com/eliachiarucci/serterm/actions/workflows/ci.yml/badge.svg)](https://github.com/eliachiarucci/serterm/actions/workflows/ci.yml)

A simple serial terminal for the command line.

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
| `serterm open <device> [flags]` | connect and stream logs to stdout until interrupted (`ctrl+c`) |
| `serterm update` | update serterm to the latest release (runs the install script) |
| `serterm help` | show usage |

`open` flags:

| Flag | Action |
|------|--------|
| `-b`, `--baud N` | baud rate (default 115200) |
| `-s`, `--send TEXT` | send `TEXT` to the device after opening, `\n` appended (the response shows up in the stream) |
| `-ca`, `--close-after N` | exit after `N` seconds (max 60) |
| `-c`, `--close` | close right after sending, without reading a response (use with `--send` to just fire a message) |

```sh
serterm open /dev/cu.usbmodem1101 -b 9600 -ca 5   # capture 5 seconds of logs
serterm open /dev/cu.usbmodem1101 -s "status" -ca 3   # send a command, capture the reply
serterm open /dev/cu.usbmodem1101 -s "reboot" -c   # just send a message and close
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
| `ctrl+s` | save the output stream to a log file |
| `ctrl+l` | clear the output stream |
| `ctrl+d` | back to the device picker |
| `pgup`/`pgdn` | scroll the output |
| `ctrl+c` | quit |

The output also scrolls with the mouse wheel (via the emulator's alternate
scroll mode). serterm never captures the mouse, so your terminal's native
text selection and copy shortcuts work as usual.

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
