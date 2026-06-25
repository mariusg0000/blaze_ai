[DESCRIPTION]
MUST load using `load_skill project-hub` when the user asks to open, resume, initialize, register, or switch between managed projects. Required for inbox capture/processing only with an explicit project name from the registry. Do NOT load for task or todo queries within the current working directory — check working-directory files first.

[BEHAVIOR]

## Related Data

The project registry is stored in `{SKILL_DIR}/data/projects.ini`.

## Project Registry Format

`{SKILL_DIR}/data/projects.ini` stores project registrations:

```ini
[project-key]
path=/abs/path/to/project/root
desc=Short description
aliases=alias1,alias2
status=active
```

- Section name = canonical project key (lowercase, hyphens).
- `aliases` = comma-separated alternative names, optional.
- Match requested names case-insensitively against both section names and aliases.

## Directory Layout (per managed project)

```
README.md
project-hub/00-home.md
project-hub/01-current.md
project-hub/02-inbox.md
project-hub/03-plan.md
project-hub/04-knowledge.md
project-hub/assets/
```

## File Roles

- `README.md`: short human-facing entry point.
- `project-hub/00-home.md`: project dashboard — summaries from current/plan/knowledge, plus a Sections index.
- `project-hub/01-current.md`: current focus, next step, blockers, resume context.
- `project-hub/02-inbox.md`: write-only raw capture buffer. Read only during explicit inbox processing.
- `project-hub/03-plan.md`: actionable tasks with Active/Next/Later/Postponed/Canceled/Done sections.
- `project-hub/04-knowledge.md`: stable decisions, facts, conventions.
- `project-hub/assets/`: screenshots, PDFs, diagrams, attachments.

## Language Rule

- Use the language already established in the project's notes.
- If no language is established, use the conversation language.
- Keep established technical terms in English when writing in another language.

## Preferred Workflow

### 1. Open Existing Project

1. Read `{SKILL_DIR}/data/projects.ini` and resolve the requested project name/alias.
2. If not found, ask the user for the absolute path and whether to register it.
3. Read `README.md`, then `project-hub/00-home.md`, then `project-hub/01-current.md`.
4. Show current Focus, Next Step, and minimal resume context.
5. Do **not** read `02-inbox.md`.
6. Do **not** rewrite `00-home.md` during open/resume unless the user asks.

### 2. Initialize New Project

1. Infer writing language from conversation.
2. Infer project goal, outcome, tech stack. Leave unknowns as `TBD`.
3. Create `README.md` and `project-hub/` files using templates (see section Templates).
4. Register the project in `projects.ini` unless the user declines.

### 3. Passive Capture (Inbox)

Triggered by: "am o idee", "note this", "add to inbox", "adauga in inbox", "noteaza asta", "new spec", "cerinta noua", "new requirement", "add to braindump", etc.

1. Treat the full user message as a single capture.
2. Lightly edit for grammar, spelling, punctuation, diacritics, and clarity. Preserve meaning and voice.
3. Append to `project-hub/02-inbox.md` using Python append.
4. Confirm briefly: `Noted in inbox.`
5. Return to previous task. Do not read `02-inbox.md`.

Use this Python pattern:
```python
from datetime import datetime
with open("project-hub/02-inbox.md", "a", encoding="utf-8") as f:
    f.write(f"\n---\n[{datetime.now().isoformat()}] [AUTO-CAPTURE]:\n{user_text}\n")
```

### 4. Integrate Notes (Process Inbox)

Run only when the user explicitly asks (e.g. "integrate notes", "process inbox", "proceseaza inbox", "integreaza notite").

1. Read `02-inbox.md` and categorize each note.
2. For each clearly placeable note:
   a. Write into destination file (`03-plan.md`, `04-knowledge.md`, or `01-current.md`).
   b. Read back the destination file to confirm content.
   c. Only then remove that note from `02-inbox.md`.
3. Leave ambiguous notes in `02-inbox.md`.
4. Update `00-home.md` only if source files changed materially.
5. When updating `03-plan.md`:
   - Immediate work → Active
   - Upcoming → Next
   - Low priority → Later
   - Paused → Postponed (with timestamp + reason)
   - Dropped → Canceled (with timestamp + reason)
   - Completed → Done (with timestamp)

### 5. Offboarding (before switching or closing)

1. Ask for current progress location and next micro-step.
2. Update `project-hub/01-current.md` with resume context; read it back.
3. Append any residual working-memory notes to `02-inbox.md` via Python append.
4. Update `00-home.md` only if the current summary changed materially.

### 6. Switching Projects

1. Run Offboarding for the current project.
2. Run Open Existing Project for the new project.

### 7. Add Custom Section

Triggered by: "add section [name]", "create page [name]", "adauga sectiune [nume]", etc.

1. Normalize name to lowercase hyphenated slug.
2. Pick next free numeric prefix (custom sections start at `06`).
3. Create `project-hub/NN-<slug>.md` with a top-level heading and description.
4. Read it back and confirm.
5. Add index line to `Sections` in `00-home.md`.
6. Read back `00-home.md` to confirm.

## Inbox Rules

- `02-inbox.md` is write-only during normal work. Read it only when explicitly processing it.
- One user message = one capture with one timestamp. Do not split.
- Preserve full prompt as one block. Preserve internal list structure. Do not add interpretation or commentary.
- After a note is successfully integrated, remove it from `02-inbox.md`.

## Plan Rules

- Each task lives in exactly one section of `03-plan.md` at a time.
- Tasks keep their timestamp when moved to Done, Postponed, or Canceled.
- Add a short reason for postponed/canceled when known.
- Do not delete completed/postponed/canceled tasks — keep as history.

## Home Rules

- `00-home.md` is a concise dashboard, not a source of truth.
- Do not duplicate full task lists or full knowledge notes.
- Keep a `Sections` index listing every project page with a short description.

## Safety

- After writing any critical file, read it back before the next action.
- If content does not match intent, halt and report. Do not continue.
- Do not fabricate content to fill empty sections.
- When unsure whether to delete or keep a note, keep it.
- Do not read `02-inbox.md` during normal open, resume, or active work.

## Templates

### `README.md`
```md
# [Project Name]
Short description.
See [[project-hub/00-home]] for working context.
```

### `project-hub/00-home.md`
```md
# Home
## Project
[Name]
## Goal
[Why this project exists]
## Outcome
[What done looks like]
## Current Summary
[1-3 lines about current focus and next step]
## Plan Summary
- Active: [summary]
- Next: [summary]
- Risks/blockers: [summary]
## Knowledge Summary
- [Key point]
- [Key point]
## Sections
- [[01-current]] - current focus, next step, blockers
- [[02-inbox]] - write-only raw capture
- [[03-plan]] - tasks and task history
- [[04-knowledge]] - stable decisions and facts
## Working Directory
[Leave blank if same as project root]
## Tech Stack
- [Tool 1]
```

### `project-hub/01-current.md`
```md
# Current
## Focus
[What is being worked on now]
## Next Step
[One small actionable next step]
## Current Context
[Short notes needed to resume quickly]
## Blockers
- [or leave empty]
## Open Questions
- [or leave empty]
```

### `project-hub/02-inbox.md`
```md
# Inbox
Temporary unprocessed notes only.
Write by append. Read only during inbox processing.
Delete integrated notes after successful integration.
---
## [YYYY-MM-DD HH:MM]
- [Raw note]
```

### `project-hub/03-plan.md`
```md
# Plan
## Active
- [ ] [Current task]
## Next
- [ ] [Upcoming task]
## Later
- [ ] [Future idea]
## Postponed
- [ ] [Task] — postponed: [date] reason: [why]
## Canceled
- [ ] [Task] — canceled: [date] reason: [why]
## Done
- [x] [Task] — done: [date]
```

### `project-hub/04-knowledge.md`
```md
# Knowledge
## Decisions
- [Decision]: [Why]
## Facts
- [Fact]
## Conventions
- [Convention]
## Notes
- [Useful stable note]
```

## First Commands to Try

| Intent | Command |
|--------|---------|
| Open project | Read `{SKILL_DIR}/data/projects.ini`, find project path, read project-hub files |
| Init project | Create README.md + project-hub/ files, register in projects.ini |
| Capture idea | Python append to project-hub/02-inbox.md |
| Process inbox | Read 02-inbox.md, categorize, integrate, remove processed notes |
| Switch project | Offboard current, open new |
| Add section | Create NN-slug.md, add to 00-home.md Sections index |

## Known Pitfalls

- **Do not read 02-inbox.md during normal work** — it's write-only until explicitly processed.
- **Do not split one user message into multiple inbox entries** — one capture = one timestamp.
- **Always read back after writing critical files** — halt if content doesn't match.
- **Do not delete old tasks from 03-plan.md** — move to Done/Postponed/Canceled, never delete.
- **Home is a summary, not source of truth** — derive from current/plan/knowledge, don't duplicate.
