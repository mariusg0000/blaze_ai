[DESCRIPTION]
Load when the user wants to understand, map, analyze, document, update specs, or create project documentation for the project — regardless of language. Do not use for normal code review, README edits, one-function documentation, config editing, skill creation, or routine file operations.

[BEHAVIOR]
# Specs Manager

## Purpose

Generate and maintain project context documentation: a single `specs.md` file at the project root containing project description, file structure map, and an optional index of detailed architecture specification files. When the user requests full analysis, also create individual spec files under `specs/`. The skill provides two paths: short (map only) or full (map + detailed specs). Max 20 spec files in `specs/`.

## Workflow

### Step 1: Read Existing State

1. Check if `specs.md` exists in the workdir root. If yes, read it in full.
2. Check if `specs/` folder exists. If yes, list its contents.
3. Check if a legacy `project-map.md` exists in the workdir root. If yes, read it for migration into the Map section of the new `specs.md`.
4. Read existing project docs: `README.md`, `AGENTS.md`, `CONTRIBUTING.md` (optional).

### Step 2: Choose The Path

Based on what exists:

| Existing state | Action |
|----------------|--------|
| Nothing (no `specs.md`, no `specs/`) | Go to **Path A** (propose short or full) |
| Only `specs.md` (no `specs/` folder) | Go to **Path B** (update Map, propose Specs) |
| `specs.md` + `specs/` folder | Go to **Path C** (full update) |

If a legacy `project-map.md` exists, migrate its content into the Map section of `specs.md` regardless of the chosen path, then ask whether to delete `project-map.md`.

---

### Path A: Nothing Exists (First-Time Generation)

3. **Scan the project structure:**
   - Identify the top-level layout: entry points (`main.go`, `cmd/`, `src/`, `index.*`), important folders (`internal/`, `pkg/`, `api/`, `config/`, `docs/`), build/test files, and noisy areas (vendored deps, generated output, caches, assets).
   - Check for a `decisions/` folder. If present, list its contents. Read only decisions relevant to the current section: for the Description, a few key architectural choices; for Map-only, decisions are not required — structure alone suffices; for individual spec files, only decisions related to that concept. If no `decisions/`, run `git log --oneline`. If neither exists, rely on code only.
   - Prefer `fd -t d -d 2` or `ls -la` for fast discovery. Use `rg --files` to sample relevant files.
   - Read a small sample of high-value files only when needed to explain their role.

4. **Present two options to the user BEFORE writing anything:**

```markdown
**Option 1 — Short (Map only):**
Generates `specs.md` with Description + Map sections.
Fast (~2 minutes). One file. No `specs/` folder.

**Option 2 — Full (Map + Specs):**
Generates `specs.md` with Description + Map + Specs index,
plus individual spec files under `specs/` (up to 20).
Slow (~10+ minutes). Deep architecture documentation.
```

5. Wait for user choice. If user chooses "short", go to **Path A-Short**. If "full", go to **Path A-Full**.

6. **Ask for User Directives:**
   - Before writing any file, ask: "Do you have any project-specific directives or rules you want included in the User Directives section? (no, or yes: <text>)"
   - If the user provides text: note it. You may clarify and restructure the text for readability without distorting the meaning. Mark items with !!IMPORTANT!! or !!MANDATORY!! as appropriate.
   - If the user says no: use the default placeholder.
   - Whether writing Path A-Short (step 9) or Path A-Full (step 15), include the `## User Directives` section with the user's text or placeholder between the title and Description.

#### Path A-Short: Map Only

7. **Write the Description section:**
   - 1-2 paragraphs: what the project does, primary language/framework, entry points, key technologies.
   - Be factual, not marketing. Extract from README and AGENTS.md if available.

8. **Write the Map section:**
   - Tree-like Markdown bullet structure.
   - One short sentence per important folder or file explaining its role.
   - Summarize noisy subtrees at folder level (e.g. `node_modules/`, `vendor/`, `dist/`, `__pycache__/`).
   - Go deeper only where structure alone is not enough to understand purpose.
   - Target 20-40 lines. Never exceed 60 lines. If the project is too large, summarize at higher level and offer to drill into subtrees on request.
   - Do not include secrets, `.env` values, copied file contents, or large code excerpts.

9. **Write `specs.md`** following the template in `[DATA]`. Omit the Specs section and replace it with the placeholder sentence: `*No detailed specs yet. Run the skill with full analysis to generate them.*`

10. Done. Ask if user wants to add Specs later.

#### Path A-Full: Map + Specs

11. **Write Description + Map** (same as Path A-Short steps 7-8).

12. **Identify concepts for specs (max 20):**
    - Group packages and code areas into discrete architectural concepts.
    - One concept = one spec file in `specs/`.
    - Assign 5-15 keywords per concept using real technical terms from the codebase.
    - Present the concept list to the user BEFORE creating any spec files:

```markdown
| # | File | Concept | Keywords |
|---|------|---------|----------|
| 01 | specs/01-product-scope.md | Product Scope | product, scope, users, CLI |
| 02 | specs/02-architecture.md | Architecture | modules, layers, dependency |
| ... | ... | ... | ... |
```

13. User approves, reorders, merges, or removes entries.

14. For each approved concept, generate the spec file:
    - Find relevant source files from the package mapping.
    - Read key types, interfaces, constants, config structs, and public functions.
    - Find and read test files for behavioral contracts and edge cases.
    - Write the spec file following the individual spec template in `[DATA]`.
    - Incremental mode (default): one spec at a time, user confirms each.
    - Batch mode: only if user explicitly says "generate all specs".

15. Write `specs.md` with full Description + Map + Specs index table. Specs section lists each file by its relative path (`specs/NN-concept.md`).

---

### Path B: Only `specs.md` Exists (Map Update, Propose Specs)

16. **Read existing `specs.md`** — note what Description and Map currently contain.

17. **Update the Description** if the project has changed (new entry points, new technologies). If unchanged, keep as-is.

18. **Update the Map section:**
    - Re-scan the project structure.
    - Compare with the existing Map. Add new folders/files that appeared. Remove ones that were deleted. Update descriptions for changed roles.
    - Keep the same format and style as the existing Map.

19. **Write updated `specs.md`.** The Specs section (if present) is untouched. If the Specs section is the placeholder sentence, keep it.

20. **After updating, propose Specs generation:**
    - "The Map is updated. The Specs section is still empty. Would you like to generate detailed architecture specifications now? This will create a `specs/` folder with individual spec files."

21. If user says yes, execute **Path A-Full** from step 12 onward (concept identification and spec generation).

---

### Path C: Full Update (`specs.md` + `specs/`)

22. **Read existing `specs.md`** and all files in `specs/`.

23. **Update Description** — same as step 17.

24. **Update Map** — same as step 18.

25. **Update Specs section:**
    - Check which source files have changed since the specs were last written (compare spec content against current code, or use file modification timestamps).
    - Propose which specs need updating. Do not update all blindly.
    - For each affected spec: re-read the source files, update the spec file, re-write it.
    - If new packages or modules appeared that are not covered by any existing spec, propose adding them (within the max 20 limit).
    - If packages were removed, propose removing the corresponding spec file and updating the index.

26. **Update the Specs index table** in `specs.md` to reflect any new, removed, or renamed spec files.

27. **If user says "regenerate spec for X"** — re-read the source files for that concept and rewrite the spec file completely.

---

## Spec File Rules

- One concept per file. No cross-cramming.
- Max 20 spec files total in `specs/`. If a 21st concept is needed, propose merging two lower-priority concepts first.
- File name: `NN-concept-name.md` where `NN` is the zero-padded index number (matching the `specs.md` index table).
- Title: `# Concept Name` (one per file).
- Every spec MUST include a Source Files table with real file paths.
- Code examples MUST use real types, real config keys, real constants from the source — never placeholders.
- Include defaults, thresholds, error types, and edge cases from tests.
- Reference other spec files by relative path when needed: `specs/NN-related-concept.md`.
- No filler paragraphs. No marketing language. No redundant cross-references.
- If a concept is simple and the source is under ~100 lines, make the spec proportionally short.

## Specs Index Table Rules

- Each row: `#`, `File` (relative path), `Concept` (short title), `Keywords` (5-15, comma-separated, real codebase terms).
- Keywords help search and discoverability. Use file names, type names, package names, config keys from the codebase — not generic categories.
- The table must match the actual files in `specs/` exactly.

## Map Section Rules

- Tree-like Markdown bullet structure starting from the project root.
- One short sentence per important folder or file explaining its role.
- Summarize noisy subtrees at folder level instead of listing every file.
- Target 20-40 lines. Never exceed 60 lines. If the project is too large, summarize at higher level and offer to drill into subtrees on request.
- Do not include secrets, `.env` values, copied file contents, or large code excerpts.
- Prefer fast file discovery helpers (`fd`, `rg --files`) over manual `ls`.
- Avoid broad deep reads before the structure is clear.

## Description Section Rules

- 1-2 paragraphs. Maximum 8 sentences total.
- What the project does, not how it was built.
- Primary language, framework, entry points.
- Key dependencies or technologies visible at the top level.
- Be factual, not marketing. Extract from existing `README.md` or `AGENTS.md` if available.

## Scanner Rules

- Start with a fast shallow scan: top-level listing, entry point files, config files.
- Discover project rationale: if a `decisions/` folder exists, list the files but read only those relevant to the section being written. For the Description, a few key architectural decisions may help. For Map-only generation, decisions are not needed — rely on README/AGENTS.md and code structure. For individual spec files, read only decisions relevant to that concept. If no `decisions/` folder exists, run `git log --oneline` to infer project intent. If neither is available, rely strictly on source code.
- Identify important directories versus noisy directories quickly.
- Read only the minimum number of lines from internal files needed to understand their role.
- Stop scanning and ask the user if the repository is too large or key areas stay unclear after the shallow scan.
- Never read `.env` files, secrets, keys, tokens, or password files.

## Safety Rules

- The `## User Directives` section in `specs.md` is user-owned. Read and follow its directives. Modify ONLY when the user explicitly requests it (e.g., "add a directive", "remove directive X", "update the User Directives"). When the user provides new text for this section, you may clarify and restructure it for readability without distorting the meaning. Never add, change, or remove directives on your own initiative.
- Do not read or write secrets, `.env` files, encrypted config values, API keys, or tokens.
- Do not modify source code files. Specs are documentation only.
- Do not create more than 20 spec files in `specs/`.
- If source code is unavailable (binary-only project), document only what is visible and mark gaps explicitly.
- If a `shell` command fails during discovery, report the error and stop. No fallbacks.

[DATA]
specs_dot_md_template:
```markdown
# Project: `<name>`

## User Directives

*No user directives yet. Mark items with !!IMPORTANT!! or !!MANDATORY!!.*

## Description

<1-2 paragraphs — what the project does, primary language/framework, entry points, key technologies. Max 8 sentences. Factual, not marketing.>

## Map

<tree-like Markdown bullet structure. Target 20-40 lines. Max 60.>
<Use this format:>
```
- `README.md` — project readme
- `main.go` — application entry point
- `internal/` — private application packages
  - `internal/runtime/` — agent core loop
  - `internal/config/` — configuration load/save/validate
  - `internal/tools/` — native tool implementations
- `skills/` — builtin skill templates
- `prompts/` — system prompt templates
```

## Specs

| # | File | Concept | Keywords |
|---|------|---------|----------|
| 01 | specs/01-architecture.md | Architecture | modules, layers, dependency, data flow |
| 02 | specs/02-config-schema.md | Config Schema | provider, model, roles, config.json |

*No detailed specs yet. Run the skill with full analysis to generate them.*
```

individual_spec_template:
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
ASCII diagram of the main flow or sequence. Use plaintext code blocks.

### [Configuration]
Table with field name, type, default, and meaning. Only if the concept
involves user-facing configuration.

### [Error Handling]
Error types, error messages, and error paths. Extract from test files.

### [Design Decisions]
Why a specific approach was chosen over alternatives. Extract from
decision records or AGENTS.md where available. 1-3 bullet points.

### [Constraints]
Hard limits, caps, timeouts, size limits, or platform restrictions.

## See Also
- `specs/NN-related.md` — related concept
```
