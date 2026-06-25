# Session Decision Summary: coding-skill-bugfix-protocol

Date: 2026-06-25 15:55
Base commit: 5d3013c

## Context
User requested a builtin 'coding' skill that activates on coding sessions, distilling AGENTS.md rules into actionable LLM guidance. Additionally, user identified that LLMs tend to jump directly to fixing bugs without understanding — requires a mandatory protocol requiring explanation + approval before implementation.

## Changes Made
- `skills/coding.md`: New builtin skill. DESCRIPTION triggers on code/script writing (not inline commands). BEHAVIOR contains distilled AGENTS.md rules: KISS, explicit types, quality gates per language, WHAT/WHY/HOW documentation, file headers, incremental patches, scope control. DATA provides quick-reference quality commands per language. Includes mandatory Bug Fix Protocol requiring explain → ask → implement flow.

- `AGENTS.md` §1 Identity And Mission: Added mandatory bug fix protocol paragraph after the reactive rule. When user reports a bug or asks for a fix, the assistant MUST explain what/why/how first, then ask for approval before implementing. Never jump directly to fixing.

- `skills/customize_me.md`: Removed ~40 lines of duplicated skill management content ("Customizing Builtin Skills" and "Skill Locations" sections). Replaced with a single line delegating to skill-manager. This content was a maintenance duplicate of skill-manager.md.

## Decisions And Rationale
- Bug fix protocol placed in §1 (Identity And Mission) of AGENTS.md because it defines HOW the agent operates, not scope boundaries. It's a fundamental behavioral rule that applies to all tasks.
- Coding skill excludes commit/decision-summary rules — those are workflow, not coding. Excludes mode transitions — runtime concern.
- User removed test prohibition from coding skill since it was project-specific (AGENTS.md §8), not universal.
- customize_me cleanup reduces duplication risk — skill management rules only live in skill-manager now.

## Files Included
- `skills/coding.md`: New builtin coding skill (96 lines)
- `AGENTS.md`: Bug fix protocol in §1
- `skills/customize_me.md`: Removed duplicated skill management content
- `decisions/2026-06-25-1555-coding-skill-bugfix-protocol.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
