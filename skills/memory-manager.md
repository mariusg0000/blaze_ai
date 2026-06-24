[DESCRIPTION]
Load when persistent data must be remembered or an existing memory bank must be modified. Use before creating or modifying memory banks.

[DETAILS]
# Memory Manager

## Location
- Memory banks live at {APP_HOME}/memories/.
- Each memory bank is a separate `.md` file with a descriptive category name.

## File Format
Every memory bank file MUST have exactly two sections on their own lines:

- `[DESCRIPTION]` — a single line describing the bank.
- `[DETAILS]` — the compact key=value facts.

## Required Content
- A memory bank MUST contain persistent facts, reference data, mappings, preferences, identifiers, or other long-lived information.
- A memory bank MUST stay focused on one category.
- A memory bank MUST keep `[DESCRIPTION]` short and factual.

## Data Format
- Prefer one fact per line using `scope.key=value`.
- Keep values short, dense, and factual.
- Avoid headings, prose, narratives, and decorative Markdown unless absolutely needed.
- Avoid repeating the same fact under different keys.
- Dates are allowed only when the fact is time-sensitive, such as deadlines, expirations, or changing status.
- Do not add dates for stable identity facts, stable preferences, or static project facts.
- Keep each memory bank focused on one category.

## Collision Rules
- You cannot create a memory bank with the same name as an existing one.

## Forbidden Content
- Do not store step-by-step procedures, operational playbooks, or behavior rules that belong in a skill.
- Do not turn a memory bank into a workflow document, tutorial, or general notes file.
- Do not store transient reasoning, verbose narratives, or duplicated facts.

## Related Skill Guidance
- If the memory bank implies a reusable workflow, repeated task, or domain-specific operating rules, create or update a related skill.
- Use the memory bank for long-lived data and the skill for behavior.
