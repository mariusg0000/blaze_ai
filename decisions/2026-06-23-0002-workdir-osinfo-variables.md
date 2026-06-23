# Session Decision Summary: work-dir-and-os-info-variables

Date: 2026-06-23
Base commit: 6f571c1

## Context
User requested two new auto-populated prompt variables, injected identically to {APP_HOME}:
- {WORK_DIR} — the current working directory, set at app start
- {OS_INFO} — a human-readable OS description (name + version when available)
These were to be placed in a context sentence at the very beginning of the universal sysprompt.

## Changes Made
- Added `platform.OSInfo()` function that detects the current OS name and version: reads `/etc/os-release` on Linux, calls `sw_vers` on macOS, runs `cmd /c ver` on Windows. Falls back to bare `runtime.GOOS` on failure.
- Added `OSInfo` field to `prompt.Builder`, computed once at agent construction time via `platform.OSInfo()`.
- Extended `injectVariables` to resolve {WORK_DIR} (from `Builder.WorkDir`) and {OS_INFO} (from `Builder.OSInfo`).
- Wired `OSInfo` into `runtime.NewAgent` builder construction.
- Added context sentence at the start of `prompts/sysprompt.md`: "Your working folder is {WORK_DIR} and your operating system is {OS_INFO}."
- Added 4 tests: `TestInjectVariablesWorkDir`, `TestInjectVariablesOSInfo`, `TestInjectVariablesAll`, `TestOSInfo`.

## Decisions And Rationale
- OS info is computed once at agent start (stored on Builder), not on every `injectVariables()` call. This avoids repeated `exec.Command` calls during prompt build.
- The `OS_INFO` variable name follows the existing `APP_HOME` convention (SCREAMING_SNAKE_CASE with `_` separator), not `os_info` or `OSINFO`. Prefix-based variables could work but the existing pattern is flat names.
- Linux detection via `/etc/os-release` returns `PRETTY_NAME` (e.g. "Ubuntu 22.04.5 LTS"), which is more helpful than bare "Linux". For distributions setting only `NAME` and `VERSION_ID`, fallback is "Linux".
- Added "ubuntu" as an acceptable match in the OSInfo test because Ubuntu's `PRETTY_NAME` does not contain the word "linux".

## Files Included
- `internal/platform/platform.go` — `OSInfo()`, `linuxOSInfo()`, `darwinOSInfo()`, `windowsOSInfo()`
- `internal/platform/platform_test.go` — `TestOSInfo`
- `internal/prompt/prompt.go` — `OSInfo` field on Builder, {WORK_DIR} and {OS_INFO} in `injectVariables`
- `internal/prompt/prompt_test.go` — 3 new injection tests
- `internal/runtime/runtime.go` — wire `platform.OSInfo()` into builder
- `prompts/sysprompt.md` — context sentence at top of universal prompt

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
