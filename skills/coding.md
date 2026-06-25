[DESCRIPTION]
Load when the user starts a coding session (write, edit, generate, refactor, or debug code) or when the LLM needs to produce scripts, source files, or configuration files. Also load when the user asks about architecture, structure, or design changes. Do NOT load for inline shell commands, one-liner fixes, or simple file reads.

[BEHAVIOR]
# Coding

## Priorities
1. Correctness — production-grade code that satisfies only the approved requirement.
2. Transparency — clearly explain decisions, scope, and validation.
3. Simplicity — smallest direct implementation that works now.

Be reactive: propose, wait for approval when required, then execute. Never invent requirements, expand scope, refactor unrelated code, or add abstractions without a concrete present need.

## Bug Fix Protocol (Mandatory)
When the user reports a bug, unexpected behavior, or asks for a fix, you MUST follow this protocol before any code change:
1. Explain what happened and why it happened.
2. Explain how you intend to fix it.
3. Ask for explicit approval to proceed.
4. Only after approval, implement the fix.

Never jump directly to fixing. Understanding precedes action. This applies to all bug reports, regardless of perceived triviality.

## KISS Engineering
Prefer the simplest direct implementation. Answer: what is the minimum code path from data to result?

- Optimize for boring, explicit code; plain control flow over clever one-liners.
- Prefer concrete data structures with direct fields over indirection.
- Preserve the existing model; do not replace it to match another paradigm.
- Use collections, caches, lookup tables only when clearly needed — and explain why first.
- No speculative abstractions, premature generalization, future-proof hooks, managers, registries, services, adapters, interfaces, or design patterns without a concrete present requirement.
- No extra layers for a single call site.
- New architectural concept required → stop and ask first.
- If complexity cannot be justified by a present requirement, drop it.

## Code Structure
- SRP applied proportionally. A 30-line script may stay one file. Keep modules small and focused when the project already does.
- Extract magic numbers or toggles to config only when they are meaningful project settings.
- Prefer descriptive names. Avoid nested ternaries and cryptic one-liners.
- Clarity over cleverness.

## Typing
Explicit types on all public parameters and return values:
- Go: idiomatic types on public API.
- Python: typing, Protocol, TypedDict, Literal.
- TypeScript: strict mode.
- Rust: public API types.
- Java/Kotlin: public API types.

## Quality Gates
When available in the project, validation MUST pass: formatter, linter, strict type checker. If no validation command is defined, infer the safest conventional one and report the assumption. If validation fails and cannot be fixed within approved scope, do not mark the task completed — report and ask.

Common patterns:
- Python: ruff format && ruff check && mypy --strict
- TypeScript: tsc --strict
- Rust: cargo fmt && cargo clippy
- Go: go fmt && go vet

## Documentation
All docs and comments in English, using the language-idiomatic format.

### File Header
Every source file starts with a short header: filename, 1-3 sentence purpose, layer/responsibility, direct dependencies or integration boundaries when relevant.

### What To Document
Document every module, struct, enum, trait/interface, impl block, function, method, constructor/factory, public constant/static/config item, and any private helper with non-trivial behavior (branching, I/O, validation, transformation, state mutation, protocol, rendering, persistence).

Also document logical blocks: branching, state machines, protocol handlers, persistence, rendering, validation, business-rule sections, any block >~10 lines, or code whose meaning is not obvious from names.

### Doc Template
```
WHAT:    [1-2 sentences of functionality]
WHY:     [architectural/business reason]
HOW:     [key approach/algorithm/design choice, 1-2 sentences]
PARAMS:  [name: type — meaning]   (or "none")
RETURNS: [type — meaning]         (or "none")
```

### Inline Comments
Only at decision points, explaining why a choice exists. Never narrate what the next line does.

### Documentation Rules
Update all relevant headers, docs, and comments in the same patch as code changes. Never leave new code undocumented. Exception: trivial DTO fields, direct constant aliases, one-line pass-through wrappers (only if surrounding docs already explain them). When unsure, document it.

## File Modification
Default to incremental scoped patches (search/replace or unified diff). Full rewrite only when patching is impractical — justify it first.

- Update relevant headers, docs, changelog in the same patch when applicable.
- Do not mix unrelated changes, silently reformat unrelated files, or touch generated files unless the task requires it.

## Prohibitions
- Never expand scope beyond the approved task.
- No refactors, renames, cleanup, formatting sweeps, dependency changes, or scope expansion beyond the approved task.
- Never mix unrelated changes in the same patch.

[DATA]
coding.language=English only for all comments, docs, commit messages, filenames, identifiers
coding.doc_template=WHAT:/WHY:/HOW:/PARAMS:/RETURNS:
coding.file_header=filename, 1-3 sentence purpose, layer/responsibility, dependencies
coding.quality_python=ruff format && ruff check && mypy --strict
coding.quality_typescript=tsc --strict
coding.quality_rust=cargo fmt && cargo clippy
coding.quality_go=go fmt && go vet
coding.scope=never expand beyond approved task
