// SerTerm is a small serial terminal for the command line.
//
// It shows a list of serial devices to pick from, then streams the
// device's output while a reserved bottom line accepts messages to send.
// Ctrl+D returns to the device list, Ctrl+C quits.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"go.bug.st/serial"
)

// version is the release version, shown by the -version flag.
// It is overridden at release time via -ldflags "-X main.version=...".
var version = "dev"

type screen int

const (
	screenPicker screen = iota
	screenTerminal
)

// portSelectedMsg is emitted by the picker when the user chooses a device.
type portSelectedMsg struct {
	device string
	baud   int
}

// backToPickerMsg is emitted by the terminal when the user presses Ctrl+D.
type backToPickerMsg struct{}

// openFailedMsg reports that the selected device could not be opened; the
// picker shows the error.
type openFailedMsg struct {
	err error
}

// appModel is the root model: it owns the two screens and switches between them.
type appModel struct {
	screen   screen
	picker   pickerModel
	terminal terminalModel
	width    int
	height   int

	// initial, when set, is a device to connect to immediately on startup
	// (the `open` command), skipping the picker.
	initial *portSelectedMsg
}

func newAppModel() appModel {
	return appModel{screen: screenPicker, picker: newPickerModel()}
}

func (m appModel) Init() tea.Cmd {
	if m.initial != nil {
		selected := *m.initial
		return tea.Batch(m.picker.Init(), func() tea.Msg { return selected })
	}
	return m.picker.Init()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case portSelectedMsg:
		port, err := serial.Open(msg.device, &serial.Mode{BaudRate: msg.baud})
		if err != nil {
			failed := openFailedMsg{err: fmt.Errorf("cannot open %s: %w", msg.device, err)}
			return m, func() tea.Msg { return failed }
		}
		m.terminal = newTerminalModel(port, msg.device, msg.baud, m.width, m.height)
		m.screen = screenTerminal
		return m, m.terminal.Init()

	case backToPickerMsg:
		m.screen = screenPicker
		m.picker = newPickerModel()
		return m, m.picker.Init()
	}

	var cmd tea.Cmd
	switch m.screen {
	case screenPicker:
		m.picker, cmd = m.picker.Update(msg)
	case screenTerminal:
		m.terminal, cmd = m.terminal.Update(msg)
	}
	return m, cmd
}

func (m appModel) View() string {
	if m.screen == screenTerminal {
		return m.terminal.View()
	}
	return m.picker.View()
}

func main() {
	flag.Usage = func() { fmt.Print(usageText) }
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("serterm " + version)
		return
	}

	model := newAppModel()
	if args := flag.Args(); len(args) > 0 {
		tui := runCommand(args)
		if tui == nil {
			return
		}
		model = *tui
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
