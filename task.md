---
task_id: skill-compatibility-remediation
task_revision: 1
status: completed
---

# Remediate incompatible skill files

## Objective

Make legacy skill-format failures explicit and actionable: report the
compatibility error, expose the immutable builtin `skill-manager` for
remediation, and migrate the reported `python-epub` skill to the canonical
format.

## Current Behavior And Evidence

- `internal/skills/skills.go:Parse` requires non-empty `[DESCRIPTION]` and
  `[BODY]` sections and rejects legacy `[BEHAVIOR]`/`[DATA]` content.
- `internal/skills/DiscoverAll` stops on a malformed disk skill.
- `internal/prompt/prompt.go:buildSkillsSection` propagates discovery failure,
  so `RunTurn` returns `cannot build prompt` before the model can call
  `load_skill`.
- `/home/marius/blazeai/skills/python-epub/skill.md` uses `[BEHAVIOR]` and
  `[DATA]`, causing the reported `skill missing [BODY] section` error.
- The embedded `skills/skill-manager.md` has a `[BODY]` section but still
  documents obsolete top-level sections inside its body.

## Scope

- Classify legacy disk-skill parse failures as compatibility diagnostics while
  retaining the offending path and original parse error.
- Surface the diagnostic in the runtime prompt instead of silently dropping the
  invalid skill.
- Make builtin `skill-manager` directly loadable when disk discovery has a
  compatibility diagnostic, so remediation can proceed.
- Update the embedded skill-manager documentation and the reported app-home
  `python-epub` skill to the canonical `[DESCRIPTION]`/`[BODY]` format.
- Add focused parser, discovery, prompt, and load-tool tests.
- Update the affected skill and prompt specifications to describe the
  compatibility-remediation flow.

## Out Of Scope

- Compatibility aliases for legacy sections.
- Silent omission or fallback to a local copy of an invalid skill.
- Automatic rewriting of arbitrary user skill files beyond the reported
  `python-epub` file.
- Changes to the previously committed TODO/IDEA notes.

## Approved Decisions

- The original compatibility error remains visible with its file path.
- Invalid disk skills are unavailable until repaired; they are not treated as
  valid skills.
- Only the immutable builtin `skill-manager` bypasses the failing disk scan for
  the remediation load.
- Canonical skill content uses one `[DESCRIPTION]` section and one `[BODY]`
  section; legacy section names are not parser aliases.

## Rejected Alternatives

- Silently ignoring malformed skills: violates the no-fallback directive.
- Reintroducing `[BEHAVIOR]` and `[DATA]` aliases: preserves incompatible data
  and conflicts with the current strict format.
- Loading a disk `skill-manager`: reserved builtin skills must remain immutable
  and take precedence.

## Relevant Specs

- `specs/04-prompts.md` — runtime prompt build and skill descriptions.
- `specs/09-skill-system.md` — format, discovery, loading, and errors.
- `specs/15-runtime-core.md` — prompt-build error handling in `RunTurn`.
- `specs.md` — no-fallback directive and skill-system project map.
- `AGENTS.md` — documentation, testing, and commit rules.

## Closed File Allowlist

- `read`: `AGENTS.md`, `specs.md`, `specs/04-prompts.md`,
  `specs/09-skill-system.md`, `specs/15-runtime-core.md`.
- `modify`: `internal/skills/skills.go`, `internal/skills/skills_test.go`,
  `internal/prompt/prompt.go`, `internal/prompt/prompt_test.go`,
  `internal/runtime/runtime.go`, `internal/runtime/runtime_test.go`,
  `internal/tools/skill_tools.go`, `internal/tools/skill_tools_test.go`,
  `skills/skill-manager.md`, `specs/04-prompts.md`, `specs/09-skill-system.md`,
  `specs/15-runtime-core.md`, `/home/marius/blazeai/skills/python-epub/skill.md`.
- `create`: `task.md` only as the planning artifact.

## Numbered Implementation Steps

1. Add the smallest typed compatibility diagnostic needed to distinguish legacy
   disk skill-format failures from other discovery failures.
2. Preserve valid discovery results while surfacing the diagnostic in the
   runtime skill section; keep non-compatibility discovery failures fatal.
3. Allow only builtin `skill-manager` to load directly during a compatibility
   diagnostic and keep other skill loads explicit errors.
4. Rewrite the embedded manager guidance and the reported app-home skill using
   the canonical format without legacy aliases.
5. Add focused regression tests and update the affected specifications.

## Tests And Assertions

- `go test ./internal/skills ./internal/prompt ./internal/tools ./internal/runtime`
- `go test ./...`
- `go build ./...`
- `git diff --check`
- Assert that a legacy disk skill produces a path-bearing compatibility
  diagnostic, is not listed as valid, and does not prevent builtin
  `skill-manager` from loading.
- Assert that malformed non-compatibility discovery errors remain fatal.
- Assert that the migrated `python-epub` content parses with a non-empty body.

## Acceptance Criteria

- The reported `python-epub` error is eliminated after migration.
- Legacy-format failures are explicitly returned/surfaced and never silently
  ignored.
- Builtin `skill-manager` is available for remediation when such a failure is
  present.
- No fallback or legacy parser alias is introduced.
- All listed tests, build, and whitespace checks pass.

## Permitted Verification Commands

- `git status --short`
- `go test ./internal/skills ./internal/prompt ./internal/tools ./internal/runtime`
- `go test ./...`
- `go build ./...`
- `git diff --check`

## Risks And Limitations

- The app-home skill path is outside the repository and must be changed
  explicitly; the source repository cannot embed that user-owned file.
- The remediation prompt can report only the discovery diagnostics available at
  build time; it does not automatically rewrite arbitrary invalid skills.

## Delegation Order

1. Primary implementation: source and test changes in the closed allowlist.
2. Documentation: update the three affected specs in the same implementation.
3. Operational data: migrate the explicitly reported app-home skill.
