# Session Decision Summary: host-helper-detection-and-prompt-injection

Date: 2026-06-23
Base commit: (latest on main)

## Context
User requested a system to detect cross-platform host helper utilities (rg, fd, jq, git, curl, etc.) and inject guidance into the LLM prompt based on live availability. Key constraints: no fallbacks, no config-as-truth for binary presence, cross-platform only helpers, Python treated as a restricted runtime (venv-only), installation guidance only via skill (not permanent prompt content), user preferences (dismissed/declined) stored in config for UX control.

## Changes Made
- Added `HelperSetup` struct to config with `Dismissed` and `Declined` fields for UX preference storage.
- Created `internal/helpers` package with a static catalog of 14 cross-platform helpers (5 core, 9 contextual), live detection via `exec.LookPath`, and a prompt section builder that respects availability, project ecosystem files, and user preferences.
- Extended `prompt.Builder` with `HelperSetup` and `HelperLookup` fields; injected the helper section into `BuildRuntimePart` between memory and skills.
- Wired `cfg.HelperSetup` into `runtime.NewAgent` so config preferences reach the prompt builder.
- Tightened the Python rule in `prompts/sysprompt.md` to require strict venv-only usage (no system Python/pip).
- Created builtin skill `skills/setup_helpers.md` with detection, install, and Python venv procedures.
- Added 21 new tests across config, helpers, and prompt packages.

## Decisions And Rationale
- Availability is detected live, never stored as truth in config. Config only stores UX preferences (dismissed/declined). If runtime says a helper exists but it later disappears, the LLM rule in the skill says to stop using it, verify, and continue with alternatives.
- Contextual helpers (go, node, cargo, docker) appear only when ecosystem files exist in the work directory. Core helpers (rg, fd, jq, git, curl) always appear when detected.
- `setup_helpers` is a separate skill, not merged into `customize_me`, because host setup is a different concern than provider/model config.
- `DefaultLookup` is a package-level variable to allow test injection without modifying the `Detect` signature.
- Avoided `rg=true`/`fd=false` flags in config entirely — this was rejected as fragile and misleading.

## Implementation Approach
- `helpers.Known` is the static catalog. `Detect(lookup)` iterates it.
- `BuildPromptSection` produces three possible variants: available helpers only, available + optional missing, or nothing (when dismissed and nothing to show).
- `BuildRuntimePart` uses an injected `HelperLookup` (defaults to `exec.LookPath`) for testability. When the lookup is nil, the builder uses the real OS lookup.
- Config `HelperSetup` is backward-compatible: old configs without the field load with zero-value (Dismissed=false, Declined=nil/empty).
- Python venv rule was moved from the one-liner "last resort, inside venv" to a detailed block mandating strict venv-only paths for all Python operations.

## Files Included
- `internal/config/config.go` — `HelperSetup` struct, field on Config, default in Default()
- `internal/config/config_test.go` — tests for default, backward-compat, round-trip
- `internal/helpers/doc.go` — package documentation (new)
- `internal/helpers/helpers.go` — catalog, detection, prompt section builder (new)
- `internal/helpers/helpers_test.go` — 10 unit tests (new)
- `internal/prompt/prompt.go` — HelperSetup/HelperLookup fields, helpers section injection
- `internal/prompt/prompt_test.go` — 6 integration tests, fake helper lookup utility
- `internal/runtime/runtime.go` — wire cfg.HelperSetup into prompt builder
- `prompts/sysprompt.md` — strict Python venv-only rule
- `skills/setup_helpers.md` — new builtin skill (new)

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
