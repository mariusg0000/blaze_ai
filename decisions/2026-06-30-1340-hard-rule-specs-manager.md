# Session Decision Summary: hard rule specs-manager

Date: 2026-06-30 13:40
Base commit: a12f92a

## Context
The model still bypasses specs-manager when the user says "analizează proiectul". The existing gate rule ("scan available skill descriptions") is too soft — the model doesn't connect the request to the skill's description. A hardcoded explicit rule is needed.

## Change Made
Added HARD RULE in prompts/sysprompt.md Skills section: when the user asks to analyze, map, understand, or document the project structure in any language, the model MUST load specs-manager before reading any files or generating output.

## File
- prompts/sysprompt.md: added hard rule line after the existing gate rule
- decisions/2026-06-30-1340-hard-rule-specs-manager.md: this summary
