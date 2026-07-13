# Session Decision Summary: Telegram interactive model selection and replace_block batching rule

Date: 2026-07-13 17:25

## Context

The Telegram bridge's `/model` command required `provider/model_name` as argument, unlike the console's interactive two-step provider→model selection. The user reported the console was making sequential `replace_block` calls on the same file (three tool rounds) instead of batching independent edits into one call.

## Changes Made

### Telegram interactive model selection

- `internal/telegram/state.go` — `State` extended with `PendingStage`, `PendingProvider`, `PendingModels` fields for multi-message selection flow.
- `internal/telegram/commands.go` — `beginModelSelection` lists providers (or skips to model list if one provider); `HandleModelSelection` processes numeric replies advancing the state machine; `selectProviderForModels` fetches live model list via provider endpoint; `clearModelSelection` resets state.
- `internal/telegram/telegram.go` — `runPolling` routes numeric messages through `HandleModelSelection` before normal agent processing.
- `internal/telegram/commands_test.go` — test for interactive provider/model selection with a local HTTP test server.

### replace_block batching rule

- `internal/tools/replace_block.go` — `Description()` and `Parameters()` schema now include mandatory batching guidance: all independent same-file edits must be sent as one `blocks` array in a single call.
- `internal/tools/replace_block_test.go` — tests verify that `Description()` and `Parameters()` both contain the batching rule.

## Decisions And Rationale

- Keep `/model` as the single command name; interactive mode is inferred from missing argument, matching console behavior and Telegram command list.
- Persist selection state in Telegram's `state.json` so multi-message interaction survives network interruptions.
- The response to each selection step is a single Telegram message; the user sends numbers as separate updates.
- Batching rule is tool-embedded (not prompt-level) because only `replace_block` can batch same-file edits; other tools have different constraints.
- Backup saved under `/home/marius/blazeai/backups/telegram-model-selection/` and `/home/marius/blazeai/backups/replace-block-batching/`.
