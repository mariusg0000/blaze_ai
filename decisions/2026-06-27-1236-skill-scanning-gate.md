# Session Decision Summary: skill-scanning-gate

Date: 2026-06-27 12:36
Base commit: 176ea82

## Context
On the NAS, when the user asked "restarteaza bridgeul telegram", the model jumped directly to systemd commands without loading any skill. The `customize-me` skill describes "Telegram bridge" in its description and links to `docs/telegram.md` with the full workflow, but the model had no rule telling it to scan available skill descriptions before acting on an unfamiliar domain.

## Changes Made
- Added a mandatory scanning rule in `[SKILLS]` section of `prompts/sysprompt.md`: before any task, scan available skill descriptions and load a skill if the request domain matches.
- Synced hardcoded sysprompt templates in `internal/prompt/prompt_test.go`, `internal/console/console_test.go`, and `internal/runtime/runtime_test.go`.
- Deployed to NAS.

## Decisions And Rationale
- The model needs to match user request domains against skill descriptions before acting.
- "Before performing any task" triggers at every turn, so even domain-specific requests automatically route through skill loading.
- The rule is placed before the available skills list, so the model reads it right before seeing what skills exist.
- Adding the rule only to sysprompt.md (not skill-manager.md) ensures it applies even when no skills are active, avoiding the catch-22 where the gate is inside the skill you need to load.

## Implementation Approach
- One-line rule added to `prompts/sysprompt.md` `[SKILLS]` section.
- Three test files updated with matching strings.
- Validated via `go test -count=1 ./internal/prompt ./internal/console ./internal/runtime ./internal/skills`.
- Deployed to NAS.

## Files Included
- `prompts/sysprompt.md`: added skill scanning gate
- `internal/prompt/prompt_test.go`: synced hardcoded sysprompt
- `internal/console/console_test.go`: synced hardcoded sysprompt
- `internal/runtime/runtime_test.go`: synced hardcoded sysprompt

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
