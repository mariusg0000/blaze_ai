# Session Decision Summary: Specs Regeneration

Date: 2026-07-17 16:00

## Context

User requested full specs regeneration using the `project-specs` skill. Existing specs were written with an older skill version and contained inaccuracies against the current codebase (wrong Handler interface, missing Mode fields, missing packages, stale tool counts, stale references to removed subsystems).

## Changes Made

- **specs.md** (root): Updated Handler contract description (OnReasoning→OnSystem/OnMaintenance*, added StreamPhaseHandler/AgentActivityHandler), added internal/usage/ to project map, updated tools line to include read_file/write_file
- **specs/01-product-scope.md**: Full rewrite — updated architecture overview (12 base tools, removed stale internal/web/ references, correct Handler interface, added agents/ to app home, added read_file/write_file to tools list)
- **specs/02-architecture.md**: Full rewrite — added runtime/agent_orchestration.go, runtime/mode_capabilities.go, internal/usage/, updated Handler interface, added StreamPhaseHandler/AgentActivityHandler, updated Agent struct (Definitions, BaseTools, Completion), corrected module dependency graph
- **specs/03-config-schema.md**: Added DeniedTools and Agents fields to Mode struct, updated modes.json example, added mode validation rules for denied_tools/agents
- **specs/04-prompts.md**: Added AGENTS_AVAILABLE, AGENT_INSTRUCTIONS, AGENT_TASK variables, updated build sequence to include agent section and child agent differences
- **specs/05-tools.md**: Full rewrite — correct tool count (12 base), added read_file, write_file, run_agent, agent_done tools, added emoji table including new tools
- **specs/10-sessions.md**: Updated Message struct (ReasoningPresent, ReasoningEncrypted), added AppendReadFileResult, added token-usage.json section (UsageReport, UsageSnapshot, RecordUsage, CacheKeyForSession)
- **specs/12-handler-contract.md**: Full rewrite — correct Handler interface (8 methods, no OnReasoning), added StreamPhaseHandler, AgentActivityHandler, AgentActivity type, updated all method descriptions
- **specs/13-console-ui.md**: Updated Console struct (new fields: spinner, status bar, turn state), corrected color codes
- **specs/14-telegram-bridge.md**: Removed OnReasoning, added OnSystem/OnMaintenance*, fixed OnUsage signature
- **specs/15-runtime-core.md**: Full rewrite — correct Agent struct (Definitions, BaseTools, Completion), correct Handler interface, updated RunTurn flow, updated NewAgent wiring, added child agent construction
- **specs/20-agent-orchestration.md**: NEW — agent orchestration spec (definition format, tool signatures, execution flow, persistent sessions, dual timeout, activity events)
- **specs/21-mode-capabilities.md**: NEW — mode capabilities spec (DeniedTools, Agents, refreshModeCapabilities, validation, examples)
- **specs/22-usage-normalization.md**: NEW — usage normalization spec (Usage struct, rawUsage variants, Extract function, provider format support)

## Decisions And Rationale

- **Full rewrite vs incremental update**: Chose full rewrites for the most inaccurate specs (01, 02, 05, 12, 15) rather than patching, because the changes were pervasive enough that targeted edits would miss important context
- **Three new spec files**: Agent orchestration, mode capabilities, and usage normalization were important enough subsystems to deserve dedicated specs rather than being folded into existing files
- **No code changes**: Specs-only work; no runtime code was modified
