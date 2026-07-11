# AGENTS.md

## Language

Use English for code comments, documentation, commit messages, filenames, identifiers, and decision summaries.

## Implementation Mode

Do not implement without an explicit user command such as `proceed`, `go`, `begin`, `implement this`, or `do it`.

When the user reports a bug, unexpected behavior, debug output, or requests a change:

1. Explain what is happening.
2. Explain why it is happening.
3. State the proposed remediation.
4. Wait for an explicit implementation command.

This workflow is mandatory.

## Coding Style

Follow the KISS (Keep It Simple, Stupid) principle.

## Testing

Write and run tests. Follow the user-defined test scope.

## Documentation

Use English and the language-idiomatic documentation format.

Every source file must start with a short header containing:

* filename
* purpose
* layer or responsibility
* direct dependencies or integration boundaries when relevant

Document every module, type, trait/interface, implementation block, function, method, constructor/factory, public constant/static/config item, and non-trivial private helper.

Document non-obvious logical blocks, blocks longer than approximately 10 lines, and branching, state-machine, protocol, persistence, rendering, validation, transformation, state mutation, or business-rule logic.

Use:

```text
WHAT:    [functionality]
WHY:     [reason]
HOW:     [approach]
PARAMS:  [parameters or none]
RETURNS: [return value or none]
```

Inline comments must explain why a decision exists. Do not narrate code.

Update documentation in the same patch as code changes. New code must not remain undocumented.

Exceptions: trivial DTO fields, direct constant aliases, and one-line pass-through wrappers already covered by surrounding documentation.

When unsure, document it.

## Commit Workflow

Run git status --short.

Infer known changes from the conversation. Do not perform unnecessary reads.

For modified files not explained by the conversation, run git diff only on those files and infer their purpose.

Add secrets, generated files, generated executables, dependencies, virtual environments, caches, build outputs, logs, local databases, and other repository-irrelevant files or directories to .gitignore.

Stage and commit everything else.

Create `decisions/` when required and missing.

## Decision Summary

Use:

```text
decisions/YYYY-MM-DD-HHMM-short-topic.md
```

Include major pre-existing changes committed with the work. Mention minor included changes briefly.

Use:

```md
# Session Decision Summary: <topic>

Date: YYYY-MM-DD HH:MM

## Context

<work trigger and constraints>

## Changes Made

<changed files and behavior>

## Decisions And Rationale

<supported decisions, rationale, and trade-offs>
```

Do not invent rationale. Use only the conversation, implementation, changed files, and validation results.

## Commit Message

Use an imperative subject under 50 characters.

The body must state:

* what changed
* why it changed
* how it was implemented
* validation performed
* decision summary path, when created
* included pre-existing or unrelated files

## Push

Push only when explicitly requested by the user.

