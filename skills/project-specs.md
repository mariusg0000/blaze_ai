[DESCRIPTION]
Load when the user message contains one of the trigger words: specs, specifications, specificații, specificatie, spezifikationen, spécifications, especificaciones, especificações, specifiche, or any recognizable multilingual form of "specifications". Use when the user wants to generate, update, refresh, or maintain formal architecture specification documents for the current project. Works with `shell`, project-map (if available), and file-writing tools. Generate max 20 spec files total. Do not use for normal code review, README edits, project-map generation, one-function documentation, or explaining a single file.

[BEHAVIOR]
# Project Specs

## Purpose

Generate a structured set of architecture specification documents (`specs/NN-concept.md`) that describe the project's modules, data flow, configuration, contracts, subsystems, platform rules, and key design decisions. Max 20 spec files. One concept per file.

Specs are living documents. They are read by future agents to understand the codebase faster than scanning every source file.

## When To Use

Load this skill when the user message contains a trigger word (specs, specifications, or any multilingual equivalent) AND the user wants to:

- generate new architecture specs from scratch
- add missing spec files to an existing `specs/` folder
- update a specific spec after code changes
- refresh all stale specs after a refactoring
- check spec coverage ("what specs are missing?")

Do not load for:

- writing or editing a README
- generating `project-map.md`
- documenting one function or one file
- normal code review
- writing inline code comments
- editing config, provider, model, or mode settings
- creating or editing skills

## Trigger Word Detection

The user message MUST contain a trigger word. Supported forms include:

- English: specs, specifications
- Romanian: specificații, specificatie
- German: spezifikationen, spezifikation
- French: spécifications, spécification
- Spanish: especificaciones, especificación
- Portuguese: especificações, especificação
- Italian: specifiche, specifica
- Other close variants of the word "specification" in the user's language

A message without any trigger word is not a match, even if it requests documentation.

## Workflow

### Phase 0: Understand The Project (read-only)

1. Read existing project docs: `README.md`, `AGENTS.md`, `CONTRIBUTING.md` (if present).
2. If `specs/` already exists, read `specs/README.md` (the tracker) and every existing spec file. Note what is already covered and what is stale.
3. Scan project structure:
   - If `project-map.md` exists in the workdir, read it directly.
   - Otherwise, use `fd -t d -d 2` or `ls -la` to list the top-level structure and primary packages.
4. Identify entry points: `main.go`, `cmd/`, `index.js`, `src/main/`, or equivalent. Trace imports to find the core dependency graph.
5. Identify key packages, modules, config files, API contracts, build scripts, and test directories.

Stop here. Write nothing yet. Ask the user to confirm scope before proceeding to Phase 1.

### Phase 1: Identify Concepts (proposal only)

6. Group packages and code areas into discrete architectural concepts. Each concept should be independently understandable but cross-reference related specs where needed.
7. Examples of concept categories:
   - Product scope (identity, users, priorities, interaction model)
   - Architecture (module graph, layers, dependency rules, data flow)
   - Configuration (schema, validation, modes, env vars)
   - Core runtime (agent loop, turn lifecycle, handler contract)
   - Subsystems (tools, prompts, sessions, skills, compaction)
   - I/O and transports (console, API, web, bridge)
   - Platform and ops (OS detection, build, deploy, safety, first-run)
   - Cross-cutting (error handling, logging, telemetry)
8. Assign 5-15 keywords per concept. Keywords must be real technical terms from the codebase (file names, type names, package names, config keys), not generic categories.
9. Present the concept list to the user as a numbered table BEFORE writing any spec file:

```markdown
| # | Concept | Source area | Keywords |
|---|---------|-------------|----------|
| 01 | Product Scope | main.go, firstrun.go | product, scope, users, CLI, REPL, shell, transport |
| 02 | Architecture | main.go, internal/ | modules, layers, dependency, data flow, startup |
| 03 | Config Schema | internal/config/ | provider, model, roles, compaction, config.json |
| ... | ... | ... | ... |
```

10. Max 20 entries. If the project needs more, propose merging closely related concepts or splitting only the most critical ones. The user makes the final call.
11. Wait for user approval before writing anything. User may reorder, rename, merge, split, or remove entries.

After user approval, create `specs/README.md` (the tracker) with all entries marked `⏳ pending`. Still no spec files written.

### Phase 2: Generate Specs (one at a time, incremental)

12. For each approved concept, in order:
    a. Find the relevant source files from the package/area mapping.
    b. Read key types, interfaces, constants, config structs, and public functions.
    c. Find and read test files (`*_test.go` or equivalent) to extract behavioral contracts, edge cases, and expected error conditions.
    d. If a `decisions/` folder or architecture decision records exist, read the relevant ones for design rationale.
    e. Write the spec file following the template in `[DATA]`.
    f. Update the tracker: set status to `✅ YYYY-MM-DD` for the completed entry.

13. Default mode: one spec at a time. After each spec, present the result and ask whether to continue.
14. Batch mode: if the user explicitly says "generate all specs", write all approved specs in sequence without asking per file. Still apply the max-20 cap.
15. Regeneration: if a spec file already exists and has a recent date, skip it unless the user explicitly asks to regenerate or the source files have changed since the last spec date.

### Phase 3: Maintain Specs

16. When the user says "update spec for <concept>" or "refresh specs after <change>":
    a. Read the existing spec file.
    b. Re-read the source files that changed since the last spec date (use `git diff` or file modification timestamps).
    c. Update only the affected sections: source file table, data structures, flow diagrams, config defaults, error types.
    d. Update the tracker date.
    e. Do not rewrite the entire spec unless the concept itself has fundamentally changed.

17. When the user says "what specs are missing?":
    a. Compare the tracker against the current project structure.
    b. Identify new packages, new entry points, or config areas not covered by any existing spec.
    c. Propose missing entries with keywords.
    d. Wait for approval, then generate only the missing specs.

18. When the user says nothing about specs but major code changes are visible (new packages, removed modules), suggest refreshing affected specs proactively but do not act without approval.

## Spec File Rules

- One concept per file. No cross-cramming.
- Max 20 spec files total per project. If a 21st concept is needed, propose merging two lower-priority concepts first.
- File name: `NN-concept-name.md` where `NN` is the zero-padded tracker number.
- Title: `# Concept Name` (one per file).
- Every spec MUST include the Source Files table with real file paths.
- Code examples MUST use real types, real config keys, real constants from the source — never placeholders.
- Include defaults, thresholds, error types, and edge cases from tests.
- Reference other spec files by path when needed: `specs/NN-related-concept.md`.
- No filler paragraphs. No marketing language. No redundant cross-references.
- If a concept is simple and the source is under ~100 lines, make the spec proportionally short. If complex, go deeper. The spec size should match the concept complexity, not a fixed length.

## Tracker Rules

- Tracker lives at `specs/README.md`.
- It is the first file created (after Phase 1 approval) and the last file updated (after every spec write or update).
- Format: maintenance section (how to update specs) + index table.

## Safety Rules

- No fallback if `shell` fails during discovery. Report the error and stop.
- Do not read secrets, `.env` files, or encrypted config values.
- Do not include passwords, tokens, or API keys in spec files.
- Do not modify source code files. Specs are documentation only.
- Do not create more than 20 spec files. If the project genuinely needs more, stop and ask.
- If source code is unavailable (binary-only project, closed-source deps), document only what is visible and mark gaps explicitly.

[DATA]
spec_file_template:
```markdown
# Concept Name

## Source Files

| File | Role |
|------|------|
| `path/to/file.go` | What it does — one short sentence |
| `path/to/another.go` | What it does — one short sentence |

## Overview

1-2 paragraphs defining what this concept is, where it fits in the system,
and why it exists. Mention the main entry points and the primary data types
or interfaces it exposes.

## [Detailed Sections — add as needed per concept]

Use `##` for major sections, `###` for minor ones. Typical sections:

### [Data Structures]
Go/JSON/TypeScript examples of key structs, interfaces, enums, or config shapes.

### [Flow]
ASCII diagram of the main flow or sequence. Use ` ```text ` blocks.

### [Configuration]
Table with field name, type, default, and meaning. Only if the concept
involves user-facing configuration.

### [Error Handling]
Error types, error messages, and error paths. Extract from test files
for accuracy.

### [Design Decisions]
Why a specific approach was chosen over alternatives. Extract from
decision records or AGENTS.md where available. Keep to 1-3 bullet points.

### [Constraints]
Hard limits, caps, timeouts, size limits, or platform restrictions that
affect this concept.

## See Also
- `specs/NN-related.md` — related concept
```

tracker_template:
```markdown
# Project Specs

## Maintenance

Specs are living documents. After significant code changes (new packages,
refactored modules, changed APIs, renamed files), run the `project-specs`
skill again with one of:

- "refresh specs" — checks all specs for staleness and updates affected ones
- "update spec for <concept>" — updates one specific spec
- "what specs are missing?" — compares spec coverage against current codebase

To regenerate a specific spec from scratch: "regenerate spec for <concept>".

Specs reference real source files by path. When files are moved or renamed,
the affected specs and their source file tables must be updated.

## Index

| # | File | Concept | Keywords | Status |
|---|------|---------|----------|--------|
| 01 | `01-concept.md` | Short title | kw1, kw2, ..., kw15 | ⏳ pending |
| 02 | `02-concept.md` | Short title | kw1, kw2, ..., kw15 | ⏳ pending |
```
