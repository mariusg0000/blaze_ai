# Session Decision Summary: ChatGPT Luna Lite

Date: 2026-07-11 10:05

## Context

GPT-5.6 Luna started returning `Model not found gpt-5.6-luna` on the ChatGPT OAuth path.
The issue matched the upstream OpenCode bug for GPT-5.6 Responses Lite.

## Changes Made

- Updated ChatGPT OAuth requests to send a Codex-like `User-Agent` and the Responses Lite header.
- Added `version: 0.144.0` for GPT-5.6 lite requests.
- Set lite reasoning context to `all_turns`.
- Normalized multimodal input by preserving `input_image` items and stripping image `detail`.
- Added tests for the new headers and request shape.

## Decisions And Rationale

- Kept the change scoped to the GPT-5.6 lite request path.
- Matched the upstream fix pattern instead of adding broader fallback logic.
- Preserved explicit failure behavior and avoided silent protocol degradation.

## Validation

- `go test ./internal/provider` OK
- `go test ./...` OK
- `go build -o /tmp/blazeai .` OK
