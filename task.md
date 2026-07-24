---
status: completed
---

# Bring the builtin config-manager skill up to date

## User outcome

Update the embedded builtin `config-manager` skill so that it accurately and
completely explains current BlazeAI configuration, especially how interactive
and executor agents are defined and how the UI concept named “mode” is now set,
selected, cycled, and persisted. Remove instructions that present legacy
`modes.json` as active configuration. Preserve the valid Telegram and host-helper
guidance.

The revised skill must be operational guidance an agent can safely follow, not
an architecture history or a dump of every project subsystem.

## Current state and rigorous findings

- `skills/config-manager.md` is a 380-line embedded builtin skill using the
  required `[DESCRIPTION]` and `[BODY]` sections. `embed.go` embeds `skills/`;
  builtin discovery resolves it as `builtin/config-manager`. It is not copied to
  app home and cannot be overridden by a same-named global/project skill.
- The provider, role, Telegram, and helper guidance is largely current.
- The app-home inventory is incomplete: runtime creates `skills`, `scripts`,
  `backups`, `projects`, `config`, `telegram`, and `agents` under the user-home
  `blazeai` directory. Current source resolves Unix app home as `~/blazeai` and
  Windows app home as `%USERPROFILE%\blazeai`.
- The `config.json` example omits `helperSetup` and `debugPrompt`. It also omits
  `last_model`, but that field is vestigial: first run writes it once and no
  production runtime path reads it. Active model state belongs to
  `agents.json` or Telegram `state.json`.
- Favorites, compaction, and `stripReasoning` appear in JSON but lack enough
  operational explanation. `/model`, `/model +`, `/model -`, and Ctrl+\ are not
  fully connected to provider listing, favorite persistence, and per-agent
  model state.
- The entire “Work Modes (modes.json)” section is stale. `modes.json` is now
  legacy migration input only; first run does not create it and operational
  runtime behavior never reads it after migration.
- Active modes are interactive Markdown agent definitions under
  `{APP_HOME}/agents/*.md`. `/mode <name>` selects an interactive agent and Tab
  cycles interactive agents with wrap-around. Executor agents cannot be
  selected as modes.
- `agents.json` is runtime state, not the definition source. It stores each
  interactive agent’s active model plus `last_agent`. The model declared in an
  interactive definition seeds a missing state entry; later `/model` changes
  update `agents.json`, not the Markdown definition.
- Definitions are loaded only at Agent construction. There is no hot reload or
  create/rename/delete command; filesystem definition changes require restart.
- Creating a new interactive definition is reconciled automatically into
  `agents.json`. Renaming/deleting an interactive identity is not: stale state
  or `last_agent` produces an explicit startup error. Renaming/deleting an
  executor requires updating every interactive `agents:` reference.
- With no interactive definitions and no `agents.json`, startup provisions
  `agents/default.md`. If `agents.json` exists, even empty, deleting all
  interactive definitions causes a hard error and does not recreate the
  default.
- The existing ad-hoc `fetch_models` script section is redundant and
  incomplete. The console’s `/model` flow already calls the configured
  provider’s model-list endpoint and handles the existing OAuth client path.
- No test checks config-manager’s exact prose. Existing skill tests cover the
  parser/discovery contract, and root tests cover embedded assets.

## Supporting evidence

- `.agents/explorer/000008-configuration-contract.md`
- `.agents/explorer/000009-agents-modes-contract.md`
- `.agents/explorer/000010-config-skill-conformance.md`
- `.agents/explorer/000011-agent-edit-lifecycle.md`

These reports are navigation evidence. Current source remains authoritative
where older specs still describe mode-based runtime state.

## Resolved scope and design

1. Modify only `skills/config-manager.md`.
2. Keep its `[DESCRIPTION]` / `[BODY]` format and builtin name unchanged.
3. Replace the description with this exact scope statement:

   `Load for BlazeAI configuration: providers, API keys, models, roles, compaction, reasoning stripping, interactive and executor agents (the active mode system), Telegram bridges, and host helpers.`

4. Rewrite the body as one coherent current guide. Preserve the source-backed
   Telegram and helper instructions, but integrate them under the same file
   ownership and safe-editing rules.
5. Remove the active `modes.json` operations and ad-hoc `fetch_models` script.
   Replace them with current agent configuration, runtime model commands, and a
   narrowly labeled legacy migration note.
6. Do not modify source, tests, specs, decisions, TODOs, other skills, embedded
   wiring, or runtime behavior.

## Exact content contract

The rewritten `[BODY]` must use the following heading order and contain all
listed facts. Wording may be concise, but it must preserve every stated
contract, distinction, warning, and example without introducing unsupported
behavior.

### 1. `# Config Manager`

State that the skill edits durable BlazeAI configuration and definitions. It
must inspect the complete current file before changing it, preserve unrelated
valid settings, write valid JSON/Markdown, keep secrets out of output, and stop
on missing required values or validation errors. It must never invent API keys,
provider endpoints, model IDs, tool names, agent names, Telegram IDs, or paths.

### 2. `## Configuration Sources and Ownership`

Document:

- `{APP_HOME}` is `<user-home>/blazeai` (`~/blazeai` on Unix,
  `%USERPROFILE%\blazeai` on Windows).
- Runtime-created subdirectories: `skills/`, `scripts/`, `backups/`,
  `projects/`, `config/`, `telegram/`, `agents/`.
- Manually editable sources:
  - `{APP_HOME}/config/config.json` — providers, favorites, roles, compaction,
    reasoning stripping, helper state, diagnostic prompt capture;
  - `{APP_HOME}/agents/*.md` — interactive and executor definitions;
  - `{APP_HOME}/telegram/<instance>/bridge.json` — Telegram credentials,
    authorization, and workdir.
- Runtime-managed state:
  - `{APP_HOME}/config/agents.json` — interactive agent model state and
    `last_agent`;
  - `{APP_HOME}/telegram/<instance>/state.json` — Telegram selected model and
    pending selection state.
- Legacy-only input:
  - `{APP_HOME}/config/modes.json` — one-time migration source, not current mode
    configuration.
- Persist JSON/state and generated agent files with private `0600` file mode.
- Manual `config.json` or `agents/*.md` edits require process restart. Runtime
  `/model`, `/mode`, favorite commands, and Tab act immediately and persist
  through the runtime-owned files.

### 3. `## Global config.json`

Include one complete valid template with placeholder values, covering exactly:

```json
{
  "providers": [
    {
      "name": "provider-name",
      "endpoint": "https://provider.example/v1",
      "api_key": "REPLACE_WITH_SECRET"
    }
  ],
  "favorite_models": [
    "provider-name/model-name"
  ],
  "roles": {
    "default": "provider-name/model-name",
    "vision": "provider-name/model-name",
    "summarization": "provider-name/model-name",
    "advisor": "provider-name/model-name"
  },
  "compaction": {
    "maxContextTokens": 100000,
    "minContextTokens": 50000,
    "summaryMaxTokens": 2000,
    "maxSummaryFiles": 10,
    "tokenCoefficient": 3.5,
    "maxBackoffOffsetTokens": 25000
  },
  "stripReasoning": {
    "enable": true,
    "preserveLast": 5
  },
  "helperSetup": {
    "dismissed": false,
    "declined": []
  },
  "debugPrompt": false
}
```

Explain that optional role keys may be omitted rather than set to invalid empty
model IDs. Do not put `last_model` in the editable template; identify it in a
runtime-managed/legacy note as vestigial first-run output that active model
selection does not use.

Add these subsections and rules:

- `### Providers`: unique non-empty `name`, non-empty endpoint, OpenAI-compatible
  API; API-key providers require `api_key`; ChatGPT OAuth uses
  `auth_type: "oauth"` plus OAuth credentials and should be configured through
  `/auth openai`; no provider fallback.
- `### Model IDs and Roles`: exact `provider/model_name` format, no `|` suffix;
  referenced provider must exist; `default` required; `vision`,
  `summarization`, `advisor` optional; requesting an unset role fails
  explicitly.
- `### Favorites and Model Selection`: `/model` opens live provider/model
  selection, `/model <provider/model>` assigns the current interactive agent’s
  model, `/model +` adds the current model to favorites, `/model -` removes it,
  Ctrl+\ cycles favorites, and zero/one favorites produce no cycle change.
- `### Compaction and Reasoning Payload`: list the six displayed defaults;
  explain that compaction thresholds govern context summarization and
  `stripReasoning.enable` removes older reasoning payload text while
  `preserveLast` keeps the newest N reasoning-bearing messages. Explicitly say
  this does not configure request-time reasoning effort or console reasoning
  display.
- `### Helper and Diagnostic Settings`: `helperSetup.dismissed` suppresses all
  optional helper suggestions; `helperSetup.declined` suppresses named helper
  suggestions; `debugPrompt: true` writes `prompt.json` in the session folder
  for diagnosis and defaults to false.
- `### Editing and Reload`: read-modify-validate-write the complete file;
  preserve credentials and unrelated fields; restart after manual edits;
  runtime commands update live state without requiring a restart.

### 4. `## Agent Definitions and Active Modes`

Begin with the exact clarification: an active BlazeAI “mode” is now an
interactive agent; `modes.json` is not the active configuration source.

#### `### Location and Discovery`

State that definitions are direct, non-recursive `.md` files under
`{APP_HOME}/agents/`, sorted by path at load, with globally unique frontmatter
`name` values. Invalid files, duplicate names, bad references, bad tools, or no
interactive definition stop startup with an explicit error. Definitions load
only at process startup.

#### `### Definition Format`

Include this exact schema example:

```markdown
---
name: agent-name
description: What this agent does
type: interactive
model: provider-name/model-name
timeout: 15m
directive: A short per-turn directive
tools:
  - read_file
  - shell
agents:
  - worker
---
Markdown instructions for the agent.
```

Document only the accepted keys: `name`, `description`, `type`, `model`,
`timeout`, `directive`, `tools`, `agents`. `kind:` is legacy and must not be
written. Explain:

- `name` and `description` are required;
- `type` is exactly `interactive` or `executor`;
- `model` is required for interactive agents and optional for executors;
- an executor without `model` inherits the parent interactive model;
- `timeout`, when present, is a positive Go duration;
- `tools` is required, non-empty, unique, and every name must exist in the
  runtime registry;
- `directive` and `agents` are interactive-only;
- `agents` contains unique names of existing executor definitions;
- executors cannot reference other agents or use `run_agent`;
- when an interactive agent allows at least one executor, runtime exposes
  `run_agent` automatically;
- the Markdown body is the agent’s instruction text;
- an interactive directive is appended ephemerally to the latest user message
  for the provider call and is not persisted into session history.

#### `### Interactive Agent Example`

Include a valid example named `planning`, with `type: interactive`, a placeholder
provider/model, a non-empty `tools` list, `agents: [worker]`, an optional
directive, and a body explaining its role. Do not include ellipses as tool or
agent entries.

#### `### Executor Agent Example`

Include a valid example named `worker`, with `type: executor`, no `directive`, no
`agents`, a non-empty valid-looking tool list, optional omitted model to
demonstrate inheritance, and a focused body.

Label tool names in examples as examples that must be replaced with names from
the actual runtime registry when unavailable; never claim an unverified tool is
universally installed.

#### `### Selecting a Mode and Assigning Its Model`

Document:

- `/mode <interactive-name>` selects by exact name;
- Tab cycles loaded interactive agents with wrap-around;
- executor definitions are not selectable;
- `/model <provider/model>` changes the selected interactive agent’s active
  model and writes that state to `agents.json`;
- the definition’s `model` seeds state only when the interactive agent has no
  state entry;
- `last_agent` controls next-start selection and is updated by `/mode`/Tab;
- `agents.json` is normally runtime-managed and should not be used to define
  tools, directives, instructions, or executor permissions.

#### `### Creating, Editing, Renaming, and Deleting Safely`

Provide this exact operation contract:

1. Create/edit the Markdown definitions, validate all names/tools/references,
   then restart. New interactive state entries are added automatically.
2. Renaming only a filename is safe because frontmatter `name` is identity.
3. Renaming an interactive `name` requires updating/removing the old
   `agents.json` state entry and updating `last_agent` when it points to the old
   name; otherwise startup fails.
4. Deleting an interactive requires removing its state entry, selecting an
   existing `last_agent`, and leaving at least one interactive definition.
5. Renaming/deleting an executor requires updating every interactive `agents:`
   reference first; otherwise startup fails.
6. Do not delete all interactive definitions while `agents.json` exists; the
   default is not recreated in that condition.
7. Preserve unrelated runtime state and never silently repair an ambiguous
   rename or choose a replacement agent/model without user confirmation.

#### `### Fresh-Install Default Agent`

Explain that zero interactive definitions plus absent `agents.json` provisions
`agents/default.md` from the validated default role and available runtime tools.
It is user-editable and never overwritten. Existing `agents.json` suppresses
re-provisioning.

### 5. `## Legacy modes.json Migration Only`

State prominently: do not create or edit `modes.json` to configure current
modes. Document it only for existing installations:

- migration runs only when `agents.json` is absent and `modes.json` exists;
- each legacy mode becomes `<mode.name>.md` with `type: interactive`;
- `model` and `directive` transfer;
- allowed tools are derived by subtracting `denied_tools` and control tools
  from the runtime tool set;
- legacy `agents` become executor references;
- existing destination files are never overwritten;
- unsafe names and invalid content fail explicitly;
- legacy `kind: one-shot` definitions are migrated to `type: executor`, but all
  new definitions must use `type`;
- legacy `modes` embedded directly in `config.json` have no active production
  migration caller and must not be claimed as automatically migrated.

### 6. `## Telegram Bridge Guide`

Retain the existing source-backed guide, including:

- instance path and required `bridge.json` keys `bot_token`,
  `allowed_chat_id`, `workdir`;
- non-empty token, non-zero chat ID, absolute existing directory;
- runtime-managed `state.json` with valid `selected_model`;
- `-telegram <instance>` startup, service setup, restart, authorization, and
  verification;
- no invented token/chat ID/model/path and explicit stop conditions.

Correct ownership wording so `state.json` is never presented as a definition
file users should freely rewrite.

### 7. `## Host Helper Setup Guide`

Retain the existing source-backed helper scope, detection, supported helper
table, OS-specific installation, verification, `helperSetup` dismissal/decline
behavior, and optional Python venv guidance. Preserve explicit approval before
privileged/package-manager changes and no silent fallback.

### 8. `## Final Validation Checklist`

End with a concise checklist requiring:

- valid complete JSON and private file permissions;
- required provider/default role and valid model references;
- unique agent names, valid types/tools/executor references, and at least one
  interactive agent;
- consistent `agents.json` only when a rename/delete requires state repair;
- absolute Telegram workdir and authorized chat;
- restart after manual config/definition edits;
- no fallback, guessed secret, guessed model/tool/agent, or overwrite of
  unrelated settings.

## Rejected or deferred alternatives

- Do not update stale subsystem specs in this task; the user requested the
  builtin skill, and source-backed corrections can be made entirely there.
- Do not change runtime code to revive `modes.json`, add hot reload, add agent
  management commands, or reconcile stale state automatically.
- Do not add a dedicated content-lock test for headings/prose. The skill parser
  is generic, and such a test would make future documentation maintenance
  brittle without testing runtime behavior.
- Do not document sessions, project skill authoring, or every CLI flag. Those
  belong to separate skill/session guidance and are not configuration-manager
  omissions.
- Do not keep the user-created `fetch_models` script; use the existing `/model`
  provider-listing flow.

## Exact write path

- `skills/config-manager.md`: modify

No other path may be created, modified, formatted, regenerated, or deleted.

## Delegation and execution

After approval, delegate the exact Markdown rewrite to `operator`, not Coder.
The operation must preserve valid Telegram/helper facts while replacing stale
mode and model-fetch guidance. Operator must not infer additional schema,
commands, defaults, paths, or behavior beyond this contract.

## Verification commands

1. `go test ./internal/skills`
2. `go test ./...`

Run both after the final skill write. Return one concise `PASS`, `FAIL`, or
`NOT RUN` result per command, without raw logs.

## Acceptance criteria

- Only `skills/config-manager.md` changes.
- The skill still parses as embedded builtin `config-manager` with exactly one
  `[DESCRIPTION]` and one `[BODY]` section.
- Every current user-editable config field relevant to config-manager is
  covered; vestigial/runtime-managed fields are identified but not promoted as
  manual configuration.
- Agent definition schema, interactive/executor rules, selection, model state,
  restart behavior, bootstrap, and safe rename/delete invariants are complete.
- Active mode instructions use interactive agents; `modes.json` appears only as
  legacy migration input.
- The ad-hoc model-fetch script is removed in favor of `/model`.
- Telegram and helper guidance remains complete and source-consistent.
- Both verification commands pass.

## Unresolved questions

None.

## Stop conditions

Stop if the update would require changing runtime behavior, reviving active
`modes.json`, inventing a tool/model/provider/secret, modifying another file,
or reconciling a source contradiction not resolved in this plan.

## Completion

- Accepted outcome: The embedded builtin `config-manager` skill now documents
  current config ownership/schema, interactive and executor agent definitions,
  agent selection/model persistence, bootstrap and safe edit lifecycle, and
  legacy-only modes migration while retaining Telegram and helper guidance.
- Changed paths: `skills/config-manager.md`.
- Verification: `go test ./internal/skills` — PASS; `go test ./...` — PASS.
- Remaining issues: None.
