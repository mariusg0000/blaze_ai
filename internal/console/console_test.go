// console_test.go — tests for console handler, command parsing, and TTY detection.
package console

import (
	"bytes"
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

func (h *mockHandler) OnContent(delta string)                  {}
func (h *mockHandler) OnToolCall(name string, args string)     {}
func (h *mockHandler) OnToolResult(name string, result string) {}
func (h *mockHandler) OnUsage(promptTokens int)                {}

// newConsole creates a Console with a buffer for output and non-TTY mode.
func newConsole(agent *runtime.Agent) (*Console, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &Console{
		Out:              out,
		In:               strings.NewReader(""),
		IsTTY:            false,
		Agent:            agent,
		Reader:           NewReader(strings.NewReader(""), false),
		Spinner:          NewSpinner(out, false),
		needContentLabel: true,
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

// TestOnContentHeading verifies headings are rendered without markdown markers.
func TestOnContentHeading(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("### Title\n")
	output := out.String()
	if strings.Contains(output, "###") {
		t.Errorf("output should not contain heading markers: %q", output)
	}
	if !strings.Contains(output, "Title") {
		t.Errorf("output missing heading text: %q", output)
	}
}

// TestOnContentBullet verifies bullets are normalized for terminal output.
func TestOnContentBullet(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("- item\n")
	output := out.String()
	if !strings.Contains(output, "  - item") {
		t.Errorf("output missing normalized bullet: %q", output)
	}
}

// TestOnContentCodeFence verifies fenced code blocks are indented and fence lines are hidden.
func TestOnContentCodeFence(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("```\n")
	c.OnContent("fmt.Println(42)\n")
	c.OnContent("```\n")
	output := out.String()
	if strings.Contains(output, "```") {
		t.Errorf("output should not contain fence markers: %q", output)
	}
	if !strings.Contains(output, "    fmt.Println(42)") {
		t.Errorf("output missing indented code line: %q", output)
	}
}

// TestOnContentBold verifies **bold** markers are stripped on complete lines.
func TestOnContentBold(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("**Important** text\n")
	output := out.String()
	if strings.Contains(output, "**") {
		t.Errorf("output should not contain raw ** markers: %q", output)
	}
	if !strings.Contains(output, "Important") {
		t.Errorf("output missing bold text: %q", output)
	}
}

// TestOnContentBoldSplit verifies bold rendered correctly when split across chunks.
func TestOnContentBoldSplit(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("**Impor")
	c.OnContent("tant** text\n")
	output := out.String()
	if strings.Contains(output, "**") {
		t.Errorf("output should not contain raw ** markers: %q", output)
	}
	if !strings.Contains(output, "Important") {
		t.Errorf("output missing bold text: %q", output)
	}
}

// TestOnContentItalic verifies _italic_ markers are stripped on complete lines.
func TestOnContentItalic(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("_emphasis_ text\n")
	output := out.String()
	if strings.Contains(output, "_emphasis_") {
		t.Errorf("output should not contain raw _ markers: %q", output)
	}
	if !strings.Contains(output, "emphasis") {
		t.Errorf("output missing italic text: %q", output)
	}
}

// TestOnContentItalicAsterisk verifies *italic* markers are stripped.
func TestOnContentItalicAsterisk(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("*emphasis* text\n")
	output := out.String()
	if strings.Contains(output, "*emphasis*") {
		t.Errorf("output should not contain raw * italic markers: %q", output)
	}
	if !strings.Contains(output, "emphasis") {
		t.Errorf("output missing italic text: %q", output)
	}
}

// TestOnContentItalicAsteriskSplit verifies *italic* is buffered until closed.
func TestOnContentItalicAsteriskSplit(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("*empha")
	c.OnContent("sis* text\n")
	output := out.String()
	if strings.Contains(output, "*emphasis*") {
		t.Errorf("output should not contain raw * italic markers after split chunks: %q", output)
	}
	if !strings.Contains(output, "emphasis") {
		t.Errorf("output missing italic text after split chunks: %q", output)
	}
}

// TestOnContentBoldAndItalic verifies **bold** and *italic* work together.
func TestOnContentBoldAndItalic(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("**bold** and *italic*\n")
	output := out.String()
	if strings.Contains(output, "**") || strings.Contains(output, "*italic*") {
		t.Errorf("output should not contain raw markers: %q", output)
	}
	if !strings.Contains(output, "bold") || !strings.Contains(output, "italic") {
		t.Errorf("output missing styled text: %q", output)
	}
}

// TestOnContentLink verifies [text](url) is rendered as text (url).
func TestOnContentLink(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("see [OpenAI](https://openai.com) for details\n")
	output := out.String()
	if strings.Contains(output, "[OpenAI](https://openai.com)") {
		t.Errorf("output should not contain raw link markup: %q", output)
	}
	if !strings.Contains(output, "OpenAI") {
		t.Errorf("output missing link label: %q", output)
	}
	if !strings.Contains(output, "(https://openai.com)") {
		t.Errorf("output missing link URL: %q", output)
	}
}

// TestOnContentLinkMultiple verifies multiple links in one line.
func TestOnContentLinkMultiple(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("[A](http://a.com) and [B](http://b.com)\n")
	output := out.String()
	if strings.Contains(output, "[A]") || strings.Contains(output, "[B]") {
		t.Errorf("output should not contain raw link brackets: %q", output)
	}
	if !strings.Contains(output, "(http://a.com)") || !strings.Contains(output, "(http://b.com)") {
		t.Errorf("output missing link URLs: %q", output)
	}
}

// TestOnContentTable verifies table rows are flattened and separators removed.
func TestOnContentTable(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("| A | B | C |\n")
	c.OnContent("|---|---|---|\n")
	c.OnContent("| 1 | 2 | 3 |\n")
	output := out.String()
	if strings.Contains(output, "|") {
		t.Errorf("output should not contain raw pipe characters: %q", output)
	}
	if !strings.Contains(output, "A") || !strings.Contains(output, "B") || !strings.Contains(output, "C") {
		t.Errorf("output missing table cell text: %q", output)
	}
	if strings.Contains(output, "---") {
		t.Errorf("output should not contain table separator: %q", output)
	}
}

// TestOnToolCall verifies tool call display.
func TestOnToolCall(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "ls")
	output := out.String()
	if !strings.Contains(output, "[CALL]") {
		t.Errorf("output missing [CALL]: %q", output)
	}
	if !strings.Contains(output, "shell") {
		t.Errorf("output missing tool name: %q", output)
	}
	if !strings.Contains(output, "shell        ls") {
		t.Errorf("output missing formatted args: %q", output)
	}
	if !strings.Contains(output, "tools ") {
		t.Errorf("output missing tools divider header: %q", output)
	}
}

// TestOnToolCallEmptyArgs verifies no dash when args are empty.
func TestOnToolCallEmptyArgs(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "")
	output := out.String()
	if strings.Contains(output, " - ") {
		t.Errorf("output should not contain dash for empty args: %q", output)
	}
	if !strings.Contains(output, "shell\n") {
		t.Errorf("output missing tool name: %q", output)
	}
}

// TestOnToolCallAfterContent verifies a newline is inserted before tool blocks.
func TestOnToolCallAfterContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("hello")
	c.OnToolCall("shell", "ls")
	output := out.String()
	if !strings.Contains(output, "hello\ntools ------------------------------------------------------\n[CALL]") {
		t.Errorf("output missing newline before tool call block: %q", output)
	}
}

// TestOnToolResultSuccess verifies successful shell result display.
func TestOnToolResultSuccess(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nhi\n")
	output := out.String()
	if !strings.Contains(output, "[OK]") || !strings.Contains(output, "shell") {
		t.Errorf("output missing compact tool result: %q", output)
	}
	if !strings.Contains(output, "[OK]") {
		t.Errorf("output missing [OK] status: %q", output)
	}
	if !strings.Contains(output, "[OK] shell        hi") {
		t.Errorf("output missing compact content: %q", output)
	}
	if strings.Contains(output, "exit_code") {
		t.Errorf("output should not contain raw exit_code: %q", output)
	}
}

// TestOnToolResultErrorExitCode verifies non-zero shell exit is displayed as ERROR.
func TestOnToolResultErrorExitCode(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolResult("shell", "exit_code: 1\nstderr:\nfile not found\n")
	output := out.String()
	if !strings.Contains(output, "[ERROR]") {
		t.Errorf("output missing [ERROR] status: %q", output)
	}
	if !strings.Contains(output, "file not found") {
		t.Errorf("output missing stderr content: %q", output)
	}
}

// TestOnToolResultTimeout verifies timeout messages display TIMEOUT badge.
func TestOnToolResultTimeout(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolResult("shell", "timeout 1s exceeded")
	output := out.String()
	if !strings.Contains(output, "[TIMEOUT]") {
		t.Errorf("output missing [TIMEOUT] status: %q", output)
	}
}

// TestOnToolResultGenericError verifies non-shell error messages display ERROR badge.
func TestOnToolResultGenericError(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolResult("shell", "error: unknown tool: x")
	output := out.String()
	if !strings.Contains(output, "[ERROR]") {
		t.Errorf("output missing [ERROR] status: %q", output)
	}
	if !strings.Contains(output, "unknown tool") {
		t.Errorf("output missing error content: %q", output)
	}
}

// TestOnToolRoundTripAfterContent verifies the full tool block stays on separate lines.
func TestOnToolRoundTripAfterContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("hello")
	c.OnToolCall("shell", "ls")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nok\n")
	c.closeToolGroup()
	output := out.String()
	if !strings.Contains(output, "hello\ntools ------------------------------------------------------\n[CALL]") {
		t.Errorf("tool call block not separated from content: %q", output)
	}
	if !strings.Contains(output, "shell        ls\n[OK] shell") {
		t.Errorf("tool response formatting unexpected: %q", output)
	}
	// Group should close with a trailing separator after the last response.
	if !strings.Contains(output, "[OK] shell        ok\n------------------------------------------------------------") {
		t.Errorf("tool group not closed with separator: %q", output)
	}
}

// TestToolGroupConsecutive verifies multiple consecutive tools share one group.
func TestToolGroupConsecutive(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "ls")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\na\n")
	c.OnToolCall("shell", "cat")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nb\n")
	c.closeToolGroup()
	output := out.String()
	if strings.Count(output, "tools ------------------------------------------------------") != 1 {
		t.Errorf("expected one tools header, got %q", output)
	}
	if strings.Count(output, "------------------------------------------------------------") != 1 {
		t.Errorf("expected one closing divider, got %d: %q", strings.Count(output, "------------------------------------------------------------"), output)
	}
	if !strings.Contains(output, "[CALL] shell        ls\n[OK] shell") {
		t.Errorf("first tool call missing: %q", output)
	}
	if !strings.Contains(output, "[CALL] shell        cat\n[OK] shell") {
		t.Errorf("second tool call missing: %q", output)
	}
}

// TestToolGroupInterruptedByContent verifies content between tools closes and reopens the group.
func TestToolGroupInterruptedByContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "ls")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\na\n")
	c.OnContent("continuing")
	c.OnToolCall("shell", "cat")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nb\n")
	c.closeToolGroup()
	output := out.String()
	if strings.Count(output, "tools ------------------------------------------------------") != 2 {
		t.Errorf("expected 2 tools headers, got %d: %q", strings.Count(output, "tools ------------------------------------------------------"), output)
	}
	if strings.Count(output, "------------------------------------------------------------") != 2 {
		t.Errorf("expected 2 closing dividers, got %d: %q", strings.Count(output, "------------------------------------------------------------"), output)
	}
	if !strings.Contains(output, "continuing") {
		t.Errorf("intermediate content missing: %q", output)
	}
	if !strings.Contains(output, "[BLAZE] continuing") {
		t.Errorf("content after tool group should restart with [BLAZE]: %q", output)
	}
}

// TestOnUsage verifies context usage is stored and rendered in the separator.
func TestOnUsage(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnUsage(11186)
	c.responseSeparator()
	output := out.String()
	if !strings.Contains(output, "ctx 11k") {
		t.Errorf("output missing context size: %q", output)
	}
}

// TestOnUsageZero verifies no context shown when prompt tokens are zero.
func TestOnUsageZero(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnUsage(0)
	c.responseSeparator()
	output := out.String()
	if output != "" {
		t.Errorf("output should be empty when no usage: %q", output)
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
