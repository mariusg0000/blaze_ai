// handler.go — Telegram runtime.Handler implementation.
// Buffers assistant streaming output, flushes it on a fixed interval, and maps the
// growing text into Telegram send/edit operations with message splitting.
// Layer: transport output. Dependencies: internal/runtime.
package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	messageFlushInterval = 500 * time.Millisecond
	maxTelegramTextSize  = 3500
)

var typingActionInterval = 4 * time.Second

type messenger interface {
	SendMessage(ctx context.Context, chatID int64, text string) (int, error)
	EditMessage(ctx context.Context, chatID int64, messageID int, text string) error
	SendChatAction(ctx context.Context, chatID int64, action string) error
}

// Handler adapts runtime streaming callbacks to Telegram messages.
//
// WHAT:  Buffers assistant output and periodically mirrors it into Telegram.
// WHY:   Telegram cannot safely render token-by-token terminal output.
// PARAMS: client — Telegram API sender; chatID — allowed chat target.
type Handler struct {
	client messenger
	chatID int64

	mu                sync.Mutex
	ctx               context.Context
	content           strings.Builder
	contentStart      int
	sentIDs           []int
	sentTexts         []string
	activityLines     []string
	activityPending   []int
	activityMessageID int
	activityText      string
	active            bool
	stopFlush         chan struct{}
	flushDone         chan struct{}
	lastTokens        int
	lastErr           error
}

// NewHandler creates a Telegram output handler for one chat.
func NewHandler(client messenger, chatID int64) *Handler {
	return &Handler{client: client, chatID: chatID}
}

// BeginTurn resets the streaming state and starts periodic flushing.
func (h *Handler) BeginTurn(ctx context.Context) {
	h.mu.Lock()
	h.ctx = ctx
	h.content.Reset()
	h.contentStart = 0
	h.sentIDs = nil
	h.sentTexts = nil
	h.activityLines = nil
	h.activityPending = nil
	h.activityMessageID = 0
	h.activityText = ""
	h.active = true
	h.lastErr = nil
	h.stopFlush = make(chan struct{})
	h.flushDone = make(chan struct{})
	h.mu.Unlock()
	h.sendTypingNow()
	go h.flushLoop()
	go h.typingLoop()
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

// OnReasoning is a no-op for Telegram transport (reasoning not displayed via Telegram).
func (h *Handler) OnReasoning(delta string) {
	// No-op — Telegram does not display reasoning blocks.
}

// RequestSudoApproval is not supported in the Telegram transport.
// Sudo commands are not allowed via Telegram for security.
func (h *Handler) RequestSudoApproval(command string) (bool, string) {
	return false, ""
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
	h.flushNow()

	h.mu.Lock()
	if !h.active {
		h.mu.Unlock()
		return
	}
	h.contentStart = h.content.Len()
	h.sentIDs = nil
	h.sentTexts = nil
	line := formatPendingToolLine(name, args)
	h.activityLines = append(h.activityLines, line)
	h.activityPending = append(h.activityPending, len(h.activityLines)-1)
	h.mu.Unlock()

	h.flushActivityNow()
}

// OnToolResult sends a short standalone notice for the tool result.
func (h *Handler) OnToolResult(name string, result string) {
	badge, detail := parseTelegramToolResult(result)

	h.mu.Lock()
	if len(h.activityPending) == 0 {
		h.activityLines = append(h.activityLines, formatCompletedToolLine(name, "", badge, detail))
	} else {
		lineIndex := h.activityPending[0]
		h.activityPending = h.activityPending[1:]
		args := extractToolArgsFromPendingLine(h.activityLines[lineIndex], name)
		h.activityLines[lineIndex] = formatCompletedToolLine(name, args, badge, detail)
	}
	h.mu.Unlock()

	h.flushActivityNow()
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

func (h *Handler) typingLoop() {
	ticker := time.NewTicker(typingActionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.sendTypingNow()
		case <-h.stopFlush:
			return
		}
	}
}

func (h *Handler) sendTypingNow() {
	h.mu.Lock()
	client := h.client
	chatID := h.chatID
	ctx := h.ctx
	active := h.active
	h.mu.Unlock()
	if !active || client == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_ = client.SendChatAction(ctx, chatID, "typing")
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
	if h.contentStart > len(text) {
		h.contentStart = len(text)
	}
	text = text[h.contentStart:]
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

func (h *Handler) flushActivityNow() {
	h.mu.Lock()
	client := h.client
	chatID := h.chatID
	ctx := h.ctx
	text := buildActivityMessage(h.activityLines)
	messageID := h.activityMessageID
	lastText := h.activityText
	h.mu.Unlock()

	if text == "" {
		return
	}
	if client == nil {
		h.mu.Lock()
		h.setErrLocked(fmt.Errorf("telegram handler client is nil"))
		h.mu.Unlock()
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if messageID != 0 {
		if lastText == text {
			return
		}
		if err := client.EditMessage(ctx, chatID, messageID, text); err == nil {
			h.mu.Lock()
			h.activityText = text
			h.mu.Unlock()
			return
		}
	}
	newMessageID, err := client.SendMessage(ctx, chatID, text)
	if err != nil {
		h.mu.Lock()
		h.setErrLocked(fmt.Errorf("cannot send telegram activity message: %w", err))
		h.mu.Unlock()
		return
	}
	h.mu.Lock()
	h.activityMessageID = newMessageID
	h.activityText = text
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

func buildActivityMessage(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return "🛠 Activity\n" + strings.Join(lines, "\n")
}

func formatPendingToolLine(name string, args string) string {
	icon := toolEmoji(name)
	text := summarizeToolArgs(args)
	if text == "" {
		return icon + " " + name + "..."
	}
	return icon + " " + text + "..."
}

func formatCompletedToolLine(name string, args string, badge string, detail string) string {
	base := formatPendingToolLine(name, args)
	base = strings.TrimSuffix(base, "...")
	switch badge {
	case "ERROR":
		if detail != "" {
			return base + " ❌ " + detail
		}
		return base + " ❌"
	case "TIMEOUT":
		if detail != "" {
			return base + " ⏱ " + detail
		}
		return base + " ⏱"
	default:
		return base + " ✅"
	}
}

func summarizeToolArgs(args string) string {
	text := strings.TrimSpace(args)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 90 {
		return text[:87] + "..."
	}
	return text
}

func extractToolArgsFromPendingLine(line string, name string) string {
	text := strings.TrimSpace(line)
	if text == "" {
		return ""
	}
	text = strings.TrimPrefix(text, toolEmoji(name))
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, "...")
	return text
}

func toolEmoji(name string) string {
	switch name {
	case "shell":
		return "💻"
	case "task_write":
		return "📋"
	case "task_read":
		return "📖"
	case "load_skill":
		return "📥"
	case "unload_skill":
		return "📤"
	case "replace_block":
		return "📝"
	case "run_skill":
		return "🚀"
	case "ask_a_friend":
		return "🧠"
	case "analyze_image":
		return "🖼"
	default:
		return "🔧"
	}
}

func parseTelegramToolResult(result string) (badge string, detail string) {
	result = strings.TrimSpace(result)

	if strings.HasPrefix(result, "timeout") {
		return "TIMEOUT", strings.TrimSpace(strings.TrimPrefix(result, "timeout"))
	}
	if strings.HasPrefix(result, "error:") {
		return "ERROR", summarizeToolDetail(strings.TrimSpace(strings.TrimPrefix(result, "error:")))
	}
	if strings.HasPrefix(result, "exit_code:") {
		rest := strings.TrimSpace(strings.TrimPrefix(result, "exit_code:"))
		newlineIdx := strings.Index(rest, "\n")
		if newlineIdx < 0 {
			return "DONE", ""
		}
		exitCodeStr := strings.TrimSpace(rest[:newlineIdx])
		exitCode, _ := strconv.Atoi(exitCodeStr)
		remaining := rest[newlineIdx+1:]
		stdout := extractToolSection(remaining, "stdout:")
		stderr := extractToolSection(remaining, "stderr:")
		if exitCode == 0 {
			return "DONE", ""
		}
		if stderr != "" {
			return "ERROR", summarizeToolDetail(stderr)
		}
		if stdout != "" {
			return "ERROR", summarizeToolDetail(stdout)
		}
		return "ERROR", "exit code " + exitCodeStr
	}
	if strings.HasPrefix(result, "ok") {
		return "DONE", ""
	}
	return "DONE", ""
}

func summarizeToolDetail(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 120 {
		return text[:117] + "..."
	}
	return text
}

func extractToolSection(text, label string) string {
	idx := strings.Index(text, label)
	if idx < 0 {
		return ""
	}
	after := text[idx+len(label):]
	after = strings.TrimPrefix(after, "\n")

	end := len(after)
	for _, other := range []string{"stdout:", "stderr:"} {
		if other == label {
			continue
		}
		if i := strings.Index(after, other); i >= 0 && i < end {
			end = i
		}
	}
	return strings.TrimSpace(after[:end])
}
