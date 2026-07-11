// handler.go — runtime.Handler implementation for the web transport.
// Converts streaming LLM callbacks into block events pushed to the SSE hub.
// Mirrors the console handler but produces HTML blocks instead of ANSI lines.
// Layer: transport output. Dependencies: internal/runtime, internal/provider.
package web

import (
	"fmt"
	"strings"
	"sync"

	"blazeai/internal/provider"
)

// Handler adapts runtime streaming callbacks to the web transcript.
type Handler struct {
	server *Server

	mu               sync.Mutex
	assistantStarted bool
	reasoningStarted bool
	contentBuffer    string
	lastPromptTokens int
	lastToolArgs     string
	turnErr          error
}

// NewHandler creates a web transport runtime handler bound to a Server.
func NewHandler(server *Server) *Handler {
	return &Handler{server: server}
}

// BeginTurn resets per-turn state and marks the UI busy.
func (h *Handler) BeginTurn() {
	h.mu.Lock()
	h.assistantStarted = false
	h.reasoningStarted = false
	h.contentBuffer = ""
	h.lastPromptTokens = 0
	h.lastToolArgs = ""
	h.turnErr = nil
	h.mu.Unlock()
	if h.server != nil {
		h.server.SetBusy(true)
		h.server.SetStatus("Thinking...")
	}
}

// FinishTurn closes the active blocks and updates status.
func (h *Handler) FinishTurn(err error) {
	h.mu.Lock()
	h.turnErr = err
	tokens := h.lastPromptTokens
	model := ""
	workDir := ""
	if h.server != nil && h.server.agent != nil {
		model = h.server.agent.ModelID
		workDir = h.server.agent.WorkDir
	}
	h.mu.Unlock()

	if h.server == nil {
		return
	}
	h.server.SetBusy(false)

	if err != nil {
		h.server.SetStatus("Error")
		return
	}

	// Emit separator block.
	if tokens > 0 && model != "" && workDir != "" {
		sep := separatorHTML(tokens, model, workDir)
		if sep != "" {
			h.server.sendBlock("separator", sep, false)
		}
	}

	if tokens > 0 {
		h.server.SetStatus(fmt.Sprintf("Ready • CTX %d", tokens))
	} else {
		h.server.SetStatus("Ready")
	}
}

// OnContent appends one streamed assistant text delta.
func (h *Handler) OnContent(delta string) {
	h.mu.Lock()
	reasoningActive := h.reasoningStarted
	if !h.assistantStarted {
		h.assistantStarted = true
		h.contentBuffer = ""
	}
	h.contentBuffer += delta
	html := assistantContentHTML(h.contentBuffer)
	assistantStarted := h.assistantStarted
	h.mu.Unlock()

	if h.server == nil {
		return
	}
	if reasoningActive {
		// No explicit close — reasoning block is finished implicitly.
	}

	if !assistantStarted {
		return
	}

	prefix := `<span class="orange bold">[BLAZE]</span><br>`
	h.server.sendBlock("assistant", prefix+html, false)
}

// OnToolCall emits a pending tool activity block.
func (h *Handler) OnToolCall(name string, args string) {
	h.mu.Lock()
	h.assistantStarted = false // next content starts fresh after tools
	h.lastToolArgs = args
	h.mu.Unlock()

	if h.server != nil {
		h.server.sendBlock("tool", toolLineHTML(name, args, ""), false)
	}
}

// OnToolResult replaces the pending tool block with a completed summary.
func (h *Handler) OnToolResult(name string, result string) {
	h.mu.Lock()
	args := h.lastToolArgs
	h.lastToolArgs = ""
	h.mu.Unlock()

	if h.server == nil {
		return
	}

	badge, _, _ := parseToolResult(result)

	var badgeSymbol string
	switch badge {
	case "DONE":
		badgeSymbol = "✔️"
	case "ERROR":
		badgeSymbol = "✖️"
	case "TIMEOUT":
		badgeSymbol = "⏱"
	}

	h.server.sendBlock("tool", toolLineHTML(name, args, badgeSymbol), true)
}

// OnUsage stores prompt token usage for the separator display.
func (h *Handler) OnUsage(promptTokens int) {
	h.mu.Lock()
	h.lastPromptTokens = promptTokens
	h.mu.Unlock()
}

// OnReasoning appends one streamed reasoning chunk as its own block.
func (h *Handler) OnReasoning(delta string) {
	h.mu.Lock()
	if !h.reasoningStarted {
		h.reasoningStarted = true
		h.assistantStarted = false
	}
	h.mu.Unlock()

	if h.server == nil {
		return
	}

	emoji := `<span class="reasoning">🧠 ` + escapeHTML(delta) + `</span>`
	h.server.sendBlock("reasoning", emoji, false)
}

// OnSystem appends a system notification block.
func (h *Handler) OnSystem(message string) {
	if h.server != nil {
		html := `<span class="orange">⚡ System: ` + escapeHTML(message) + `</span>`
		h.server.sendBlock("system", html, false)
	}
}

// OnMaintenanceCall renders an internal runtime operation as a tool block.
func (h *Handler) OnMaintenanceCall(name string, args string) {
	h.OnToolCall(name, args)
}

// OnMaintenanceResult renders the final internal operation status.
func (h *Handler) OnMaintenanceResult(name string, result string) {
	h.OnToolResult(name, result)
}

// OnStreamPhase updates the status line with provider connection phases.
func (h *Handler) OnStreamPhase(phase provider.StreamPhase) {
	if h.server == nil {
		return
	}
	switch phase {
	case provider.PhaseConnecting:
		h.server.SetStatus("Connecting...")
	case provider.PhaseWaitingFirstEvent:
		h.server.SetStatus("Waiting...")
	case provider.PhaseHiddenReasoning:
		h.server.SetStatus("Thinking...")
	case provider.PhaseStreaming:
		h.server.SetStatus("Streaming...")
	}
}

// RequestSudoApproval denies sudo because the web transport has no secure password flow.
func (h *Handler) RequestSudoApproval(command string) (bool, string) {
	_ = command
	if h.server != nil {
		html := `<span class="orange">⚡ Sudo is not supported in the web transport.</span>`
		h.server.sendBlock("system", html, false)
	}
	return false, ""
}

// trimReasoningPrefix removes the 🧠 emoji prefix from streaming reasoning chunks
// so the handler can accumulate clean content without repeated prefixes.
func trimReasoningPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "🧠"), " ")
}
