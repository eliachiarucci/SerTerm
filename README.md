# SerTerm

[![CI](https://github.com/eliachiarucci/serterm/actions/workflows/ci.yml/badge.svg)](https://github.com/eliachiarucci/serterm/actions/workflows/ci.yml)

A simple serial terminal for the command line.
Talk to anything that speaks serial.

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

`serterm -version` prints the version.

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
- `ports.go` — device discovery (hides macOS `/dev/tty.*` duplicates, USB first)
- `picker.go` — device selection screen
- `terminal.go` — streaming view, input line, serial reader goroutine
- `styles.go` — shared lipgloss styles

## License

[MIT](LICENSE)
