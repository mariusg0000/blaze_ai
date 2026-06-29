# Prompts

## Source Files

| File | Role |
|------|------|
| `prompts/sysprompt.md` | Universal system prompt (shared across OSes) |
| `prompts/sysprompt.linux.md` | Linux-specific additions |
| `prompts/sysprompt.darwin.md` | macOS-specific additions |
| `prompts/sysprompt.windows.md` | Windows-specific additions |
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
3. **Host helpers advisory** — helper verification status (optional)
4. **Host helpers available** — live-detected and available helpers (optional)
5. **Host helpers optional** — missing helpers the user may want to install (optional)
6. **Skills available** — descriptions of all discovered skills (optional)
7. **Runnable skills section** — skills with [CODE] sections and their syntax (optional)
8. **Skills active** — [BEHAVIOR] and [DATA] of loaded skills (optional)
9. **Project map** — `project-map.md` from work folder (optional)
10. **AGENTS.md** — project rules from work folder (optional)

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
[TRANSPORT]            — {TRANSPORT_CONTEXT} injection point
[OUTPUT STYLE]         — Markdown rules, emoji guidance, structure
[COMMUNICATION PROTOCOL] — message optimization rules (from AGENTS.md)
[SKILLS]               — skill usage rules, {SKILLS_AVAILABLE}, {SKILLS_ACTIVE}, {RUNNABLE_SKILLS_SECTION}
[SECONDARY MODEL CONSULTATION] — ask_a_friend guidance
[HOST ENVIRONMENT HELPERS] — {HOST_HELPERS_ADVISORY}, available, optional
[PROJECT RULES]        — {AGENTS_CONTENT}
[PROJECT MAP]          — {PROJECT_MAP_CONTENT}
```

## OS-Specific Prompts

Each OS file is short (~12 lines) and covers:

- Platform identification
- Shell preference (bash/sh for Linux/macOS, pwsh/powershell/cmd for Windows)
- Path separator character
- Environment variable syntax
- Script storage conventions
- OS-specific notes (macOS coreutils, Windows case-insensitive paths, etc.)

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
| `{TRANSPORT_CONTEXT}` | `Builder.TransportContext` | empty string (set per transport) |
| `{SKILL_DIR}` | Skill's directory (per-skill injection) | `"NULL"` if not in skill context |
| `{OS_PROMPT}` | OS-specific sysprompt content | injected directly |
| `{HOST_HELPERS_ADVISORY}` | helper verification status | empty string |
| `{HOST_HELPERS_AVAILABLE}` | detected + available helpers | empty string |
| `{HOST_HELPERS_OPTIONAL}` | missing helpers (undismissed) | empty string |
| `{SKILLS_AVAILABLE}` | all discovered skill descriptions | empty string (allows empty) |
| `{RUNNABLE_SKILLS_SECTION}` | runnable skills [SYNTAX] list | empty string (allows empty) |
| `{SKILLS_ACTIVE}` | loaded skills [BEHAVIOR]/[DATA] | empty string (allows empty) |
| `{PROJECT_MAP_CONTENT}` | project-map.md content | empty string |
| `{AGENTS_CONTENT}` | AGENTS.md content with variable injection | empty string |

Variables that are `"NULL"` render literally as `NULL` in the prompt — the LLM
sees a clear indicator that a value is missing.

Section-level variables (`{SKILLS_AVAILABLE}`, `{SKILLS_ACTIVE}`,
`{RUNNABLE_SKILLS_SECTION}`) allow empty resolution — when empty, their
entire section (including surrounding context like "**Active skills:**") is
removed from the prompt rather than showing an empty section.

### Escape Sequences

| Escape | Rendered As |
|--------|-------------|
| `\{` | `{` |
| `\}` | `}` |
| `\[` | `[` (prevents Markdown section header rendering) |
| `\]` | `]` |

### Template-Specific (Extra) Variables

The `buildSkillsSection()` method injects per-skill directory resolution before
injecting content into `{SKILLS_AVAILABLE}` and `{SKILLS_ACTIVE}` sections.
Skills can reference `{SKILL_DIR}` in their [BEHAVIOR] or [DATA] sections to
self-reference their directory for file includes or skill-local resources.

## Prompt Build Sequence in Code

```
Builder.Build(session, activeSkills)
  ├─ Builder.BuildRuntimePart(activeSkills)
  │    ├─ 1. readFileRequiredFS("sysprompt.md") → universal
  │    ├─ 2. readFileRequiredFS("sysprompt.{os}.md") → osPrompt
  │    │    └─ injectVariables(osPrompt)
  │    ├─ 3. buildHostHelpersAdvisory()
  │    ├─ 4. helpers.Detect(lookup) → statuses
  │    │    └─ buildHostHelpersSection(statuses)
  │    ├─ 5. buildSkillsSection(active)
  │    │    ├─ skills.DiscoverAll(workDir) → all skills
  │    │    ├─ Format available skills list
  │    │    ├─ Format runnable skills section
  │    │    └─ Format active skills content
  │    ├─ 6. readFileOptional("project-map.md")
  │    ├─ 7. readFileOptional("AGENTS.md")
  │    │    └─ injectVariables(agents)
  │    └─ 8. injectTemplateVariables(universal, {
  │            OS_PROMPT, HOST_HELPERS_ADVISORY, HOST_HELPERS_AVAILABLE,
  │            HOST_HELPERS_OPTIONAL, SKILLS_AVAILABLE, RUNNABLE_SKILLS_SECTION,
  │            SKILLS_ACTIVE, PROJECT_MAP_CONTENT, AGENTS_CONTENT
  │         })
  └─ return []Message{system(runtimePart)} + session.Messages
```

## Debug Artifact

After the full prompt is built and reasoning is stripped (the exact payload the
LLM receives), an artifact is written to `{session_folder}/prompt.json`. This is
a human-readable version of the message array with `\n` unescaped (technically
invalid JSON but readable). Write errors are silently ignored — the debug file
must never break the runtime.

## Skill Content in Prompts

### Available Skills Section

Compact one-line-per-skill format:
```
- skill-name = Description text
- another-skill = Description text
```

Only skills with `HasPromptContent()` (i.e., they have a non-empty [DESCRIPTION]
and at least one of [BEHAVIOR], [DATA], or [CODE]) appear in the available list.

### Runnable Skills Section

```
[RUNNABLE SKILLS]

run_skill(name, arguments)

- skill-name | args: <text> | Description text
```

Only skills with `IsRunnable()` (valid [CODE] section) appear here.

### Active Skills Section

Full Markdown sections with skill name as heading, followed by [BEHAVIOR] and
[DATA] blocks if present:

```
### skill-name

[BEHAVIOR]
behavior content here

[DATA]
data content here
```

Active skills are sorted by their scoped ID (project/ prefix before global/builtin).

## Host Helpers in Prompts

### Advisory (first-run only)

```
helper_setup = unverified
task could benefit from host helpers → suggest verification_or_setup
guidance needed → load_skill setup_helpers
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
before install" and "install guidance → load_skill setup_helpers".

Empty if all core helpers are present or user dismissed.
