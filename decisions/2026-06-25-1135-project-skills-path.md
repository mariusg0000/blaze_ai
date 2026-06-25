# Session Decision Summary: project-skills-path

Date: 2026-06-25 11:35
Base commit: dd8cee3

## Context
User corrected the project skills location: project-scoped skills must live under `app_home/projects/<project>/skills/`, not `workdir/.blazeai/skills/`. This aligns project skills with project sessions, both under the same `app_home/projects/<project>/` directory.

## Changes Made
- `internal/skills/skills.go`: `DiscoverProject` now resolves the project dir via `platform.ProjectDir(workDir)` and joins `/skills` instead of using `workdir/.blazeai/skills`. Doc strings updated.
- `internal/platform/platform.go`: `EnsureProjectDir` now also creates a `skills/` subfolder alongside the existing `sessions/` subfolder.
- `internal/skills/doc.go`: Updated path reference.
- `prompts/sysprompt.md`: Project skill path changed to `{APP_HOME}/projects/<project>/skills/<name>/skill.md`.
- `skills/skill-manager.md`: DATA section updated — `skill.project_layout` changed to `app_home/projects/<project>/skills/<name>/skill.md`, `skill.scopes` updated.
- `skills/customize_me.md`: Added "Skill Locations" section with full rules for global vs project scope decision, path layouts, creation guidelines, and generalization rules for moving skills from project to global.
- `internal/platform/apphome_readmes/skills/README.md`: Path reference updated.

## Decisions And Rationale
- Project skills live under `app_home/projects/` alongside sessions, not in the working directory. This keeps the workdir clean (no `.blazeai/` folder) and groups all project artifacts under app home.
- `EnsureProjectDir` creates both `sessions/` and `skills/` subfolders, ensuring the project skills directory exists when the LLM needs to write a new project skill.

## Implementation Approach
- `DiscoverProject` computes the project dir via `platform.ProjectDir()` which sanitizes the workdir path, then appends `/skills`.
- `DiscoverAll` unchanged at call sites — it delegates to `DiscoverProject` which handles the path internally.

## Files Included
- `internal/skills/skills.go`: Path change + doc updates
- `internal/platform/platform.go`: `skills/` subfolder creation
- `internal/skills/doc.go`: Path reference
- `prompts/sysprompt.md`: Updated project path
- `skills/skill-manager.md`: DATA layout rules
- `skills/customize_me.md`: New "Skill Locations" section with scope decision rules
- `internal/platform/apphome_readmes/skills/README.md`: Path reference
- `decisions/2026-06-25-1135-project-skills-path.md`: This summary

## Commit Linkage
This summary is committed together with the implementation changes.
