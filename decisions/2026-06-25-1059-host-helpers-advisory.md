# Session Decision Summary: host-helpers-advisory

Date: 2026-06-25 10:59
Base commit: dab7df3

## Context
The `setup_helpers` skill was not being loaded proactively by the LLM. Users could run sessions for a long time without the LLM ever suggesting helper verification. The existing `helperSetup.dismissed` flag existed but had no visible prompt-level consequence — it only suppressed the optional helpers display line.

## Changes Made
- Added `{HOST_HELPERS_ADVISORY}` variable injection into the universal sysprompt, rendering a persistent reminder whenever `helperSetup.dismissed` is `false`.
- The advisory appears every prompt build until `dismissed: true` is set, ensuring the LLM always knows helpers need verification regardless of conversation context.
- Updated the `setup_helpers` skill to make dismissal part of the normal completion flow (not requiring an explicit user preference request).

## Decisions And Rationale
- Injecting the advisory into the system prompt (runtime part) rather than the conversation ensures it survives across all turns and sessions. A one-shot tool result would disappear after the conversation window shifts.
- The advisory does not check whether helpers are actually installed — it relies on `dismissed: false` alone. This is intentional: even fully installed helpers should be verified once per deployment. The `setup_helpers` skill handles the verification and dismisses afterward.
- `{HOST_HELPERS_ADVISORY}` renders empty string when `dismissed: true`, keeping the prompt clean post-setup with zero overhead.

## Implementation Approach
- New method `buildHostHelpersAdvisory()` in `internal/prompt/prompt.go` checks `HelperSetup.Dismissed` and returns advisory text or empty string.
- The advisory variable is injected into the `injectTemplateVariables` map alongside the existing helper variables.
- Placeholder `{HOST_HELPERS_ADVISORY}` added between the section header and "Available helpers:" in `prompts/sysprompt.md`.
- `skills/setup_helpers.md` "Config Preferences" section replaced with "Dismissing The Helper Reminder" with explicit dismissal instructions.
- Test fixtures synchronized in both `internal/prompt/prompt_test.go` and `internal/runtime/runtime_test.go`.

## Files Included
- `prompts/sysprompt.md`: new `{HOST_HELPERS_ADVISORY}` placeholder in helpers section
- `internal/prompt/prompt.go`: `buildHostHelpersAdvisory()` method and injection wiring
- `skills/setup_helpers.md`: updated dismissal instructions
- `internal/prompt/prompt_test.go`: fixture updated with advisory placeholder
- `internal/runtime/runtime_test.go`: fixture updated with advisory placeholder
- `decisions/2026-06-25-1059-host-helpers-advisory.md`: this summary

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
