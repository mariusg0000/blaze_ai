# Session Decision Summary: advisor-role-and-learning-ideas

Date: 2026-06-28 09:41
Base commit: 6aab2e6

## Context
The user wanted the configuration flow to include a stronger delegated-analysis role in addition to the existing `vision` and `summarization` roles. In parallel, the user wanted the new learning workflow and the `ask_a_friend` delegation concept captured as ideas before implementation.

## Changes Made
Added `advisor` as a first-class optional role in `config.json`, config validation, and first-run setup. Updated the first-run tests and config documentation to match the new role. Added two idea notes: `ask-a-friend` for a one-shot delegated LLM tool routed by preconfigured roles, and `session-learning-review` for the session/transcript review pipeline that will generate `learning.md` reports.

## Decisions And Rationale
`advisor` was added as a role instead of introducing arbitrary model names in the tool layer because the routing should stay anchored to preconfigured config entries. The name `advisor` is clearer than a playful label like `smarty` for long-term config and docs. The learning and delegation concepts were written as idea notes first so the runtime design can be discussed before implementation.

## Implementation Approach
The `Roles` struct now includes `Advisor`, and `Validate()` checks it with the same provider/model rules as the other roles. First-run setup now offers `vision`, `summarization`, and `advisor` in sequence and persists the chosen model IDs. Tests were extended to cover the extra prompt branch and config round-trip behavior. The idea files capture the planned `ask_a_friend` delegation tool and the two-stage learning review workflow.

## Alternatives Considered
A generic `send_to_llm(model_name, text)` tool was not implemented because it would expose arbitrary model selection and make the delegation path too broad. A separate ad hoc secondary-agent design was also deferred because the desired behavior is one-shot consultation, not a nested agent loop.

## Files Included
- internal/config/config.go: added `advisor` role field and validation.
- firstrun.go: prompted for `advisor` during first-run setup.
- firstrun_test.go: covered the new first-run role branch.
- internal/config/config_test.go: covered `advisor` config load/save round-trip.
- specs.md: updated top-level role summary.
- specs/02-core-runtime.md: updated runtime role description.
- specs/04-platform-ops.md: updated setup role assignment description.
- internal/platform/apphome_readmes/config/README.md: updated config folder docs.
- home_folder_backup/config/README.md: updated backup config docs.
- skills/customize-me.md: updated config helper docs.
- home_folder_backup/skills/customize_me/skill.md: updated backup skill docs.
- ideas/ask-a-friend.md: concept note for the delegated one-shot LLM tool.
- ideas/session-learning-review.md: concept note for the session learning pipeline.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
