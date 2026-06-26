// handler.go — Telegram runtime.Handler implementation.
// Buffers assistant streaming output, flushes it on a fixed interval, and maps the
// growing text into Telegram send/edit operations with message splitting.
// Layer: transport output. Dependencies: internal/runtime.
package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	messageFlushInterval = 500 * time.Millisecond
	maxTelegramTextSize  = 3500
)

type messenger interface {
	SendMessage(ctx context.Context, chatID int64, text string) (int, error)
	EditMessage(ctx context.Context, chatID int64, messageID int, text string) error
}

// Handler adapts runtime streaming callbacks to Telegram messages.
//
// WHAT:  Buffers assistant output and periodically mirrors it into Telegram.
// WHY:   Telegram cannot safely render token-by-token terminal output.
// PARAMS: client — Telegram API sender; chatID — allowed chat target.
type Handler struct {
	client messenger
	chatID int64

	mu         sync.Mutex
	ctx        context.Context
	content    strings.Builder
	sentIDs    []int
	sentTexts  []string
	active     bool
	stopFlush  chan struct{}
	flushDone  chan struct{}
	lastTokens int
	lastErr    error
}

// NewHandler creates a Telegram output handler for one chat.
func NewHandler(client messenger, chatID int64) *Handler {
	return &Handler{client: client, chatID: chatID}
}

// BeginTurn resets the streaming state and starts periodic flushing.
func (h *Handler) BeginTurn(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ctx = ctx
	h.content.Reset()
	h.sentIDs = nil
	h.sentTexts = nil
	h.active = true
	h.lastErr = nil
	h.stopFlush = make(chan struct{})
	h.flushDone = make(chan struct{})
	go h.flushLoop()
}

// FinishTurn stops periodic flushing and sends the final buffered content.
func (h *Handler) FinishTurn() error {
	h.mu.Lock()
	if !h.active {
		err := h.lastErr
		h.mu.Unlock()
		return err
	}
	close(h.stopFlush)
	done := h.flushDone
	h.active = false
	h.mu.Unlock()
	<-done
	h.flushNow()
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastErr
}

// OnUsage records prompt token usage from the last provider response.
func (h *Handler) OnUsage(promptTokens int) {
	h.mu.Lock()
	h.lastTokens = promptTokens
	h.mu.Unlock()
}

// OnContent appends a streamed text delta to the Telegram buffer.
func (h *Handler) OnContent(delta string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.active {
		return
	}
	h.content.WriteString(delta)
}

// OnToolCall sends a short standalone notice for tool execution.
func (h *Handler) OnToolCall(name string, args string) {
	h.sendNotice(fmt.Sprintf("[tool] %s %s", name, strings.TrimSpace(args)))
}

// OnToolResult sends a short standalone notice for the tool result.
func (h *Handler) OnToolResult(name string, result string) {
	text := strings.TrimSpace(result)
	if len(text) > 300 {
		text = text[:300] + "..."
	}
	h.sendNotice(fmt.Sprintf("[tool result] %s\n%s", name, text))
}

func (h *Handler) flushLoop() {
	ticker := time.NewTicker(messageFlushInterval)
	defer ticker.Stop()
	defer close(h.flushDone)
	for {
		select {
		case <-ticker.C:
			h.flushNow()
		case <-h.stopFlush:
			return
		}
	}
}

func (h *Handler) flushNow() {
	h.mu.Lock()
	if h.client == nil {
		h.setErrLocked(fmt.Errorf("telegram handler client is nil"))
		h.mu.Unlock()
		return
	}
	ctx := h.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	text := h.content.String()
	if text == "" {
		h.mu.Unlock()
		return
	}
	chunks := splitTelegramText(text, maxTelegramTextSize)
	sentIDs := append([]int(nil), h.sentIDs...)
	sentTexts := append([]string(nil), h.sentTexts...)
	h.mu.Unlock()

	for i, chunk := range chunks {
		if i < len(sentIDs) {
			if sentTexts[i] == chunk {
				continue
			}
			if err := h.client.EditMessage(ctx, h.chatID, sentIDs[i], chunk); err != nil {
				h.sendNotice(fmt.Sprintf("telegram edit failed for message %d: %v", sentIDs[i], err))
				messageID, sendErr := h.client.SendMessage(ctx, h.chatID, chunk)
				if sendErr != nil {
					h.mu.Lock()
					h.setErrLocked(fmt.Errorf("cannot send telegram message: %w", sendErr))
					h.mu.Unlock()
					return
				}
				sentIDs[i] = messageID
				sentTexts[i] = chunk
				continue
			}
			sentTexts[i] = chunk
			continue
		}
		messageID, err := h.client.SendMessage(ctx, h.chatID, chunk)
		if err != nil {
			h.mu.Lock()
			h.setErrLocked(fmt.Errorf("cannot send telegram message: %w", err))
			h.mu.Unlock()
			return
		}
		sentIDs = append(sentIDs, messageID)
		sentTexts = append(sentTexts, chunk)
	}

	h.mu.Lock()
	h.sentIDs = sentIDs
	h.sentTexts = sentTexts
	h.mu.Unlock()
}

func (h *Handler) sendNotice(text string) {
	h.mu.Lock()
	client := h.client
	chatID := h.chatID
	ctx := h.ctx
	h.mu.Unlock()
	if client == nil {
		h.mu.Lock()
		h.setErrLocked(fmt.Errorf("telegram handler client is nil"))
		h.mu.Unlock()
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := client.SendMessage(ctx, chatID, text); err != nil {
		h.mu.Lock()
		h.setErrLocked(fmt.Errorf("cannot send telegram notice: %w", err))
		h.mu.Unlock()
	}
}

func (h *Handler) setErrLocked(err error) {
	if h.lastErr == nil {
		h.lastErr = err
	}
}

func splitTelegramText(text string, limit int) []string {
	if limit <= 0 || len(text) <= limit {
		return []string{text}
	}
	parts := make([]string, 0, (len(text)/limit)+1)
	remaining := text
	for len(remaining) > limit {
		splitAt := strings.LastIndex(remaining[:limit], "\n")
		if splitAt <= 0 {
			splitAt = limit
		}
		parts = append(parts, remaining[:splitAt])
		remaining = strings.TrimLeft(remaining[splitAt:], "\n")
	}
	if remaining != "" {
		parts = append(parts, remaining)
	}
	return parts
}
