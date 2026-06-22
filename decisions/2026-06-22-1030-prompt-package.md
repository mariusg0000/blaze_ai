# Session Decision Summary: prompt-package

Date: 2026-06-22 10:30
Base commit: 16f0f86 (Implement memory, skills, and session packages)

## Context
- Implemented the prompt assembly package, which was the next dependency layer after memory, skills, and session.
- Variable injection ({APP_HOME}) was the remaining feature needed from the earlier skeleton — prompt and skill files already contained the placeholder.
- PromptsDir was added as an explicit Builder field for testability (vs the earlier fragile relative-path approach using ".." from skills dir).

## Changes Made
- **internal/prompt**: new package (prompt.go + prompt_test.go)
  - Builder struct with PromptsDir, BuiltinSkillsDir, WorkDir, OS fields
  - injectVariables(text) — regex replacement of {VARIABLE_NAME} placeholders; {APP_HOME} is resolved via platform.AppHome(); unknown variables left as-is per spec §02
  - BuildRuntimePart(active) — assembles in spec order: universal sysprompt → OS sysprompt → AGENTS.md → memory.md → skills section
  - buildSkillsSection(active) — two-part section: Available Skills (all [DESCRIPTION] + file names, sorted) then Active Skills ([DETAILS] of loaded skills)
  - Build(sess, active) — returns full message array (system + session messages)
  - Required source handling: universal and OS prompts error if missing
  - Optional source handling: AGENTS.md, memory, skills omitted silently when absent
  - 13 tests covering injection (APP_HOME, unknown, multiple, none), full assembly, required/optional sources, active skills, empty session, source order validation

## Decisions And Rationale
- PromptsDir is an explicit field rather than derived from BuiltinSkillsDir + ".." because the relative path was fragile and untestable without real project structure.
- Skills discovery uses the existing skills.Discover() function with the projectbuiltin skills directory; testtime directories simulate both builtin and custom skills.
- injectVariables accepts an error return (from platform.AppHome()) even though the variable replacement itself cannot fail, keeping the abstraction consistent.
- Regex pattern restricts variables to uppercase letters, digits, and underscores ([A-Z_][A-Z0-9_]*) to match {VARIABLE_NAME} format without false matches.

## Implementation Approach
- Wrote Builder as a value struct with all config injected at creation.
- Test helper setupTestDirs creates realistic temp directories with prompt files, skill files, and AGENTS.md.
- All tests use temp directories; no dependency on real project files or app home for prompt/skill sources.
- Full validation: go build ./..., go vet ./..., go test ./... (79 tests, all pass).

## Files Included
- internal/prompt/prompt.go: Builder, injectVariables, BuildRuntimePart, buildSkillsSection, Build
- internal/prompt/prompt_test.go: 13 tests
- decisions/2026-06-22-1030-prompt-package.md: this summary
