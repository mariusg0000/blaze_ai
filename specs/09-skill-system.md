# Skill System

## Sources

| File | Role |
|------|------|
| `internal/skills/skills.go` | Skill model, strict parser, CompatDiag type, discovery, resolution, and builtin precedence |
| `internal/skills/skills_test.go` | Format, validation, discovery, collision, and embedded builtin precedence tests |
| `internal/tools/skill_tools.go` | `load_skill` tool contract |
| `internal/tools/skill_tools_test.go` | Tool-result and error behavior tests |
| `internal/prompt/prompt.go` | Available-description injection and body variable rendering |
| `internal/runtime/runtime.go` | Tool registration and standard tool-result persistence |
| `skills/` | Embedded builtin skill templates |

## Format

Each skill is a Markdown file with exactly two required, non-empty sections:

```text
[DESCRIPTION]
A compact description used to decide when the skill is relevant.

[BODY]
Instructions, reference data, examples, or workflows loaded into the conversation.
```

`[DESCRIPTION]` is injected into the runtime system prompt as part of the available-skill list. `[BODY]` is never injected into the system prompt. The old `[BEHAVIOR]` and `[DATA]` sections are invalid and are not accepted as aliases.

`Skill` contains `Name`, `Description`, `Body`, `Dir`, and `Scope`. `Parse()` returns `ErrMissingDescription` or `ErrMissingBody` when a required section is missing or empty.

## Storage and Scopes

Three scopes exist with canonical IDs:

- **Builtin** — `builtin/<name>`: read directly from the immutable embedded FS. Reserved builtin names `skill-manager`, `config-manager`, and `audit-manager` always take priority and completely shadow same-name global/project files, including qualified disk lookup, because conflicting files are filtered from the discovered map. Builtin `Skill.Dir` is empty and renders `{SKILL_DIR}` as `NULL`.
- **Global** — `global/<name>`: `app_home/skills/<name>/skill.md`.
- **Project** — `project/<name>`: `app_home/projects/<project>/skills/<name>/skill.md`.

A bare name resolves through the existing scope rules; callers may use `project/<name>` explicitly. A trailing `.md` is normalized by `load_skill`.

## Discovery and Validation

`DiscoverAll(workDir, builtinFS)` first discovers embedded builtins: parses top-level `*.md` files from the embedded FS, ignores embedded directories and non-Markdown entries, and fails on nil or malformed builtin FS or content. Then it discovers disk global and project skills and filters collisions where reserved builtin names take priority. Returns `(map[string]*Skill, []CompatDiag, error)`.

### Compatibility Diagnostics

A legacy disk skill with missing `[BODY]` or `[DESCRIPTION]` sections (e.g. still using obsolete `[BEHAVIOR]` or `[DATA]`) produces a `CompatDiag` instead of a fatal error. The diagnostic preserves the absolute file path, skill name, scope, and original parse error. Non-compatibility errors such as read failures or malformed builtin content remain fatal.

Valid disk skills are still discovered alongside diagnostics. The invalid skill is never added to the valid skills map. Missing scope directories and skill subdirectories without `skill.md` are skipped silently.

### Prompt Integration

The `buildSkillsSection` method in the prompt builder renders compatibility diagnostics as a `[SKILL COMPATIBILITY DIAGNOSTICS]` section before the available skill descriptions. This means the LLM sees the file path and error, and can immediately call `load_skill skill-manager` to remediate the legacy format. Builtin `skill-manager` is always available because embedded builtins are never subject to disk compatibility diagnostics.

Descriptions are rediscovered from the immutable embedded FS and disk on every prompt build. The system prompt renders one compact entry per discovered skill:

```text
- skill-name = description
```

Builtin and global IDs omit their prefix in the displayed list; project IDs retain their `project/` prefix.

## Loading Flow

`load_skill` accepts one required `name` string.

1. Normalize an optional `.md` suffix.
2. Rediscover embedded builtins plus disk user skills.
3. Resolve the requested name to a canonical ID.
4. Expand supported prompt variables in `Skill.Body`, including `{SKILL_DIR}` (real disk directory for global/project skills, `NULL` for builtins).
5. Return the expanded body from the tool:

```text
Skill loaded: <display-name>

<expanded body>
```

The runtime's existing generic tool-call flow appends this result as a standard `tool` message in `session.json`. It is sent on subsequent LLM calls as ordinary conversation history. There is no active-skill list, special skill persistence, repeated system-prompt injection, deduplication, or `unload_skill` tool. Calling `load_skill` again creates another normal tool result.

## Prompt Boundary

`Builder.BuildRuntimePart()` injects only discovered descriptions through `{SKILLS_AVAILABLE}`. `Builder.RenderSkillBody()` expands variables for the selected body when `load_skill` executes. `Builder.Build(session)` prepends the system message and then appends persisted session messages unchanged.

Because loaded bodies are ordinary history, session resume and context compaction follow the same rules as every other tool result. No separate skill state must be reconstructed.

## Errors and Constraints

- Empty or missing `[DESCRIPTION]`: `ErrMissingDescription`.
- Empty or missing `[BODY]`: `ErrMissingBody`.
- Missing skill: explicit resolution error.
- Malformed discovered builtin: contextual discovery error with path (fatal).
- Legacy disk skill (missing section): `CompatDiag` with path and parse error (non-fatal).
- Non-compatibility disk error (read failure, etc.): fatal discovery error.
- Invalid JSON or empty tool name: explicit tool error.
- Variable rendering failure: propagated as a load error.
- No compatibility fallback for old section names.
