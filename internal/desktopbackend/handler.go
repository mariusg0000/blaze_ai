// handler.go — desktop runtime.Handler implementation.
// Streams assistant text and tool activity into the desktop transcript while the
// UI keeps one fixed persistent conversation open.
// Layer: transport output. Dependencies: internal/runtime.
package desktopbackend

import (
	"fmt"
	"sync"
)

type transcriptSink interface {
	AppendUser(text string)
	AppendSystem(text string)
	StartAssistant()
	AppendAssistant(delta string)
	FinishAssistant()
	StartReasoning()
	AppendReasoning(delta string)
	FinishReasoning()
	SetToolActivity(text string)
	SetStatus(text string)
	SetBusy(active bool)
	SetPromptTokens(tokens int)
}

// Handler adapts runtime streaming callbacks to the desktop transcript.
//
// WHAT:  Feeds assistant content and tool events into the desktop chat view.
// WHY:   The shared runtime talks to desktop UI only through the Handler contract.
// PARAMS: sink — transcript/status writer owned by the desktop UI.
type Handler struct {
	sink transcriptSink

	mu            sync.Mutex
	activeSegment string
	lastTokens    int
	lastErr       error
	activity      toolActivity
}

const (
	segmentAssistant = "assistant"
	segmentReasoning = "reasoning"
	segmentTool      = "tool"
)

// NewHandler creates a desktop runtime handler.
//
// WHAT:  Binds a desktop transcript sink to the runtime.Handler interface.
// WHY:   Desktop Run wires this handler into the shared agent core.
// PARAMS: sink — transcript/status writer owned by the UI.
// RETURNS: *Handler — ready desktop handler.
func NewHandler(sink transcriptSink) *Handler {
	return &Handler{sink: sink}
}

// BeginTurn resets per-turn state and marks the UI busy.
//
// WHAT:  Starts one desktop turn lifecycle.
// WHY:   The UI needs a clean streaming state for each submitted prompt.
func (h *Handler) BeginTurn() {
	h.mu.Lock()
	h.activeSegment = ""
	h.lastTokens = 0
	h.lastErr = nil
	h.activity.Reset()
	h.mu.Unlock()
	if h.sink != nil {
		h.sink.SetBusy(true)
		h.sink.SetStatus("Thinking...")
	}
}

// FinishTurn closes the active assistant block and updates status text.
//
// WHAT:  Finalizes one desktop turn lifecycle.
// WHY:   The UI needs the final busy/status state after runtime completes.
// PARAMS: err — final turn error, if any.
func (h *Handler) FinishTurn(err error) {
	h.mu.Lock()
	h.lastErr = err
	tokens := h.lastTokens
	h.mu.Unlock()
	if h.sink == nil {
		return
	}
	h.sink.FinishReasoning()
	h.sink.FinishAssistant()
	h.sink.SetBusy(false)
	if err != nil {
		h.sink.SetStatus("Error")
		return
	}
	if tokens > 0 {
		h.sink.SetStatus(fmt.Sprintf("Ready • CTX %d", tokens))
		return
	}
	h.sink.SetStatus("Ready")
}

// OnContent appends one streamed assistant text delta.
// A pending reasoning block is closed before the assistant block opens.
func (h *Handler) OnContent(delta string) {
	h.mu.Lock()
	started := h.activeSegment == segmentAssistant
	reasoningActive := h.activeSegment == segmentReasoning
	if !started {
		h.activeSegment = segmentAssistant
		h.activity.Reset()
	}
	h.mu.Unlock()
	if h.sink == nil {
		return
	}
	if reasoningActive {
		h.sink.FinishReasoning()
	}
	if !started {
		h.sink.StartAssistant()
	}
	h.sink.AppendAssistant(delta)
}

// OnToolCall updates the compact desktop activity block for one pending tool call.
func (h *Handler) OnToolCall(name string, args string) {
	h.mu.Lock()
	if h.activeSegment != segmentTool {
		h.activeSegment = segmentTool
		h.activity.Reset()
	}
	h.activity.AddCall("", name, args)
	activityText := h.activity.Render()
	h.mu.Unlock()
	if h.sink != nil {
		h.sink.SetToolActivity(activityText)
	}
}

// OnToolResult replaces the pending tool line with a compact completed summary.
func (h *Handler) OnToolResult(name string, result string) {
	h.mu.Lock()
	if h.activeSegment != segmentTool {
		h.activeSegment = segmentTool
		h.activity.Reset()
	}
	h.activity.ApplyResult("", name, result)
	activityText := h.activity.Render()
	h.mu.Unlock()
	if h.sink != nil {
		h.sink.SetToolActivity(activityText)
	}
}

// OnUsage stores prompt token usage for the last provider response.
func (h *Handler) OnUsage(promptTokens int) {
	h.mu.Lock()
	h.lastTokens = promptTokens
	h.mu.Unlock()
	if h.sink != nil {
		h.sink.SetPromptTokens(promptTokens)
	}
}

// OnReasoning appends one streamed reasoning/thinking chunk as its own transcript row.
func (h *Handler) OnReasoning(delta string) {
	h.mu.Lock()
	started := h.activeSegment == segmentReasoning
	if !started {
		h.activeSegment = segmentReasoning
		h.activity.Reset()
	}
	h.mu.Unlock()
	if h.sink == nil {
		return
	}
	if !started {
		h.sink.StartReasoning()
	}
	h.sink.AppendReasoning(delta)
}

// RequestSudoApproval denies sudo because the desktop transport has no secure password flow yet.
func (h *Handler) RequestSudoApproval(command string) (bool, string) {
	_ = command
	if h.sink != nil {
		h.sink.AppendSystem("Sudo approval is not supported in the desktop transport yet.")
	}
	return false, ""
}
