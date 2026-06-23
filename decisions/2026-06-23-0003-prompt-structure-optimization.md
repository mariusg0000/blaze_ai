# Session Decision Summary: prompt-structure-optimization-and-debug-output

Date: 2026-06-23
Base commit: b018522

## Context
Two requests from the user:
1. Debug visibility: save the full built prompt JSON (what's sent to the LLM) to a file in the session folder on every call, with human-readable newlines instead of JSON-escaped `\n` sequences.
2. Prompt structure optimization: the host helpers section was logically misplaced between memory and skills. The user noted it describes host/OS capabilities, so it belongs near the OS-specific prompt section. Additionally, the universal sysprompt contained a contradictory instruction ("Prefer OS-native shell commands") that could discourage use of detected helper utilities like rg, fd, jq.

## Changes Made

### Debug output
- Added `prompt.json` write in `runtime.RunTurn`: after building the full prompt and stripping reasoning, the complete message array (exactly what the LLM receives) is written to `{session_folder}/prompt.json`.
- Applied `strings.ReplaceAll(data, "\\n", "\n")` to convert JSON-escaped newlines to real newlines for human readability. This makes the file technically invalid JSON (raw newlines in strings) but far more readable for debugging.

### Prompt structure
- Moved helper section from position 5 (between memory and skills) to position 3 (immediately after OS-specific sysprompt, before AGENTS.md). New order: universal → OS → helpers → AGENTS → memory → skills.
- Renamed headings for clarity:
  - `## Available Host Helpers` → `## Host Environment Helpers`
  - `## Optional Host Helpers` → `## Optional Host Environment Helpers`
- Rewrote execution model instructions to eliminate contradiction:
  - Old: "Prefer OS-native shell commands for simple tasks" (conflicts with helper use)
  - New: "Prefer direct shell-native execution for simple tasks" + "Use detected host helper utilities when they make commands faster, clearer, or safer" + "Do not assume optional helpers exist unless they are listed in Host Environment Helpers"

### Test updates
- Updated order tests (`TestBuildRuntimePartOrder`, `TestBuildRuntimePartHelperOrder`) to verify new placement: OS < helpers < AGENTS < skills.
- Updated all string matches from old heading names to new heading names in helpers_test.go and prompt_test.go.
- Added `HelperLookup` injection to `TestBuildRuntimePartOrder` so the helpers section renders with detected helpers.

## Decisions And Rationale
- `prompt.json` is written AFTER reasoning stripping, so the file shows what the LLM actually receives, not the raw pre-strip build.
- `prompt.json` write errors are silently ignored (debug feature, must never break the runtime).
- The heading change to "Host Environment Helpers" emphasizes that helpers describe the execution environment, not project resources or skills — exactly what the user wanted.
- The execution model now explicitly permits helper use while still favoring direct shell-native execution as the primary approach.

## Files Included
- `internal/runtime/runtime.go` — added prompt.json debug write with newline unescaping
- `internal/helpers/helpers.go` — renamed headings in BuildPromptSection
- `internal/helpers/helpers_test.go` — updated heading name assertions
- `internal/prompt/prompt.go` — moved helpers section before AGENTS.md, updated comment
- `internal/prompt/prompt_test.go` — updated order tests for new placement, renamed heading assertions
- `prompts/sysprompt.md` — rewritten execution model (3 lines), renamed heading reference

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
