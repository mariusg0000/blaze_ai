# Decision Summary: protocol adapters

Date: 2026-07-28 12:00

## Context

The originating goal was to implement the approved OpenCode-style provider protocol adapter foundation and correct one missed test fixture found during full verification. The implementation establishes an explicit model catalog and adapters for the two existing OpenAI protocol families. The project constraint against fallbacks requires missing or inconsistent metadata to fail explicitly. Native providers, reasoning normalization, and other broader compatibility features remain deferred. Historical rationale beyond the approved task context was not supplied.

## Changes Made

Added explicit model definitions and validation, first-run catalog construction, a provider-neutral request contract, OpenAI Chat and OAuth Responses protocol lowerers, and catalog-selected client routing. Updated related provider, configuration, first-run, runtime, console, Telegram, and compaction tests. The compaction fixture now constructs its client through `provider.NewClient` with explicit metadata after full verification exposed its raw client lacked a selected protocol. Resynchronized affected specifications and recorded completion in `task.md`.

## Decisions And Rationale

The approved design uses metadata-driven protocol selection rather than model-name inference, automatic catalog discovery, or fallback routing. It preserves existing HTTP, OAuth, SSE parsing, tool conversion, and Responses-lite behavior while moving request construction behind validation and lowering adapters. Only OpenAI Chat and OAuth Responses are included because those are the implemented paths; additional providers and reasoning-level support are explicitly deferred. The compaction change is fixture-only because the production contract correctly requires a selected protocol. Validation supplied by the session was `go test ./...` passing, a successful build, and a smoke run showing the expected catalog-missing error for an existing unupdated configuration.
