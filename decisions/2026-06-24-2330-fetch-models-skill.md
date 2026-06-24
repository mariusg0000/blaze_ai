# Session Decision Summary: fetch_models skill instruction

Date: 2026-06-24 23:30
Base commit: 8aa6cfb

## Context
The customize_me skill needed a way to fetch available models from providers when the user wants to create or assign a model. Previously the skill lacked any model discovery mechanism — the LLM had to guess or read config.json manually. A permanent tool was rejected to avoid wasting tokens on a rare operation.

## Changes Made
Added `## Fetching Models from Providers` section to `skills/customize_me.md` with algorithm pseudocode and creation guidelines. No hardcoded implementations — the LLM generates the script on-the-fly using shell tools, caching it at `{APP_HOME}/scripts/fetch_models` for reuse.

## Decisions And Rationale
- **Shell-first, Python last resort**: blazeai prioritizes shell tools. The skill provides the algorithm but lets the LLM choose the implementation (curl + grep/jq, PowerShell Invoke-RestMethod, or Python as fallback).
- **Cached script**: first use creates the script, subsequent uses run it directly — avoids regenerating code.
- **API keys never in prompt**: the generated script reads config.json locally. Keys are not passed as arguments or embedded in the skill.
- **No permanent tool**: rejected the `list_models` tool in favor of a script-based approach. Model listing is a rare operation used only when configuring modes/providers, not during normal agent interaction.

## Implementation Approach
Algorithm in 6 lines of pseudocode covering: (1) reuse cached script, (2) create if missing, (3) read config for endpoint+key, (4) call /models endpoint, (5) parse+filter JSON, (6) output provider/model per line. Usage: fetch per provider with optional filter, present numbered list to user.

## Files Included
- skills/customize_me.md: added Fetching Models section with algorithm and creation guidelines

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
