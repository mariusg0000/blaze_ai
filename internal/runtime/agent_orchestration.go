// agent_orchestration.go — ephemeral executor child-agent execution.
// Runs Markdown executor agents independently and returns ordered results.
// Layer: runtime orchestration. Dependencies: agents, provider, session, tools.
package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"blazeai/internal/agents"
	"blazeai/internal/session"
	"blazeai/internal/tools"
)

const (
	maxParallelChildren        = 4
	childInactivityTimeout     = 2 * time.Minute
	defaultChildOverallTimeout = 20 * time.Minute
)

// childHandler suppresses child transcript text but forwards scoped tool activity immediately.
// WHAT:  Bridges child runtime handler events to the parent's agent-activity emitter.
// HOW:   OnUsage tracks the latest prompt token count so that OnToolResult can attach CTX
//
//	data matching main-runtime tool result behavior.
type childHandler struct {
	agentID          string
	emit             func(AgentActivity)
	lastPromptTokens int
}

func (h childHandler) OnContent(string) {}
func (h childHandler) OnToolCall(name, args string) {
	h.emit(AgentActivity{Agent: h.agentID, Kind: "tool_call", Tool: name, Status: "running", Text: args})
}
func (h childHandler) OnToolResult(name, result string) {
	h.emit(AgentActivity{Agent: h.agentID, Kind: "tool_result", Tool: name, Status: "ok", Text: result, LastPromptTokens: h.lastPromptTokens})
}
func (h *childHandler) OnUsage(promptTokens, cachedTokens, uncachedTokens int) {
	h.lastPromptTokens = promptTokens
}
func (h childHandler) OnSystem(message string) {
	h.emit(AgentActivity{Agent: h.agentID, Kind: "system", Status: "info", Text: message})
}
func (h childHandler) OnMaintenanceCall(name, args string) {
	h.emit(AgentActivity{Agent: h.agentID, Kind: "tool_call", Tool: name, Status: "running", Text: args})
}
func (h childHandler) OnMaintenanceResult(name, result string) {
	h.emit(AgentActivity{Agent: h.agentID, Kind: "tool_result", Tool: name, Status: "ok", Text: result})
}
func (h childHandler) RequestSudoApproval(string) (bool, string) { return false, "" }

// activityForwarder wraps a Handler and signals an activity channel on each tool event.
// WHAT: Bridges childHandler events to the inactivity timer reset channel.
// HOW: Non-blocking send on each tool call/result; default case avoids blocking when
//
//	channel buffer is full (timer already resetting).
type activityForwarder struct {
	inner    Handler
	activity chan<- struct{}
}

func (f *activityForwarder) OnContent(delta string) { f.inner.OnContent(delta) }
func (f *activityForwarder) OnUsage(p, c, u int)    { f.inner.OnUsage(p, c, u) }
func (f *activityForwarder) OnToolResult(name, result string) {
	f.inner.OnToolResult(name, result)
	f.signal()
}
func (f *activityForwarder) OnSystem(message string) { f.inner.OnSystem(message) }
func (f *activityForwarder) OnMaintenanceCall(name, args string) {
	f.inner.OnMaintenanceCall(name, args)
}
func (f *activityForwarder) OnMaintenanceResult(name, result string) {
	f.inner.OnMaintenanceResult(name, result)
}
func (f *activityForwarder) RequestSudoApproval(cmd string) (bool, string) {
	return f.inner.RequestSudoApproval(cmd)
}

func (f *activityForwarder) OnToolCall(name, args string) {
	f.inner.OnToolCall(name, args)
	f.signal()
}

func (f *activityForwarder) signal() {
	select {
	case f.activity <- struct{}{}:
	default:
	}
}

// runAgent resolves and executes one or more child tasks.
// WHAT: Orchestrates executor definitions with ordered parallel results.
// HOW: Validates definitions, starts bounded workers, and persists each child session for resume.
func (a *Agent) runAgent(ctx context.Context, args tools.RunAgentArgs) string {
	tasks := args.Tasks
	if len(tasks) == 0 {
		tasks = []tools.RunAgentTask{{Agent: args.Agent, Task: args.Task, Context: args.Context, ID: args.ID}}
	}
	for _, task := range tasks {
		if !a.interactiveAllowsExecutor(strings.TrimSpace(task.Agent)) {
			agentName := ""
			if a.CurrentAgent != nil {
				agentName = a.CurrentAgent.Name
			}
			return fmt.Sprintf("error: agent %q does not allow executor %q", agentName, strings.TrimSpace(task.Agent))
		}
	}
	results := make([]string, len(tasks))
	sem := make(chan struct{}, maxParallelChildren)
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for i, task := range tasks {
		wg.Add(1)
		go func(index int, task tools.RunAgentTask) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-childCtx.Done():
				setFirstError(&errMu, &firstErr, childCtx.Err())
				return
			}
			defer func() { <-sem }()
			childID := strings.TrimSpace(task.ID)
			if childID == "" {
				childID = shortChildID()
			}
			displayID := fmt.Sprintf("[%s_%s]", task.Agent, childID)
			result, err := a.runOneChild(childCtx, task, displayID, childID)
			if err != nil {
				setFirstError(&errMu, &firstErr, err)
				cancel()
				return
			}
			results[index] = result
		}(i, task)
	}
	wg.Wait()
	if firstErr != nil {
		return "error: " + firstErr.Error()
	}
	if len(results) == 1 {
		return results[0]
	}
	return formatOrderedResults(results)
}

// setFirstError stores one deterministic error while workers continue cleanup.
func setFirstError(mu *sync.Mutex, target *error, err error) {
	if err == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if *target == nil {
		*target = err
	}
}

// emitAgentActivity forwards transient child activity to transports that support it.
func (a *Agent) emitAgentActivity(activity AgentActivity) {
	if handler, ok := a.Handler.(AgentActivityHandler); ok {
		handler.OnAgentActivity(activity)
	}
}

// runOneChild creates, runs, and removes one ephemeral child session.
// WHAT: Executes one child with dual timeout: overall hard cap + inactivity reset.
// HOW: Overall timeout (per-agent or 20min default) never resets. Inactivity timeout (2min)
//
//	resets on every tool call or result forwarded by the activity handler.
func (a *Agent) runOneChild(parentCtx context.Context, task tools.RunAgentTask, displayID, childID string) (result string, err error) {
	// Decorate failures before they reach the parent so resume always uses the exact session ID.
	defer func() {
		if err != nil {
			err = formatChildError(task.Agent, childID, err)
		}
	}()

	definition, ok := a.executorDefinition(task.Agent)
	if !ok {
		a.emitAgentActivity(AgentActivity{Agent: task.Agent, Kind: "failed", Status: "error", Text: "executor agent not found"})
		return "", fmt.Errorf("executor agent not found: %s", task.Agent)
	}
	if strings.TrimSpace(task.Task) == "" {
		return "", fmt.Errorf("child %q task is empty", displayID)
	}
	folder, childSession, resumed, err := openChildSession(a.Session.Folder, childID)
	if err != nil {
		return "", err
	}

	model := strings.TrimSpace(definition.Model)
	if model == "" {
		model = a.ModelID
	}
	a.emitAgentActivity(AgentActivity{Agent: displayID, Kind: "started", Status: "running", Text: fmt.Sprintf("child agent started — model: %s", model)})
	defer func() {
		if err != nil {
			kind := "failed"
			if strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "inactivity") {
				kind = "timed out"
			} else if strings.Contains(err.Error(), "cancelled") {
				kind = "cancelled"
			}
			a.emitAgentActivity(AgentActivity{Agent: displayID, Kind: kind, Status: "error", Text: err.Error()})
			return
		}
		a.emitAgentActivity(AgentActivity{Agent: displayID, Kind: "completed", Status: "ok", Text: "child agent completed"})
	}()

	taskPath := filepath.Join(folder, "agent_task.md")
	if !resumed {
		if err := os.WriteFile(taskPath, []byte(strings.TrimSpace(task.Task)+"\n"), 0644); err != nil {
			return "", fmt.Errorf("cannot write child task file: %w", err)
		}
	}
	child, err := newChildAgent(a.Config, childSession, a.OS, a.Builder.PromptsFS, a.Builder.BuiltinSkillsFS, a.WorkDir, &childHandler{agentID: displayID, emit: a.emitAgentActivity}, a.Builder.TransportName, model)
	if err != nil {
		return "", fmt.Errorf("cannot initialize child agent %q: %w", definition.Name, err)
	}
	child.Builder.SystemPromptName = "sysprompt.agent.md"
	child.Builder.AgentInstructions = definition.Instructions
	child.Builder.AgentTaskFile = taskPath
	child.Builder.Agents = nil

	completion := ""
	child.BaseTools.Register(tools.NewAgentDoneTool(func(answer string) { completion = strings.TrimSpace(answer) }))
	filteredNames := append([]string(nil), definition.ToolNames...)
	if !containsToolName(filteredNames, "agent_done") {
		filteredNames = append(filteredNames, "agent_done")
	}
	child.Tools, err = child.BaseTools.Filter(filteredNames)
	if err != nil {
		return "", fmt.Errorf("cannot build child tool registry for %q: %w", definition.Name, err)
	}
	child.Completion = ""

	// Dual timeout: overall hard cap + inactivity reset.
	overallTimeout := defaultChildOverallTimeout
	if definition.Timeout > 0 {
		overallTimeout = definition.Timeout
	}
	otctx, otCancel := context.WithTimeout(parentCtx, overallTimeout)
	defer otCancel()
	actx, aCancel := context.WithCancel(otctx)
	defer aCancel()

	// Inactivity timer: resets on every child activity event.
	inactivityTimer := time.AfterFunc(childInactivityTimeout, func() { aCancel() })
	defer inactivityTimer.Stop()

	// Activity signal channel — childHandler forwards tool calls/results here.
	activity := make(chan struct{}, 1)
	origHandler := child.Handler
	child.Handler = &activityForwarder{
		inner:    origHandler,
		activity: activity,
	}

	go func() {
		for {
			select {
			case <-activity:
				inactivityTimer.Reset(childInactivityTimeout)
			case <-actx.Done():
				return
			}
		}
	}()

	input := buildChildInput(task, resumed)
	if err := child.RunTurn(actx, input); err != nil {
		if parentCtx.Err() != nil {
			return "", fmt.Errorf("child %q cancelled: %w", definition.Name, parentCtx.Err())
		}
		if actx.Err() != nil && otctx.Err() == nil {
			return "", fmt.Errorf("child %q timed out due to inactivity", definition.Name)
		}
		if otctx.Err() != nil {
			return "", fmt.Errorf("child %q timed out after %s", definition.Name, overallTimeout)
		}
		return "", fmt.Errorf("child %q failed: %w", definition.Name, err)
	}
	warning := ""
	if completion == "" {
		completion = lastAssistantAnswer(childSession)
		if completion == "" {
			return "", fmt.Errorf("child %q incomplete: agent_done was not called and no final assistant answer was available", definition.Name)
		}
		warning = "status: completed-with-warning\nagent_done was not called; the last assistant message was used.\n\n"
	}
	return formatChildResult(definition.Name, childID, warning, completion), nil
}

// lastAssistantAnswer returns a final assistant text answer when no agent_done callback ran.
// WHAT: Extracts only a plain assistant message, never a tool-call message.
// HOW: Inspects the child session backwards and accepts the last assistant string without tool calls.
func lastAssistantAnswer(child *session.Session) string {
	for i := len(child.Messages) - 1; i >= 0; i-- {
		message := child.Messages[i]
		if message.Role != "assistant" || message.ToolCalls != nil {
			continue
		}
		answer, ok := message.Content.(string)
		if ok && strings.TrimSpace(answer) != "" {
			return strings.TrimSpace(answer)
		}
		return ""
	}
	return ""
}

// formatChildResult adds identity and optional resume metadata to every child result.
// WHAT: Makes the child identity available to the parent for optional future resume.
// HOW: Returns the agent name, exact id, and neutral resume possibility before the answer.
func formatChildResult(agentName, childID, warning, answer string) string {
	return fmt.Sprintf("%sagent: %s\nchild session id: %s\nThis child session can be resumed later with the same agent name, this id, and a new task, if needed.\n\n%s", warning, agentName, childID, answer)
}

// formatChildError adds exact resume metadata to a child failure.
// WHAT: Makes failed child sessions resumable by the parent model.
// HOW: Places the agent name, exact session ID, and valid run_agent syntax in the error text.
func formatChildError(agentName, childID string, err error) error {
	return fmt.Errorf("agent: %s\nchild session id: %s\n%s\nResume with agent=%q and id=%q", agentName, childID, err, agentName, childID)
}

// shortChildID generates a compact 5-character hex identifier for child sessions.
func shortChildID() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// executorDefinition resolves only TypeExecutor definitions.
func (a *Agent) executorDefinition(name string) (agents.Definition, bool) {
	for _, definition := range a.Definitions {
		if definition.Name == strings.TrimSpace(name) && definition.Type == agents.TypeExecutor {
			return definition, true
		}
	}
	return agents.Definition{}, false
}

// containsToolName reports whether a capability is already explicitly declared.
func containsToolName(names []string, wanted string) bool {
	for _, name := range names {
		if name == wanted {
			return true
		}
	}
	return false
}

// buildChildInput creates the child turn input for a new task or a resume request.
// WHAT: Keeps the original task in the system prompt and sends resume work as a user message.
// HOW: New children receive optional context; resumed children receive the new task explicitly.
func buildChildInput(task tools.RunAgentTask, resumed bool) string {
	if resumed {
		input := "[RESUME TASK]\n" + strings.TrimSpace(task.Task)
		if strings.TrimSpace(task.Context) != "" {
			input += "\n\n[CONTEXT]\n" + strings.TrimSpace(task.Context)
		}
		return input + "\n\nCall agent_done with a concise non-empty final answer when complete."
	}
	if strings.TrimSpace(task.Context) == "" {
		return "Continue with the current task from the system prompt. Call agent_done with a concise non-empty final answer when complete."
	}
	return "[CONTEXT]\n" + strings.TrimSpace(task.Context) + "\n\nCall agent_done with a concise non-empty final answer when complete."
}

// openChildSession creates or resumes a persistent child session beneath the main session.
func openChildSession(mainFolder, id string) (string, *session.Session, bool, error) {
	if !validChildID(id) {
		return "", nil, false, fmt.Errorf("invalid child session id %q", id)
	}
	root := filepath.Join(mainFolder, "agents")
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", nil, false, fmt.Errorf("cannot create child sessions directory: %w", err)
	}
	folder := filepath.Join(root, id)
	if info, err := os.Stat(folder); err == nil {
		if !info.IsDir() {
			return "", nil, false, fmt.Errorf("child session path is not a directory: %s", folder)
		}
		child, err := session.Load(folder)
		if err != nil {
			return "", nil, false, fmt.Errorf("cannot resume child session %q: %w", id, err)
		}
		return folder, child, true, nil
	} else if !os.IsNotExist(err) {
		return "", nil, false, fmt.Errorf("cannot inspect child session %q: %w", id, err)
	}
	if err := os.Mkdir(folder, 0755); err != nil {
		return "", nil, false, fmt.Errorf("cannot create child session %q: %w", id, err)
	}
	child := &session.Session{Messages: []session.Message{}, Folder: folder}
	if err := child.Save(); err != nil {
		return "", nil, false, fmt.Errorf("cannot initialize child session %q: %w", id, err)
	}
	return folder, child, false, nil
}

// validChildID rejects path separators and traversal from resume identifiers.
func validChildID(id string) bool {
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id {
		return false
	}
	for _, r := range id {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// formatOrderedResults preserves requested task order in the parent tool result.
func formatOrderedResults(results []string) string {
	var b strings.Builder
	for i, result := range results {
		fmt.Fprintf(&b, "[%d]\n%s", i+1, result)
		if i+1 < len(results) {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}
