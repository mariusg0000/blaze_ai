// console_test.go — tests for console handler, command parsing, and TTY detection.
package console

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	"blazeai/internal/runtime"
	"blazeai/internal/session"
)

// mockAgent creates a minimal agent for console tests (no real provider needed).
func mockAgent(t *testing.T) *runtime.Agent {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	cfg := &config.Config{
		Providers:      []config.Provider{{Name: "test", Endpoint: "http://localhost", APIKey: "sk-test"}},
		Roles:          config.Roles{Default: "test/test-model"},
		FavoriteModels: []string{"test/test-model", "test/other-model"},
	}
	sess, _ := session.CreateInDir(dir)
	promptsDir := filepath.Join(dir, "prompts")
	os.MkdirAll(promptsDir, 0755)
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.md"), []byte("sys"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.linux.md"), []byte("linux"), 0644)

	agent, err := runtime.NewAgent(cfg, sess, platform.Linux, filepath.Join(dir, "skills"), promptsDir, dir, &mockHandler{})
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	return agent
}

// mockHandler is a no-op handler for agent construction.
type mockHandler struct{}

func (h *mockHandler) OnContent(delta string)                              {}
func (h *mockHandler) OnToolCall(name string, args json.RawMessage)        {}
func (h *mockHandler) OnToolResult(name string, result string)             {}

// newConsole creates a Console with a buffer for output and non-TTY mode.
func newConsole(agent *runtime.Agent) (*Console, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &Console{
		Out:     out,
		In:      strings.NewReader(""),
		IsTTY:   false,
		Agent:   agent,
		Reader:  NewReader(strings.NewReader(""), false),
		Spinner: NewSpinner(out, false),
	}, out
}

// TestOnContent verifies content is written to output with [BLAZE] label on first chunk.
func TestOnContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("hello ")
	c.OnContent("world")
	output := out.String()
	if !strings.Contains(output, "[BLAZE]") {
		t.Errorf("output missing [BLAZE] label: %q", output)
	}
	if !strings.Contains(output, "hello world") {
		t.Errorf("output missing content: %q", output)
	}
}

// TestOnToolCall verifies tool call display.
func TestOnToolCall(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", json.RawMessage(`{"command":"ls"}`))
	output := out.String()
	if !strings.Contains(output, "TOOL CALL") {
		t.Errorf("output missing [TOOL CALL]: %q", output)
	}
	if !strings.Contains(output, "shell") {
		t.Errorf("output missing tool name: %q", output)
	}
}

// TestOnToolResultSuccess verifies successful tool result display.
func TestOnToolResultSuccess(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nhi\n")
	output := out.String()
	if !strings.Contains(output, "TOOL RESPONSE") {
		t.Errorf("output missing [TOOL RESPONSE]: %q", output)
	}
	if !strings.Contains(output, "ok") {
		t.Errorf("output missing 'ok' status: %q", output)
	}
}

// TestOnToolResultError verifies error tool result display.
func TestOnToolResultError(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolResult("shell", "timeout 1s exceeded")
	output := out.String()
	if !strings.Contains(output, "error") {
		t.Errorf("output missing 'error' status: %q", output)
	}
}

// TestHandleCommandExit verifies /exit closes session.
func TestHandleCommandExit(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	handled, exit, err := c.handleCommand("/exit")
	if err != nil {
		t.Fatalf("/exit error: %v", err)
	}
	if !handled {
		t.Error("/exit not handled")
	}
	if !exit {
		t.Error("/exit should signal exit")
	}
	if !c.Agent.Session.ClosedCleanly {
		t.Error("session not closed cleanly")
	}
}

// TestHandleCommandModelList verifies /model without arg lists models.
func TestHandleCommandModelList(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	handled, exit, err := c.handleCommand("/model")
	if err != nil {
		t.Fatalf("/model error: %v", err)
	}
	if !handled || exit {
		t.Errorf("handled=%v exit=%v, want true/false", handled, exit)
	}
	if !strings.Contains(out.String(), "test-model") {
		t.Errorf("output missing model list: %q", out.String())
	}
}

// TestHandleCommandModelSet verifies /model with arg sets model.
func TestHandleCommandModelSet(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	handled, _, err := c.handleCommand("/model test/test-model")
	if err != nil {
		t.Fatalf("/model set error: %v", err)
	}
	if !handled {
		t.Error("/model not handled")
	}
	if !strings.Contains(out.String(), "Model set to") {
		t.Errorf("output missing confirmation: %q", out.String())
	}
}

// TestHandleCommandModelInvalid verifies /model with bad model errors.
func TestHandleCommandModelInvalid(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	_, _, err := c.handleCommand("/model ghost/bad")
	if err == nil {
		t.Error("/model with bad model should error")
	}
}

// TestHandleCommandCd verifies /cd changes work dir.
func TestHandleCommandCd(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	dir := t.TempDir()
	handled, _, err := c.handleCommand("/cd " + dir)
	if err != nil {
		t.Fatalf("/cd error: %v", err)
	}
	if !handled {
		t.Error("/cd not handled")
	}
	if c.Agent.WorkDir != dir {
		t.Errorf("WorkDir = %q, want %q", c.Agent.WorkDir, dir)
	}
	if !strings.Contains(out.String(), "Work folder") {
		t.Errorf("output missing confirmation: %q", out.String())
	}
}

// TestHandleCommandCdInvalid verifies /cd with bad path errors.
func TestHandleCommandCdInvalid(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	_, _, err := c.handleCommand("/cd /nonexistent/xyz")
	if err == nil {
		t.Error("/cd with bad path should error")
	}
}

// TestHandleCommandCdNoArg verifies /cd without arg errors.
func TestHandleCommandCdNoArg(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	_, _, err := c.handleCommand("/cd")
	if err == nil {
		t.Error("/cd without arg should error")
	}
}

// TestHandleCommandUnknown verifies unknown slash commands are not handled.
func TestHandleCommandUnknown(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	handled, exit, err := c.handleCommand("/unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Error("unknown command should not be handled")
	}
	if exit {
		t.Error("unknown command should not exit")
	}
}

// TestIsTerminal verifies TTY detection on stdout.
func TestIsTerminal(t *testing.T) {
	// os.Stdout may or may not be a TTY depending on test runner.
	// Just verify the function doesn't panic and returns a bool.
	result := isTerminal(os.Stdout)
	_ = result
}

// TestIsTerminalOnFile verifies non-terminal file returns false.
func TestIsTerminalOnFile(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "test")
	defer f.Close()
	if isTerminal(f) {
		t.Error("regular file should not be detected as terminal")
	}
}

// TestReaderReadLine verifies basic line reading.
func TestReaderReadLine(t *testing.T) {
	r := NewReader(strings.NewReader("hello\nworld\n"), false)
	line, err := r.ReadLine()
	if err != nil {
		t.Fatalf("ReadLine() error: %v", err)
	}
	if line != "hello" {
		t.Errorf("ReadLine() = %q, want 'hello'", line)
	}
}

// TestReaderReadLineEOF verifies EOF on empty input.
func TestReaderReadLineEOF(t *testing.T) {
	r := NewReader(strings.NewReader(""), false)
	_, err := r.ReadLine()
	if err == nil {
		t.Error("ReadLine() expected EOF, got nil")
	}
}
