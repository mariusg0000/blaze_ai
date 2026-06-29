# Session Decision Summary: Sudo Prompt Filter

Date: 2026-06-29 06:29
Base commit: 4634ea5

## Context

LLM confused `[sudo] password for <user>:` in stderr output with a command
failure, causing redundant tool calls and unnecessary user approval requests.
The prompt was printed by sudo on the piped stderr before executing the
command successfully using cached credentials.

## Changes Made

- `internal/tools/shell.go`: Wrapped stderr output with
  `stripSudoPasswordPrompt()` filter that removes lines matching
  `[sudo] password for ` prefix. If all stderr was just sudo prompts,
  the stderr section is omitted entirely.

## Implementation Approach

Added a filter function in shell.go that splits stderr by newlines,
drops lines with the `[sudo] password for ` prefix, and rejoins.
Minimal change with no new dependencies or config.

## Files Included
- `internal/tools/shell.go`: stderr filtering for sudo prompts
- `decisions/2026-06-29-0629-sudo-prompt-filter.md`

## Commit Linkage
This summary is committed together with the implementation changes.
