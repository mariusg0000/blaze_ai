// console_test.go — tests for console handler, command parsing, and TTY detection.
package console

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"blazeai/internal/config"
	"blazeai/internal/helpers"
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
	writePromptFixtures(t, promptsDir)

	agent, err := runtime.NewAgent(cfg, sess, platform.Linux, os.DirFS(promptsDir), dir, &mockHandler{}, "console")
	if err != nil {
		t.Fatalf("NewAgent() error: %v", err)
	}
	return agent
}

// mockHandler is a no-op handler for agent construction.
type mockHandler struct{}

func (h *mockHandler) OnContent(delta string)                            {}
func (h *mockHandler) OnToolCall(name string, args string)               {}
func (h *mockHandler) OnToolResult(name string, result string)           {}
func (h *mockHandler) OnUsage(promptTokens int)                          {}
func (h *mockHandler) OnReasoning(delta string)                          {}
func (h *mockHandler) RequestSudoApproval(command string) (bool, string) { return false, "" }

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

// writePromptFixtures creates the prompt templates required by runtime prompt assembly.
func writePromptFixtures(t *testing.T, promptsDir string) {
	t.Helper()
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.md"), []byte("# Universal System Prompt\n\nApp home is at {APP_HOME}.\nUnknown var: {UNKNOWN_VAR}.\n\n## Tool Discipline\n- Keep relevant loaded skills active across follow-up turns on the same topic or task.\n- Do not unload a skill immediately after one successful action if the user is likely to continue in the same domain.\n- Unload a skill only when the user clearly changes topic or task, or when the loaded skill would interfere with the next turn.\n\n## Active State Rules\n- Only skills listed under `## Active Skills` are active right now. Do not infer current active skills from older `load_skill` or `unload_skill` tool results in the conversation history. If there is no `## Active Skills` section below, then no skills are currently active.\n- Only memories listed under `## Active Memories` are active right now. Do not infer current active memories from older `load_memory` or `unload_memory` tool results in the conversation history. If there is no `## Active Memories` section below, then no memories are currently active.\n\n{OS_PROMPT}\n\n## Transport\n{TRANSPORT_PROMPT}\n\n{TRANSPORT_CONTEXT}\n\n## Host Environment Helpers\nAvailable helpers:\n{HOST_HELPERS_AVAILABLE}\n\nOptional helpers:\n{HOST_HELPERS_OPTIONAL}\n\n## Skills\nBefore performing any task, scan available skill descriptions. If a domain or system mentioned in the request appears in a skill's description, you MUST load that skill first. Do not act on an unfamiliar domain without loading the relevant skill.\n\nAvailable skills:\n{SKILLS_AVAILABLE}\n\nActive skills:\n{SKILLS_ACTIVE}\n\n{RUNNABLE_SKILLS_SECTION}\n\n## Memories\nAvailable memories:\n{MEMORIES_AVAILABLE}\n\nActive memories:\n{MEMORIES_ACTIVE}\n\n## Project Rules (AGENTS.md)\n{AGENTS_CONTENT}\n"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "sysprompt.linux.md"), []byte("linux"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "transport.console.md"), []byte("console transport"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "transport.telegram.md"), []byte("telegram transport"), 0644)
	os.WriteFile(filepath.Join(promptsDir, "transport.web.md"), []byte("web transport"), 0644)
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
	output := out.String()
	if !strings.Contains(output, "💻") {
		t.Errorf("output missing wrench icon: %q", output)
	}
}

// TestOnToolCallAfterContent verifies a newline is inserted before tool blocks.
func TestOnToolCallAfterContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("hello")
	c.OnToolCall("shell", "ls")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nok\n")
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "hello\n💻 ls …") {
		t.Errorf("output missing newline before tool call: %q", out.String())
	}
}

// TestOnToolResultSuccess verifies successful tool results append a checkmark.
func TestOnToolResultSuccess(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "inspect package.json scripts")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nhi\n")
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "💻 inspect package.json scripts … ✔️") {
		t.Errorf("output missing appended success line: %q", plain)
	}
	if strings.Contains(plain, "exit_code") {
		t.Errorf("output should not contain raw exit_code: %q", plain)
	}
}

// TestOnToolResultSuccessTTY verifies the status is appended to the tool line.
func TestOnToolResultSuccessTTY(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnToolCall("shell", "inspect package.json scripts")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nhi\n")
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
	output := out.String()
	if !strings.Contains(output, "✖️") {
		t.Errorf("output missing ✖️ badge: %q", output)
	}
	if !strings.Contains(output, "unknown tool") {
		t.Errorf("output missing error content: %q", output)
	}
}

// TestOnToolRoundTripAfterContent verifies content, tool call, and CTX on result line.
func TestOnToolRoundTripAfterContent(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnContent("hello")
	c.OnUsage(11186)
	c.OnToolCall("shell", "inspect package.json scripts")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nok\n")
	plain := stripANSICodes(out.String())
	if !strings.Contains(plain, "[BLAZE] hello") {
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
	c.OnUsage(11186)
	c.OnToolCall("shell", "list root")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\na\n")
	c.OnToolCall("shell", "inspect config")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nb\n")
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
	c.OnUsage(11186)
	c.OnToolCall("shell", "list root")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\na\n")
	c.OnContent("continuing")
	c.OnToolCall("shell", "inspect config")
	c.OnToolResult("shell", "exit_code: 0\nstdout:\nb\n")
	plain := stripANSICodes(out.String())
	if strings.Contains(plain, "tools ") {
		t.Errorf("expected no tools header, got %q", plain)
	}
	if strings.Count(plain, "CTX: 11k") != 2 {
		t.Errorf("expected CTX after each tool, got %d: %q", strings.Count(plain, "CTX: 11k"), plain)
	}
	if !strings.Contains(plain, "[BLAZE] continuing") {
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
		"unload_skill":  "📤",
		"replace_block": "📝",
		"run_skill":     "🚀",
		"ask_a_friend":  "🧠",
		"analyze_image": "🖼",
		"unknown":       "🔧",
	}
	for name, want := range tests {
		if got := toolEmoji(name); got != want {
			t.Errorf("toolEmoji(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestOnUsage verifies context usage is stored and rendered in the separator.
func TestOnUsage(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnUsage(11186)
	c.responseSeparator()
	output := out.String()
	if !strings.Contains(output, "CTX: 11k") {
		t.Errorf("output missing context size: %q", output)
	}
	if !strings.Contains(output, "test/test-model") {
		t.Errorf("output missing model: %q", output)
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
			c.Agent.Active.Load("music_player")
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
			if len(c.Agent.Active.List()) != 0 {
				t.Errorf("active skills = %v, want empty", c.Agent.Active.List())
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

// TestPromptLabelWithMode verifies prompt label shows [<mode> mode]> format.
func TestPromptLabelWithMode(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.Agent.Modes = &config.ModesConfig{
		Modes: []config.Mode{
			{Name: "default", Model: "test/test-model"},
			{Name: "planning", Model: "test/test-model"},
		},
	}
	c.Agent.CurrentMode = &c.Agent.Modes.Modes[1]
	label := c.promptLabel()
	if !strings.Contains(out.String(), "") {
		// promptLabel returns a string, doesn't write to out
	}
	if !strings.Contains(label, "[planning mode]") {
		t.Errorf("promptLabel() = %q, want [planning mode]>", label)
	}
}

// TestPromptLabelWithoutMode verifies prompt label defaults to [default mode]> when no mode.
func TestPromptLabelWithoutMode(t *testing.T) {
	c, _ := newConsole(mockAgent(t))
	c.Agent.CurrentMode = nil
	label := c.promptLabel()
	if !strings.Contains(label, "[default mode]") {
		t.Errorf("promptLabel() = %q, want [default mode]>", label)
	}
	if strings.Contains(label, "USER") {
		t.Errorf("promptLabel() = %q, should not contain USER", label)
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
	if !strings.Contains(output, "/model [model]") {
		t.Error("output missing /model command")
	}
	if !strings.Contains(output, "/cd <path>") {
		t.Error("output missing /cd command")
	}
	if !strings.Contains(output, "/clear") {
		t.Error("output missing /clear command")
	}
	if !strings.Contains(output, "/new") {
		t.Error("output missing /new command")
	}
	if !strings.Contains(output, "/exit") {
		t.Error("output missing /exit command")
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
