# Decision Summary: default interactive agent bootstrap

Date: 2026-07-24 10:14

## Context

A fresh BlazeAI app home with no interactive agent definitions and no agents.json
state would fail at NewAgent with a bare "no interactive agent definitions found"
error, requiring manual agent creation before first use. The user authorized an
approved task to bootstrap a user-editable default.md on first run so new users
get a working interactive agent immediately.

The implementation scope was restricted to exactly four source/test paths:
internal/agents/bootstrap.go, internal/agents/bootstrap_test.go,
internal/runtime/runtime.go, and internal/runtime/runtime_test.go, plus a
task.md status update.

## Changes Made

- internal/agents/bootstrap.go: new EnsureDefaultInteractive helper that
  creates a starter agents/default.md with validated model, sorted tool allowlist
  (excluding agent_done and run_agent), sorted executor names, and a fixed
  interactive body. Never overwrites an existing file. Validates model format
  before writing.

- internal/agents/bootstrap_test.go: four tests covering creation with correct
  parsed output, overwrite prevention, invalid model rejection, and write error
  propagation.

- internal/runtime/runtime.go: inserted bootstrap flow after initial definition
  load but before the hard error for zero interactive definitions. When
  agents.json does not exist and no interactive definitions are found, collects
  executor names, calls EnsureDefaultInteractive with cfg.Roles.Default and
  registeredToolNames, then reloads definitions. When agents.json already exists,
  the existing missing-interactive-agent error is returned unchanged.

- internal/runtime/runtime_test.go: replaced the old no-definition failure test
  with TestNewAgentBootstrapsDefaultInteractiveAgent (isolated HOME, no
  definitions or agents.json, verifies default.md creation, parsed output,
  agent wiring, and agents.json LastAgent). Added
  TestNewAgentDoesNotBootstrapWhenAgentStateExists (agents.json present, no
  definitions, verifies error and no default.md creation).

- task.md: updated status from approved to completed with accepted outcome,
  changed paths, and verification results.

- .gitignore: added .agents/ directory to prevent committing investigation
  artifacts.

## Decisions And Rationale

1. Bootstrap placement: inserted after the first agents.Load and executor
   collection, before the existing hard error, so it runs exactly once on fresh
   app homes. This was chosen because it reuses all existing validation and
   loading infrastructure without adding a separate initialization path.

2. agents.json guard: the bootstrap only fires when agents.json does not exist.
   This was the approved approach because agents.json presence means the user
   has previously interacted with the agent system and intentionally removed
   definitions; automatic recreation would override that intent.

3. Model source: uses cfg.Roles.Default, the validated configured default model,
   rather than hardcoding a model. This was required by the approved task to
   respect user configuration without introducing a fallback.

4. Tool filtering: agent_done and run_agent are excluded from the generated
   allowlist because they are control tools injected by the runtime, not
   user-facing tools. The approved specification mandated this filtering.

5. File protection: EnsureDefaultInteractive checks target existence via
   os.Stat before any write. If default.md already exists (user-edited or
   previously created), it returns (false, nil) without reading or modifying
   the file. This was selected as the simplest correct provisioning behavior
   that preserves user edits.

6. No schema change: the implementation adds no new fields to agents.json,
   no new definition types, and no new fallback behavior. The approved task
   explicitly prohibited schema changes and fallbacks.

7. Investigation artifacts: .agents/ contains explorer reports and must not
   be committed. Added to .gitignore per repository rules for
   repository-irrelevant content.
