[DESCRIPTION]
Load when the user explicitly wants to analyze recent BlazeAI sessions for recurring issues, missing or weak skills, inefficient tool use, or cross-session workflow improvements. Use with `shell` and `ask_a_friend` to generate per-session review reports and a consolidated improvement plan. Do not use for normal code review, current-task debugging, project mapping, config editing, or direct skill creation.

[BEHAVIOR]
# Session Retrospective

## Purpose

Review recent real BlazeAI sessions to find evidence-based improvements to skills, memories, tool use, and transport-specific workflows.

This workflow is review-only:

1. inspect recent sessions
2. generate missing per-session review reports
3. synthesize a cross-session improvement plan
4. stop and discuss what should actually change

Never auto-create skills, memories, configs, docs, or code changes from this workflow.

## When To Use

Use this skill when the user asks to:

- review recent sessions
- find repeated mistakes or inefficient tool patterns
- identify missing, weak, unused, or over-broad skills
- compare terminal and Telegram usage patterns
- generate missing per-session `review.md` reports
- synthesize a cross-session improvement plan
- decide what skill, memory, or workflow change has the best payoff

Do not use this skill for:

- a normal code review
- debugging only the current task
- creating or editing a skill directly
- generating a project map
- editing provider, model, mode, or Telegram configuration
- summarizing one file or one transcript outside the session-review workflow

## Source Of Truth

The source for per-session analysis is `<session_dir>/prompt.json`, not `session.json`.

`prompt.json` is the final payload sent to the LLM, including sysprompt, compaction summaries, and conversation messages. If `prompt.json` is missing, report the error and stop unless the user explicitly chooses a different review scope.

## Workflow

1. Use `shell` to inspect the newest session folders under `{APP_HOME}/projects/*/sessions/*` and `{APP_HOME}/telegram/*/session`.
2. Build a compact working list with at most `30` sessions.
3. Include `session_dir`, `prompt.json`, `review.md`, transport, source kind, source name, timestamp, and whether `review.md` already exists.
4. Prefer newest sessions first.
5. Focus first on sessions where `review.md` is missing.
6. Skip existing `review.md` reports unless the user explicitly asks to regenerate them.
7. For each target session, call `ask_a_friend` with `role="summarization"` and `input_file="<session_dir>/prompt.json"`.
8. Put the session path, transport, source kind, and source name in the `context`.
9. If the session came from `{APP_HOME}/telegram/`, explicitly tell the summarization model that it was a Telegram bridge interaction and should not be judged as a normal console REPL transcript.
10. Write the returned report to `<session_dir>/review.md`.
11. After the newest relevant reports exist, collect up to `30` `review.md` files.
12. Send the combined reports to `ask_a_friend` with `role="advisor"` for cross-session synthesis.
13. Present the improvement plan to the user.
14. Stop for discussion. Do not implement recommendations automatically.

## Review Rules

- Prefer the newest sessions first.
- Skip sessions that already have a good `review.md` unless the user explicitly wants regeneration.
- The analysis source is `<session_dir>/prompt.json`, not `session.json`. If `prompt.json` does not exist for a session, report the error and stop — no fallback.
- If `shell` or `ask_a_friend` returns a hard error, surface it clearly and stop unless the user explicitly asks for partial review.
- Keep per-session reports concise and evidence-based.
- Separate missing-skill recommendations from skill-optimization recommendations.
- Distinguish memory-bank opportunities from runnable-skill opportunities.
- Keep Telegram bridge findings separate from terminal findings when the usage pattern or UX constraints differ.
- Prefer high-payoff improvements over exhaustive lists.
- Do not invent missing skills from weak evidence.
- Do not treat one isolated mistake as a recurring pattern unless it appears across sessions.

## Safety Rules

- No fallback if `summarization` or `advisor` is not configured.
- No recursive delegation chains.
- Do not ask the secondary model to call tools.
- Do not leak unrelated session content into the final user-facing synthesis.
- Do not delete existing `review.md` files unless the user explicitly requests regeneration.
- Do not create or edit skills, memories, docs, specs, or code from this workflow.

[DATA]
per_session_report_format:

```md
# Session Review Report

## Session
- path
- transport
- source_kind
- source_name
- timestamp

## Outcome
- short goal summary
- success / failure / partial

## Skill Findings
- existing skills used well
- existing skills used poorly
- missing skill opportunities
- over-broad or unclear skill triggers
- memory-bank opportunities
- runnable-skill opportunities

## Inefficiencies
- repeated tool calls
- weak command sequencing
- unnecessary inspection
- avoidable retries
- unclear stop conditions

## Recommendations
- create skill X
- optimize skill Y
- add memory bank Z
- leave unchanged if nothing useful was found
```

cross_session_output_format:

```md
# Session Retrospective

## Priorities
- highest payoff changes first

## New Skill Candidates
- candidate
- evidence
- proposed type: memory / behavior / runnable
- expected payoff

## Skill Updates
- skill name
- observed weakness
- recommended fix

## Memory Opportunities
- reusable fact or preference
- evidence
- proposed scope

## Repeated Failures
- recurring instruction gaps
- recurring tool misuse
- recurring UX problem

## Transport-Specific Findings
- terminal findings
- Telegram findings

## Discussion Points
- decisions the user should make before implementation
```
