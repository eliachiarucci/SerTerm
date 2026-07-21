package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"go.bug.st/serial"
)

const usageText = `serterm — a serial terminal for the command line

usage:
  serterm                                start the interactive device picker
  serterm list                           list connected serial devices
  serterm open [flags] <device> [seconds]   connect to a device; with [seconds] the
                                         logs stream to stdout for that many
                                         seconds (max 60) and then serterm
                                         exits (handy for scripts and agents)
  serterm help                           show this help

open flags (single or double dash both work):
  --baud N      baud rate (default 115200)
  --send TEXT   send TEXT to the device after opening, \n appended
                (requires a time limit; the response shows up in the stream)

examples:
  serterm list
  serterm open /dev/cu.usbmodem1101
  serterm open --baud 9600 /dev/cu.usbmodem1101
  serterm open --baud 115200 --send "status" /dev/cu.usbmodem1101 3

serterm --version prints the version.
`

// runCommand dispatches the non-TUI subcommands. It returns the model to run
// as a TUI, or nil if the command completed (or failed) headlessly.
func runCommand(args []string) *appModel {
	switch args[0] {
	case "list":
		if err := runList(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

	case "open":
		return runOpen(args[1:])

	case "help", "-h", "--help":
		fmt.Print(usageText)

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", args[0], usageText)
		os.Exit(2)
	}
	return nil
}

// runList prints the connected serial devices, one per line.
func runList() error {
	ports, err := listPorts()
	if err != nil {
		return err
	}
	if len(ports) == 0 {
		fmt.Fprintln(os.Stderr, "no serial devices found")
		return nil
	}
	for _, p := range ports {
		if p.description != "" {
			fmt.Printf("%s\t%s\n", p.device, p.description)
		} else {
			fmt.Println(p.device)
		}
	}
	return nil
}

// runOpen handles `serterm open`. Without a duration it returns a TUI model
// that goes straight to the terminal screen; with one it streams headlessly.
func runOpen(args []string) *appModel {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	baud := fs.Int("baud", baudRates[defaultBaudIndex], "baud rate")
	send := fs.String("send", "", "text to send to the device after opening (a \\n is appended)")
	fs.Parse(args)

	device, duration, err := parseOpenArgs(fs.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\n%s", err, usageText)
		os.Exit(2)
	}

	if duration == 0 {
		if *send != "" {
			fmt.Fprintln(os.Stderr, "error: -send requires a time limit, e.g. serterm open -send \"cmd\" "+device+" 5")
			os.Exit(2)
		}
		// Refuse to start the interactive screen when output is piped
		// (e.g. a script or agent): a forgotten background TUI would hold
		// the port open indefinitely.
		if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
			fmt.Fprintln(os.Stderr, "error: stdout is not a terminal; pass a time limit, e.g. serterm open "+device+" 5")
			os.Exit(2)
		}
		m := newAppModel()
		m.initial = &portSelectedMsg{device: device, baud: *baud}
		return &m
	}

	if err := streamFor(device, *baud, duration, *send); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	return nil
}

// maxOpenSeconds caps the time limit of a timed open.
const maxOpenSeconds = 60

// parseOpenArgs validates the positional arguments of the open command.
// A zero duration means "no time limit was given".
func parseOpenArgs(args []string) (device string, duration time.Duration, err error) {
	switch len(args) {
	case 1:
		return args[0], 0, nil
	case 2:
		secs, perr := strconv.ParseFloat(args[1], 64)
		if perr != nil || secs <= 0 {
			return "", 0, fmt.Errorf("invalid duration %q: expected a positive number of seconds", args[1])
		}
		if secs > maxOpenSeconds {
			return "", 0, fmt.Errorf("duration %q exceeds the maximum of %d seconds", args[1], maxOpenSeconds)
		}
		return args[0], time.Duration(secs * float64(time.Second)), nil
	case 0:
		return "", 0, fmt.Errorf("open requires a device, e.g. serterm open /dev/cu.usbmodem1101")
	default:
		return "", 0, fmt.Errorf("open takes at most a device and a number of seconds")
	}
}

// streamFor copies device output to stdout for the given duration. If send is
// non-empty it is written to the device first, with a newline appended, so the
// stream captures the response.
func streamFor(device string, baud int, d time.Duration, send string) error {
	port, err := serial.Open(device, &serial.Mode{BaudRate: baud})
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", device, err)
	}
	defer port.Close()

	if send != "" {
		if _, err := port.Write([]byte(send + "\n")); err != nil {
			return fmt.Errorf("write to %s failed: %w", device, err)
		}
	}

	// Poll with a short read timeout so the deadline is honored even while
	// the device is silent.
	if err := port.SetReadTimeout(100 * time.Millisecond); err != nil {
		return err
	}
	deadline := time.Now().Add(d)
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		n, err := port.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
		}
		if err != nil {
			return fmt.Errorf("read from %s failed: %w", device, err)
		}
	}
	return nil
}
