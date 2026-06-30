# Session Retrospective

## Goal

Add an explicit retrospective workflow that reviews recent BlazeAI sessions, extracts the useful execution flow, and produces actionable skill improvement reports.

This should help answer:

- should a new skill be created?
- should it be a memory-only skill, a behavior skill, or a runnable skill?
- were existing skills used efficiently?
- were there inefficient tool calls, repeated loops, or avoidable mistakes?
- should an existing skill be optimized, split, merged, or replaced?

## Naming

`learn` is short, but too generic.

Better candidates:

- `session-review`
- `session-retrospective`
- `self-review`

Recommended name: `session-retrospective`.

It is specific enough to describe what is being reviewed and does not sound like a general-purpose learning subsystem or an automatic training feature.

## High-Level Flow

The workflow has two stages.

### Stage 1: Per-Session Review Report

1. Scan `app_home/projects/*/sessions/` and `app_home/telegram/*/session/`.
2. Sort sessions by recency.
3. Take the newest 30 sessions.
4. For each session:
5. If `review.md` already exists in that session folder, skip it.
6. Otherwise extract a compact transcript from `session.json`.
7. Send the compact transcript to the summarization model.
8. Ask the model to analyze skill usage, inefficiencies, and missing skill opportunities.
9. Save the result as `review.md` in that session folder.

### Stage 2: Cross-Session Improvement Plan

1. Collect the newest 30 `review.md` reports.
2. Send them to the current active model.
3. Ask for a consolidated improvement plan.
4. Present the result to the user.
5. Stop and discuss what should actually be implemented.

This is review-only. It must not auto-create or auto-edit skills.

## Session Sources

The review should include both transport families:

- normal terminal sessions under `app_home/projects/*/sessions/`
- Telegram sessions under `app_home/telegram/*/session/`

The review should treat them uniformly after extraction.

## Extraction Script

This needs a dedicated script because raw `session.json` is too noisy.

The script should read one `session.json` and output a compact Markdown transcript with only the useful parts.

### Keep

- `system` messages
- `user` messages
- `assistant` messages
- tool call summary
- tool response summary

### Drop

- verbose reasoning content
- large raw tool output
- irrelevant internal fields
- anything not needed to reconstruct the flow

### Tool Call Extraction

For each tool call, extract:

- tool name
- tool purpose, if present
- first 100 characters of the command or main payload

Example shape:

```md
[TOOL CALL]
name: shell
purpose: inspect signal-cli installation
payload: python3 -m pip install signal-cli-rest-api typing requests chardet...
```

### Tool Response Extraction

For each tool response, extract only a short outcome:

- `ok`
- `timeout`
- `error`
- first 100 characters of the most relevant output

Example shape:

```md
[TOOL RESULT]
name: shell
status: error
summary: subliminal 2.6.0 requires chardet>=5.0, but you have chardet 3.0.4...
```

## Per-Session Review Prompt

The summarization model should receive the compact transcript and be asked to produce a strict review report.

Main questions:

- Did this session reveal a missing skill?
- If yes, should it be:
  - memory-only
  - behavior-only
  - runnable
  - mixed but still minimal
- Did the agent use existing skills efficiently?
- Were instructions inside the relevant skills actually followed well?
- Were there repeated tool calls, unnecessary retries, or avoidable exploration?
- Could an existing skill be optimized to reduce future token use, tool calls, or mistakes?
- Did the session reveal reusable operator knowledge that belongs in a memory bank?

## Per-Session Report Format

Each `review.md` should be structured and concise.

Suggested sections:

```md
# Session Review Report

## Session
- path
- transport
- timestamp

## Outcome
- short summary of what the session tried to do
- whether it succeeded

## Skill Findings
- existing skill used well / poorly
- missing skill opportunities
- memory-bank opportunities

## Inefficiencies
- repeated tool calls
- weak command sequencing
- unnecessary inspection
- bad retry patterns

## Recommendations
- create skill X
- optimize skill Y
- add memory bank Z
- leave unchanged if nothing useful was found
```

## Cross-Session Meta-Review

After 30 session reports exist, the current active model should analyze them together.

The output should answer:

- which skills should be created
- which skills should be updated
- which memory banks should be added
- which existing skills are underperforming
- which instruction patterns are repeatedly ignored or misunderstood
- which optimizations have the best payoff

This stage should produce a user-facing plan, not direct changes.

## User Workflow

1. User triggers the review explicitly.
2. Runtime generates missing `review.md` files for the newest 30 sessions.
3. Runtime runs the cross-session meta-review.
4. User receives a Markdown improvement plan.
5. User decides what should be implemented.

No automatic skill writing. No automatic skill mutation. No automatic memory-bank creation.

## Constraints

- No fallbacks.
- If the summarization model is not configured, stop with a clear error.
- If a session file is malformed or unreadable, report it clearly and continue only if the workflow explicitly allows partial review.
- If partial review is not desired, stop immediately on the first invalid session.

The exact failure policy should be chosen during implementation.

## Why This Is Useful

- Turns real usage into structured product feedback.
- Helps detect skill gaps from actual sessions instead of guesses.
- Makes optimization decisions evidence-based.
- Keeps the final decision with the user instead of inventing autonomous behavior.

## Open Questions

- Should the extractor be a standalone script under `app_home/scripts/` or a builtin runtime command?
- Should `review.md` be regenerated only manually, or also when stale?
- Should skipped sessions with existing `review.md` still be counted inside the newest 30?
- Should the per-session report be strict Markdown only, or partially structured JSON inside Markdown?
- Should memory-bank recommendations be kept distinct from skill recommendations in the final plan?
