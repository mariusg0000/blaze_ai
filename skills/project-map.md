[DESCRIPTION]
Load when the user wants a project overview, repository map, codebase tree, architecture map, or a generated `project-map.md`. Use for scanning the current working directory, filtering noise, and writing a concise Markdown structure map. The generated file is auto-injected into every prompt as project-local context, so keep it tight.

[BEHAVIOR]
# Project Map

## Purpose

Generate `project-map.md` in the current working directory. Make it a fast orientation map. Describe important folders and files without dumping every path.

`project-map.md` is auto-injected into every LLM prompt as project-local context under `[PROJECT MAP]`. Keep the map concise so it does not waste tokens on every call. Target 20-40 lines, never exceed 60 lines.

## Workflow

1. Inspect the top-level structure first.
2. Identify important areas: entrypoints, `src`, `internal`, `api`, `config`, `docs`, build files, tests.
3. Identify noisy or low-value areas: `node_modules`, generated output, assets, caches, vendor folders, repetitive fragments.
4. Read a small sample of high-value files only when needed to explain their role.
5. Write or regenerate `project-map.md` as one coherent Markdown document.

## Output Rules

- Start with `# Project Map`.
- Use a tree-like Markdown bullet structure.
- Give each important folder or file one short sentence explaining its role.
- Summarize noisy subtrees at folder level instead of listing every file.
- Go deeper only where structure alone is not enough to understand purpose.
- Target 20-40 lines. Never exceed 60 lines. If the project is too large to fit, summarize at higher level and offer to drill into subtrees on request.
- Do not include secrets, `.env` values, copied file contents, or large code excerpts.

## Scanning Guidance

- Prefer fast file discovery helpers such as `fd` or `rg --files` when available.
- If helpers are missing, use safe OS-native shell listing commands.
- Avoid broad deep reads before the structure is clear.
- Avoid reading large generated files unless the user explicitly asks.

## Usage

When `project-map.md` already exists, read it first before broad file exploration. Use it to locate files, understand module boundaries, and find relevant code faster.

## Staleness

If `project-map.md` already exists and is older than ~7 days, or if the user mentions new folders, moved files, or structure changes, suggest regeneration before relying on the old map.

## Validation

- Major entrypoints and important directories should be covered.
- Low-value or repetitive areas should be summarized without clutter.
- The result should be readable as a quick orientation document for future work.

## Stop Conditions

If the repository is too large, or key areas stay unclear after a shallow scan, stop and ask whether to keep the map high-level or drill into specific subtrees.
