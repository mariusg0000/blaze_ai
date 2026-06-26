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
