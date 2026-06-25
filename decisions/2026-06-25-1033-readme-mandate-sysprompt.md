# Session Decision Summary: readme mandate sysprompt

Date: 2026-06-25 10:33
Base commit: 228835e

## Context
The user tested the agent with a query about its config files. The LLM correctly described the folder contents but did not proactively read the `README.md` first. The sysprompt mentioned READMEs were available but did not mandate reading them before folder inspection. The user requested an imperative instruction forcing the LLM to check the README before touching any other file in an app-home folder.

## Changes Made
- Updated `prompts/sysprompt.md` line 9 to use a bold `MUST` directive requiring README-first reads for any task involving app-home folders.
- Aligned both test fixtures in `internal/prompt/prompt_test.go` and `internal/runtime/runtime_test.go` with the new wording. The runtime fixture was also missing the README reference entirely, which was added.

## Decisions And Rationale
- "Concise README.md files guide execution" was too passive. The LLM treated it as optional context rather than a required first step.
- The imperative `MUST read its README.md first` combined with bold formatting is the minimum change that forces desired behavior without adding tokens to other prompt sections.

## Implementation Approach
- Replaced the passive README guidance line in three files with an explicit mandate.
- Validated with `go test ./internal/prompt ./internal/runtime`.

## Files Included
- `prompts/sysprompt.md`: imperative README-first mandate.
- `internal/prompt/prompt_test.go`: fixture aligned.
- `internal/runtime/runtime_test.go`: fixture aligned and missing reference added.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
