# Session Decision Summary: Web Slash Commands

Date: 2026-07-11 15:35

## Context

The web transport sent every input to the LLM, including recognized console slash commands. This caused `/clear`, `/model`, `/cd`, and other commands to be interpreted as user prompts.

## Changes Made

- Added `internal/web/commands.go` with local dispatch for:
  - `/clear` and `/new`
  - `/cd <path>`
  - `/model <provider/model>`, `/model +`, `/model -`
  - `/auth` with an explicit unsupported error
  - `/exit`
  - `/help`
- `POST /input` now runs recognized commands locally before appending a user block or calling `Agent.RunTurn`.
- Unknown slash commands preserve console behavior and continue to the LLM.
- Added an SSE `clear` event so `/clear` and `/new` also clear the browser transcript.

## Decisions And Rationale

Known console commands must be intercepted at the transport boundary because they are transport actions, not model prompts. Interactive `/model` selection is replaced by the web model selector; `/auth` is rejected explicitly because browser OAuth is not wired into this transport. Unknown commands remain normal input to preserve existing console semantics.

## Validation

- `gofmt -w internal/web/*.go` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `go test ./...` — passed
- `git diff --check` — passed
