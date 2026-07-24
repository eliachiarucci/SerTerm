package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"go.bug.st/serial"
)

// fakePort records writes so terminal logic can be tested without hardware.
type fakePort struct {
	serial.Port // panics if an untested method is called
	written     []byte
	writeErr    error
}

func (f *fakePort) Close() error { return nil }

func (f *fakePort) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.written = append(f.written, p...)
	return len(p), nil
}

func testTerminal(port serial.Port) terminalModel {
	input := textinput.New()
	input.Focus()
	return terminalModel{
		port:     port,
		viewport: viewport.New(viewport.WithWidth(80), viewport.WithHeight(24)),
		input:    input,
		done:     make(chan struct{}),
		width:    80,
		height:   27,
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   string
	}{
		{"crlf in one chunk", []string{"a\r\nb\r\n"}, "a\nb\n"},
		{"crlf split across chunks", []string{"a\r", "\nb\r\n"}, "a\nb\n"},
		{"bare cr", []string{"a\rb"}, "a\nb"},
		{"bare lf", []string{"a\nb"}, "a\nb"},
		{"cr at end then plain text", []string{"a\r", "b"}, "a\nb"},
		{"consecutive split pairs", []string{"a\r", "\n", "b\r", "\nc"}, "a\nb\nc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m terminalModel
			got := ""
			for _, c := range tt.chunks {
				got += m.normalize(c)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendTrimsOldContentAtLineBoundary(t *testing.T) {
	m := testTerminal(nil)

	// Append twice the cap's worth of data. Large lines keep the iteration
	// count low: append re-wraps the whole buffer each call, which would
	// make thousands of tiny appends slow.
	const lineLen = 4096
	line := strings.Repeat("x", lineLen-1) + "\n"
	for i := 0; i < 2*maxContentBytes/lineLen; i++ {
		m.append(line)
	}

	if len(m.content) > maxContentBytes {
		t.Errorf("content grew past the cap: %d bytes", len(m.content))
	}
	// After trimming, the buffer must still start on a full line.
	if !strings.HasPrefix(m.content, "x") || strings.Index(m.content, "\n") != lineLen-1 {
		t.Errorf("content does not start at a line boundary: %q...", m.content[:20])
	}
}

func TestCtrlLClearsStream(t *testing.T) {
	m := testTerminal(nil)
	m.append("some output\nmore output\n")

	m, _ = m.Update(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})

	if m.content != "" {
		t.Errorf("ctrl+l should clear the stream, got %q", m.content)
	}
}

func TestSendWritesLineAndEchoes(t *testing.T) {
	port := &fakePort{}
	m := testTerminal(port)
	m.input.SetValue("hello")

	m = m.send()

	if got := string(port.written); got != "hello\n" {
		t.Errorf("port received %q, want %q", got, "hello\n")
	}
	if !strings.Contains(m.content, "hello") {
		t.Error("sent message should be echoed in the stream")
	}
	if m.input.Value() != "" {
		t.Errorf("input should be cleared after send, got %q", m.input.Value())
	}
}

func TestSendIgnoresEmptyInput(t *testing.T) {
	port := &fakePort{}
	m := testTerminal(port)

	m = m.send()

	if len(port.written) != 0 {
		t.Errorf("nothing should be written for empty input, got %q", port.written)
	}
}

func TestSendAfterDisconnectDoesNotWrite(t *testing.T) {
	port := &fakePort{}
	m := testTerminal(port)
	m.disconnected = true
	m.input.SetValue("hello")

	m = m.send()

	if len(port.written) != 0 {
		t.Errorf("nothing should be written after disconnect, got %q", port.written)
	}
}

func manyLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// Wheel motion reaches the app as up/down arrow keys via the emulator's alternate scroll mode
func TestArrowKeysScrollViewport(t *testing.T) {
	m := testTerminal(nil)
	m.append(manyLines(100)) // append pins the view to the bottom

	if !m.viewport.AtBottom() {
		t.Fatal("viewport should start at the bottom")
	}
	before := m.viewport.YOffset()

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.viewport.YOffset() >= before {
		t.Errorf("up should scroll up: offset went from %d to %d", before, m.viewport.YOffset())
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.viewport.YOffset() != before {
		t.Errorf("down should scroll back down: want offset %d, got %d", before, m.viewport.YOffset())
	}
}

func TestPlaceholderRendersFully(t *testing.T) {
	m := testTerminal(nil)
	m.input.Prompt = "❯ "
	m.input.Placeholder = "type a message, enter to send"
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 27})

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "type a message, enter to send") {
		t.Errorf("placeholder should render fully, view line: %q", view)
	}
}

func TestCtrlCQuits(t *testing.T) {
	m := testTerminal(&fakePort{})

	m, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !m.closed {
		t.Error("ctrl+c should close the session")
	}
	if cmd == nil {
		t.Fatal("ctrl+c should emit quit")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Error("expected the quit command")
	}
}

func TestCtrlSSavesLogAndRemembersDirectory(t *testing.T) {
	dir := t.TempDir()
	saveDir = dir
	t.Cleanup(func() { saveDir = "" })

	m := testTerminal(&fakePort{})
	m.append("device output\n")
	m.input.SetValue("half-typed message")

	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if m.mode != modeSaveName {
		t.Fatal("ctrl+s should enter file name mode")
	}
	if m.input.Value() != "" {
		t.Errorf("input should be cleared for the file name, got %q", m.input.Value())
	}

	m.input.SetValue("session.log")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != modeSavePath {
		t.Fatal("enter should move to directory confirmation mode")
	}
	if m.input.Value() != dir {
		t.Errorf("input should show the save directory %q, got %q", dir, m.input.Value())
	}

	// Edit the directory before confirming.
	other := filepath.Join(dir, "logs")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	m.input.SetValue(other)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	moved := filepath.Join(other, "session.log")
	data, err := os.ReadFile(moved)
	if err != nil {
		t.Fatalf("log file should exist: %v", err)
	}
	if string(data) != "device output\n" {
		t.Errorf("log content = %q, want %q", data, "device output\n")
	}
	if m.mode != modeMessage {
		t.Error("saving should return to message mode")
	}
	if m.input.Value() != "half-typed message" {
		t.Errorf("draft should be restored, got %q", m.input.Value())
	}
	if saveDir != other {
		t.Errorf("save directory should be remembered: got %q, want %q", saveDir, other)
	}
	if !strings.Contains(ansi.Strip(m.content), "log saved to "+moved) {
		t.Error("a save notice should be appended to the stream")
	}
}

func TestSaveOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	saveDir = dir
	t.Cleanup(func() { saveDir = "" })

	path := filepath.Join(dir, "session.log")
	if err := os.WriteFile(path, []byte("old contents"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := testTerminal(&fakePort{})
	m.append("new output\n")
	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m.input.SetValue("session.log")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new output\n" {
		t.Errorf("file should be overwritten without asking, got %q", data)
	}
}

func TestOverwriteHintOnlyWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	saveDir = dir
	t.Cleanup(func() { saveDir = "" })

	m := testTerminal(&fakePort{})
	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m.input.SetValue("session.log")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if strings.Contains(ansi.Strip(m.View()), "(overwrites)") {
		t.Error("overwrite hint should not show for a new file")
	}

	if err := os.WriteFile(filepath.Join(dir, "session.log"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Any edit to the directory re-checks for an existing file.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})

	if !strings.Contains(ansi.Strip(m.View()), "(overwrites)") {
		t.Error("overwrite hint should show when the file already exists")
	}
}

func TestEscCancelsSaveModeAndRestoresDraft(t *testing.T) {
	m := testTerminal(&fakePort{})
	m.input.SetValue("half-typed message")

	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m.input.SetValue("session.log")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.mode != modeMessage {
		t.Error("esc should leave save mode")
	}
	if m.input.Value() != "half-typed message" {
		t.Errorf("draft should be restored, got %q", m.input.Value())
	}
}

func TestSaveIgnoresEmptyFileName(t *testing.T) {
	m := testTerminal(&fakePort{})

	m, _ = m.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.mode != modeSaveName {
		t.Error("enter on an empty file name should stay in name mode")
	}
}

func TestSendReportsWriteError(t *testing.T) {
	port := &fakePort{writeErr: errors.New("device gone")}
	m := testTerminal(port)
	m.input.SetValue("hello")

	m = m.send()

	if !strings.Contains(m.content, "write failed") {
		t.Error("write errors should be shown in the stream")
	}
	if m.input.Value() != "hello" {
		t.Error("input should be kept when the write fails")
	}
}
