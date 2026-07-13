# Session Decision Summary: Decode Readline Shortcut Sequences

Date: 2026-07-11 17:30

## Context

The restored console shortcut bindings still did not execute. `reeflective/readline` receives raw terminal bytes, while `Config.Bind` stores the provided key string directly. The previous implementation passed literal inputrc notation such as `\\C-r`, so the dispatcher could not match byte `0x12`.

## Changes Made

- Imported `github.com/reeflective/readline/inputrc`.
- Decode every shortcut notation with `inputrc.Unescape` before calling `Config.Bind`.
- Added regression tests for Tab, Ctrl+\\, Ctrl+F, Ctrl+R, and Ctrl+T byte decoding.

## Decisions And Rationale

The binding API requires raw key bytes, not inputrc notation. Keeping human-readable notation in the shortcut map and decoding at the binding boundary makes the mapping clear while ensuring the dispatcher receives the exact terminal bytes.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
