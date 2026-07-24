// console_test.go — tests for console handler, command parsing, and TTY detection.
package console

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"blazeai/internal/config"
	"blazeai/internal/helpers"
	"blazeai/internal/platform"
	"blazeai/internal/provider"
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
	writePromptFixtures(t, promptsDir)

	// Create interactive agent definitions in the agent dir for NewAgent.
	home := t.TempDir()
	t.Setenv("HOME", home)
	agentsHome := filepath.Join(home, "blazeai")
	agentsDir := filepath.Join(agentsHome, "agents")
	os.MkdirAll(agentsDir, 0755)
	defaultDef := "---\nname: default\ndescription: Default test agent\ntype: interactive\nmodel: test/test-model\ntools:\n  - shell\n  - read_file\n  - write_file\n  - replace_block\n  - ask_a_friend\n  - analyze_image\n  - load_skill\n  - task_write\n  - task_read\n---\nDefault agent instructions.\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "default.md"), []byte(defaultDef), 0600); err != nil {
		t.Fatalf("cannot write default agent definition: %v", err)
	}
	planningDef := "---\nname: planning\ndescription: Planning test agent\ntype: interactive\nmodel: test/other-model\ntools:\n  - shell\n  - read_file\n  - write_file\n  - replace_block\n  - ask_a_friend\n  - analyze_image\n  - load_skill\n  - task_write\n  - task_read\n---\nPlanning agent instructions.\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "planning.md"), []byte(planningDef), 0600); err != nil {
		t.Fatalf("cannot write planning agent definition: %v", err)
	}
	workerDef := "---\nname: worker\ndescription: Worker test agent\ntype: executor\nmodel: test/test-model\ntools:\n  - shell\n  - read_file\n  - write_file\n  - replace_block\n  - ask_a_friend\n  - analyze_image\n  - load_skill\n  - task_write\n  - task_read\n---\nWorker agent instructions.\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "worker.md"), []byte(workerDef), 0600); err != nil {
		t.Fatalf("cannot write worker agent definition: %v", err)
	}

	agent, err := runtime.NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), fstest.MapFS{}, dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	return agent
}

// mockHandler is a no-op handler for agent construction.
type mockHandler struct{}

func (h *mockHandler) OnContent(delta string)                                 {}
func (h *mockHandler) OnToolCall(name string, args string)                    {}
func (h *mockHandler) OnToolResult(name string, result string)                {}
func (h *mockHandler) OnUsage(promptTokens, cachedTokens, uncachedTokens int) {}
func (h *mockHandler) OnSystem(message string)                                {}
func (h *mockHandler) OnMaintenanceCall(name string, args string)             {}
func (h *mockHandler) OnMaintenanceResult(name string, result string)         {}
func (h *mockHandler) RequestSudoApproval(command string) (bool, string)      { return false, "" }

// newConsole creates a Console with a buffer for output in TTY mode.
func newConsole(agent *runtime.Agent) (*Console, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &Console{
		Out:    out,
		In:     strings.NewReader(""),
		Agent:  agent,
		Reader: NewReader(strings.NewReader(""), true),
	}, out
}

func stripANSICodes(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

func TestFormatSkillName(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"builtin/skill-manager", "skill-manager"},
		{"global/custom", "custom"},
		{"project/custom", "project/custom"},
	} {
		if got := formatSkillName(tc.input); got != tc.want {
			t.Errorf("formatSkillName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// writePromptFixtures creates the prompt templates required by runtime prompt assembly.
func writePromptFixtures(t *testing.T, promptsDir string) {
	t.Helper()
	content := "# Universal System Prompt\n\nApp home is at {APP_HOME}.\nUnknown var: {UNKNOWN_VAR}.\n\n{OS_PROMPT}\n\n## Transport\n{TRANSPORT_PROMPT}\n\n{TRANSPORT_CONTEXT}\n\n## Host Environment Helpers\nAvailable helpers:\n{HOST_HELPERS_AVAILABLE}\n\nOptional helpers:\n{HOST_HELPERS_OPTIONAL}\n\n## Skills\nBefore performing any task, scan available skill descriptions. If a domain or system mentioned in the request appears in a skill's description, you MUST load that skill first.\n\nAvailable skills:\n{SKILLS_AVAILABLE}\n\n## Project Rules (AGENTS.md)\n{AGENTS_CONTENT}\n"
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.md"), []byte(content), 0644)
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.linux.md"), []byte("linux"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "transport.console.md"), []byte("console transport"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "transport.telegram.md"), []byte("telegram transport"), 0644)
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

// TestOnToolCall verifies tool args are printed immediately and buffered for result append.
func TestOnToolCall(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "inspect package.json scripts")
	if c.lastToolArgs != "inspect package.json scripts" {
		t.Errorf("lastToolArgs = %q, want 'inspect package.json scripts'", c.lastToolArgs)
	}
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "💻 inspect package.json scripts …") {
		t.Errorf("output missing tool purpose line: %q", out.String())
	}
}

// TestOnToolCallEmptyArgs verifies tool group header appears even with empty args.
func TestOnToolCallEmptyArgs(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "")
	c.OnToolResult("shell", "ok")
	c.stopSpinner()
	output := out.String()
	if !strings.Contains(output, "💻") {
		t.Errorf("output missing wrench icon: %q", output)
	}
}

// TestOnToolCallAfterContent verifies a blank line separates content from the first tool block.
func TestOnToolCallAfterContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("hello")
	c.OnToolCall("shell", "ls")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nok\n")
	c.stopSpinner()
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "hello\n\n💻 ls …") {
		t.Errorf("output missing blank line before tool call: %q", out.String())
	}
}

// TestOnToolResultSuccess verifies successful tool results append a checkmark.
func TestOnToolResultSuccess(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "inspect package.json scripts")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nhi\n")
	c.stopSpinner()
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "💻 inspect package.json scripts … ✔️") {
		t.Errorf("output missing appended success line: %q", plain)
	}
	if strings.Contains(plain, "exit_code") {
		t.Errorf("output should not contain raw exit_code: %q", plain)
	}
}

// TestOnMaintenanceResultOmitsContextAndKeepsDetailInline verifies maintenance status formatting.
func TestOnMaintenanceResultOmitsContextAndKeepsDetailInline(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.lastPromptTokens = 106000
	c.OnMaintenanceCall("compaction", "Compacting on max token limits")
	c.OnMaintenanceResult("compaction", "ok 20 messages pruned and summarized")
	c.stopSpinner()
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "🗜️ Compacting on max token limits … ✔️  20 messages pruned and summarized") {
		t.Errorf("maintenance output missing inline completion: %q", plain)
	}
	if strings.Contains(plain, "CTX:") {
		t.Errorf("maintenance output should not contain context usage: %q", plain)
	}

	out.Reset()
	c.OnMaintenanceCall("compaction", "Compacting on max token limits")
	c.OnMaintenanceResult("compaction", "error: summarization failed")
	c.stopSpinner()
	plain = stripANSICodes(out.String())
	if !strings.Contains(plain, " … ✖️ summarization failed") {
		t.Errorf("maintenance output missing inline error: %q", plain)
	}
}

// TestOnToolResultSuccessTTY verifies the status is appended to the tool line.
func TestOnToolResultSuccessTTY(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "inspect package.json scripts")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nhi\n")
	c.stopSpinner()
	output := out.String()
	plain := stripANSICodes(output)
	if strings.Contains(output, "\r\033[K") {
		t.Errorf("output should not clear and redraw the tool line: %q", output)
	}
	if !strings.Contains(plain, "💻 inspect package.json scripts … ✔️\n") {
		t.Errorf("TTY output missing appended success status: %q", plain)
	}
	if strings.Count(plain, "inspect package.json scripts") != 1 {
		t.Errorf("TTY output should print the purpose once: %q", plain)
	}
}

// TestOnToolResultErrorExitCode verifies non-zero shell exit shows ✖️ with message.
func TestOnToolResultErrorExitCode(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "inspect config")
	c.OnToolResult("shell", "exit_code: 1\nstderr:\nfile not found\n")
	c.stopSpinner()
	output := out.String()
	if !strings.Contains(output, "✖️") {
		t.Errorf("output missing ✖️ badge: %q", output)
	}
	if !strings.Contains(output, "file not found") {
		t.Errorf("output missing stderr content: %q", output)
	}
}

// TestOnToolResultTimeout verifies timeout messages display ⏱ badge.
func TestOnToolResultTimeout(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "list large directory")
	c.OnToolResult("shell", "timeout 1s exceeded")
	c.stopSpinner()
	output := out.String()
	if !strings.Contains(output, "⏱") {
		t.Errorf("output missing ⏱ badge: %q", output)
	}
}

// TestOnToolResultGenericError verifies error messages display ✖️ badge.
func TestOnToolResultGenericError(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "run unknown command")
	c.OnToolResult("shell", "error: unknown tool: x")
	c.stopSpinner()
	output := out.String()
	if !strings.Contains(output, "✖️") {
		t.Errorf("output missing ✖️ badge: %q", output)
	}
	if !strings.Contains(output, "unknown tool") {
		t.Errorf("output missing error content: %q", output)
	}
}

// TestFormatTurnErrorProviderIdleTimeout verifies hung provider streams show a concise message.
func TestFormatTurnErrorProviderIdleTimeout(t *testing.T) {
	err := errors.New("provider stream idle timeout after 3m0s with no events")
	got := formatTurnError(err)
	want := "error: provider stream stalled for 3m with no events"
	if got != want {
		t.Fatalf("formatTurnError() = %q, want %q", got, want)
	}
}

// TestFormatTurnErrorProviderHeaderTimeout verifies startup header stalls show a concise message.
func TestFormatTurnErrorProviderHeaderTimeout(t *testing.T) {
	err := errors.New("HTTP request failed: Post \"https://example.com/chat/completions\": net/http: timeout awaiting response headers")
	got := formatTurnError(err)
	want := "error: provider timed out before starting the response"
	if got != want {
		t.Fatalf("formatTurnError() = %q, want %q", got, want)
	}
}

// TestSpinnerStopsBeforeContent verifies the spinner clears before assistant text starts.
func TestSpinnerStopsBeforeContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.spinnerInterval = 5 * time.Millisecond
	c.startSpinner("thinking...")
	time.Sleep(20 * time.Millisecond)
	c.stopSpinner()
	c.lockOutput()
	c.contentStarted = true
	c.ensureLineBreakBeforeBlock()
	fmt.Fprint(c.Out, c.color(colorOrange, c.bold("[BLAZE]")))
	fmt.Fprintln(c.Out)
	c.writeRenderedLine("hello", true)
	c.unlockOutput()

	plain := stripANSICodes(strings.ReplaceAll(out.String(), "\r", ""))
	if c.statusPhase != "Working" {
		t.Fatalf("status phase = %q, want Working", c.statusPhase)
	}
	if !strings.Contains(plain, "[BLAZE]\nhello") {
		t.Fatalf("output missing assistant content after spinner: %q", plain)
	}
}

// TestSpinnerStopsBeforeToolCall verifies the spinner clears before tool activity lines.
func TestSpinnerStopsBeforeToolCall(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.spinnerInterval = 5 * time.Millisecond
	c.startSpinner("thinking...")
	time.Sleep(20 * time.Millisecond)
	c.lockOutput()
	c.stopSpinnerLocked()
	c.setStatusPhaseLocked("Tool", "shell")
	c.ensureLineBreakBeforeBlock()
	c.toolsStarted = true
	c.lastToolArgs = "ls"
	fmt.Fprintf(c.Out, "%s %s …", c.color(colorGreen, toolEmoji("shell")), c.color(colorCtx, "ls"))
	fmt.Fprintln(c.Out, " ✔️")
	c.toolsStarted = false
	c.lastToolArgs = ""
	c.unlockOutput()
	c.stopSpinner()

	plain := stripANSICodes(strings.ReplaceAll(out.String(), "\r", ""))
	if c.statusPhase != "Tool" {
		t.Fatalf("status phase = %q, want Tool", c.statusPhase)
	}
	if !strings.Contains(plain, "💻 ls … ✔️") {
		t.Fatalf("output missing tool line after spinner: %q", plain)
	}
}

// TestOnStreamPhaseUpdatesSpinnerLabel verifies phase notifications change the live spinner text.
func TestOnStreamPhaseUpdatesSpinnerLabel(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.spinnerInterval = 5 * time.Millisecond
	c.startSpinner("Connecting")
	time.Sleep(20 * time.Millisecond)
	c.OnStreamPhase(provider.PhaseWaitingFirstEvent)
	time.Sleep(20 * time.Millisecond)
	c.stopSpinner()
	time.Sleep(5 * time.Millisecond)

	plain := stripANSICodes(strings.ReplaceAll(out.String(), "\r", ""))
	if c.statusPhase != "Wait" {
		t.Fatalf("status phase = %q, want Wait", c.statusPhase)
	}
	if plain != "" {
		t.Fatalf("status-only spinner should not write output: %q", plain)
	}
}

// TestOnToolRoundTripAfterContent verifies content, tool call, and CTX on result line.
func TestOnToolRoundTripAfterContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("hello")
	c.OnUsage(11186, 0, 11186)
	c.OnToolCall("shell", "inspect package.json scripts")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nok\n")
	c.stopSpinner()
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "[BLAZE]\nhello") {
		t.Errorf("content should show [BLAZE] label: %q", plain)
	}
	if !strings.Contains(plain, "💻 inspect package.json scripts … ✔️") {
		t.Errorf("tool response formatting unexpected: %q", plain)
	}
	if !strings.Contains(plain, "✔️  CTX: 11k") {
		t.Errorf("CTX should appear on same line after checkmark: %q", plain)
	}
}

// TestToolGroupConsecutive verifies multiple consecutive tools each show CTX inline.
func TestToolGroupConsecutive(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnUsage(11186, 0, 11186)
	c.OnToolCall("shell", "list root")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\na\n")
	c.OnToolCall("shell", "inspect config")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nb\n")
	c.stopSpinner()
	plain := stripANSICodes(out.String())
	if strings.Contains(plain, "tools ") {
		t.Errorf("expected no tools header, got %q", plain)
	}
	if strings.Count(plain, "CTX: 11k") != 2 {
		t.Errorf("expected CTX after each tool, got %d: %q", strings.Count(plain, "CTX: 11k"), plain)
	}
	if !strings.Contains(plain, "💻 list root … ✔️") {
		t.Errorf("first tool call missing: %q", plain)
	}
	if !strings.Contains(plain, "💻 inspect config … ✔️") {
		t.Errorf("second tool call missing: %q", plain)
	}
}

// TestToolGroupInterruptedByContent verifies content between tools shows [BLAZE] on new line.
func TestToolGroupInterruptedByContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnUsage(11186, 0, 11186)
	c.OnToolCall("shell", "list root")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\na\n")
	c.OnContent("continuing")
	c.OnToolCall("shell", "inspect config")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nb\n")
	c.stopSpinner()
	plain := stripANSICodes(out.String())
	if strings.Contains(plain, "tools ") {
		t.Errorf("expected no tools header, got %q", plain)
	}
	if strings.Count(plain, "CTX: 11k") != 2 {
		t.Errorf("expected CTX after each tool, got %d: %q", strings.Count(plain, "CTX: 11k"), plain)
	}
	if !strings.Contains(plain, "[BLAZE]\ncontinuing") {
		t.Errorf("content between tools should show [BLAZE] label: %q", plain)
	}
	if !strings.Contains(plain, "💻 list root … ✔️") {
		t.Errorf("first tool call missing: %q", plain)
	}
	if !strings.Contains(plain, "💻 inspect config … ✔️") {
		t.Errorf("second tool call missing: %q", plain)
	}
}

// TestToolEmojiMapping verifies dedicated tool emoji assignments.
func TestToolEmojiMapping(t *testing.T) {
	tests := map[string]string{
		"shell":         "💻",
		"task_write":    "📋",
		"task_read":     "📖",
		"load_skill":    "📥",
		"replace_block": "📝",
		"ask_a_friend":  "🤝",
		"analyze_image": "🖼",
		"unknown":       "🔧",
	}
	for name, want := range tests {
		if got := toolEmoji(name); got != want {
			t.Errorf("toolEmoji(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestBuildStatusBarContextBreakdown verifies CTX shows total, cache hit, cache miss, and summary tokens.
func TestBuildStatusBarContextBreakdown(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	c.lastPromptTokens = 12345
	c.lastCachedTokens = 2345
	c.lastUncachedInputTokens = 10000

	plain := stripANSICodes(c.buildStatusBar())
	if !strings.Contains(plain, "CTX 12k(H:2.3k|M:10k|S:0)") {
		t.Fatalf("status bar context = %q, want CTX breakdown", plain)
	}
}

// TestResponseSeparator verifies a completed response gets a plain divider.
func TestResponseSeparator(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("response")
	c.responseSeparator()
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if got := lines[len(lines)-1]; got != strings.Repeat("-", 80) {
		t.Errorf("separator = %q, want 80 hyphens", got)
	}
}

// TestResponseSeparatorWithoutContent verifies empty turns do not get a divider.
func TestResponseSeparatorWithoutContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.responseSeparator()
	if output := out.String(); output != "" {
		t.Errorf("output should be empty without assistant content: %q", output)
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

// TestHandleCommandAuthUsage verifies /auth rejects unsupported providers explicitly.
func TestHandleCommandAuthUsage(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	handled, exit, err := c.handleCommand("/auth")
	if !handled || exit {
		t.Errorf("handled=%v exit=%v, want true/false", handled, exit)
	}
	if err == nil || err.Error() != "usage: /auth openai" {
		t.Fatalf("/auth error = %v, want usage error", err)
	}
}

// TestHandleCommandModelList verifies /model without arg is handled (starts interactive flow).
func TestHandleCommandModelList(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	handled, exit, _ := c.handleCommand("/model")
	if !handled || exit {
		t.Errorf("handled=%v exit=%v, want true/false", handled, exit)
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

// TestHandleCommandClear verifies /clear and /new reset session state in place.
func TestHandleCommandClear(t *testing.T) {
	for _, cmd := range []string{"/clear", "/new"} {
		t.Run(strings.TrimPrefix(cmd, "/"), func(t *testing.T) {
			c, out := newConsole(mockAgent(t))
			if err := c.Agent.Session.Append(session.Message{Role: "user", Content: "old context"}); err != nil {
				t.Fatalf("Append() failed: %v", err)
			}
			summaryDir := filepath.Join(c.Agent.Session.Folder, "summaries")
			if err := os.MkdirAll(summaryDir, 0755); err != nil {
				t.Fatalf("MkdirAll() failed: %v", err)
			}
			if err := os.WriteFile(filepath.Join(summaryDir, "000001.md"), []byte("summary"), 0644); err != nil {
				t.Fatalf("WriteFile() failed: %v", err)
			}
			if err := os.WriteFile(filepath.Join(c.Agent.Session.Folder, "prompt.json"), []byte("debug"), 0644); err != nil {
				t.Fatalf("prompt write failed: %v", err)
			}

			handled, exit, err := c.handleCommand(cmd)
			if err != nil {
				t.Fatalf("%s error: %v", cmd, err)
			}
			if !handled || exit {
				t.Errorf("handled=%v exit=%v, want true/false", handled, exit)
			}
			if len(c.Agent.Session.Messages) != 0 {
				t.Errorf("session has %d messages, want 0", len(c.Agent.Session.Messages))
			}
			if c.Agent.Session.ClosedCleanly {
				t.Error("session should remain open after clear")
			}

			if _, err := os.Stat(summaryDir); !os.IsNotExist(err) {
				t.Errorf("summaries dir still exists: %v", err)
			}
			if _, err := os.Stat(filepath.Join(c.Agent.Session.Folder, "prompt.json")); !os.IsNotExist(err) {
				t.Errorf("prompt.json still exists: %v", err)
			}
			if !strings.Contains(out.String(), "Session cleared.") {
				t.Errorf("output missing confirmation: %q", out.String())
			}
		})
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

// TestReaderHistoryDeduplicatesConsecutiveEntries verifies history storage rules.
func TestReaderHistoryDeduplicatesConsecutiveEntries(t *testing.T) {
	r := NewReader(strings.NewReader(""), false)
	r.AddHistory("first")
	r.AddHistory("first")
	r.AddHistory("second")

	got := r.History()
	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("History() = %#v, want %#v", got, want)
	}
	got[0] = "changed"
	if r.History()[0] != "first" {
		t.Fatal("History() exposed internal storage")
	}
}

// TestReaderHistoryNavigationRestoresDraft verifies Up/Down navigation preserves the draft.
func TestReaderHistoryNavigationRestoresDraft(t *testing.T) {
	r := NewReader(strings.NewReader(""), false)
	r.AddHistory("older")
	r.AddHistory("newer")
	buf := []byte("draft")
	pos := len(buf)

	r.navigateHistory(&buf, &pos, true)
	if string(buf) != "newer" {
		t.Fatalf("first Up = %q, want newer", buf)
	}
	r.navigateHistory(&buf, &pos, true)
	if string(buf) != "older" {
		t.Fatalf("second Up = %q, want older", buf)
	}
	r.navigateHistory(&buf, &pos, false)
	if string(buf) != "newer" {
		t.Fatalf("first Down = %q, want newer", buf)
	}
	r.navigateHistory(&buf, &pos, false)
	if string(buf) != "draft" {
		t.Fatalf("second Down = %q, want draft", buf)
	}
	if r.historyActive {
		t.Fatal("history remains active after draft restoration")
	}
}

// TestDeleteBeforeCursorAcrossNewline verifies backspace can continue into the previous line.
func TestDeleteBeforeCursorAcrossNewline(t *testing.T) {
	buf := []byte("first\nsecond")
	pos := len(buf)

	buf, pos = deleteBeforeCursor(buf, pos)
	if string(buf) != "first\nsecon" || pos != len(buf) {
		t.Fatalf("delete character = %q, pos=%d", buf, pos)
	}
	for i := 0; i < 5; i++ {
		buf, pos = deleteBeforeCursor(buf, pos)
	}
	if string(buf) != "first\n" || pos != len(buf) {
		t.Fatalf("delete second-line text = %q, pos=%d", buf, pos)
	}
	buf, pos = deleteBeforeCursor(buf, pos)
	if string(buf) != "first" || pos != len(buf) {
		t.Fatalf("delete newline = %q, pos=%d", buf, pos)
	}
	buf, pos = deleteBeforeCursor(buf, pos)
	if string(buf) != "firs" || pos != len(buf) {
		t.Fatalf("delete first-line character = %q, pos=%d", buf, pos)
	}
}

// TestPromptLabel verifies the input prompt is a stable lightning marker.
func TestPromptLabel(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	label := stripANSICodes(c.promptLabel())
	if label != "⚡ " {
		t.Errorf("promptLabel() = %q, want lightning marker", label)
	}
	if strings.Contains(label, "default") || strings.Contains(label, c.Agent.ModelID) {
		t.Errorf("promptLabel() = %q, must not contain session state", label)
	}
}

// TestPromptLabelWithAgent verifies agent changes do not alter the input prompt.
func TestPromptLabelWithAgent(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	if got := stripANSICodes(c.promptLabel()); got != "⚡ " {
		t.Errorf("promptLabel() = %q, want stable lightning marker", got)
	}
}

// TestReadEventNonTTY verifies ReadEvent on non-TTY returns an error.
func TestReadEventNonTTY(t *testing.T) {
	r := NewReader(strings.NewReader("hello\n"), false)
	_, _, err := r.ReadEvent()
	if err == nil {
		t.Fatal("ReadEvent() expected error on non-TTY, got nil")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("ReadEvent() error = %v, want terminal-related error", err)
	}
}

// TestReadEventNonTTYEOF verifies ReadEvent on non-TTY returns an error.
func TestReadEventNonTTYEOF(t *testing.T) {
	r := NewReader(strings.NewReader(""), false)
	_, _, err := r.ReadEvent()
	if err == nil {
		t.Error("ReadEvent() expected error on non-TTY, got nil")
	}
}

// writeSkillDir creates a skill folder with skill.md under a skills root.
func writeSkillDir(t *testing.T, root, name, content string) {
	t.Helper()
	skillDir := filepath.Join(root, name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("cannot create skill dir %s: %v", skillDir, err)
	}
	path := filepath.Join(skillDir, "skill.md")
	content = strings.ReplaceAll(content, "[DATA]", "[BODY]")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("cannot write skill %s: %v", path, err)
	}
}

// TestStartupSplashTTY verifies the full splash output in TTY mode with skills.
func TestStartupSplashTTY(t *testing.T) {
	agent := mockAgent(t)
	originalLookup := helpers.DefaultLookup
	helpers.DefaultLookup = func(name string) (string, error) {
		switch name {
		case "rg", "git", "curl":
			return "/usr/bin/" + name, nil
		default:
			return "", errors.New("not found")
		}
	}
	t.Cleanup(func() {
		helpers.DefaultLookup = originalLookup
	})

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	skillsDir := filepath.Join(home, "blazeai", "skills")
	writeSkillDir(t, skillsDir, "music_player", "[DESCRIPTION]\nMusic player skill.\n[DATA]\nk=v\n")
	writeSkillDir(t, skillsDir, "my-network", "[DESCRIPTION]\nNetwork info.\n[DATA]\nip=1.2.3.4\n")

	out := &bytes.Buffer{}
	c := &Console{
		Out:   out,
		Agent: agent,
	}
	c.showStartupSplash()

	output := out.String()
	if !strings.Contains(output, "BlazeAI") {
		t.Error("output missing title")
	}
	if !strings.Contains(output, "blazing-fast AI terminal agent") {
		t.Error("output missing subtitle")
	}
	if !strings.Contains(output, "Commands") {
		t.Error("output missing Commands section")
	}

	// Verify all slash commands are present.
	for _, cmd := range []string{"/auth openai", "/model [model]", "/cd <path>", "/clear", "/new", "/exit"} {
		if !strings.Contains(output, cmd) {
			t.Errorf("output missing command %q", cmd)
		}
	}

	// Verify all shortcut labels are present.
	for _, shortcut := range []string{"Tab", "Ctrl+\\", "Ctrl+F", "Ctrl+R", "ESC", "Ctrl+D"} {
		if !strings.Contains(output, shortcut) {
			t.Errorf("output missing shortcut label %q", shortcut)
		}
	}

	// Verify exactly one "Shortcuts" section header (strip ANSI for robust count).
	plain := stripANSICodes(output)
	if got := strings.Count(plain, "Shortcuts"); got != 1 {
		t.Errorf("output has %d Shortcuts sections, want exactly 1", got)
	}

	// Verify Commands region does not contain Ctrl+ entries.
	cmdIdx := strings.Index(plain, "Commands")
	shortcutsIdx := strings.Index(plain, "Shortcuts")
	if cmdIdx >= 0 && shortcutsIdx >= 0 {
		commandsRegion := plain[cmdIdx:shortcutsIdx]
		if strings.Contains(commandsRegion, "Ctrl+") {
			t.Errorf("Commands region must not contain Ctrl+ labels: %q", commandsRegion)
		}
	}

	if !strings.Contains(output, "Skills") {
		t.Error("output missing Skills section")
	}
	if !strings.Contains(output, "music_player") {
		t.Error("output missing music_player skill")
	}
	if !strings.Contains(output, "my-network") {
		t.Error("output missing my-network skill")
	}
	if strings.Contains(output, "global/") {
		t.Error("output contains global/ prefix on skill names")
	}
	if !strings.Contains(output, "Helpers") {
		t.Error("output missing Helpers section")
	}
	if !strings.Contains(output, "rg") {
		t.Error("output missing rg helper")
	}
	if !strings.Contains(output, "git") {
		t.Error("output missing git helper")
	}
	if strings.Contains(output, "fd") {
		t.Error("output should not include unavailable helper fd")
	}
	if strings.Index(output, "Skills") > strings.Index(output, "Helpers") {
		t.Error("Helpers section should appear after Skills")
	}
	if strings.Index(output, "Helpers") > strings.Index(output, "Session") {
		t.Error("Helpers section should appear before Session")
	}

	// Verify full section order: Commands < Shortcuts < Skills < Helpers < Session.
	sections := []string{"Commands", "Shortcuts", "Skills", "Helpers", "Session"}
	for i := 0; i < len(sections)-1; i++ {
		a := strings.Index(plain, sections[i])
		b := strings.Index(plain, sections[i+1])
		if a < 0 {
			t.Errorf("section %q not found", sections[i])
			continue
		}
		if b < 0 {
			t.Errorf("section %q not found", sections[i+1])
			continue
		}
		if a > b {
			t.Errorf("section order: %q (pos %d) appears after %q (pos %d)", sections[i], a, sections[i+1], b)
		}
	}

	if !strings.Contains(output, "Model") {
		t.Error("output missing Model line")
	}
	if !strings.Contains(output, "Folder") {
		t.Error("output missing Folder line")
	}
	if !strings.Contains(output, "Session") {
		t.Error("output missing Session section")
	}
}

// TestStartupSplashSkillsEmpty verifies splash shows (none) when no skills exist.
func TestStartupSplashSkillsEmpty(t *testing.T) {
	agent := mockAgent(t)
	// mockAgent sets HOME to a temp dir with no blazeai/skills/.
	// DiscoverAll returns empty map, not an error.
	out := &bytes.Buffer{}
	c := &Console{
		Out:   out,
		Agent: agent,
	}
	c.showStartupSplash()

	output := out.String()
	if !strings.Contains(output, "(none)") {
		t.Errorf("expected (none) for empty skills, got: %q", output)
	}
}

// TestReadLineSafelyConvertsPanic verifies readline panics become diagnostic errors.
func TestReadLineSafelyConvertsPanic(t *testing.T) {
	_, err := readLineSafely(nil)
	if err == nil {
		t.Fatal("expected readline panic to become an error")
	}
	if !strings.Contains(err.Error(), "readline panic") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "readLineSafely") {
		t.Fatalf("error does not include a useful stack trace: %v", err)
	}
}

// TestOnAgentActivityResultShowsCTX verifies child tool_result lines display
// CTX when LastPromptTokens is set, matching main-runtime tool result behavior.
// WHAT: CTX suffix must appear on DONE tool_result lines with token data.
// HOW: Sends tool_call then tool_result with LastPromptTokens and checks output.
func TestOnAgentActivityResultShowsCTX(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnAgentActivity(runtime.AgentActivity{
		Agent:            "[code_abc]",
		Kind:             "tool_call",
		Tool:             "shell",
		Status:           "running",
		Text:             "list files",
		LastPromptTokens: 0,
	})
	c.OnAgentActivity(runtime.AgentActivity{
		Agent:            "[code_abc]",
		Kind:             "tool_result",
		Tool:             "shell",
		Status:           "ok",
		Text:             "ok",
		LastPromptTokens: 12500,
	})
	plain := stripANSICodes(out.String())
	// formatCompactInt(12500) = "12k" (>=10000 range)
	if !strings.Contains(plain, "CTX: 12k") {
		t.Errorf("output missing CTX on agent tool_result: %q", plain)
	}
	if !strings.Contains(plain, "✔️") {
		t.Errorf("output missing checkmark on agent tool_result: %q", plain)
	}
}

// TestOnAgentActivityResultNoCTXWhenZeroTokens verifies that CTX is not shown
// when LastPromptTokens is zero (no usage data available).
// WHAT: No CTX suffix when token count is zero.
// HOW: Sends tool_result with zero tokens and checks output lacks CTX.
func TestOnAgentActivityResultNoCTXWhenZeroTokens(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnAgentActivity(runtime.AgentActivity{
		Agent:            "[code_abc]",
		Kind:             "tool_call",
		Tool:             "shell",
		Status:           "running",
		Text:             "list files",
		LastPromptTokens: 0,
	})
	c.OnAgentActivity(runtime.AgentActivity{
		Agent:            "[code_abc]",
		Kind:             "tool_result",
		Tool:             "shell",
		Status:           "ok",
		Text:             "ok",
		LastPromptTokens: 0,
	})
	plain := stripANSICodes(out.String())
	if strings.Contains(plain, "CTX:") {
		t.Errorf("output should not contain CTX when tokens are zero: %q", plain)
	}
}

// TestOnAgentActivityResultErrorNoCTX verifies that ERROR tool_result lines
// display the error content without CTX, matching main-runtime error formatting.
// WHAT: Error badge with content, no CTX suffix.
// HOW: Sends error tool_result and checks for error content without CTX.
func TestOnAgentActivityResultErrorNoCTX(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnAgentActivity(runtime.AgentActivity{
		Agent:            "[code_abc]",
		Kind:             "tool_call",
		Tool:             "shell",
		Status:           "running",
		Text:             "run command",
		LastPromptTokens: 0,
	})
	c.OnAgentActivity(runtime.AgentActivity{
		Agent:            "[code_abc]",
		Kind:             "tool_result",
		Tool:             "shell",
		Status:           "ok",
		Text:             "error: command failed",
		LastPromptTokens: 10000,
	})
	plain := stripANSICodes(out.String())
	// parseToolResult returns badge="ERROR" which renders as literal text.
	if !strings.Contains(plain, "command failed") {
		t.Errorf("output missing error content: %q", plain)
	}
	if strings.Contains(plain, "CTX:") {
		t.Errorf("error output should not contain CTX: %q", plain)
	}
}

// TestOnAgentActivityResultStandaloneCTX verifies CTX shows when tool_result
// arrives without a matching prior tool_call (e.g. after clear).
// WHAT: Standalone tool_result with CTX renders correctly.
// HOW: Sends only tool_result without preceding tool_call.
func TestOnAgentActivityResultStandaloneCTX(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnAgentActivity(runtime.AgentActivity{
		Agent:            "[code_abc]",
		Kind:             "tool_result",
		Tool:             "shell",
		Status:           "ok",
		Text:             "ok",
		LastPromptTokens: 8500,
	})
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "CTX: 8.5k") {
		t.Errorf("standalone tool_result missing CTX: %q", plain)
	}
	if !strings.Contains(plain, "✔️") {
		t.Errorf("standalone tool_result missing checkmark: %q", plain)
	}
}

// TestHandleCommandModeSet verifies /mode with a valid agent name selects
// that agent, updates the status bar, and does not submit a turn.
//
// WHAT: /mode <name> selects the named interactive agent.
// HOW: Calls handleCommand with a valid interactive agent name, checks
// handled flag, no exit, no error, and that CurrentAgent changed.
func TestHandleCommandModeSet(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	// Start on "default" (first interactive agent).
	if c.Agent.CurrentAgent == nil || c.Agent.CurrentAgent.Name != "default" {
		t.Fatalf("initial agent = %v, want 'default'", c.Agent.CurrentAgent)
	}
	handled, exit, err := c.handleCommand("/mode planning")
	if err != nil {
		t.Fatalf("/mode planning error: %v", err)
	}
	if !handled {
		t.Error("/mode not handled")
	}
	if exit {
		t.Error("/mode should not exit")
	}
	if c.Agent.CurrentAgent == nil || c.Agent.CurrentAgent.Name != "planning" {
		t.Errorf("CurrentAgent = %v, want 'planning'", c.Agent.CurrentAgent)
	}
	if !strings.Contains(out.String(), "Agent set to: planning") {
		t.Errorf("output missing confirmation: %q", out.String())
	}
}

// TestHandleCommandModeRequiresName verifies /mode without an argument returns
// a usage error and does not change CurrentAgent.
//
// WHAT: /mode alone is a usage error.
// HOW: Calls handleCommand with just "/mode", checks error and no agent change.
func TestHandleCommandModeRequiresName(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	originalAgent := c.Agent.CurrentAgent
	_, _, err := c.handleCommand("/mode")
	if err == nil {
		t.Fatal("/mode without arg expected error, got nil")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("error = %v, want usage error", err)
	}
	if c.Agent.CurrentAgent != originalAgent {
		t.Error("CurrentAgent should not change on usage error")
	}
}

// TestHandleCommandModeRejectsExecutor verifies /mode with an executor agent name
// returns an error and leaves the current agent unchanged.
//
// WHAT: /mode rejects non-interactive (executor) agent names.
// HOW: Calls handleCommand with "worker" (an executor), checks error and no change.
func TestHandleCommandModeRejectsExecutor(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	originalAgent := c.Agent.CurrentAgent
	_, _, err := c.handleCommand("/mode worker")
	if err == nil {
		t.Fatal("/mode with executor expected error, got nil")
	}
	if c.Agent.CurrentAgent != originalAgent {
		t.Error("CurrentAgent should not change when selecting executor")
	}
}

// TestTabCyclesInteractiveAgents verifies the Tab key action cycles through
// interactive agents with wrap-around, skipping executor definitions.
//
// WHAT: Tab cycles InteractiveDefs with wrap-around.
// HOW: Calls the Tab action callback twice and asserts wrap-around through
// interactive agents only (executor "worker" is skipped).
func TestTabCyclesInteractiveAgents(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	// Start on "default" (first interactive agent).
	if c.Agent.CurrentAgent == nil || c.Agent.CurrentAgent.Name != "default" {
		t.Fatalf("initial agent = %v, want 'default'", c.Agent.CurrentAgent)
	}
	// First Tab: default → planning (next interactive).
	if _, err := c.Agent.NextAgent(); err != nil {
		t.Fatalf("first Tab error: %v", err)
	}
	if c.Agent.CurrentAgent == nil || c.Agent.CurrentAgent.Name != "planning" {
		t.Errorf("after first Tab: CurrentAgent = %v, want 'planning'", c.Agent.CurrentAgent)
	}
	// Second Tab: planning → default (wrap-around).
	if _, err := c.Agent.NextAgent(); err != nil {
		t.Fatalf("second Tab error: %v", err)
	}
	if c.Agent.CurrentAgent == nil || c.Agent.CurrentAgent.Name != "default" {
		t.Errorf("after second Tab: CurrentAgent = %v, want 'default'", c.Agent.CurrentAgent)
	}
}

// TestStartupSplashAgentTerminology verifies the startup splash uses agent
// terminology for Tab shortcut and includes the /mode command.
//
// WHAT: Splash output reflects agent cycling and /mode command.
// HOW: Calls showStartupSplash and checks for agent-specific text.
func TestStartupSplashAgentTerminology(t *testing.T) {
	agent := mockAgent(t)
	out := &bytes.Buffer{}
	c := &Console{
		Out:   out,
		Agent: agent,
	}
	c.showStartupSplash()

	output := out.String()
	if !strings.Contains(output, "/mode [agent]") {
		t.Errorf("output missing /mode command: %q", output)
	}
	if !strings.Contains(output, "cycle interactive agent") {
		t.Errorf("output missing agent cycling description: %q", output)
	}
}
