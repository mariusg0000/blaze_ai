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
HOW:     [approach]
```

Inline comments must explain why a decision exists. Do not narrate code.

Update documentation in the same patch as code changes. New code must not remain undocumented.

Exceptions: trivial DTO fields, direct constant aliases, and one-line pass-through wrappers already covered by surrounding documentation.

When unsure, document it.

## Commit Workflow

**MANDATORY: Run this workflow only when the user explicitly requests a commit in the current turn.**

Finishing an implementation, fixing tests, or completing a task does not authorize `git add`, creation of a decision file, or `git commit`.

When a commit is explicitly requested, follow these steps in order:

1. Run:

```bash
git status --short
```

2. Identify changes explained by the conversation.

3. For changed files not explained by the conversation, run `git diff` only on those files and infer their purpose.

4. Add secrets, generated files, executables, dependencies, virtual environments, caches, build outputs, logs, local databases, and other repository-irrelevant files or directories to `.gitignore`.

5. Review every remaining repository change. Include all relevant changes in the commit, including pre-existing or unrelated changes. Do not leave relevant files uncommitted.

6. Create `decisions/` if missing, then create the decision summary:

```text
decisions/YYYY-MM-DD-HHMM-short-topic-keyword1-keyword2-keyword3-keyword4-keyword5.md
```

Use exactly five concise, lowercase keywords describing the main decisions.

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

Include major pre-existing or unrelated changes committed with the work. Mention minor included changes briefly.

Do not invent rationale. Use only the conversation, implementation, changed files, diffs, and validation results.

7. Stage all relevant changes, including the decision summary.

8. Commit using an imperative subject under 50 characters.

The commit body must state:

* what changed
* why it changed
* how it was implemented
* validation performed
* decision summary path
* all included pre-existing or unrelated changes

9. Run:

```bash
git status --short
```

Verify that no relevant repository changes remain uncommitted.


## Push

Push only when explicitly requested by the user.

