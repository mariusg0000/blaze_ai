# Skill System

## Source Files

| File | Role |
|------|------|
| `internal/skills/skills.go` | Skill struct, Parse, ActiveList, DiscoverAll, Resolve, SeedBuiltins |
| `internal/tools/skill_tools.go` | load_skill, unload_skill, run_skill tool implementations |
| `internal/skills/skills_test.go` | Unit tests for parsing, discovery, resolution |

## Overview

Skills extend the agent's capabilities in three ways:
- `[BEHAVIOR]` — instructions that shape how the agent behaves when the skill is loaded
- `[DATA]` — persistent knowledge data that the agent reads when the skill is loaded
- `[CODE]` — runnable code that executes via `run_skill` tool (dynamic tool system)

There is no separate memory subsystem. Skills with `[DATA]` sections are the memory
mechanism. Skills with `[CODE]` sections are the dynamic tool system.

## Skill Format

Skills are Markdown files with section markers at the start of a line:

```markdown
[DESCRIPTION]
Find Java project files, build files, and Maven/Gradle configurations.

[BEHAVIOR]
When searching for Java project files, use rg with these patterns:
- `rg --type java "<pattern>"` for Java code
- Search for `pom.xml` or `build.gradle` to find build files
- Use `fd -e java` for file discovery

[DATA]
Known project patterns:
- Maven: pom.xml at project root
- Gradle: build.gradle at project root
- Module: module-info.java in source tree

[SYNTAX]
find_java <dir> <pattern>

[CODE]
```shell
rg --type java "{{PATTERN}}" {{DIR}} 2>/dev/null || echo "no matches"
```
```

### Section Rules

| Section | Required | Content |
|---------|----------|---------|
| `[DESCRIPTION]` | Yes | Short description for the available-skills list (one line) |
| `[BEHAVIOR]` | No | Instructions for the agent when loaded |
| `[DATA]` | No | Data/knowledge for the agent when loaded |
| `[SYNTAX]` | No | Syntax description for runnable skills (shown in prompt) |
| `[CODE]` | No | Fenced code block with runnable code (shell only in v1) |

A skill must provide at least one of:
- Prompt content: `[BEHAVIOR]` or `[DATA]` (or both)
- Runnable pair: Both `[SYNTAX]` and `[CODE]`

Section markers must appear at the start of a line (position 0 or after `\n`).
Escaped markers like `\[BEHAVIOR\]` remain literal text and do not open sections.

### CODE Section Format

```
[CODE]
```<language>
<code body>
```
```

Fence rules:
- Must start with ` ``` ` at the beginning of the section content
- Language identifier required on the opening fence line
- Closing fence ` ``` ` must be on its own line
- No text allowed after the closing fence
- v1 only supports `shell` language

Parsing errors are stored in `Skill.CodeError` — the skill is still discoverable
but `IsRunnable()` returns false.

## Skill Scopes

| Scope | Directory | Prefix | Priority |
|-------|-----------|--------|----------|
| Builtin | Embedded in binary (seeded to `app_home/skills/`) | `global/` | Lowest |
| Global | `app_home/skills/<name>/skill.md` | `global/` | Medium |
| Project | `app_home/projects/<project>/skills/<name>/skill.md` | `project/` | Highest |

All scopes are read every prompt build. Priority only matters for name resolution:
project skills shadow global skills on name collision.

### Builtin Skills

Embedded in the binary via `//go:embed skills/*`. Seeded to `app_home/skills/` at
startup if not already present. Existing files are never overwritten. Deleting a
seeded skill and restarting restores the original.

Current builtins:
- `skill-manager.md` — creates new skills, edits existing ones, validates format
- `customize-me.md` — LLM-assisted configuration of modes, roles, providers
- `session-retrospective.md` — retrospective session review via shell + ask_a_friend
- `project-map.md` — project context initialization

### Discovery Layout

Each skill lives in a subdirectory: `<scope>/<name>/skill.md`. The directory name
is the skill name used in resolution.

```
app_home/skills/
  customize-me/
    skill.md
    docs/
      modes.md
      providers.md
```

The subtree (`docs/` above) is copied alongside the skill file during builtin
seeding.

## Active Skills List

`ActiveList` is an in-memory slice of scoped skill IDs (e.g. `"global/customize-me"`,
`"project/my-tools"`). Properties:
- Starts empty at session start
- Modified only by `load_skill` / `unload_skill` tools
- NOT persisted in session JSON
- NOT deduced from message history

Two tools manage the active list:
- `load_skill(name)` — resolves name, adds canonical ID to active list
- `unload_skill(name)` — removes name from active list (by bare name and resolved ID)

## Name Resolution

`Resolve(name, allSkills)`:
- If name starts with `project/` → look up `project/<name>` in map
- If bare name → resolve as `global/<name>`
- If not found → error

The `.md` suffix is stripped from names before resolution.

Session-learning-review uses `ask_a_friend` for the review phase, and uses `shell`
for extracting files. It runs outside the active skills system.

## Runnable Skills (Dynamic Tools)

Skills with valid `[SYNTAX]` and `[CODE]` (shell) sections are runnable via
`run_skill(name, arguments)`. The tool:
1. Resolves the skill name
2. Validates `CodeLang == "shell"` and code is parseable
3. Executes the code via `executeShell()` with four environment variables:
   - `BLAZE_SKILL_ARGS` — the raw arguments string from the tool call
   - `BLAZE_SKILL_DIR` — the skill's directory path
   - `BLAZE_SKILL_ID` — the canonical scoped skill ID
   - `BLAZE_SKILL_NAME` — the display name (without scope prefix)
4. Runs in the agent's current work directory

Same timeout and output cap as regular shell tool (default 60s, 150kB).

## Prompt Integration

Skills appear in three places in the runtime prompt:

### Available Skills

Compact list in `{SKILLS_AVAILABLE}`:

```
- customize-me = LLM-assisted configuration of modes, roles, providers
- skill-manager = create and edit skills, validate skill format
```

Only skills with `HasPromptContent()` (non-empty Behavior or Data) appear here.

### Runnable Skills Section

In `{RUNNABLE_SKILLS_SECTION}`:

```
[RUNNABLE SKILLS]

run_skill(name, arguments)

- find-java | args: <dir> <pattern> | Find Java project files
```

Only skills with `IsRunnable()` appear here.

### Active Skills

In `{SKILLS_ACTIVE}` when loaded:

```
### customize-me

[BEHAVIOR]
behavior content here

[DATA]
data content here
```

All sections use variable injection (`{APP_HOME}`, `{SKILL_DIR}`, etc.) before
insertion into the prompt.

## Variable Injection in Skills

Skills can reference these variables in their content:

| Variable | Resolved To |
|----------|-------------|
| `{APP_HOME}` | `$HOME/blazeai` |
| `{WORK_DIR}` | Current working directory |
| `{OS_INFO}` | OS description from `platform.OSInfo()` |
| `{SKILL_DIR}` | Skill's absolute directory path |
| `{GLOBAL_SKILLS_DIR}` | `app_home/skills/` |
| `{PROJECT_SKILLS_DIR}` | `app_home/projects/<project>/skills/` |

Variables are resolved at prompt build time, not at skill parse time.
