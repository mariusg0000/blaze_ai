# Session Decision Summary: Fix console test HOME isolation

Date: 2026-06-22 13:05
Base commit: cf3de72

## Context
User's `config.json` at `/home/marius/blazeai/config/config.json` contained test data (`test/test-model`, `http://localhost`, `sk-test`) instead of the real opencode-go provider. User denied manually editing it.

## Root Cause
`internal/console/console_test.go:mockAgent()` did not isolate HOME. `TestHandleCommandModelSet` calls `SetModel()` which calls `Config.Save()`, overwriting the user's real config with test scaffolding data.

## Fix
Added `t.Setenv("HOME", t.TempDir())` to `mockAgent()` in `console_test.go`, matching the pattern already used in `runtime_test.go:setupAgent()`.

## Validation
`go test ./...` — all 197 tests pass.

## Files Changed
- `internal/console/console_test.go`: added HOME override in mockAgent
