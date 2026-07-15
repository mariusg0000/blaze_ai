// agent_tools.go — tools used by interactive and one-shot agent runtimes.
// Defines the run_agent orchestration adapter and strict agent_done completion protocol.
// Layer: tool execution. Dependencies: standard library only.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RunAgentTask is one requested child-agent task.
type RunAgentTask struct {
	Agent   string `json:"agent"`
	Task    string `json:"task"`
	Context string `json:"context,omitempty"`
	ID      string `json:"id,omitempty"`
}

// RunAgentArgs supports one task or an ordered list of parallel tasks.
type RunAgentArgs struct {
	Purpose string         `json:"purpose"`
	Agent   string         `json:"agent,omitempty"`
	Task    string         `json:"task,omitempty"`
	Context string         `json:"context,omitempty"`
	ID      string         `json:"id,omitempty"`
	Tasks   []RunAgentTask `json:"tasks,omitempty"`
}

// RunAgentTool launches child agents through a runtime-owned callback.
type RunAgentTool struct {
	callback func(context.Context, RunAgentArgs) string
}

// NewRunAgentTool creates a run_agent adapter.
func NewRunAgentTool(callback func(context.Context, RunAgentArgs) string) *RunAgentTool {
	return &RunAgentTool{callback: callback}
}

// Name identifies the tool.
func (t *RunAgentTool) Name() string { return "run_agent" }

// Description explains single and parallel child execution.
func (t *RunAgentTool) Description() string {
	return "Run one child agent, or multiple child agents in parallel by supplying tasks in order."
}

// Parameters returns the JSON schema accepted by the tool.
func (t *RunAgentTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"purpose":{"type":"string","description":"purpose = exactly 3 user-visible sentences. Sentence 1 must name the child agent or agent group and the requested task. Sentence 2 must describe the execution scope, task ordering, or context passed to children. Sentence 3 must explain what the returned child result should produce and how it advances the task."},"agent":{"type":"string"},"task":{"type":"string"},"context":{"type":"string"},"id":{"type":"string","description":"Optional persistent child-session ID. Omit it to create a new child session; provide it to resume and replace that session's current task."},"tasks":{"type":"array","items":{"type":"object","required":["agent","task"],"properties":{"agent":{"type":"string"},"task":{"type":"string"},"context":{"type":"string"},"id":{"type":"string"}}}}},"required":["purpose"]}`)
}

// Execute validates arguments and delegates execution to runtime.
func (t *RunAgentTool) Execute(ctx context.Context, raw json.RawMessage) string {
	args, err := ParseToolCallArgs[RunAgentArgs](raw)
	if err != nil {
		return "error: " + err.Error()
	}
	if len(args.Tasks) == 0 && (strings.TrimSpace(args.Agent) == "" || strings.TrimSpace(args.Task) == "") {
		return "error: agent and task are required"
	}
	if len(args.Tasks) > 0 {
		for i := range args.Tasks {
			if strings.TrimSpace(args.Tasks[i].Agent) == "" || strings.TrimSpace(args.Tasks[i].Task) == "" {
				return fmt.Sprintf("error: tasks[%d] requires agent and task", i)
			}
		}
	}
	if t.callback == nil {
		return "error: agent orchestration is not configured"
	}
	return t.callback(ctx, args)
}

// FormatArgs returns the explicit purpose, with a bounded task fallback for malformed legacy calls.
func (t *RunAgentTool) FormatArgs(raw json.RawMessage) string {
	args, err := ParseToolCallArgs[RunAgentArgs](raw)
	if err != nil {
		return "Running agent"
	}
	if purpose := strings.TrimSpace(args.Purpose); purpose != "" {
		return purpose
	}
	task := strings.TrimSpace(args.Task)
	if task == "" && len(args.Tasks) > 0 {
		task = strings.TrimSpace(args.Tasks[0].Task)
	}
	if task == "" {
		return "Running agent"
	}
	return truncateDisplay(task, 80)
}

// AgentDoneArgs is the mandatory child completion payload.
type AgentDoneArgs struct {
	Answer string `json:"answer"`
}

// AgentDoneTool marks a child runtime complete.
type AgentDoneTool struct {
	callback func(string)
}

// NewAgentDoneTool creates the internal completion adapter.
func NewAgentDoneTool(callback func(string)) *AgentDoneTool {
	return &AgentDoneTool{callback: callback}
}

// Name identifies the protocol tool.
func (t *AgentDoneTool) Name() string { return "agent_done" }

// Description requires a non-empty final answer.
func (t *AgentDoneTool) Description() string {
	return "Required completion protocol for one-shot agents. Submit the final non-empty answer."
}

// Parameters returns the completion schema.
func (t *AgentDoneTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["answer"],"properties":{"answer":{"type":"string"}}}`)
}

// Execute validates and records the final answer.
func (t *AgentDoneTool) Execute(_ context.Context, raw json.RawMessage) string {
	args, err := ParseToolCallArgs[AgentDoneArgs](raw)
	if err != nil {
		return "error: " + err.Error()
	}
	args.Answer = strings.TrimSpace(args.Answer)
	if args.Answer == "" {
		return "error: agent_done answer must be non-empty"
	}
	if t.callback != nil {
		t.callback(args.Answer)
	}
	return "completed"
}

// FormatArgs returns the answer payload for activity display.
func (t *AgentDoneTool) FormatArgs(raw json.RawMessage) string { return string(raw) }
