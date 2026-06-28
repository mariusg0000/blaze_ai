# Session Decision Summary: ask-friend-input-file

Date: 2026-06-28 19:17
Base commit: f3b65b89aa51150c54d6a565da0cc3b50bec120c

## Context
The user wanted `ask_a_friend` to accept a file directly because the delegated model currently depends entirely on text packaged by the main model. The user also wanted the system prompt to explain when to use `summarization` versus `advisor`, and earlier requested stronger `customize-me` guidance for advisor-role configuration.

## Changes Made
Added `input_file` support to `ask_a_friend` with a strict `150000` byte limit and explicit file-read failures. Updated tests to cover valid file input, missing files, and oversized files. Added a short `ask_a_friend` usage section to the sysprompt and expanded `customize-me` role guidance for `summarization` and `advisor`.

## Decisions And Rationale
The file-input path was added inside the tool rather than by changing the delegated caller contract, because the tool already owns input validation and user-facing errors. The size limit is strict with no fallback or silent truncation, matching the project rule against hidden degradation. The system-prompt guidance stays short so it teaches the tool correctly without bloating every request.

## Implementation Approach
`internal/tools/ask_friend.go` now accepts an optional `input_file`, validates that it is a readable regular file under the byte cap, reads it, and appends a tagged block to `context` before calling the secondary model. `prompts/sysprompt.md` explains the intended use of `summarization`, `advisor`, and `input_file`. `skills/customize-me.md` now documents the practical role assignments and restart requirement for the learning-review workflow.

## Alternatives Considered
No fallback to partial file reads or silent clipping was kept. No separate file-only secondary-call tool was introduced because the existing `ask_a_friend` surface was sufficient with one additional parameter.

## Files Included
- `internal/tools/ask_friend.go`: added `input_file` schema, validation, file reading, and context injection.
- `internal/tools/ask_friend_test.go`: added success and failure coverage for file input.
- `prompts/sysprompt.md`: added concise guidance for `ask_a_friend`, `summarization`, `advisor`, and `input_file`.
- `skills/customize-me.md`: documented advisor/summarization role usage and configuration guidance.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
