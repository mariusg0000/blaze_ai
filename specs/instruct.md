# Specification Writing Instructions

## Goal
Each file in `new_specs/` is a complete, accurate specification for ONE concept. Skeletons exist — flesh them out with real content from code + decisions.

## Process (per file)
1. **Choose next unfinished file** from progress tracker below (first unfinished, top to bottom).
2. **Search decisions/** — grep for any `.md` files mentioning the topic. Read relevant ones for architectural rationale.
3. **Analyze source code** — read every Go file in the relevant packages. Extract key types, functions, constants, interfaces.
4. **Search tests** — check `*_test.go` files for edge cases and behavioral contracts.
5. **Write the file**:
   - Expand skeleton with real content
   - Document key data structures with JSON or Go-like examples
   - Reference source files by path
   - Include relevant defaults, thresholds, error types
   - Add behavioral rules and constraints from code
   - Keep one concept per file — no cross-cramming
6. **Update progress** below: mark the file as `✅ YYYY-MM-DD` with the date completed.

## Format
- Title: `# Title` (one per file)
- Sections per topic, `##` for major, `###` for minor
- Code blocks with language tags where useful
- Source file references: `path/to/file.go` in context
- No filler, no marketing, no redundant cross-references

## Progress

| # | File | Status |
|---|------|--------|
| 01 | `01-product-scope.md` | ✅ 2026-06-29 |
| 02 | `02-architecture.md` | ✅ 2026-06-29 |
| 03 | `03-config-schema.md` | ✅ 2026-06-29 |
| 04 | `04-prompts.md` | ✅ 2026-06-29 |
| 05 | `05-tools.md` | ✅ 2026-06-29 |
| 06 | `06-shell-execution.md` | ✅ 2026-06-29 |
| 07 | `07-file-editing.md` | ✅ 2026-06-29 |
| 08 | `08-cross-model-delegation.md` | ✅ 2026-06-29 |
| 09 | `09-skill-system.md` | ✅ 2026-06-29 |
| 10 | `10-sessions.md` | ✅ 2026-06-29 |
| 11 | `11-context-compaction.md` | ✅ 2026-06-29 |
| 12 | `12-handler-contract.md` | ✅ 2026-06-29 |
| 13 | `13-console-ui.md` | ✅ 2026-06-29 |
| 14 | `14-telegram-bridge.md` | ✅ 2026-06-29 |
| 15 | `15-runtime-core.md` | ✅ 2026-06-29 |
| 16 | `16-first-run.md` | ✅ 2026-06-29 |
| 17 | `17-platform.md` | ✅ 2026-06-29 |
| 18 | `18-safety.md` | ✅ 2026-06-29 |
| 19 | `19-build-deploy.md` | ✅ 2026-06-29 |
