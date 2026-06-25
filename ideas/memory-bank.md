# Memory-Bank

## Goal

Add on-demand contextual knowledge files that are loaded into the prompt only when relevant.

## Format

Each memory bank uses two sections, similar to skills:

```md
[DESCRIPTION]
Short summary of what this memory bank contains.

[DETAILS]
Full knowledge block injected into the prompt when active.
```

## Examples

- `my-network`: IPs, servers, roles, and topology for the local network.
- `project-deploy`: deployment targets, credential references, and CI/CD details.
- `client-acme`: client-specific infrastructure, contacts, and conventions.

## Behavior

- Store files under `app_home/memory-banks/<name>/memory-bank.md`.
- Discover them alongside skills from builtin and custom sources.
- Show `[DESCRIPTION]` blocks as available memory banks in the prompt.
- Inject `[DETAILS]` only when explicitly loaded or auto-loaded by a router.
- Load and unload them through `load_memory_bank` and `unload_memory_bank`.
- Keep manual loading available to the main LLM.
