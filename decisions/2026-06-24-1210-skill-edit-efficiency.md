# Session Decision Summary: skill edit efficiency

Date: 2026-06-24 12:10
Base commit: 8b70a0aae83ffd4036433a27343ffeb027094136

## Context
The working tree contained direct edits to the universal system prompt and `skill-manager`. The update focuses on making skill edits more efficient by encouraging fewer edit calls, larger coherent transforms, and cleaner verification.

## Changes Made
Updated `prompts/sysprompt.md` with a new `File Edit Efficiency` section that tells the model to minimize edit tool calls, prefer one batched transformation for small files, and avoid more than a few edits to the same file. Updated `skills/skill-manager.md` to describe skills more clearly as procedural content and to tighten its guidance about what belongs in a skill versus a memory bank.

## Decisions And Rationale
The prompt needed an explicit rule that discourages fragmented file edits because repeated small tool calls bloat context and slow down the task. `skill-manager` was also simplified to keep its advice focused on skill content rather than low-level editing behavior.

## Implementation Approach
This was a prompt-and-skill-text change only. The runtime prompt now includes a specific file-edit efficiency block, and `skill-manager` now frames skill content in terms of workflow, pitfalls, and memory separation rather than edit mechanics.

## Alternatives Considered
Leaving the instructions broad would keep the same fragmented edit behavior. Adding more edit exceptions was rejected because the goal was to make the guidance simpler and more direct.

## Files Included
- `prompts/sysprompt.md`: added file-edit efficiency guidance.
- `skills/skill-manager.md`: refined skill guidance to stay procedural and memory-aware.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
