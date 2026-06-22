# Session Decision Summary: platform-and-config

Date: 2026-06-22 09:55
Base commit: 80ac729 (Initial project structure and Go environment setup)

## Context
- Continued implementation from the initial project skeleton.
- Implemented the first two foundational packages: platform (OS detection, shell selection, app home) and config (JSON loading, validation, saving, first-run detection).
- Ordered dependency-first: platform has no dependencies, config depends on platform for path resolution.

## Changes Made
- **internal/platform**: new package
  - `Detect()` — maps runtime.GOOS to Linux/Darwin/Windows, error on unsupported
  - `ShellChain(os)` — returns shell preference list (bash→sh for Linux/macOS, pwsh→powershell→cmd for Windows)
  - `SelectShell(os)` — resolves first available shell via exec.LookPath
  - `AppHome()` — returns ~/blazeai path from os.UserHomeDir
  - `Bootstrap()` — creates app home + 6 subfolders (skills, scripts, backups, sessions, memory, config), excludes scripts/venv (lazy per spec)
  - 8 tests covering all functions, idempotent bootstrap, and edge cases
- **internal/config**: new package
  - Struct tree: Config, Provider, Roles, Compaction, StripReasoning with JSON tags
  - `Load()`/`LoadFrom(path)` — read and validate config.json
  - `Save()`/`SaveTo(path)` — write config with 0600 permissions
  - `Validate()` — checks default role required, provider uniqueness, model format (provider/model_name), provider references
  - `NeedsFirstRun()`/`NeedsFirstRunAt(path)` — trigger on missing file or empty default role
  - `DefaultCompaction()`, `DefaultStripReasoning()`, `Default()` — spec defaults from spec 05
  - `ProviderByName(name)`, `SplitModelID(id)` — lookup helpers
  - 23 tests covering load, save, validate (happy + all error paths), first-run detection, defaults, and helpers
- Updated internal/config/doc.go to reflect platform dependency

## Decisions And Rationale
- Separated loadRawFrom from LoadFrom so NeedsFirstRunAt can inspect the default role before full validation rejects it.
- SaveTo and LoadFrom accept explicit paths for testability with temp dirs; Save/Load wrap with platform-resolved path.
- config tests use t.TempDir() to avoid touching the real app home directory.
- scripts/venv excluded from Bootstrap (lazy per spec 04).
- Validate stops on the first error found; no accumulation of all errors.
- Config file permissions set to 0600 (owner read/write only) because it contains API keys.

## Implementation Approach
- Platform first (no dependencies), then config (depends on platform for AppHome resolution).
- Each package written with full file headers per AGENTS.md §9.1.
- Test-first: tests written alongside implementation, covering happy path, all error paths, and edge cases.
- Validated with go build ./..., go vet ./..., and go test ./... (all clean).

## Alternatives Considered
- Considered making config not depend on platform (pass path from caller). Rejected: config.json path is a fixed location relative to AppHome, so encapsulating it is cleaner.
- Considered accumulating all validation errors. Rejected per KISS: first error tells the whole story; user can fix and retry.

## Files Included
- internal/platform/platform.go: OS detection, shell selection, app home
- internal/platform/platform_test.go: 8 tests
- internal/config/doc.go: updated dependency line
- internal/config/config.go: config types, load, save, validate, first-run, defaults
- internal/config/config_test.go: 23 tests
- decisions/2026-06-22-0955-platform-and-config.md: this summary
