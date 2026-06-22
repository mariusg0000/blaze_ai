[DESCRIPTION]
Persistent memory for BlazeAI. Store and recall facts, preferences, and project context across sessions in a single memory file.

[DETAILS]
# Memory Skill

## Location
- Memory lives at {APP_HOME}/memory/memory.md.
- A single file is used in this phase.

## Reading
- Memory is read fresh from disk on every prompt build.
- You do not need to load this skill to read memory; it is always injected into the runtime prompt.

## Writing
- Update memory explicitly using the `shell` tool to append or edit {APP_HOME}/memory/memory.md.
- The runtime does not automatically write to memory.
- Keep memory concise and structured with Markdown headings.
- Remove outdated information when facts change.

## Format
- Use Markdown.
- Group related facts under headings.
- Prefer short bullet points over long paragraphs.
- Include dates for time-sensitive information.
