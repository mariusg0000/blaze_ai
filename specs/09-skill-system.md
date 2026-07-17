# Skill System

## Sources

| File | Role |
|------|------|
| `internal/skills/skills.go` | Skill model, strict parser, discovery, resolution, and builtin seeding |
| `internal/skills/skills_test.go` | Format, validation, discovery, collision, and seeding tests |
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

Global skills use `app_home/skills/<name>/skill.md`. Project skills use `app_home/projects/<project>/skills/<name>/skill.md`. Embedded builtin templates are seeded into the global directory only when the destination file is missing; existing user files are never overwritten.

Canonical IDs use `global/<name>` and `project/<name>`. A bare name resolves through the existing scope rules; callers may use `project/<name>` explicitly. A trailing `.md` is normalized by `load_skill`.

## Discovery and Validation

`DiscoverAll(workDir)` scans global and project skill directories and returns parsed skills keyed by canonical ID. Missing scope directories and skill subdirectories without `skill.md` are skipped. A present but malformed `skill.md` stops discovery with an error containing the file path and parse failure; malformed skills are not silently ignored.

Descriptions are rebuilt from disk on every prompt build. The system prompt renders one compact entry per discovered skill:

```text
- skill-name = description
```

Project IDs retain their `project/` prefix in the displayed list; the `global/` prefix is omitted.

## Loading Flow

`load_skill` accepts one required `name` string.

1. Normalize an optional `.md` suffix.
2. Rediscover skills from disk.
3. Resolve the requested name to a canonical ID.
4. Expand supported prompt variables in `Skill.Body`, including `{SKILL_DIR}`.
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
- Malformed discovered file: contextual discovery error with path.
- Invalid JSON or empty tool name: explicit tool error.
- Variable rendering failure: propagated as a load error.
- No compatibility fallback for old section names.
