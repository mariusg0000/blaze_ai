# Prompt Variables

This directory stores the runtime system prompt fragments used by BlazeAI.

## Variables
- `{WORK_DIR}`: current working directory.
- `{OS_INFO}`: human-readable operating system info.
- `{APP_HOME}`: BlazeAI app home path.
- `{OS_PROMPT}`: rendered OS-specific prompt content.
- `{HOST_HELPERS_SECTION}`: rendered host helper section.
- `{SKILLS_SECTION}`: rendered skills section.
- `{MEMORIES_SECTION}`: rendered memories section.
- `{AGENTS_SECTION}`: rendered AGENTS.md wrapper section.
- `{SKILL_DIR}`: injected when rendering a specific skill file.

## Files
- `sysprompt.md`: universal prompt layout.
- `sysprompt.linux.md`, `sysprompt.darwin.md`, `sysprompt.windows.md`: OS-specific prompt content.
