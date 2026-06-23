[DESCRIPTION]
Load when the user wants to store, update, or clean persistent memory facts. Use for the write rules and compact `memory.md` format.

[DETAILS]
# Memory Skill

## Location
- Memory lives at {APP_HOME}/memory/memory.md.
- A single file is used in this phase.

## Reading
- Memory is read fresh from disk on every prompt build.

## Writing
- Update memory explicitly using the `shell` tool to append or edit {APP_HOME}/memory/memory.md.
- The runtime does not automatically write to memory.
- Keep memory as compact machine-readable facts, not human-oriented notes.
- Merge related facts instead of appending redundant new lines.
- Rewrite existing lines when new information overlaps with old information.
- Remove outdated information when facts change.
- Drop low-value details that do not help future sessions.

## Format
- Prefer one fact per line using `scope.key=value`.
- Keep values short, dense, and factual.
- Avoid headings, prose, narratives, and decorative Markdown unless absolutely needed.
- Avoid repeating the same fact under different keys.
- Dates are allowed only when the fact is time-sensitive, such as deadlines, expirations, or changing status.
- Do not add dates for stable identity facts, stable preferences, or static project facts.
- Treat memory as a compact working state, not a transcript.

## Examples
- Good: `user.name=Marius Gheorghiu`
- Good: `user.lang=ro`
- Good: `project.deadline=2026-07-01`
- Bad: `# User Information`
- Bad: `- Name: Marius Gheorghiu`
- Bad: `- First recorded: 2026-06-03`
