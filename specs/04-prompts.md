# Prompts

## Source Files

| File | Role |
|------|------|
| `prompts/sysprompt.md` | Universal system prompt (shared across OSes) |
| `prompts/sysprompt.linux.md` | Linux-specific additions |
| `prompts/sysprompt.darwin.md` | macOS-specific additions |
| `prompts/sysprompt.windows.md` | Windows-specific additions |
| `prompts/transport.console.md` | Console-specific formatting and interaction rules |
| `prompts/transport.telegram.md` | Telegram-specific formatting and interaction rules |
| `internal/prompt/prompt.go` | Builder struct, Build(), BuildRuntimePart(), variable injection |
| `internal/prompt/doc.go` | Package docs |
| `internal/helpers/helpers.go` | Helper detection for {HOST_HELPERS_*} injection |
| `embeddedPrompts` (embed.go) | Go `//go:embed prompts/*` — embedded at compile time |

## Prompt Philosophy

Prompt behavior is a major source of product personality and control. The runtime
is deliberately minimal — most agent behavior is shaped by prompt templates, not
Go code. The prompt is rebuilt from disk on every LLM call; nothing is cached or
reused.

All prompt templates are embedded in the binary via `//go:embed` directives at
compile time, making the application a single self-contained executable. The
embedded filesystem is passed to the prompt builder at startup.

## Build Order

The full prompt is assembled in two parts: runtime part (static) + conversation
part (session history).

### Runtime Part (rebuilt every LLM call)

1. **Universal sysprompt** — `sysprompt.md` (required, fails if missing)
2. **OS-specific sysprompt** — `sysprompt.{os}.md` (required, fails if missing)
3. **Transport prompt** — `transport.{name}.md` selected by `Builder.TransportName` (required, fails if missing)
4. **Host helpers advisory** — helper verification status (optional)
5. **Host helpers available** — live-detected and available helpers (optional)
6. **Host helpers optional** — missing helpers the user may want to install (optional)
7. **Skills available** — descriptions of all discovered skills (optional)
8. **Project context** — `specs.md` from work folder (optional)
9. **AGENTS.md** — project rules from work folder (optional)

All optional sections disappear entirely if their content is empty (no empty
placeholders or stale headers).

### Conversation Part

Session message history appended as-is after the runtime part. Prepended as a
single `system`-role message before the user/assistant/tool message array.

```
messages = [
  { role: "system", content: runtimePart },
  // ... session.Messages ...
]
```

## Prompt Template Structure (sysprompt.md)

The universal sysprompt is an 112-line Markdown file with labelled sections and
`{VARIABLE}` injection points:

```
[IDENTITY]             — agent identity statement
[ENVIRONMENT]          — OS, paths, working directory
[SAFETY]               — destructive commands, backups, sudo, passwords
[OS PROMPT]            — {OS_PROMPT} injection point
[TRANSPORT]            — {TRANSPORT_PROMPT} + {TRANSPORT_CONTEXT}
[OUTPUT STYLE]         — transport override rule + shared structure guidance
[COMMUNICATION PROTOCOL] — message optimization rules (from AGENTS.md)
[SKILLS]               — skill usage rules, {SKILLS_AVAILABLE}
[SECONDARY MODEL CONSULTATION] — ask_a_friend and analyze_image guidance
[HOST ENVIRONMENT HELPERS] — {HOST_HELPERS_ADVISORY}, available, optional
[PROJECT RULES]        — {AGENTS_CONTENT}
[PROJECT CONTEXT]      — {PROJECT_CONTENT}
```

## OS-Specific Prompts

Each OS file is short (~12 lines) and covers:

- Platform identification
- Shell preference (bash/sh for Linux/macOS, pwsh/powershell/cmd for Windows)
- Path separator character
- Environment variable syntax
- Script storage conventions
- OS-specific notes (macOS coreutils, Windows case-insensitive paths, etc.)

## Transport-Specific Prompts

Each transport file is short and focused on transport-only constraints:

- `transport.console.md` — terminal Markdown subset, streaming behavior, emoji guidance
- `transport.telegram.md` — plain-text chat formatting, narrow-screen constraints, no Markdown reliance

Core behavior stays in `sysprompt.md`; transport files must not duplicate safety,
tool, skill, or project rules.

## Variable Injection

Variables are resolved at build time via `injectTemplateVariables()`. The function
handles escape sequences (`\{`, `\}`, `\[`, `\]`), empty-value handling, and
template-specific extras.

### Built-in Variables

| Variable | Source | Empty Handling |
|----------|--------|---------------|
| `{APP_HOME}` | `platform.AppHome()` | `"NULL"` if unresolvable |
| `{GLOBAL_SKILLS_DIR}` | `app_home/skills/` | `"NULL"` if unresolvable |
| `{PROJECT_SKILLS_DIR}` | `platform.ProjectDir()` + `/skills` | `"NULL"` if unresolvable |
| `{WORK_DIR}` | `Builder.WorkDir` | `"NULL"` if empty |
| `{OS_INFO}` | `platform.OSInfo()` | `"NULL"` if empty |
| `{TRANSPORT_PROMPT}` | `transport.{name}.md` rendered through `Builder.TransportName` | required; build fails if missing |
| `{TRANSPORT_CONTEXT}` | `Builder.TransportContext` | empty string (set per transport) |
| `{SKILL_DIR}` | Skill's directory (per-skill injection) | `"NULL"` if not in skill context |
| `{OS_PROMPT}` | OS-specific sysprompt content | injected directly |
| `{HOST_HELPERS_ADVISORY}` | helper verification status | empty string |
| `{HOST_HELPERS_AVAILABLE}` | detected + available helpers | empty string |
| `{HOST_HELPERS_OPTIONAL}` | missing helpers (undismissed) | empty string |
| `{SKILLS_AVAILABLE}` | all discovered skill descriptions | empty string (allows empty) |
| `{AGENTS_AVAILABLE}` | discovered agent definitions (runtime prompt) or empty (agent prompt) | empty string (allows empty) |
| `{AGENT_INSTRUCTIONS}` | agent's Markdown body instructions (for child agents only) | empty string (allows empty) |
| `{AGENT_TASK}` | agent_task.md content (for resumed child agents only) | empty string (allows empty) |
| `{PROJECT_CONTENT}` | specs.md content | empty string |
| `{AGENTS_CONTENT}` | AGENTS.md content with variable injection | empty string |

Variables that are `"NULL"` render literally as `NULL` in the prompt — the LLM
sees a clear indicator that a value is missing.

The section-level variable `{SKILLS_AVAILABLE}` allows empty resolution, so no empty available-skills content is rendered.

### Escape Sequences

| Escape | Rendered As |
|--------|-------------|
| `\{` | `{` |
| `\}` | `}` |
| `\[` | `[` (prevents Markdown section header rendering) |
| `\]` | `]` |

### Template-Specific (Extra) Variables

The `buildSkillsSection()` method injects per-skill variables into descriptions before `{SKILLS_AVAILABLE}` is rendered. `RenderSkillBody()` applies the same variable expansion, including `{SKILL_DIR}`, immediately before `load_skill` returns a body.

## Prompt Build Sequence in Code

```
Builder.Build(session)
  ├─ Builder.BuildRuntimePart()
  │    ├─ 1. readFileRequiredFS("sysprompt.md") → universal
  │    ├─ 2. readFileRequiredFS("sysprompt.{os}.md") → osPrompt
  │    ├─ 3. readFileRequiredFS("transport.{name}.md") → transportPrompt (main runtime only)
  │    ├─ 4. buildHostHelpersAdvisory()
  │    ├─ 5. helpers.Detect(lookup) → statuses
  │    │    └─ buildHostHelpersSection(statuses)
  │    ├─ 6. buildSkillsSection()
  │    │    ├─ skills.DiscoverAll(workDir) → all skills
  │    │    └─ Format available skills list
  │    ├─ 7. buildAgentsSection() → AGENTS_AVAILABLE (from Builder.Agents, mode-filtered)
  │    ├─ 8. readFileOptional("specs.md") → PROJECT_CONTENT
  │    ├─ 9. readFileOptional("AGENTS.md") → AGENTS_CONTENT (with variable injection)
  │    ├─ 10. Read AgentTaskFile (for resumed child agents)
  │    ├─ 11. Inject all template values into osPrompt → templateValues.OS_PROMPT
  │    ├─ 12. Inject all template values into transportPrompt → templateValues.TRANSPORT_PROMPT
  │    └─ 13. injectTemplateVariables(universal, templateValues) → final runtime part
  └─ return []Message{system(runtimePart)} + session.Messages
```

### Agent Prompt Differences

When `SystemPromptName == "sysprompt.agent.md"` (child agent runtime):
- Transport prompt is skipped entirely (no `transport.{name}.md`)
- `AgentInstructions` is injected as `{AGENT_INSTRUCTIONS}` from the definition body
- `AgentTaskFile` content is injected as `{AGENT_TASK}` from `agent_task.md`
- `AGENTS_AVAILABLE` comes from `Builder.Agents` (mode-filtered for main, nil for children)

## Debug Artifact

When `config.debugPrompt` is `true`, the runtime writes the exact fully built payload sent to the LLM to `{session_folder}/prompt.json` before each LLM call. When the field is omitted or `false`, no prompt debug artifact is written.

## Skill Content and Message Loading

The system prompt contains compact one-line descriptions only:

```text
- skill-name = Description text
- another-skill = Description text
```

Every discovered skill has already passed strict `[DESCRIPTION]` and `[BODY]` validation. Skill bodies never enter the system prompt. When `load_skill` runs, `RenderSkillBody()` expands body variables and the tool returns the body through the ordinary tool-result message path. The persisted result then participates in session resume and compaction like any other conversation message.

## Host Helpers in Prompts

### Advisory (first-run only)

```
helper_setup = unverified
task could benefit from host helpers → suggest verification_or_setup
guidance needed → load_skill config-manager
all helpers verified ∨ user declines → reminder stops
```

Empty if `HelperSetup.Dismissed` is true.

### Available Helpers

```
- rg = fast recursive code and text search
- fd = fast file and directory discovery
- jq = query, filter, and transform JSON
```

Only helpers found on PATH via `exec.LookPath`.

### Optional Helpers

Same format but for missing helpers. Only shown if `HelperSetup.Dismissed` is false.
Includes guidance line: "helper would materially help → explain benefit + ask user
before install" and "install guidance → load_skill config-manager".

Empty if all core helpers are present or user dismissed.
