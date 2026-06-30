// handler_test.go — tests for Telegram output buffering and message splitting.
// Uses a mock messenger to verify send/edit behavior without Telegram API calls.
package telegram

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type mockMessenger struct {
	nextID  int
	sent    []string
	edits   []string
	ids     []int
	actions []string
}

func (m *mockMessenger) SendMessage(_ context.Context, _ int64, text string) (int, error) {
	m.nextID++
	m.sent = append(m.sent, text)
	m.ids = append(m.ids, m.nextID)
	return m.nextID, nil
}

func (m *mockMessenger) EditMessage(_ context.Context, _ int64, messageID int, text string) error {
	m.edits = append(m.edits, fmt.Sprintf("%d:%s", messageID, text))
	return nil
}

func (m *mockMessenger) SendChatAction(_ context.Context, _ int64, action string) error {
	m.actions = append(m.actions, action)
	return nil
}

func TestHandlerFinishTurnSendsBufferedContent(t *testing.T) {
	m := &mockMessenger{}
	h := NewHandler(m, 42)
	h.BeginTurn(context.Background())
	h.OnContent("Hello")
	h.OnContent(" world")
	if err := h.FinishTurn(); err != nil {
		t.Fatalf("FinishTurn() error: %v", err)
	}
	if len(m.sent) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(m.sent))
	}
	if m.sent[0] != "Hello world" {
		t.Fatalf("sent text = %q", m.sent[0])
	}
	if len(m.actions) == 0 || m.actions[0] != "typing" {
		t.Fatalf("typing actions = %v, want first action typing", m.actions)
	}
}

func TestSplitTelegramTextSplitsLongContent(t *testing.T) {
	text := strings.Repeat("a", maxTelegramTextSize+10)
	parts := splitTelegramText(text, maxTelegramTextSize)
	if len(parts) != 2 {
		t.Fatalf("split parts = %d, want 2", len(parts))
	}
	if len(parts[0]) > maxTelegramTextSize || len(parts[1]) > maxTelegramTextSize {
		t.Fatal("split chunk exceeds limit")
	}
}

func TestToolActivityUsesSingleEditableBubble(t *testing.T) {
	m := &mockMessenger{}
	h := NewHandler(m, 42)
	h.BeginTurn(context.Background())
	h.OnToolCall("shell", "Check signal-cli installation and if it is usable")
	h.OnToolResult("shell", "exit_code: 0\nstdout:\nok\n")
	if err := h.FinishTurn(); err != nil {
		t.Fatalf("FinishTurn() error: %v", err)
	}
	if len(m.sent) != 1 {
		t.Fatalf("sent messages = %d, want 1 activity bubble", len(m.sent))
	}
	if !strings.HasPrefix(m.sent[0], "🛠 Activity\n💻 Check signal-cli installation") {
		t.Fatalf("activity bubble = %q", m.sent[0])
	}
	if len(m.edits) != 1 {
		t.Fatalf("edits = %d, want 1 completion edit", len(m.edits))
	}
	if !strings.Contains(m.edits[0], "✅") {
		t.Fatalf("completion edit = %q, want success badge", m.edits[0])
	}
}

func TestContentAfterToolUsesNewLowerBubble(t *testing.T) {
	m := &mockMessenger{}
	h := NewHandler(m, 42)
	h.BeginTurn(context.Background())
	h.OnContent("Initial answer")
	h.flushNow()
	h.OnToolCall("shell", "Run environment check")
	h.OnToolResult("shell", "exit_code: 0\nstdout:\nok\n")
	h.OnContent("Final answer")
	if err := h.FinishTurn(); err != nil {
		t.Fatalf("FinishTurn() error: %v", err)
	}
	if len(m.sent) != 3 {
		t.Fatalf("sent messages = %d, want 3", len(m.sent))
	}
	if m.sent[0] != "Initial answer" {
		t.Fatalf("first message = %q, want initial answer", m.sent[0])
	}
	if !strings.HasPrefix(m.sent[1], "🛠 Activity\n💻 Run environment check...") {
		t.Fatalf("second message = %q, want activity bubble", m.sent[1])
	}
	if m.sent[2] != "Final answer" {
		t.Fatalf("third message = %q, want final answer", m.sent[2])
	}
	for _, edit := range m.edits {
		if strings.HasPrefix(edit, "1:") {
			t.Fatalf("initial response was edited after tool activity: %q", edit)
		}
	}
}

func TestToolActivitySummarizesErrors(t *testing.T) {
	m := &mockMessenger{}
	h := NewHandler(m, 42)
	h.BeginTurn(context.Background())
	h.OnToolCall("shell", "Run failing command")
	h.OnToolResult("shell", "exit_code: 1\nstdout:\n\nstderr:\nvery detailed failure output that should be summarized for telegram display\n")
	if err := h.FinishTurn(); err != nil {
		t.Fatalf("FinishTurn() error: %v", err)
	}
	if len(m.edits) != 1 {
		t.Fatalf("edits = %d, want 1", len(m.edits))
	}
	if !strings.Contains(m.edits[0], "❌") {
		t.Fatalf("error edit = %q, want error badge", m.edits[0])
	}
	if strings.Contains(m.edits[0], "stdout:") || strings.Contains(m.edits[0], "stderr:") {
		t.Fatalf("error edit leaked raw tool sections: %q", m.edits[0])
	}
	if !strings.Contains(m.edits[0], "very detailed failure output") {
		t.Fatalf("error edit missing summarized detail: %q", m.edits[0])
	}
}

// TestToolEmojiMapping verifies Telegram tool emoji assignments match console output.
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
