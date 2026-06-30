# Session Decision Summary: transport prompts

Date: 2026-06-30 12:01
Base commit: 4b672a5

## Context
The prompt system had a universal `[TRANSPORT]` placeholder and a universal output-style block, but only Telegram populated dynamic transport context. Console had no explicit transport profile, and planned web support would need the same separation. Telegram output formatting was also conflicting with console-oriented Markdown guidance.

## Changes Made
Added required transport-specific prompt fragments for console, Telegram, and web. Updated prompt assembly to require a transport name and load `transport.<name>.md` on every build. Wired console and Telegram startup through the new transport selector. Updated prompt, runtime, console, and Telegram tests. Refreshed prompt and transport specs to document the new architecture and updated build/deploy specs to reflect the new embedded assets and runtime startup contract.

## Decisions And Rationale
Kept one universal `sysprompt.md` as the source of truth for core behavior and moved only transport-specific behavior into dedicated fragments. This avoids duplicating safety, tool, and project rules across three full system prompts while still giving each transport strict formatting guidance. The transport prompt is required and missing files fail the build path explicitly, matching the project's no-fallback rule.

## Implementation Approach
Introduced `Builder.TransportName` and two explicit errors for missing transport configuration or transport prompt files. `BuildRuntimePart()` now loads `transport.<name>.md` after the OS prompt and injects it through `{TRANSPORT_PROMPT}`. `runtime.NewAgent()` now accepts `transportName` and passes it into the builder. Console uses `console`; Telegram uses `telegram`; web is prepared through `transport.web.md` for future transport wiring. Tests were updated to include transport prompt fixtures and to assert required transport behavior.

## Alternatives Considered
Using three full system prompts was rejected because it would duplicate common rules and increase drift risk across transports. Keeping only a dynamic `TransportContext` string was rejected because formatting and interaction rules are static enough to belong in dedicated prompt files, not ad-hoc runtime strings.

## Files Included
- `prompts/sysprompt.md`: injects transport prompt content before dynamic transport context.
- `prompts/transport.console.md`: console-specific formatting and interaction rules.
- `prompts/transport.telegram.md`: Telegram-specific formatting and interaction rules.
- `prompts/transport.web.md`: web-specific formatting rules for future transport work.
- `prompts/readme.md`: documents new prompt variables and transport prompt files.
- `internal/prompt/prompt.go`: requires and loads transport-specific prompt fragments.
- `internal/prompt/doc.go`: updates prompt package overview order.
- `internal/runtime/runtime.go`: passes transport selection into prompt builder construction.
- `main.go`: starts the console agent with `transportName="console"`.
- `internal/telegram/telegram.go`: starts the Telegram agent with `transportName="telegram"`.
- `internal/prompt/prompt_test.go`: covers transport prompt loading and missing-transport errors.
- `internal/runtime/runtime_test.go`: updates agent fixtures and constructor calls.
- `internal/console/console_test.go`: updates constructor calls and prompt fixtures.
- `internal/telegram/commands_test.go`: updates constructor calls and prompt fixtures.
- `specs/02-architecture.md`: updates runtime startup contract.
- `specs/04-prompts.md`: documents transport prompt architecture.
- `specs/13-console-ui.md`: documents console transport prompt linkage.
- `specs/14-telegram-bridge.md`: documents Telegram transport prompt linkage.
- `specs/19-build-deploy.md`: updates embedded prompt assets and startup signature.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
