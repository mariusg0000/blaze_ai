# Session Decision Summary: reasoning max/ultra normalization and OpenAI wire behavior

Date: 2026-07-15 16:30

## Context

After the initial `model_id[|reasoning_level]` migration, verification of `/home/marius/codex/` showed OpenAI uses `max` as a distinct wire value and does not clamp `max` to `xhigh`. The user then requested removing Ultra support entirely and keeping only `max`.

## Changes Made

- Modified `internal/reasoning/openai_chat.go`, `internal/reasoning/openai_responses.go`, and their tests so `max` maps directly to `"max"` instead of being clamped to `xhigh`.
- Updated normalizer/provider tests and comments to describe the corrected OpenAI wire behavior.
- Preserved `min → minimal` and `med → medium` mapping while keeping the normalized vocabulary fixed to `none,min,low,med,high,xhigh,max`.
- Kept invalid-level rejection for `ultra` and removed misleading Ultra/max-clamping documentation from implementation and comments.

## Decisions And Rationale

- Ultra support was intentionally excluded at the user’s request; `max` is the only top reasoning level in this normalized set.
- `max` and `xhigh` must remain distinct both abstractly and on the wire, matching the verified Codex/OpenAI behavior.
- No fallbacks were added; invalid reasoning strings still produce explicit validation errors.
