---
status: postponed
---

# Normalized reasoning level with Ctrl+]

## User outcome

Re-introduce a provider-neutral reasoning level for the current OpenAI-compatible
request paths. The active level is normalized to exactly
`none`, `min`, `low`, `med`, `high`, `xhigh`, or `max`, cycles in that order with
wrap-around when the user presses Ctrl+], is visible in the console status
surface, is persisted per full model ID, and is sent as the appropriate OpenAI
wire effort field.

## Scope

OpenAI-compatible Chat Completions + ChatGPT OAuth Responses/Codex only.
Deferred: multi-provider, `/reasoning`, Ctrl+T, reasoning display rendering.

## Saved plan

`plans/2026-07-24-normalized-reasoning-level-ctrl-bracket.md`

## Resume conditions

User request to proceed with implementation.
