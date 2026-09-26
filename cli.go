package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"go.bug.st/serial"
)

const usageText = `serterm — a serial terminal for the command line

usage:
  serterm                        start the interactive device picker
  serterm list                   list connected serial devices
  serterm open <device> [flags]  connect to a device and stream its logs to
                                 stdout until interrupted (ctrl+c)
  serterm update                 update serterm to the latest release
  serterm help                   show this help (also -h, --help)

open flags (single or double dash both work):
  -b,  --baud N         baud rate (default 115200)
  -s,  --send TEXT      send TEXT to the device after opening, \n appended
                        (the response shows up in the stream)
  -ca, --close-after N  exit after N seconds (max 60; handy for scripts
                        and agents)
  -c,  --close          close right after sending, without reading a response
                        (use with --send to just fire a message)

examples:
  serterm list
  serterm open /dev/cu.usbmodem1101
  serterm open /dev/cu.usbmodem1101 -b 9600
  serterm open /dev/cu.usbmodem1101 -s "status" -ca 3
  serterm open /dev/cu.usbmodem1101 -s "reboot" -c

serterm --version prints the version.
`

// runCommand dispatches the subcommands. All subcommands run headlessly; the
// TUI only starts when serterm is invoked with no arguments.
func runCommand(args []string) {
	switch args[0] {
	case "list":
		if err := runList(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

	case "open":
		runOpen(args[1:])

	case "update":
		if err := runUpdate(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

	case "help", "-h", "--help":
		fmt.Print(usageText)

	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", args[0], usageText)
		os.Exit(2)
	}
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

// runOpen handles `serterm open`: it streams the device's logs to stdout,
// optionally sending a message first, until interrupted or -ca expires.
// With -c it just sends and closes.
func runOpen(args []string) {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(usageText)
		return
	}
	device, flagArgs, err := parseOpenArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\n%s", err, usageText)
		os.Exit(2)
	}

	fs := flag.NewFlagSet("open", flag.ExitOnError)
	fs.Usage = func() { fmt.Print(usageText) }
	var baud int
	fs.IntVar(&baud, "b", baudRates[defaultBaudIndex], "baud rate")
	fs.IntVar(&baud, "baud", baudRates[defaultBaudIndex], "baud rate")
	var send string
	fs.StringVar(&send, "s", "", "text to send to the device after opening (a \\n is appended)")
	fs.StringVar(&send, "send", "", "text to send to the device after opening (a \\n is appended)")
	var closeAfter float64
	fs.Float64Var(&closeAfter, "ca", 0, "exit after N seconds")
	fs.Float64Var(&closeAfter, "close-after", 0, "exit after N seconds")
	var closeNow bool
	fs.BoolVar(&closeNow, "c", false, "close right after sending, without reading a response")
	fs.BoolVar(&closeNow, "close", false, "close right after sending, without reading a response")
	fs.Parse(flagArgs)

	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "error: unexpected argument %q\n\n%s", fs.Arg(0), usageText)
		os.Exit(2)
	}
	duration, err := parseCloseAfter(closeAfter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n\n%s", err, usageText)
		os.Exit(2)
	}

	if closeNow {
		if duration > 0 {
			fmt.Fprintln(os.Stderr, "error: --close cannot be combined with --close-after")
			os.Exit(2)
		}
		if send == "" {
			fmt.Fprintln(os.Stderr, "error: --close requires --send, e.g. serterm open "+device+" -s \"reboot\" -c")
			os.Exit(2)
		}
		if err := sendAndClose(device, baud, send); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	if err := startInlineStream(device, baud, duration, send); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const (
	installScriptURL   = "https://raw.githubusercontent.com/eliachiarucci/serterm/main/install.sh"
	installScriptURLPS = "https://raw.githubusercontent.com/eliachiarucci/serterm/main/install.ps1"
)

const latestReleaseURL = "https://api.github.com/repos/eliachiarucci/serterm/releases/latest"

// runUpdate reinstalls serterm by piping the install script through sh
// (or PowerShell on Windows), unless the installed version already matches
// the latest release.
func runUpdate() error {
	latest, err := latestVersion()
	if err != nil {
		return fmt.Errorf("cannot check the latest version: %v\n"+
			"check your internet connection, or download the latest release manually from\n"+
			"https://github.com/eliachiarucci/serterm/releases/latest", err)
	}
	if isUpToDate(version, latest) {
		fmt.Println("Already up to date (v" + version + ")")
		return nil
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "irm "+installScriptURLPS+" | iex")
	} else {
		cmd = exec.Command("sh", "-c", "curl -fsSL "+installScriptURL+" | sh")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	return nil
}

// latestVersion returns the tag of the latest GitHub release, e.g. "v1.2.3".
func latestVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(latestReleaseURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", latestReleaseURL, resp.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

// isUpToDate reports whether the running version matches the latest release
// tag. A "dev" build (built from source, no ldflags) never matches.
func isUpToDate(current, latestTag string) bool {
	latest := strings.TrimPrefix(latestTag, "v")
	return latest != "" && current == latest
}

// maxOpenSeconds caps the time limit of a timed open.
const maxOpenSeconds = 60

// parseOpenArgs splits the open command's arguments into the device (which
// must come first) and the flags that follow it.
func parseOpenArgs(args []string) (device string, flags []string, err error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("open requires a device, e.g. serterm open /dev/cu.usbmodem1101")
	}
	if strings.HasPrefix(args[0], "-") {
		return "", nil, fmt.Errorf("the device comes before the flags, e.g. serterm open /dev/cu.usbmodem1101 -b 9600")
	}
	return args[0], args[1:], nil
}

// parseCloseAfter validates the --close-after flag value. A zero duration
// means the flag was not given.
func parseCloseAfter(secs float64) (time.Duration, error) {
	if secs < 0 {
		return 0, fmt.Errorf("invalid --close-after value %v: expected a positive number of seconds", secs)
	}
	if secs > maxOpenSeconds {
		return 0, fmt.Errorf("--close-after value %v exceeds the maximum of %d seconds", secs, maxOpenSeconds)
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// sendAndClose opens the device, writes the message, and closes the port
// without reading a response.
func sendAndClose(device string, baud int, send string) error {
	port, err := serial.Open(device, &serial.Mode{BaudRate: baud})
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", device, err)
	}
	defer port.Close()

	if _, err := port.Write([]byte(send + "\n")); err != nil {
		return fmt.Errorf("write to %s failed: %w", device, err)
	}
	return port.Drain()
}

// startInlineStream copies device output to stdout.
func startInlineStream(device string, baud int, d time.Duration, send string) error {
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
	buf := make([]byte, 4096)
	if d == 0 {
		for {
			n, err := port.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
			}
			if err != nil {
				return fmt.Errorf("read from %s failed: %w", device, err)
			}
		}
	} else {
		deadline := time.Now().Add(d)
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
}
