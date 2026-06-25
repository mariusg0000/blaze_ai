# Session Decision Summary: simplified-scope-injectable-vars

Date: 2026-06-25 11:50
Base commit: 0cc4b9d

## Context
User identified confusion in the LLM: it was creating a `skills/global/` subdirectory because it conflated the `global/` canonical ID prefix with a filesystem path. User proposed simplifying the model: bare names for global skills (default scope), `project/name` only for project skills. Also needed a complete, well-explained list of injectable variables in skill-manager.

## Changes Made
- `internal/skills/skills.go`: `Resolve()` simplified — bare name → `global/name` (global is the default), `project/name` → exact project lookup. Removed ambiguity checking (global wins by default; project requires explicit prefix). Removed `global/` as a valid user-facing input prefix.
- `internal/prompt/prompt.go`: Available skills list strips `global/` prefix from display names (shows `skill-manager.md` not `global/skill-manager.md`). Active skill headers also stripped. Added `{GLOBAL_SKILLS_DIR}` and `{PROJECT_SKILLS_DIR}` to variable injection system.
- `prompts/sysprompt.md`: Simplified skill section to single sentence — no paths, no verbose scoping rules. "Load by name or project/name."
- `skills/skill-manager.md`: Added "Where to Create Skills" section in BEHAVIOR with global vs project decision rules, generalization guidelines, shell commands with resolved paths, and builtin restore procedure. DATA section now has one `skill.variable.*` entry per injectable variable with description and example. All teaching references to variables use escaped braces (`\{VAR\}`) so the LLM learns the variable names. Shell command examples use unescaped braces (`{VAR}`) so the LLM gets real paths.
- `skills/customize_me.md`: Updated all path/variable references to escaped form. Added `{GLOBAL_SKILLS_DIR}` and `{PROJECT_SKILLS_DIR}` references.

## Decisions And Rationale
- Bare name → global by default because global is the common case and having a `global/` prefix adds cognitive overhead for both users and the LLM. Project scope is explicit only when needed.
- Teaching text uses `\{VAR\}` (escaped, LLM sees literal variable name) while shell commands use `{VAR}` (resolved, LLM gets real path). This separation makes the LLM both informed about variables and able to use real paths.

## Implementation Approach
- `Resolve()` uses `strings.HasPrefix("project/")` for scoped detection, falling through to default global lookup.
- `injectTemplateVariables` supports both `{GLOBAL_SKILLS_DIR}` (home/skills) and `{PROJECT_SKILLS_DIR}` (ProjectDir/skills).
- Display names stripped with `strings.TrimPrefix(id, "global/")` in the prompt builder.

## Files Included
- `internal/skills/skills.go`: Simplified Resolve
- `internal/prompt/prompt.go`: Display + variable injection
- `prompts/sysprompt.md`: Minimal scope description
- `skills/skill-manager.md`: Creation rules, variable catalog
- `skills/customize_me.md`: Escaped variables + updated paths
- `decisions/2026-06-25-1150-simplified-scope-injectable-vars.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
