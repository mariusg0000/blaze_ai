# Session Decision Summary: shell-purpose-wording

Date: 2026-06-29 06:31
Base commit: 2386a67

## Context
The console was showing tool purpose text that described why `shell` was needed instead of showing the actual command scope or helpers being run. The user asked to adjust the wording only, without changing runtime behavior.

## Changes Made
- Updated `internal/tools/shell.go` so the `purpose` schema asks for the command's approach, scope, or target instead of a justification for using shell.
- Updated `internal/tools/ask_friend.go` so the `purpose` schema asks for the consultation scope or focus instead of a justification for using a secondary model.

## Decisions And Rationale
The old wording encouraged meta explanations like "shell is needed to scan the filesystem fast," which is not useful in the console. The new wording steers the model toward describing what it will actually run or inspect.

## Implementation Approach
Only the schema descriptions were changed. Runtime execution, tool formatting, and console rendering were left untouched.

## Alternatives Considered
Keeping the "why" phrasing was rejected because it produced the exact console text the user wanted removed.

## Files Included
- `internal/tools/shell.go`: updated `purpose` schema wording.
- `internal/tools/ask_friend.go`: updated `purpose` schema wording.
- `decisions/2026-06-29-0631-shell-purpose-wording.md`: session record for the change.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
