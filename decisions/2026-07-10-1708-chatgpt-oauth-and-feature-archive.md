# Session Decision Summary: ChatGPT OAuth and feature archive

Date: 2026-07-10 17:08

## Context

The current worktree contains a desktop ChatGPT OAuth integration, encrypted Responses API reasoning support, repository workflow documentation updates, and a request to move the existing `features/` documents into `decisions/`. The generated `blazeai` executable is excluded from the commit.

## Changes Made

- Added PKCE browser OAuth, token refresh, credential persistence, and ChatGPT Responses API streaming support.
- Added desktop backend and Electron UI actions for starting and polling the ChatGPT connection flow.
- Preserved encrypted reasoning continuation data in session messages.
- Updated configuration validation and tests for OAuth-backed providers.
- Moved 13 documents from `features/` to `decisions/` without changing their contents.
- Updated `AGENTS.md` with the repository workflow rules present in the worktree.

## Decisions And Rationale

OAuth credentials remain in the Go backend and protected config persistence; only non-secret flow state crosses the Electron IPC boundary. ChatGPT models use the Responses API adapter because the OAuth-backed endpoint is separate from the public OpenAI-compatible chat-completions route. The feature documents were moved rather than rewritten to preserve their content.

## Validation

- `GOCACHE=/tmp/blazeai-go-cache go test ./...` — passed.
- `GOCACHE=/tmp/blazeai-go-cache go build ./...` — passed.
