# Runnable Skills

Allow some skills to be executed directly like tools, but through prompt-level behavior instead of OpenAI function calling.

## Core Idea

- A skill becomes runnable if it contains an optional `[SYNTAX]` section.
- `[SYNTAX]` explains exactly how the skill is called and what arguments it expects.
- The LLM still uses the `load_skill` tool, but with an extra `arguments` field in addition to `name`.
- The runtime treats this as "load and run this skill" rather than just adding it to the active skill list.

Example shape:

```json
{
  "name": "my-runnable-skill",
  "arguments": "arg1 arg2 --flag"
}
```

## Skill Format

Potential sections:

- `[DESCRIPTION]` - normal skill description
- `[BEHAVIOR]` - normal guidance
- `[SYNTAX]` - marks the skill as runnable and defines invocation syntax
- `[CODE]` - executable script body

`[CODE]` could contain shell, Python, or another script type. The exact execution model is still open.

## Runtime Direction

- `load_skill` needs an optional `arguments` field.
- If the target skill has no `[SYNTAX]`, `load_skill` behaves exactly as today.
- If the target skill has `[SYNTAX]`, runtime should recognize it as runnable.
- The LLM learns when and how to call it from the `[SYNTAX]` section, not from tool schema alone.

## Open Questions

- How should `[CODE]` declare its runtime type: inferred, header field, or separate metadata?
- Should runnable skills stay loaded after execution, or unload automatically?
- How should stdout/stderr be captured and returned?
- How should errors be surfaced back to the model and the user?
- How should argument parsing work: raw string, array, JSON object, or syntax-specific parser?

## Attached Folder

Runnable skills may later need an attached folder, similar to normal folder-based skills.

Possible uses:

- helper scripts
- local libraries
- templates
- support files used by `[CODE]`

This part is intentionally unresolved for now.

## Why This Matters

- Lets some skills act like reusable scripted operations without forcing everything into native tool definitions.
- Keeps invocation discoverable in the prompt via `[SYNTAX]`.
- Preserves the skill model while adding direct executable behavior.
- Could reduce the need for many one-off hardcoded tools.

## AI Analysis

### Overall Assessment

The idea is strong and worth exploring. It fits BlazeAI well because the project already treats skills as editable prompt assets and already supports local execution through native tools.

The main value is that it could add reusable, script-backed automations without forcing every new capability into Go-native tool code.

### Strong Parts

- Good fit for project-scoped and user-editable automation.
- `[SYNTAX]` is a good prompt-facing contract for teaching the LLM how the runnable skill is invoked.
- `[CODE]` keeps the executable part close to the prompt behavior and any attached resources.
- A folder-based runnable skill could later support support scripts, libraries, and templates without changing the top-level concept.

### Main Concern

Using `load_skill` for both activation and execution would make the model and runtime semantics ambiguous.

Today, `load_skill` clearly means: add this skill to the active prompt state.

If it also starts executing `[CODE]`, then one tool would have two different meanings:

- load context
- execute code

That would make behavior harder to predict, harder to debug, and harder to explain in prompt rules.

### Recommended Direction

Keep the idea, but separate activation from execution.

Recommended tool split:

- `load_skill` = activate a skill in prompt context
- `unload_skill` = remove it from prompt context
- `run_skill` = execute a runnable skill

This still keeps the "skill as tool" concept, but avoids overloading `load_skill`.

### Recommended Runtime Rules

- A skill is runnable only if it has both `[SYNTAX]` and `[CODE]`.
- If `[SYNTAX]` exists but `[CODE]` is missing, runtime should fail with a clear error.
- `run_skill` should accept `name` plus `arguments`.
- The LLM learns invocation shape from `[SYNTAX]`, not from per-skill OpenAI function schemas.
- `load_skill` should keep its current behavior unchanged.

### Recommended Execution Constraints

First implementation should stay strict and boring:

- explicit code fence language in `[CODE]` such as `bash`, `sh`, or `python`
- no fallback runtime if the interpreter is missing
- default timeout like other tools
- clear stdout/stderr/exit-code capture
- visible UI output such as `Running skill: <name>`

### Security And Trust Notes

Runnable skills are executable code. That is powerful, but also risky, especially for project-scoped skills coming from repositories.

BlazeAI already has `shell`, so this is not a brand new trust class, but runnable skills would make execution easier and more implicit. The runtime should make execution visible and explicit.

### Final Opinion

The idea is good.

The main adjustment is architectural, not conceptual: do not make `load_skill` execute code. Add a separate `run_skill` path and let `[SYNTAX]` + `[CODE]` define runnable skill behavior cleanly.
