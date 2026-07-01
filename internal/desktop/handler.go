// handler.go — desktop runtime.Handler implementation.
// Streams assistant text and tool activity into the desktop transcript while the
// UI keeps one fixed persistent conversation open.
// Layer: transport output. Dependencies: internal/runtime.
package desktop

import (
	"fmt"
	"strings"
	"sync"
)

type transcriptSink interface {
	AppendUser(text string)
	AppendSystem(text string)
	AppendTool(text string)
	StartAssistant()
	AppendAssistant(delta string)
	FinishAssistant()
	SetStatus(text string)
	SetBusy(active bool)
}

// Handler adapts runtime streaming callbacks to the desktop transcript.
//
// WHAT:  Feeds assistant content and tool events into the desktop chat view.
// WHY:   The shared runtime talks to desktop UI only through the Handler contract.
// PARAMS: sink — transcript/status writer owned by the desktop UI.
type Handler struct {
	sink transcriptSink

	mu               sync.Mutex
	assistantStarted bool
	lastTokens       int
	lastErr          error
}

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
	h.assistantStarted = false
	h.lastTokens = 0
	h.lastErr = nil
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
func (h *Handler) OnContent(delta string) {
	h.mu.Lock()
	started := h.assistantStarted
	if !started {
		h.assistantStarted = true
	}
	h.mu.Unlock()
	if h.sink == nil {
		return
	}
	if !started {
		h.sink.StartAssistant()
	}
	h.sink.AppendAssistant(delta)
}

// OnToolCall appends a short tool activity line to the transcript.
func (h *Handler) OnToolCall(name string, args string) {
	if h.sink == nil {
		return
	}
	args = strings.TrimSpace(args)
	if args == "" {
		h.sink.AppendTool(fmt.Sprintf("Running %s", name))
		return
	}
	h.sink.AppendTool(fmt.Sprintf("Running %s\n%s", name, args))
}

// OnToolResult appends a short tool result line to the transcript.
func (h *Handler) OnToolResult(name string, result string) {
	if h.sink == nil {
		return
	}
	result = strings.TrimSpace(result)
	if result == "" {
		h.sink.AppendTool(fmt.Sprintf("%s completed", name))
		return
	}
	h.sink.AppendTool(fmt.Sprintf("%s result\n%s", name, result))
}

// OnUsage stores prompt token usage for the last provider response.
func (h *Handler) OnUsage(promptTokens int) {
	h.mu.Lock()
	h.lastTokens = promptTokens
	h.mu.Unlock()
}

// OnReasoning does not display hidden reasoning blocks in the desktop transport.
func (h *Handler) OnReasoning(delta string) {
	_ = delta
}

// RequestSudoApproval denies sudo because the desktop transport has no secure password flow yet.
func (h *Handler) RequestSudoApproval(command string) (bool, string) {
	_ = command
	if h.sink != nil {
		h.sink.AppendSystem("Sudo approval is not supported in the desktop transport yet.")
	}
	return false, ""
}
