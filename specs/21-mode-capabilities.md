# Mode Capabilities

## Source Files

| File | Role |
|------|------|
| `internal/runtime/mode_capabilities.go` | refreshModeCapabilities, definitionsNeedRunAgent, modeAllowsAgent |
| `internal/config/config.go` | Mode struct (DeniedTools, Agents fields) |
| `internal/config/modes.go` | ModesConfig, LoadModes, DefaultMode |

## Overview

Modes define the main runtime's operational policy: which tools are available
and which one-shot agents may be called. Mode capabilities are enforced at
two levels:

1. **Tool deny-list** — removes tools from the main runtime's registry
2. **Agent allow-list** — limits which agent definitions are visible to the LLM

Child agents are NOT affected by mode policy. Each child uses its own explicit
tool allowlist from the agent definition.

## Mode Struct

```go
type Mode struct {
    Name        string   `json:"name"`                  // unique mode identifier
    Model       string   `json:"model"`                 // provider/model_name
    Directive   string   `json:"directive,omitempty"`   // volatile mode instruction
    DeniedTools []string `json:"denied_tools,omitempty"` // tools unavailable to main runtime
    Agents      []string `json:"agents,omitempty"`       // one-shot agents the runtime may call
}
```

### DeniedTools

A list of tool names that are removed from the main runtime's tool registry.
When a mode specifies `denied_tools`, those tools are completely unavailable
to the LLM in the main conversation. The LLM cannot call them and does not
see them in the tool definitions sent to the provider.

Validation at mode switch time:
- Each denied tool name must exist in the base tool registry
- Unknown tool names cause a hard error (no silent ignore)

### Agents

A list of agent definition names that the main runtime may call via `run_agent`.
Only agents listed here appear in the prompt's AGENTS section and are available
for delegation.

Validation at mode switch time:
- Each agent name must match a loaded one-shot definition
- Unknown agent names cause a hard error (no silent ignore)

## refreshModeCapabilities

Called during agent construction and on every mode switch.

```
refreshModeCapabilities()
  ├─ Validate base tool registry is set
  ├─ Validate current mode is set
  ├─ Build denied set from mode.DeniedTools
  ├─ Validate each denied name exists in BaseTools
  ├─ Filter BaseTools → Tools (exclude denied)
  ├─ Build allowed agents set from mode.Agents
  ├─ Validate each agent name matches a loaded definition
  ├─ Filter Definitions → Builder.Agents (include only allowed)
  └─ Return error if any validation fails
```

### Tool Filtering

```
Tools = BaseTools.Filter(allowedNames)
```

Where `allowedNames` = all BaseTools names minus DeniedTools names.
The filtered registry is what `RunTurn` uses for tool execution.

### Agent Filtering

```
Builder.Agents = allowed definitions
```

Only agents listed in `mode.Agents` are passed to the prompt builder.
The prompt's AGENTS section only describes allowed agents, so the LLM
cannot discover or call agents that the mode does not permit.

## definitionsNeedRunAgent

Reports whether `run_agent` should be registered as a base tool.

```
definitionsNeedRunAgent(definitions)
  └─ Returns true if any definition has Kind == KindOneShot
```

`run_agent` is registered in the base tool registry only when at least
one valid one-shot agent definition exists. If no definitions are loaded,
the tool is not available at all.

## modeAllowsAgent

Called by `runAgent` before executing any child task. Checks whether the
current mode explicitly permits the named agent.

```
modeAllowsAgent(name)
  ├─ If CurrentMode is nil → false (child agents cannot have modes)
  ├─ Search mode.Agents for name
  └─ Return true if found
```

This is a hard gate — `runAgent` returns an error message if the mode
does not allow the agent, rather than executing it silently.

## Interaction With Prompt Builder

`refreshModeCapabilities` updates `Builder.Agents` with only the allowed
definitions. The prompt builder uses this list to:

1. Generate the AGENTS section in the system prompt
2. Describe available agents, their tools, and run/completion instructions
3. Inject agent-specific content into the prompt

When `Builder.Agents` is empty (mode allows no agents), the AGENTS section
is omitted from the prompt entirely.

## Error Handling

All mode capability errors are hard errors — the runtime stops with a clear
message rather than silently ignoring invalid configuration:

- Unknown denied tool → `"mode %q denies unknown tool %q"`
- Unknown allowed agent → `"mode %q allows unknown one-shot agent %q"`
- Base tools not configured → `"base tool registry is not configured"`
- Current mode not set → `"current mode is not configured"`
- Filter failure → `"cannot apply mode %q tool policy: %w"`

## Example Mode Configurations

### Read-Only Planning Mode

```json
{
  "name": "planning",
  "model": "openai/o3",
  "directive": "Analyze and plan only. Do not execute changes.",
  "denied_tools": ["shell", "replace_block", "write_file"],
  "agents": ["coder"]
}
```

The LLM can read files and analyze code but cannot execute commands or write.
It can delegate implementation to the `coder` agent.

### Full-Access Default Mode

```json
{
  "name": "default",
  "model": "openai/gpt-4o",
  "directive": "Be concise and direct. Prefer shell execution."
}
```

No denied tools, no agent restrictions. All 12+ tools and all agents available.

### Restricted Review Mode

```json
{
  "name": "review",
  "model": "openai/o3",
  "directive": "Review code for issues. Do not modify files.",
  "denied_tools": ["shell", "replace_block", "write_file", "task_write"],
  "agents": []
}
```

Read-only with no delegation. The LLM can only read, ask, and analyze.
