# Decision Summary: skill-compatibility-remediation

Date: 2026-07-21 00:00

## Context

Legacy disk skills using obsolete `[BEHAVIOR]`/`[DATA]` sections caused `DiscoverAll` to fail fatally, blocking `RunTurn` with `cannot build prompt` before the LLM could call `load_skill skill-manager` to fix them. The reported `python-epub` skill under app-home was one such failure.

## Changes Made

- `internal/skills/skills.go`: Added `CompatDiag` struct, `isCompatError` helper, updated `discoverFromSubdirs` to collect legacy parse errors as diagnostics instead of failing; changed `DiscoverAll` and `DiscoverProject` signatures to return `[]CompatDiag`.
- `internal/skills/skills_test.go`: Added `TestDiscoverAllCompatDiag` and `TestDiscoverAllNonCompatErrorFatal` to verify diagnostics are collected, legacy skills excluded from valid map, and non-compat errors remain fatal. Updated existing tests for new `DiscoverAll` signature.
- `internal/prompt/prompt.go`: Updated `buildSkillsSection` to render `[SKILL COMPATIBILITY DIAGNOSTICS]` section before valid skills list; extracted `displaySkillName` helper.
- `internal/prompt/prompt_test.go`: Added `TestBuildRuntimePartCompatDiag` verifying diagnostic rendering, legacy exclusion, and valid-skill preservation.
- `internal/runtime/runtime.go`: Updated `DiscoverAll` call to discard diags in load_skill path (prompt builder already surfaces them).
- `internal/console/console.go`: Updated startup splash to display compat diagnostic count when present, and handle empty valid-skill list gracefully.
- `skills/skill-manager.md`: Rewrote body to document canonical `[DESCRIPTION]`/`[BODY]` format; removed obsolete `[BEHAVIOR]`/`[DATA]` section documentation; added legacy migration guidance.
- `specs/04-prompts.md`: Documented compatibility diagnostics rendering in the prompt build flow.
- `specs/09-skill-system.md`: Updated discovery contract, error catalog, and added compatibility diagnostics section.
- `specs/15-runtime-core.md`: Updated flow diagrams to reflect compat diagnostics in prompt build and DiscoverAll return signature.
- `task.md`: Planning artifact for the completed remediation.

## Decisions And Rationale

- Legacy disk skill parse errors classified as `CompatDiag` (non-fatal) while preserving file path and original error for actionable diagnostics.
- Invalid skills excluded from valid map; they remain unavailable until repaired.
- Only builtin `skill-manager` remains loadable during a compatibility diagnostic since embedded builtins are never subject to disk compat errors.
- No parser aliases for `[BEHAVIOR]`/`[DATA]` introduced, preserving strict canonical format per no-fallback directive.
- Console startup splash adapted to show diagnostic count without breaking display when no valid skills are present.
