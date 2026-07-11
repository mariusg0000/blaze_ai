# Session Decision Summary: Chronological compaction prompts and skill cleanup

Date: 2026-07-11 12:15

## Context

The generated compaction summary contained literal prompt excerpts instead of a useful continuation record. The project also contained a hard runtime rule and builtin documentation for `specs-manager`, although that skill is not available or seeded.

## Changes Made

The token-compaction prompt now requests a telegraphic chronological work log preserving user requirements, approved plans, task-list status, decisions, implementation, validation, and unresolved items. The TaskSwitcher pre-switch summary follows the same format while retaining its strict `null` or JSON protocol. Both runtime prompt copies no longer require `specs-manager`; builtin skill documentation and current skill maps were updated accordingly. Historical decision files were left unchanged.

## Decisions And Rationale

Chronological lists preserve causality, approvals, plan progression, and task status better than thematic summaries. Plans and task lists are kept nearly complete because they are continuation-critical; wording is compressed instead of removing steps. Source code, prompt templates, tool arguments, and long quotes are explicitly excluded so summaries retain behavior and outcomes rather than transcript artifacts. The obsolete hard rule was removed instead of adding a replacement fallback because the skill no longer exists.

## Validation

- `go test ./internal/compaction`
- `git diff --check`
