// handler.go — runtime.Handler implementation for the web transport.
// Converts streaming LLM callbacks into block events pushed to the SSE hub.
// Tracks block indices explicitly so streaming replacements never hit the wrong block.
// Layer: transport output. Dependencies: internal/runtime, internal/provider.
package web

import (
	"fmt"
	"sync"

	"blazeai/internal/provider"
	"blazeai/internal/runtime"
)

// Handler adapts runtime streaming callbacks to the web transcript.
type Handler struct {
	server *Server

	mu                sync.Mutex
	assistantBlockIdx int // index in server's blocks; -1 means not created yet
	assistantStarted  bool
	reasoningBlockIdx int
	reasoningStarted  bool
	contentBuffer     string
	reasoningBuffer   string
	lastPromptTokens  int
	lastToolArgs      string
	agentToolBlocks   map[string]int
	turnErr           error
}

// NewHandler creates a web transport runtime handler bound to a Server.
func NewHandler(server *Server) *Handler {
	return &Handler{
		server:            server,
		assistantBlockIdx: -1,
		reasoningBlockIdx: -1,
		agentToolBlocks:   make(map[string]int),
	}
}

// BeginTurn resets per-turn state and marks the UI busy.
func (h *Handler) BeginTurn() {
	h.mu.Lock()
	h.assistantStarted = false
	h.assistantBlockIdx = -1
	h.reasoningStarted = false
	h.reasoningBlockIdx = -1
	h.contentBuffer = ""
	h.reasoningBuffer = ""
	h.lastPromptTokens = 0
	h.lastToolArgs = ""
	h.agentToolBlocks = make(map[string]int)
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

	if tokens > 0 && model != "" && workDir != "" {
		sep := separatorHTML(tokens, model, workDir)
		if sep != "" {
			h.server.sendBlock("separator", sep)
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
	if !h.assistantStarted {
		h.assistantStarted = true
		h.assistantBlockIdx = -1
		h.contentBuffer = ""
	}
	h.contentBuffer += delta
	html := assistantContentHTML(h.contentBuffer)
	idx := h.assistantBlockIdx
	h.mu.Unlock()

	if h.server == nil {
		return
	}

	prefix := `<span class="orange bold">[BLAZE]</span><br>`
	fullHTML := prefix + html

	if idx < 0 {
		// First chunk — append a new block.
		newIdx := h.server.appendBlock("assistant", fullHTML)
		h.mu.Lock()
		h.assistantBlockIdx = newIdx
		h.mu.Unlock()
	} else {
		// Second+ chunk — replace the block at the tracked index.
		h.server.replaceBlock(idx, "assistant", fullHTML)
	}
}

// OnToolCall emits a pending tool activity block.
func (h *Handler) OnToolCall(name string, args string) {
	h.mu.Lock()
	h.assistantStarted = false
	h.assistantBlockIdx = -1
	h.lastToolArgs = args
	h.mu.Unlock()

	if h.server != nil {
		h.server.sendBlock("tool", toolLineHTML(name, args, ""))
	}
}

// OnToolResult replaces the last pending tool block with the completed summary.
// Uses replace on the most recently appended block (which should be the tool call).
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

	// Replace the most recently appended block — this is always the tool call block.
	// Note: if no block exists (clear was pressed mid-turn), append instead.
	h.server.mu.Lock()
	idx := len(h.server.blocks) - 1
	h.server.mu.Unlock()

	if idx >= 0 {
		h.server.replaceBlock(idx, "tool", toolLineHTML(name, args, badgeSymbol))
	} else {
		h.server.sendBlock("tool", toolLineHTML(name, args, badgeSymbol))
	}
}

// OnUsage stores prompt token usage for the separator display.
func (h *Handler) OnUsage(promptTokens, cachedTokens, uncachedTokens int) {
	h.mu.Lock()
	h.lastPromptTokens = promptTokens
	h.mu.Unlock()
}

// OnReasoning accumulates and streams reasoning text as a single block.
func (h *Handler) OnReasoning(delta string) {
	h.mu.Lock()
	if !h.reasoningStarted {
		h.reasoningStarted = true
		h.reasoningBlockIdx = -1
		h.reasoningBuffer = ""
	}
	h.reasoningBuffer += delta
	html := `<span class="reasoning">🧠 ` + escapeHTML(h.reasoningBuffer) + `</span>`
	idx := h.reasoningBlockIdx
	h.mu.Unlock()

	if h.server == nil {
		return
	}

	if idx < 0 {
		newIdx := h.server.appendBlock("reasoning", html)
		h.mu.Lock()
		h.reasoningBlockIdx = newIdx
		h.mu.Unlock()
	} else {
		h.server.replaceBlock(idx, "reasoning", html)
	}
}

// OnSystem appends a system notification block.
func (h *Handler) OnSystem(message string) {
	if h.server != nil {
		html := `<span class="orange">⚡ System: ` + escapeHTML(message) + `</span>`
		h.server.sendBlock("system", html)
	}
}

// OnAgentActivity renders child activity as Agent-scoped transcript blocks.
// Child tool events reuse the main tool-line renderer and add the child identity prefix.
func (h *Handler) OnAgentActivity(activity runtime.AgentActivity) {
	if h.server == nil {
		return
	}
	if activity.Kind == "tool_call" {
		h.server.sendBlock("tool", agentToolLineHTML(activity.Agent, activity.Tool, activity.Text, ""))
		h.server.mu.Lock()
		h.agentToolBlocks[activity.Agent] = len(h.server.blocks) - 1
		h.server.mu.Unlock()
		return
	}
	if activity.Kind == "tool_result" {
		badge, _, _ := parseToolResult(activity.Text)
		badgeSymbol := ""
		switch badge {
		case "DONE":
			badgeSymbol = "✔️"
		case "ERROR":
			badgeSymbol = "✖️"
		case "TIMEOUT":
			badgeSymbol = "⏱"
		}
		h.mu.Lock()
		idx, ok := h.agentToolBlocks[activity.Agent]
		delete(h.agentToolBlocks, activity.Agent)
		h.mu.Unlock()
		if ok {
			h.server.replaceBlock(idx, "tool", agentToolLineHTML(activity.Agent, activity.Tool, "", badgeSymbol))
		} else {
			h.server.sendBlock("tool", agentToolLineHTML(activity.Agent, activity.Tool, "", badgeSymbol))
		}
		return
	}
	text := "Agent [" + activity.Agent + "] " + activity.Kind
	if activity.Text != "" {
		text += ": " + activity.Text
	}
	h.server.sendBlock("system", `<span class="orange">⚡ `+escapeHTML(text)+`</span>`)
}

// OnMaintenanceCall renders an internal runtime operation as a tool block.
func (h *Handler) OnMaintenanceCall(name string, args string) {
	h.OnToolCall(name, args)
}

// OnMaintenanceResult replaces the maintenance call block with its result.
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
		h.server.sendBlock("system", html)
	}
	return false, ""
}
