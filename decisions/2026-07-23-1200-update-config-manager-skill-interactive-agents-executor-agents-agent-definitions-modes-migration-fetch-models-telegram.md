# Decision Summary: Update builtin config-manager skill

Date: 2026-07-23 12:00

## Context

The builtin `config-manager` skill had accumulated stale content: the entire "Work Modes (modes.json)" section described modes.json as active configuration, which is no longer true after the interactive/executor agent system replaced it. The ad-hoc `fetch_models` script section was redundant with the existing `/model` provider-listing flow. Agent definition schema, runtime model/state ownership, safe rename/delete invariants, and fresh-install defaults were undocumented. The task was to bring the skill fully up to date.

## Changes Made

- `skills/config-manager.md`: Complete rewrite of the embedded builtin skill. Replaced description and body with current documentation covering: configuration sources and ownership (config.json, agents/*.md, bridge.json, agents.json, state.json, modes.json as legacy-only), global config.json template with all current fields (providers, favorites, roles, compaction, stripReasoning, helperSetup, debugPrompt), agent definition format (interactive/executor frontmatter, model/tool/agent/directive rules), mode selection via /mode/Tab, model assignment via /model and agents.json, safe create/edit/rename/delete operations, fresh-install default agent provisioning, legacy modes.json migration notes, and preserved Telegram and helper guidance. Removed the entire "Work Modes (modes.json)" active operations section and the ad-hoc fetch_models script section.
- `task.md`: Marked as `status: completed` and populated with the full task specification, findings, scope, and verification results.

## Decisions And Rationale

- **Single commit for both files**: `task.md` was the task tracker for this exact skill rewrite; committing them together keeps the completion signal with the work.
- **Removed modes.json as active config**: Source evidence confirms runtime reads only `agents/*.md` and `agents.json`; `modes.json` is read only for one-time legacy migration and is never written by current runtime. Documenting it as active would be misleading.
- **Removed fetch_models script section**: The console's `/model` flow already calls the configured provider's model-list endpoint and handles OAuth; a duplicate ad-hoc script adds confusion without value.
- **Preserved Telegram and helper guides**: Both are source-backed, current, and operationally useful; no rationale exists to remove or restructure them.
- **Added validation checklist**: Prevents common misconfiguration without requiring architectural knowledge.
- **No rationale was supplied** for any pre-existing sub-decision within the original task scope (e.g., why interactive/executor was chosen over modes.json); the task contract described findings, not historical rationale. The decision file records this absence.
