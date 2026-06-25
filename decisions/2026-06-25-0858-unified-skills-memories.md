# Session Decision Summary: unified-skills-memories

Date: 2026-06-25 08:58
Base commit: 845bc4e

## Context
The prior session established the plan to merge skills and memories into a single unified `Skill` entity with `[BEHAVIOR]` and `[DATA]` sections. This session executed that plan in full, completing the remaining implementation and validation.

## Changes Made
- **Skill format changed**: `[DETAILS]` → split into `[BEHAVIOR]` (procedural) and `[DATA]` (persistent facts). At least one required alongside mandatory `[DESCRIPTION]`.
- **Three skill scopes**: `builtin` (embedded), `global` (`app_home/skills/<name>/skill.md`), `project` (`<workdir>/.blazeai/skills/<name>/skill.md`). Project skills use `project/` prefix.
- **Scope-aware resolution**: `Resolve()` handles bare names (unique matches) and scoped names (`project/foo`). Ambiguous bare names (global + project) error with candidates.
- **Memories package removed**: `internal/memories/` deleted; `internal/memory/` (single `memory.md`) deleted as dead code.
- **Memory tools removed**: `load_memory`/`unload_memory` tools and test files deleted.
- **`memory-manager` builtin removed**: rules about memory-bank authoring folded into `skill-manager`.
- **Runtime updated**: `Agent.Memories` field removed; `ResetConversation` clears only skills; skill tools use injectable resolver.
- **Platform updated**: `memories/` and `memory/` subfolders no longer created; root and skills READMEs updated.
- **Prompt builder**: Single `## Skills` section with `#### Behavior` and `#### Data` subsections; `{MEMORIES_AVAILABLE}`/`{MEMORIES_ACTIVE}` placeholders removed.
- **sysprompt.md**: Removed memories section labels and rules; unified skill loading/retention guidance.
- **Specs updated**: `specs.md`, `01-product-scope.md`, `02-core-runtime.md`, `04-platform-ops.md` reflect unified model.

## Decisions And Rationale
- `[BEHAVIOR]` and `[DATA]` were chosen over a single `[DETAILS]` section to cleanly separate procedural guidance from persistent facts, enabling the LLM to distinguish "how to work" from "what is true" without ambiguous parsing.
- The resolver in skill tools is `nil`-safe: when no resolver is provided (tests), the tool loads/unloads the name directly without validation. This keeps tests simple while the runtime always wires real resolution.
- Global skills override builtin by bare name (existing rule); project skills use `project/` prefix to avoid accidental collision with global/builtin names.
- `internal/memory/memory.go` was already dead code (not wired into prompt) and was removed alongside the memories package.

## Implementation Approach
- `skills.go`: New `Parse()` extracts `[BEHAVIOR]` and `[DATA]` via `extractOptionalSection`; `DiscoverAll()` combines `DiscoverFromFS` (builtin), `DiscoverFromSubdirs` (global), and `DiscoverProject` (project). `Resolve()` iterates scopes and returns error on ambiguity.
- `skill_tools.go`: `LoadSkillTool`/`UnloadSkillTool` accept `ResolveFunc`; nil-safe so tests don't need a resolver.
- `prompt.go`: `buildSkillsSection()` calls `skills.DiscoverAll()` with the work dir; renders Behavior/Data sections for active skills.
- `runtime.go`: `NewAgent()` creates a resolver closure calling `skills.DiscoverAll()` + `skills.Resolve()`, passing it to skill tools.

## Files Included
- `internal/skills/skills.go`: new Skill struct, parser, discovery, resolution
- `internal/skills/skills_test.go`: updated parser/discovery tests for Behavior/Data
- `internal/skills/doc.go`: updated package doc
- `internal/memories/`: deleted (memories.go, memories_test.go)
- `internal/memory/`: deleted (memory.go, memory_test.go)
- `internal/tools/skill_tools.go`: resolver-based load/unload
- `internal/tools/skill_tools_test.go`: nil resolver in tests
- `internal/tools/memory_tools.go`, `internal/tools/memory_tools_test.go`: deleted
- `internal/tools/tools.go`: updated doc comment
- `internal/tools/tools_test.go`: nil resolver in format tests
- `internal/prompt/prompt.go`: single skills section, Behavior/Data rendering
- `internal/prompt/prompt_test.go`: updated for new API, removed memories tests
- `internal/prompt/doc.go`: updated package doc
- `internal/runtime/runtime.go`: removed Memories, wired resolver
- `internal/platform/platform.go`: removed memories/memory from subfolders
- `internal/platform/platform_test.go`: removed memories README check
- `internal/platform/apphome_readmes.go`: removed memories README entry
- `internal/platform/apphome_readmes/README.md`: updated folder listing
- `internal/platform/apphome_readmes/memories/README.md`: deleted
- `internal/platform/apphome_readmes/skills/README.md`: updated for Behavior/Data format
- `internal/console/console_test.go`: removed Memories references
- `prompts/sysprompt.md`: unified skills, removed memories placeholders
- `prompts/readme.md`: removed `{MEMORIES_*}` entries
- `skills/skill-manager.md`: folded memory-manager rules, Behavior/Data guidance
- `skills/memory-manager.md`: deleted
- `skills/customize_me.md`: `[DETAILS]` → `[BEHAVIOR]`
- `skills/setup_helpers.md`: `[DETAILS]` → `[BEHAVIOR]`
- `specs.md`, `specs/01-product-scope.md`, `specs/02-core-runtime.md`, `specs/04-platform-ops.md`: updated for unified model
- `AGENTS.md`: pre-existing edit removing "tests" from scope-expansion rule
- `home_folder_backup/`: pre-existing project files, included to keep repo clean
- `ideas/project-scoped-skills-memories.md`: pre-existing ideas file, included to keep repo clean

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
