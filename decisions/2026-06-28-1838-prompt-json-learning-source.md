# Session Decision Summary: prompt-json-learning-source

Date: 2026-06-28 18:38
Base commit: 879bc933e1f82f8145ae84eace05e219495cc1a3

## Context
The learning-review workflow needed to match the user's clarified source of truth: `prompt.json`, the final JSON payload sent to the LLM, not `session.json`.

## Changes Made
Updated `skills/session-learning-review.md` to state that per-session learning must be based on `prompt.json`. The workflow now tells the reviewer to read `<session_dir>/prompt.json`, and the review rules explicitly require stopping with an error if it is missing.

## Decisions And Rationale
The skill text was the correct place for this clarification because the user explicitly asked for documentation/behavior guidance rather than runtime code changes. The goal is to keep the learning workflow aligned with the payload the model actually sees.

## Implementation Approach
Edited the session-learning-review skill description, workflow steps, and review rules to name `prompt.json` as the analysis source and to describe it as the final prompt payload including sysprompt, summaries, and current conversation messages.

## Alternatives Considered
No code changes were retained. Runtime and extractor changes were intentionally discarded to keep the scope limited to the requested skill clarification.

## Files Included
- `skills/session-learning-review.md`: clarified the learning source and failure rule.

## Commit Linkage
This summary is committed together with the skill update to keep rationale linked to the repository history.
