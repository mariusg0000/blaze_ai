[DESCRIPTION]
Load when the user wants a project overview, repository map, codebase tree, architecture map, or a generated `project-map.md`. Use for scanning the current working directory, filtering noise, and writing a concise Markdown structure map.

[BEHAVIOR]
# Project Map

## Purpose

Generate `project-map.md` in the current working directory. Make it a fast orientation map. Describe important folders and files without dumping every path.

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
- Do not include secrets, `.env` values, copied file contents, or large code excerpts.

## Scanning Guidance

- Prefer fast file discovery helpers such as `fd` or `rg --files` when available.
- If helpers are missing, use safe OS-native shell listing commands.
- Avoid broad deep reads before the structure is clear.
- Avoid reading large generated files unless the user explicitly asks.

## Validation

- Major entrypoints and important directories should be covered.
- Low-value or repetitive areas should be summarized without clutter.
- The result should be readable as a quick orientation document for future work.

## Stop Conditions

If the repository is too large, or key areas stay unclear after a shallow scan, stop and ask whether to keep the map high-level or drill into specific subtrees.
