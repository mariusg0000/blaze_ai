# Session Decision Summary: clean specs-manager triggers

Date: 2026-06-30 13:25
Base commit: dba5aa2

## Context
Multilingual trigger word lists in specs-manager were redundant — LLMs understand concepts, not word lists. Description simplified to semantic English. When To Use section was also redundant with Description and was removed.

## Changes Made
- [DESCRIPTION]: replaced multilingual word list with single English concept description; added "project documentation" trigger
- [BEHAVIOR]: removed entire ## When To Use section (duplicates Description)
- [DESCRIPTION] and [BEHAVIOR] both updated to include "update specs" and "project documentation" concepts

## Implementation Approach
Direct edit of skills/specs-manager.md. No other files affected.

## Files Included
- `skills/specs-manager.md`: trigger and behavior cleanup
- `decisions/2026-06-30-1325-clean-specs-manager-triggers.md`: this summary
