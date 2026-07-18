# Decision Summary: immutable embedded prompts and builtin skills

Date: 2026-07-18 18:45

## Context

User requested that builtin prompts and the three manager skills (`skill-manager`, `config-manager`, `audit-manager`) remain hardcoded in the binary, never copied as editable files to app home. The three manager names must be reserved for builtins. Existing app-home copies must be deleted. Builtins must take priority over conflicting disk skills.

## Changes Made

**Source code:**
- `main.go` — removed `prepareBuiltinAssets` disk seeding and `seedMissingPromptFiles`; function now returns both embedded prompt and builtin skill `fs.FS` directly; no app-home writes.
- `internal/skills/skills.go` — removed `SeedBuiltins` and `copyBuiltinSubtree`; added `ScopeBuiltin`; `DiscoverAll` now accepts `builtinFS fs.FS`, parses top-level embedded `*.md` as builtin skills, filters colliding disk skills; `Resolve` prefers bare builtin over bare global.
- `internal/skills/skills_test.go` — replaced obsolete seeding tests with builtin discovery, collision filtering, nil/malformed FS, and resolution tests.
- `internal/prompt/prompt.go` and `doc.go` — added `BuiltinSkillsFS` to `Builder`; `buildSkillsSection` uses embedded FS; comments updated for immutable embedded prompts.
- `internal/prompt/prompt_test.go` — all `Builder` instances receive explicit `BuiltinSkillsFS`; added builtin listing/collision test.
- `internal/runtime/runtime.go` — `NewAgent`, `newChildAgent`, `newAgent` accept `builtinSkillsFS`; load_skill uses it.
- `internal/runtime/agent_orchestration.go` — child agents inherit parent's `builtinSkillsFS`.
- `internal/console/console.go` — splash discovery uses builtin FS; `formatSkillName` strips `builtin/` prefix.
- `internal/telegram/telegram.go` — accepts and threads builtin FS.
- `internal/tools/skill_tools.go` — `Execute` strips `builtin/` and `global/` from display; retains `project/`.
- All constructor-using test files updated for new signatures.

**Embedded skill content:**
- `skills/config-manager.md` — inlined `docs/telegram.md` and `docs/helpers.md` into body; replaced `{SKILL_DIR}/docs/...` references with inline guide references.
- `skills/config-manager/docs/telegram.md` and `skills/config-manager/docs/helpers.md` — deleted after inlining.
- `skills/skill-manager.md` — added reserved-name prohibition for `skill-manager`, `config-manager`, `audit-manager`; replaced `Restoring Builtin Skills` section with immutable `Builtin Skills` section; updated scope list to three scopes with builtin priority.
- `internal/platform/apphome_readmes/skills/README.md` — rewritten to match new behavior (three scopes, reserved names, builtin priority).

**Documentation and specs:**
- `README.md` — updated skills section, removed `prompts/` from tree, added reserved-names note.
- `specs.md` — updated Project Map descriptions.
- `specs/02-architecture.md` — updated package roles, dependency graph, skills subsection, startup flow.
- `specs/04-prompts.md` — updated prompt philosophy, variable table, build sequence.
- `specs/09-skill-system.md` — comprehensive rewrite: three scopes, canonical IDs, collision filtering, builtin precedence, discovery signature, display behavior.
- `specs/19-build-deploy.md` — updated embed directives, startup flow, signatures.
- `specs/20-agent-orchestration.md` — child inheritance includes `builtinSkillsFS`.

## Decisions And Rationale

1. **Embedded filesystem threading over disk seeding.** Prompt templates and builtin skills are passed as `fs.FS` values through startup, runtime constructors, prompt Builder, and child agents. No app-home copies are created. This satisfies the immutable requirement and eliminates stale-copy drift.

2. **Three-scope model: builtin/global/project.** Canonical IDs use `builtin/<name>`, `global/<name>`, `project/<name>`. Builtin scope was removed in a previous decision (`2026-06-25-1120`) when skills were seeded to disk. With runtime-immutable builtins, the scope is reintroduced because builtins are now a distinct source.

3. **Builtin collision filtering.** When a builtin and a disk skill share the same bare name, the disk entry is excluded from the discovered map entirely (not just overridden). This means even qualified disk lookups (`global/skill-manager`, `project/skill-manager`) fail, preventing accidental use of stale/custom content.

4. **Reserved names.** `skill-manager`, `config-manager`, `audit-manager` are hardcoded reserved names. `skill-manager` guidance prohibits creating global/project skills with these names. This is enforced at the skill guidance level, not by runtime errors, keeping the implementation simple.

5. **Config-manager docs inlined.** The `config-manager` skill's `docs/telegram.md` and `docs/helpers.md` were inlined into the skill body. This eliminates the `{SKILL_DIR}/docs/...` disk dependency and keeps the embedded skill self-contained. No embedded URI or `read_file` layer was introduced.

6. **App-home copies deleted manually.** The obsolete `~/blazeai/prompts/` directory and `~/blazeai/skills/{skill-manager,config-manager,audit-manager}/` directories were deleted by operator command, not by source code. Automatic deletion was rejected as destructive and outside KISS scope.

7. **Display prefix stripping.** `builtin/` and `global/` prefixes are hidden from prompt lists and load_skill output. `project/` prefix is retained. This keeps the user-facing display clean while preserving scope clarity for project skills.

## Included pre-existing or unrelated changes

None. All committed files are directly related to the immutable embedded assets work.
