# Session Decision Summary: skill directive rules + tab-cycle reminder

Date: 2026-06-24 23:45
Base commit: ed3d637

## Context
Two UX problems: (1) the LLM was creating bilingual directives (Romanian + English) even though the skill said "English only", and (2) after editing a mode's directive, the user had no way to know the changes were inactive until a Tab-cycle.

## Changes Made
1. `skills/customize_me.md` — Strengthened directive language rule: CRITICAL, English only, no dual-language, no separator labels like `[MODE DIRECTIVE]`, no translations. Directive is for the LLM, not the user.
2. `skills/customize_me.md` — Added Tab-cycle reminder: after creating or editing a mode, the LLM must remind the user to Tab-cycle out and back in for changes to take effect. CurrentMode holds a stale snapshot until the next cycle.

## Decisions And Rationale
- "CRITICAL" tag and explicit prohibitions ("no translations", "no dual-language") needed because the original phrasing ("Always write directives in English") was too weak — the LLM was still bilingualizing due to conversational context.
- Tab-cycle reminder added as a mandatory LLM instruction because the hot-reload only works when `SetMode` creates a fresh pointer. The `CurrentMode` field points to the old slice element and stays stale until explicitly re-pointed.
- No code changes needed — both fixes are pure skill-level instructions.

## Files Included
- skills/customize_me.md: directive English-only enforcement, Tab-cycle reminder after mode edit

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
