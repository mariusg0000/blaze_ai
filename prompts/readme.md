# Prompt Variables

This directory stores the runtime system prompt fragments used by BlazeAI.

## Variables
- `{WORK_DIR}`: current working directory.
- `{OS_INFO}`: human-readable operating system info.
- `{APP_HOME}`: BlazeAI app home path.
- `{OS_PROMPT}`: rendered OS-specific prompt content.
- `{HOST_HELPERS_AVAILABLE}`: available host helper list content.
- `{HOST_HELPERS_OPTIONAL}`: optional host helper warning content.
- `{SKILLS_AVAILABLE}`: available skill list content.
- `{SKILLS_ACTIVE}`: active skill details content.
- `{AGENTS_CONTENT}`: AGENTS.md content for the current work tree.
- `{SKILL_DIR}`: injected when rendering a specific skill file.

## Files
- `sysprompt.md`: universal prompt layout.
- `sysprompt.linux.md`, `sysprompt.darwin.md`, `sysprompt.windows.md`: OS-specific prompt content.
