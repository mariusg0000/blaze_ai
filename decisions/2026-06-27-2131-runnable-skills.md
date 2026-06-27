# Session Decision Summary: Runnable Skills v1

Date: 2026-06-27 21:31
Base commit: e8682cac614fade9577f4daa017ebc06a289aae4

## Context

Greenfield implementation of Runnable Skills — skills that carry executable [CODE] alongside prompt behavior, invoked via a new `run_skill` tool. Based on `ideas/runnable-skills.md`. Prioritized token-efficient syntax, no fallbacks, shell-only v1.

## Changes Made

- Added `[SYNTAX]` and `[CODE]` parsing to the skill parser (`internal/skills/skills.go`)
- Added `Syntax`, `CodeLang`, `Code`, `CodeError` fields to the `Skill` struct
- Added `HasPromptContent()` and `IsRunnable()` helper methods
- Added fenced code block parser for `[CODE]` sections with language detection
- Added `run_skill` native tool (`internal/tools/skill_tools.go`) accepting `name`, `arguments` (raw string), `timeout`
- Extracted shared `executeShell()` helper from `ShellTool` for reuse by `run_skill`
- Split prompt skills section: available skills → runnable skills → active skills
- Added `RUNNABLE_SKILLS_AVAILABLE` template placeholder
- Updated `prompts/sysprompt.md` with `[RUNNABLE SKILLS]` section
- Updated `skills/skill-manager.md` with Runnable Skills documentation and examples
- Registered `run_skill` in runtime tool registry
- Added tests for: runnable skill parsing, malformed code, prompt rendering, run_skill execution, unsupported language rejection
- Updated all existing test fixtures to include the new prompt placeholder

## Decisions And Rationale

- **Separate `run_skill` tool**: Using `load_skill` for both activation and execution would create ambiguous semantics. A dedicated tool keeps responsibilities clean.
- **Raw string `arguments`**: Token-efficient. No JSON schema per skill, no complex parsing. The model sees `[SYNTAX]` in the prompt and passes the exact string.
- **Shell-only v1**: Reuses existing shell execution infrastructure (timeout, output limiting, platform-specific shells). Python and other runtimes are explicitly excluded.
- **No builtin runnable skills**: The `skill-manager` skill documents the format instead, keeping the builtin set clean.
- **`BLAZE_SKILL_*` env vars**: The skill code receives arguments, its directory, and identity via environment variables — no CLI wrapping needed.
- **Strict fenced code validation**: [CODE] must be a fenced block with explicit language. No implicit or unfenced bodies.

## Implementation Approach

1. Extended `Parse()` in `skills.go` to extract optional `[SYNTAX]` and `[CODE]` sections, with a dedicated `parseCodeFence()` function for the fenced block.
2. `Skill` struct gained runnable fields; validation allows either prompt content or a runnable pair.
3. `buildSkillsSection()` in `prompt.go` now returns three strings: loadable skills, runnable skills, active skills.
4. `RunSkillTool` in `skill_tools.go` resolves the skill by name, validates runnability, injects `BLAZE_SKILL_*` env vars, and delegates to `executeShell()`.
5. `shell.go` extracted `executeShell()` from `ShellTool.Execute()` to share the command lifecycle (process group, timeout, output limiting).
6. Test fixtures in `console_test.go`, `runtime_test.go`, `prompt_test.go` updated to include `{RUNNABLE_SKILLS_AVAILABLE}` placeholder.

## Alternatives Considered

- **Overloading `load_skill` with an `arguments` field**: Rejected — would make tool semantics ambiguous between "activate context" and "execute code."
- **JSON object or array arguments**: Rejected — raw string uses fewer tokens and maps directly from `[SYNTAX]`.
- **Multiple code languages in v1**: Postponed. Shell covers the vast majority of use cases.

## Files Included

- `internal/console/console_test.go`: test fixture prompt updated with runnable placeholder
- `internal/prompt/prompt.go`: runnable skills section builder + new placeholder injection
- `internal/prompt/prompt_test.go`: runnable section rendering tests + fixture updates
- `internal/runtime/runtime.go`: runnable skill resolver + tool registration
- `internal/runtime/runtime_test.go`: fixture update + run_skill registration check
- `internal/skills/doc.go`: updated module doc
- `internal/skills/skills.go`: runnable parsing, code fence parser, IsRunnable/HasPromptContent
- `internal/skills/skills_test.go`: runnable parse tests + malformed code test
- `internal/tools/doc.go`: updated module doc
- `internal/tools/shell.go`: extracted shared executeShell() helper
- `internal/tools/skill_tools.go`: run_skill tool implementation
- `internal/tools/skill_tools_test.go`: run_skill execution and rejection tests
- `internal/tools/tools.go`: updated interface doc + RunSkillArgs
- `internal/tools/tools_test.go`: FormatArgs tests for run_skill
- `prompts/sysprompt.md`: new [RUNNABLE SKILLS] section
- `skills/skill-manager.md`: Runnable Skills documentation (behavior + syntax examples)
- `skills/setup_helpers.md`: pre-existing deletion (unrelated, included for repo cleanliness)
- `skills/telegram_bridge.md`: pre-existing deletion (unrelated, included for repo cleanliness)

## Commit Linkage

This summary is committed together with the implementation changes to keep rationale linked to code history.
