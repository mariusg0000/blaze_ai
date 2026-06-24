# Session Decision Summary: project-based sessions + /clear

Date: 2026-06-24 15:30
Base commit: acc72f1

## Context
Session storage was flat — all sessions lived in `app_home/sessions/` regardless of which project (working directory) launched the app. The user wanted sessions grouped by project. Additionally, a previous session left uncommitted context-reset changes (`/clear`, `/new` commands) that needed to be included.

## Changes Made

### Project-based session storage
- `platform.go`: Added `ProjectFolderName()` (path sanitization: `/` → `_`, lowercase), `ProjectDir()`, `EnsureProjectDir()`. Updated `subfolders` from `sessions` to `projects`. Updated `Bootstrap()` to remove legacy `sessions/` dir.
- `platform_test.go`: Tests for `ProjectFolderName` and `EnsureProjectDir`.
- `session.go`: `Create()`, `LastClean()`, `Last()` now accept `workDir string` parameter and resolve project-specific sessions dir via `platform.EnsureProjectDir()`. Removed `sessionsDir()`.
- `main.go`: Resolves `workDir` before session creation, passes it to session functions.

### Context reset commands (pre-existing)
- `compaction.go`: Added `ClearSummaries()` method.
- `console.go`: Added `/clear` and `/new` slash commands.
- `console_test.go`: Tests for `/clear` and `/new`.
- `memories.go`: Added `ActiveList.Clear()`.
- `runtime.go`: Added `ResetConversation()` method.
- `skills.go`: Added `ActiveList.Clear()`.

### Future plan
- `next_todo.md`: Smart input plan (liner + history + autocomplete).

## Decisions And Rationale
- `ProjectFolderName` uses underscore-joined lowercase path segments — portable across OS, sortable, readable.
- Legacy `sessions/` is silently removed at bootstrap — clean migration without data loss risk (old sessions were unstructured).
- `CreateInDir`/`LastCleanInDir`/`LastInDir` kept as test-friendly variants — no test changes needed.
- `/clear` and `/new` both reset session in place without changing folder name — per spec.

## Implementation Approach
Platform package provides project path resolution. Session package delegates to platform for project dir. Main wires workDir from `os.Getwd()` into session operations. Pre-existing context-reset changes from a previous session were included in this commit.

## Alternatives Considered
Keeping flat session storage was simpler but didn't support project grouping. Moving to a database was overkill for this phase.

## Files Included
- `internal/platform/platform.go`: project path functions, Bootstrap cleanup
- `internal/platform/platform_test.go`: new project path tests
- `internal/session/session.go`: workDir-aware session functions
- `main.go`: workDir resolution before session creation
- `internal/compaction/compaction.go`: ClearSummaries method
- `internal/console/console.go`: /clear and /new commands
- `internal/console/console_test.go`: /clear and /new tests
- `internal/memories/memories.go`: ActiveList.Clear method
- `internal/runtime/runtime.go`: ResetConversation method
- `internal/skills/skills.go`: ActiveList.Clear method
- `next_todo.md`: smart input plan (unrelated pre-existing file)

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
