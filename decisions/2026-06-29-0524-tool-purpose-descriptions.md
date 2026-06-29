# Session Decision Summary: tool-purpose-descriptions

Date: 2026-06-29 05:24
Base commit: f28dd11c71d8ca4725dde0a26fe6139b2c6b2464

## Context
User wanted richer, user-visible `purpose` text for tool calls. The initial shell and replace_block descriptions were too short, and the `ask_a_friend` tool description was too technical and repeated the schema instead of explaining the tool's role.

## Changes Made
- Updated `purpose` schema descriptions for `shell`, `replace_block`, and `ask_a_friend` to require exactly 3 user-visible sentences with explicit content requirements.
- Rewrote `AskFriendTool.Description()` to explain the tool's behavior, appropriate use cases, and the no-tools/no-conversation constraint.
- Kept required fields, execution logic, and UI formatting unchanged.

## Decisions And Rationale
- Chose a strict 3-sentence contract to make the LLM output predictable and useful for users.
- Kept the changes in schema descriptions rather than adding runtime validation because the request was about prompt guidance, not execution behavior.
- Replaced the `ask_a_friend` description because the old text described the input shape instead of the tool's purpose.

## Implementation Approach
- Edited the JSON schema description strings in `internal/tools/shell.go`, `internal/tools/replace_block.go`, and `internal/tools/ask_friend.go`.
- Updated `AskFriendTool.Description()` to a plain-language explanation of delegation behavior and constraints.
- Verified the change with `go test ./internal/tools`.

## Alternatives Considered
- Leaving `ask_a_friend` description as parameter-oriented text was rejected because it did not describe the tool's use or constraints clearly enough.

## Files Included
- internal/tools/shell.go: stricter `purpose` guidance for shell calls.
- internal/tools/replace_block.go: stricter `purpose` guidance for file replacement calls.
- internal/tools/ask_friend.go: stricter `purpose` guidance and clearer tool description.
- decisions/2026-06-29-0524-tool-purpose-descriptions.md: session record and rationale.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
