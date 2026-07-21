# Agent Orchestration

## Source Files

| File | Role |
|------|------|
| `internal/runtime/agent_orchestration.go` | One-shot child-agent execution, persistent child sessions, dual timeout, activity forwarding |
| `internal/tools/agent_tools.go` | RunAgentTool (run_agent), AgentDoneTool (agent_done), RunAgentArgs, RunAgentTask |
| `internal/agents/agents.go` | Definition struct, Load, ParseFile, validateDefinition |

## Overview

Agent orchestration runs Markdown-defined one-shot child agents independently
from the main runtime. Each child gets its own session, tool registry, model,
and bounded execution context. Results are returned in order to the parent.

## Agent Definition Format

```markdown
---
name: coder
description: Write and modify code files
kind: one-shot
model: openai/gpt-4o
timeout: 10m
tools:
  - shell
  - read_file
  - write_file
  - replace_block
---

You are an expert code writer. Follow these rules:
- Always read before writing
- Use replace_block for edits
...
```

### Front Matter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique agent identifier |
| `description` | Yes | Human-readable description |
| `kind` | Yes | Must be `"one-shot"` (only supported kind) |
| `model` | No | `provider/model_name` (inherits parent model if empty) |
| `timeout` | No | Go duration string (default 20 minutes) |
| `tools` | Yes | Explicit tool allowlist (cannot be empty; `agent_done` auto-included) |

### Validation Rules

- `name` must be non-empty and unique across all definitions
- `description` must be non-empty
- `kind` must be `"one-shot"` — interactive agents are not supported
- `model` must be in `provider/model_name` format if provided
- `timeout` must be positive if provided
- `tools` allowlist must not be empty
- `run_agent` cannot appear in a child's tools allowlist (no recursion)
- `agent_done` is auto-included and does not need to be listed
- All tool names must exist in the base tool registry

### Definition Loading

Definitions are loaded from `app_home/agents/` at agent construction time.
Only `.md` files directly in the directory are parsed (no recursion).
Definitions are sorted alphabetically by filename.
Duplicate names across files cause a hard error at startup.

## Tool Signatures

### run_agent

```go
type RunAgentArgs struct {
    Purpose string         `json:"purpose"`           // required — 3-sentence explanation
    Agent   string         `json:"agent,omitempty"`   // single task: agent name
    Task    string         `json:"task,omitempty"`    // single task: task description
    Context string         `json:"context,omitempty"` // single task: optional context
    ID      string         `json:"id,omitempty"`      // single task: resume session ID
    Tasks   []RunAgentTask `json:"tasks,omitempty"`   // parallel tasks
}

type RunAgentTask struct {
    Agent   string `json:"agent"`            // required — agent name
    Task    string `json:"task"`             // required — task description
    Context string `json:"context,omitempty"`// optional context
    ID      string `json:"id,omitempty"`     // optional resume session ID
}
```

### agent_done

```go
type AgentDoneArgs struct {
    Answer string `json:"answer"` // required, non-empty
}
```

## Execution Flow

```
runAgent(ctx, args)
  ├─ Resolve tasks: single (args.Agent/Task) or parallel (args.Tasks)
  ├─ Validate mode allows each requested agent
  ├─ Bounded parallel execution (maxParallelChildren = 4)
  │    ├─ Semaphore limits concurrent children
  │    ├─ Per-child: runOneChild(ctx, task, displayID, childID)
  │    └─ Ordered results preserved by index
  ├─ Single result → return directly
  └─ Multiple results → formatOrderedResults (numbered list)
```

### runOneChild

```
runOneChild(parentCtx, task, displayID, childID)
  ├─ Resolve one-shot definition by name
  ├─ Validate task is non-empty
  ├─ openChildSession(mainFolder, childID)
  │    ├─ Existing folder → Load and resume session
  │    └─ New folder → Create empty session
  ├─ Write agent_task.md (new) or preserve (resume)
  ├─ Create child agent via newChildAgent()
   │    ├─ Inherits: config, OS, promptsFS, builtinSkillsFS, workDir, transport
   │    ├─ No modes (nil) — child has own tool allowlist
   │    └─ Model: definition.Model or parent's ModelID
   ├─ Configure child builder:
   │    ├─ SystemPromptName = "sysprompt.agent.md"
   │    ├─ AgentInstructions = definition.Instructions
   │    ├─ AgentTaskFile = taskPath
   │    └─ Agents = nil (no recursive agents)
  │    Child prompt list and load_skill behavior use the same immutable builtin catalog as the parent.
  ├─ Build filtered tool registry from definition.ToolNames
  │    └─ Always includes agent_done
  ├─ Dual timeout:
  │    ├─ Overall: definition.Timeout or 20min default (never resets)
  │    └─ Inactivity: 2min (resets on every tool call/result)
  ├─ Activity forwarder: wraps handler, signals channel on tool events
  ├─ child.RunTurn(ctx, input)
  │    └─ input = buildChildInput(task, resumed)
  ├─ Extract result:
   │    ├─ completion (from agent_done callback) → trimmed and returned completely
  │    └─ fallback: lastAssistantAnswer(childSession)
  └─ formatChildResult(name, childID, warning, answer)
```

## Persistent Child Sessions

Child sessions persist under the main session folder:

```
app_home/projects/<project>/sessions/<main-session>/
  session.json
  agents/
    abc12/
      session.json
      agent_task.md
    def34/
      session.json
      agent_task.md
```

- Session ID: 5-character hex string (e.g., `a1b2c`)
- Display ID: `[agent_name_id]` (e.g., `[coder_a1b2c]`)
- `agent_task.md`: stores the original task for system prompt injection on resume

### Resume Protocol

When a child is resumed with an existing session ID:
1. Existing session is loaded (conversation history preserved)
2. `agent_task.md` is NOT overwritten (original task preserved)
3. New task is sent as a `[RESUME TASK]` user message
4. Context is appended as `[CONTEXT]` if provided
5. Child continues from where it left off

### Child Input Formatting

New child:
```
[CONTEXT]
<context if provided>

Call agent_done with a concise non-empty final answer when complete.
```

Resumed child:
```
[RESUME TASK]
<new task>

[CONTEXT]
<context if provided>

Call agent_done with a concise non-empty final answer when complete.
```

## Dual Timeout

### Overall Timeout

- Default: 20 minutes (`defaultChildOverallTimeout`)
- Override: definition's `timeout` field
- Never resets — hard cap on child execution
- On expiry: child context cancelled, error propagated to parent

### Inactivity Timeout

- Default: 2 minutes (`childInactivityTimeout`)
- Resets on every tool call or tool result forwarded by the activity handler
- Detects stuck children that stopped producing useful work
- On expiry: child context cancelled, error reported as "timed out due to inactivity"

### Activity Forwarding

The `activityForwarder` wraps the child handler and signals a channel on each
tool call and tool result. A goroutine watches the channel and resets the
inactivity timer. Non-blocking sends prevent stalling when the buffer is full.

## Activity Events

Child lifecycle events are emitted via `AgentActivityHandler` (optional transport extension):

| Kind | Status | When |
|------|--------|------|
| `started` | `running` | Child agent begins execution |
| `completed` | `ok` | Child finished successfully |
| `failed` | `error` | Child encountered a fatal error |
| `timed out` | `error` | Child exceeded timeout (overall or inactivity) |
| `cancelled` | `error` | Parent context was cancelled |
| `tool_call` | `running` | Child is about to execute a tool |
| `tool_result` | `ok` | Child tool execution completed |
| `system` | `info` | Child system notification |

Tool events are forwarded immediately (not batched) to the parent transport
for real-time visibility.

## Result Formatting

Single child:
```
agent: <name>
child session id: <id>
This child session can be resumed later with the same agent name, this id, and a new task, if needed.

<answer>
```

Multiple children (ordered):
```
[1]
<child 1 result>

[2]
<child 2 result>
```

Warnings (no agent_done called):
```
status: completed-with-warning
agent_done was not called; the last assistant message was used.

<last assistant answer>
```

## Child Answers

Child answers are trimmed of surrounding whitespace and otherwise returned
completely before insertion into the parent context.

## Error Formatting

Failed children include resume metadata:
```
agent: <name>
child session id: <id>
<error message>
Resume with agent="<name>" and id="<id>"
```

This allows the parent model to resume a failed child with a new task.

## Mode Policy Integration

Before executing any child task, `runAgent` checks that the current mode
explicitly allows the requested agent via `modeAllowsAgent()`. This check
happens in the parent runtime, not in the child. A mode's `agents` field
controls which one-shot agents are available.

See `21-mode-capabilities.md` for mode policy details.
