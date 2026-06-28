[DESCRIPTION]
Load when the user wants to review recent BlazeAI sessions, generate per-session `learning.md` reports, identify missing or weak skills, or synthesize a cross-session improvement plan. Use with `session_review_extract`, `ask_a_friend`, and normal file-writing tools. Do not auto-create or auto-edit skills from this review.

[BEHAVIOR]
# Session Learning Review

## Purpose

Turn recent real sessions into explicit improvement suggestions.

The workflow is review-only:

1. inspect recent sessions
2. generate missing per-session learning reports
3. synthesize a cross-session plan
4. stop and discuss what should actually change

Never auto-create skills, memories, or code changes from this review.

## Workflow

1. Call `session_review_extract` with `action="list"` and a limit no higher than `30`.
2. Focus first on sessions where `has_learning_md=false`.
3. For each target session, call `session_review_extract` with `action="extract"`.
4. Send that compact transcript to `ask_a_friend` with `role="summarization"`.
5. Write the returned report to `<session_dir>/learning.md`.
6. After the newest relevant reports exist, collect up to `30` `learning.md` files.
7. Send the combined report set to `ask_a_friend` with `role="advisor"` for the cross-session synthesis.
8. Present the final plan to the user and stop for discussion.

## Review Rules

- Prefer the newest sessions first.
- Skip sessions that already have a good `learning.md` unless the user explicitly wants regeneration.
- If `session_review_extract` or `ask_a_friend` returns a hard error, surface it clearly and stop unless the user explicitly asks for partial review.
- Keep per-session reports concise and evidence-based.
- Separate missing-skill recommendations from skill-optimization recommendations.
- Distinguish memory-bank opportunities from runnable-skill opportunities.

## Safety Rules

- No fallback if `summarization` or `advisor` is not configured.
- No recursive delegation chains.
- Do not ask the secondary model to call tools.
- Do not leak unrelated session content into the final user-facing synthesis.
- Do not delete existing `learning.md` files unless the user explicitly requests regeneration.

[DATA]
per_session_report_format:

```md
# Learning Report

## Session
- path
- transport
- timestamp

## Outcome
- short goal summary
- success / failure / partial

## Skill Findings
- existing skill used well / poorly
- missing skill opportunities
- memory-bank opportunities

## Inefficiencies
- repeated tool calls
- weak command sequencing
- unnecessary inspection
- avoidable retries

## Recommendations
- create skill X
- optimize skill Y
- add memory bank Z
- leave unchanged if nothing useful was found
```

cross_session_output_format:

```md
# Session Learning Review

## Priorities
- highest payoff changes first

## New Skills
- candidate
- why
- proposed type: memory / behavior / runnable

## Skill Updates
- skill name
- weakness
- recommended fix

## Memory Opportunities
- what reusable knowledge should move into memory

## Repeated Failures
- recurring instruction gaps
- recurring tool misuse

## Discussion Points
- what should be reviewed with the user before implementation
```
