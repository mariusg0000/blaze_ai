// agent_orchestration.go — ephemeral one-shot child-agent execution.
// Runs Markdown one-shot agents independently and returns bounded ordered results.
// Layer: runtime orchestration. Dependencies: agents, provider, session, tools.
package runtime

import (
	"context"
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
	maxChildAnswerRunes        = 12000
)

// childHandler suppresses child transcript text but forwards scoped tool activity immediately.
type childHandler struct {
	agentID string
	emit    func(AgentActivity)
}

func (h childHandler) OnContent(string) {}
func (h childHandler) OnToolCall(name, args string) {
	h.emit(AgentActivity{Agent: h.agentID, Kind: "tool_call", Tool: name, Status: "running", Text: args})
}
func (h childHandler) OnToolResult(name, result string) {
	h.emit(AgentActivity{Agent: h.agentID, Kind: "tool_result", Tool: name, Status: "ok", Text: result})
}
func (h childHandler) OnUsage(int, int, int) {}
func (h childHandler) OnReasoning(string)    {}
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

func (f *activityForwarder) OnContent(delta string)   { f.inner.OnContent(delta) }
func (f *activityForwarder) OnReasoning(delta string) { f.inner.OnReasoning(delta) }
func (f *activityForwarder) OnUsage(p, c, u int)      { f.inner.OnUsage(p, c, u) }
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
// WHAT: Orchestrates one-shot definitions with ordered parallel results.
// HOW: Validates definitions, starts bounded workers, and persists each child session for resume.
func (a *Agent) runAgent(ctx context.Context, args tools.RunAgentArgs) string {
	tasks := args.Tasks
	if len(tasks) == 0 {
		tasks = []tools.RunAgentTask{{Agent: args.Agent, Task: args.Task, Context: args.Context, ID: args.ID}}
	}
	for _, task := range tasks {
		if !a.modeAllowsAgent(strings.TrimSpace(task.Agent)) {
			return fmt.Sprintf("error: mode %q does not allow one-shot agent %q", a.CurrentMode.Name, strings.TrimSpace(task.Agent))
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
			displayID := strings.TrimSpace(task.ID)
			if displayID == "" {
				displayID = fmt.Sprintf("%s_%02d_%d", task.Agent, index+1, time.Now().UnixNano())
			}
			result, err := a.runOneChild(childCtx, task, displayID)
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
func (a *Agent) runOneChild(parentCtx context.Context, task tools.RunAgentTask, displayID string) (result string, err error) {
	definition, ok := a.oneShotDefinition(task.Agent)
	if !ok {
		a.emitAgentActivity(AgentActivity{Agent: task.Agent, Kind: "failed", Status: "error", Text: "one-shot agent not found"})
		return "", fmt.Errorf("one-shot agent not found: %s", task.Agent)
	}
	a.emitAgentActivity(AgentActivity{Agent: displayID, Kind: "started", Status: "running", Text: "child agent started"})
	folder, childSession, err := openChildSession(a.Session.Folder, displayID)
	if err != nil {
		return "", err
	}
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
	if strings.TrimSpace(task.Task) == "" {
		return "", fmt.Errorf("child %q task is empty", displayID)
	}
	if err := os.WriteFile(taskPath, []byte(strings.TrimSpace(task.Task)+"\n"), 0644); err != nil {
		return "", fmt.Errorf("cannot write child task file: %w", err)
	}

	model := strings.TrimSpace(definition.Model)
	if model == "" {
		model = a.ModelID
	}
	child, err := newChildAgent(a.Config, childSession, a.OS, a.Builder.PromptsFS, a.WorkDir, childHandler{agentID: displayID, emit: a.emitAgentActivity}, a.Builder.TransportName, model)
	if err != nil {
		return "", fmt.Errorf("cannot initialize child agent %q: %w", definition.Name, err)
	}
	child.Builder.SystemPromptName = "sysprompt.agent.md"
	child.Builder.AgentInstructions = definition.Instructions
	child.Builder.AgentTaskFile = taskPath
	child.Builder.Agents = nil

	completion := ""
	child.BaseTools.Register(tools.NewAgentDoneTool(func(answer string) { completion = boundAnswer(answer) }))
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

	input := buildChildInput(task)
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
	if completion == "" {
		return "", fmt.Errorf("child %q incomplete: agent_done was not called", definition.Name)
	}
	return fmt.Sprintf("child session id: %s\n\n%s", displayID, completion), nil
}

// oneShotDefinition resolves only one-shot definitions and never falls back to modes.
func (a *Agent) oneShotDefinition(name string) (agents.Definition, bool) {
	for _, definition := range a.Definitions {
		if definition.Name == strings.TrimSpace(name) && definition.Kind == agents.KindOneShot {
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

// buildChildInput passes only optional context because the current task is part of the system prompt.
func buildChildInput(task tools.RunAgentTask) string {
	if strings.TrimSpace(task.Context) == "" {
		return "Continue with the current task from the system prompt. Call agent_done with a concise non-empty final answer when complete."
	}
	return "[CONTEXT]\n" + strings.TrimSpace(task.Context) + "\n\nCall agent_done with a concise non-empty final answer when complete."
}

// openChildSession creates or resumes a persistent child session beneath the main session.
func openChildSession(mainFolder, id string) (string, *session.Session, error) {
	if !validChildID(id) {
		return "", nil, fmt.Errorf("invalid child session id %q", id)
	}
	root := filepath.Join(mainFolder, "agents")
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", nil, fmt.Errorf("cannot create child sessions directory: %w", err)
	}
	folder := filepath.Join(root, id)
	if info, err := os.Stat(folder); err == nil {
		if !info.IsDir() {
			return "", nil, fmt.Errorf("child session path is not a directory: %s", folder)
		}
		child, err := session.Load(folder)
		if err != nil {
			return "", nil, fmt.Errorf("cannot resume child session %q: %w", id, err)
		}
		return folder, child, nil
	} else if !os.IsNotExist(err) {
		return "", nil, fmt.Errorf("cannot inspect child session %q: %w", id, err)
	}
	if err := os.Mkdir(folder, 0755); err != nil {
		return "", nil, fmt.Errorf("cannot create child session %q: %w", id, err)
	}
	child := &session.Session{Messages: []session.Message{}, Folder: folder}
	if err := child.Save(); err != nil {
		return "", nil, fmt.Errorf("cannot initialize child session %q: %w", id, err)
	}
	return folder, child, nil
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

// boundAnswer limits child output before inserting it into parent context.
func boundAnswer(answer string) string {
	runes := []rune(strings.TrimSpace(answer))
	if len(runes) <= maxChildAnswerRunes {
		return string(runes)
	}
	return string(runes[:maxChildAnswerRunes-3]) + "..."
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
