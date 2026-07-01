// tool_activity.go — compact desktop tool activity formatting and state.
// Builds one editable activity block like Telegram instead of dumping raw tool
// output into the transcript. Layer: transport output. Dependencies: tools.
package desktop

import (
	"encoding/json"
	"strconv"
	"strings"

	"blazeai/internal/tools"
)

type toolActivity struct {
	lines   []string
	pending []pendingToolLine
}

type pendingToolLine struct {
	callID string
	name   string
	args   string
	index  int
}

// Reset clears the in-memory editable tool activity block state.
func (a *toolActivity) Reset() {
	a.lines = nil
	a.pending = nil
}

// AddCall appends one pending tool line to the activity block.
func (a *toolActivity) AddCall(callID string, name string, args string) {
	a.lines = append(a.lines, formatPendingToolLine(name, args))
	a.pending = append(a.pending, pendingToolLine{
		callID: callID,
		name:   name,
		args:   args,
		index:  len(a.lines) - 1,
	})
}

// ApplyResult replaces the matching pending line with its compact completed form.
func (a *toolActivity) ApplyResult(callID string, name string, result string) {
	badge, detail := parseToolResultSummary(result)
	if len(a.pending) == 0 {
		a.lines = append(a.lines, formatCompletedToolLine(name, "", badge, detail))
		return
	}
	match := a.findPending(callID, name)
	if match < 0 {
		a.lines = append(a.lines, formatCompletedToolLine(name, "", badge, detail))
		return
	}
	pending := a.pending[match]
	a.pending = append(a.pending[:match], a.pending[match+1:]...)
	a.lines[pending.index] = formatCompletedToolLine(pending.name, pending.args, badge, detail)
}

// Render returns the current multiline tool activity block text.
func (a *toolActivity) Render() string {
	if len(a.lines) == 0 {
		return ""
	}
	return strings.Join(a.lines, "\n")
}

func (a *toolActivity) findPending(callID string, name string) int {
	if callID != "" {
		for i := range a.pending {
			if a.pending[i].callID == callID {
				return i
			}
		}
	}
	for i := range a.pending {
		if a.pending[i].name == name {
			return i
		}
	}
	if len(a.pending) == 0 {
		return -1
	}
	return 0
}

type replayToolCall struct {
	callID string
	name   string
	args   string
}

// decodeReplayToolCalls reads persisted assistant tool_calls into display-ready values.
func decodeReplayToolCalls(raw interface{}, registry *tools.Registry) []replayToolCall {
	if raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var calls []tools.OpenAIToolCall
	if err := json.Unmarshal(data, &calls); err != nil {
		return nil
	}
	decoded := make([]replayToolCall, 0, len(calls))
	for _, call := range calls {
		args := call.Function.Arguments
		if registry != nil {
			args = registry.FormatArgs(call.Function.Name, json.RawMessage(call.Function.Arguments))
		}
		decoded = append(decoded, replayToolCall{callID: call.ID, name: call.Function.Name, args: args})
	}
	return decoded
}

func formatPendingToolLine(name string, args string) string {
	icon := toolEmoji(name)
	if args == "" {
		return icon + " " + name + "..."
	}
	return icon + " " + args + "..."
}

func formatCompletedToolLine(name string, args string, badge string, detail string) string {
	icon := toolEmoji(name)
	var base string
	if args == "" {
		base = icon + " " + name
	} else {
		base = icon + " " + args
	}
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

// parseToolResultSummary turns raw tool output into a compact badge and detail.
func parseToolResultSummary(result string) (badge string, detail string) {
	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "timeout") {
		return "TIMEOUT", strings.TrimSpace(strings.TrimPrefix(result, "timeout"))
	}
	if strings.HasPrefix(result, "error:") {
		return "ERROR", strings.TrimSpace(strings.TrimPrefix(result, "error:"))
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
			return "ERROR", stderr
		}
		if stdout != "" {
			return "ERROR", stdout
		}
		return "ERROR", "exit code " + exitCodeStr
	}
	if strings.HasPrefix(result, "ok") {
		return "DONE", ""
	}
	return "DONE", ""
}

func extractToolSection(text string, label string) string {
	idx := strings.Index(text, label)
	if idx < 0 {
		return ""
	}
	after := strings.TrimPrefix(text[idx+len(label):], "\n")
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
