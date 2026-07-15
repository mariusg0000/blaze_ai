# Session Decision Summary: Responses Lite reasoning context requirement

Date: 2026-07-15 05:58

## Context

ChatGPT Codex Responses Lite started rejecting requests for `gpt-5.6-*` models with:

```
"X-OpenAI-Internal-Codex-Responses-Lite requires `reasoning.context` to be `all_turns`."
```

The provider adapter previously sent `reasoning.context` only when a reasoning level suffix was present, so models without the suffix produced an invalid payload for the Lite endpoint.

## Changes Made

- `internal/provider/openai_responses.go`: Always include `reasoning.context: "all_turns"` in Lite requests, then attach optional `effort` and `summary` when a reasoning level is configured.
- `internal/provider/openai_responses_test.go`: Updated tests and renamed `TestBuildChatGPTLiteRequestNoSuffixOmitsReasoning` to `TestBuildChatGPTLiteRequestNoSuffixUsesAllTurnsContext` to reflect the new required field.

## Decisions And Rationale

- Required `reasoning.context: "all_turns"` for all Responses Lite requests because the backend now enforces this field as part of the contract.
- Kept the request minimal by only populating `effort` and `summary` when the user has an explicit reasoning level configured.
- Aligned tests with the new backend expectation instead of preserving the previous omission behavior.

Validations performed: `go test ./...`, `go build ./...`, `git diff --check`.
