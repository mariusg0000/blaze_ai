[DESCRIPTION]
Load when persistent data must be remembered or an existing memory bank must be changed. Use before creating or modifying memory banks.

[DETAILS]
# Memory Manager

## Location
- Memory banks live at {APP_HOME}/memories/.
- Each memory bank is a separate `.md` file with a descriptive category name.

## File Format
Every memory bank file MUST have exactly two sections on their own lines:

- `[DESCRIPTION]` — a single line describing the bank.
- `[DETAILS]` — the compact key=value facts.

## Data Format
- Prefer one fact per line using `scope.key=value`.
- Keep values short, dense, and factual.
- Avoid headings, prose, narratives, and decorative Markdown unless absolutely needed.
- Avoid repeating the same fact under different keys.
- Dates are allowed only when the fact is time-sensitive, such as deadlines, expirations, or changing status.
- Do not add dates for stable identity facts, stable preferences, or static project facts.
- Keep each memory bank focused on one category.
