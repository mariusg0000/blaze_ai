# Session Decision Summary: prompt escape and skill cleanup

Date: 2026-06-24 11:03
Base commit: 1ec483d409d2ded53195af516f6750d198c6d4c9

## Context
The prompt interpolator needed a literal-brace escape path so prompt, skill, and memory text could contain `{VAR}` sequences without triggering substitution. The working tree also still contained the prior cleanup to remove `Load Rules` from the builtin skill and memory-manager docs.

## Changes Made
Added brace escaping to the interpolator so `\{` and `\}` render as literal braces while normal `{VAR}` placeholders still resolve. Added tests for the escaped and unescaped cases. Kept the earlier builtin skill cleanup in the same commit so the repository stays coherent.

## Decisions And Rationale
The escape behavior is limited to prompt, skill, and memory interpolation paths because those are the only textual sources that need literal placeholder syntax. The builtin skill cleanup remains separate in intent but is included here because it is already present in the worktree and belongs with the current repo state.

## Implementation Approach
`internal/prompt/prompt.go` now shields escaped braces before regex substitution and restores them afterward. `internal/prompt/prompt_test.go` verifies literal brace rendering and normal placeholder injection. The builtin skill files keep the simpler content-only model without `Load Rules` sections.

## Alternatives Considered
Leaving the interpolator unchanged would have forced users to avoid writing literal placeholder text in prompt files. Adding section-specific hacks was rejected because the escape mechanism needed to stay generic and minimal.

## Files Included
- `internal/prompt/prompt.go`: added brace escaping around interpolation.
- `internal/prompt/prompt_test.go`: added escape behavior tests.
- `skills/skill-manager.md`: builtin skill cleanup already present in worktree.
- `skills/memory-manager.md`: builtin memory cleanup already present in worktree.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
