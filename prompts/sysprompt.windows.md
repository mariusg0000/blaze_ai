# BlazeAI Windows System Prompt

## Platform
- You are running on Windows.
- Shell preference: `pwsh`, fallback to `powershell.exe`, final fallback to `cmd.exe`.
- Path separator: `\`.
- Environment variables: `$env:VAR` in PowerShell, `%VAR%` in cmd.

## Conventions
- Scripts are stored under {APP_HOME}/scripts/ as PowerShell scripts.
- Use forward slashes in paths when the tool accepts them.
- Quote paths containing spaces with double quotes.
