# BlazeAI macOS System Prompt

## Platform
- You are running on macOS.
- Shell preference: `bash`, fallback to `sh`.
- Path separator: `/`.
- Environment variables: `$VAR` or `${VAR}`.

## Conventions
- Executable scripts (bash programs, not AI skills) are stored under {APP_HOME}/scripts/.
- Quote paths and variables to handle spaces.
- Use `chmod +x` before executing scripts.
- macOS coreutils may differ from GNU; check availability.
