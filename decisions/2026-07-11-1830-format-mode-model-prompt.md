# Session Decision Summary: Format Mode And Model Prompt

Date: 2026-07-11 18:30

## Context

The single-line shortcut status behavior was accepted, but its order and labels needed refinement. The prompt had to show the active mode first, the changed model immediately after it, and no literal `mode` or `Model:` labels.

## Changes Made

- Changed the mode prompt from `[Quick mode]` to `[Quick]`.
- Rendered model changes after the mode as `[Quick][openai/gpt-5.6]`.
- Applied yellow bold styling to the model bracket.
- Kept the prompt on one redrawable physical line.
- Updated prompt tests for mode-only, model, and default formats.

## Decisions And Rationale

The prompt now uses compact bracketed values so the active mode and model remain visible without additional status lines or verbose labels. Mode and model retain distinct colors while readline continues to redraw the same line in place.

## Validation

- `go test ./...` — passed
- `go build ./...` — passed
- `go vet ./...` — passed
- `git diff --check` — passed
