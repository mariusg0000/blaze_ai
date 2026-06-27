# Helper Setup Guide

Use this document when host tools are missing or the user asks about helper installation.

## Scope
- Configure helper utilities only when the user asks or when a missing helper would materially improve current work.
- Never install anything without explicit user approval.
- Never run `sudo` without explicit user approval per command.
- After installation, verify that the helper is usable before the next prompt build.

## Core Cross-Platform Helpers

| Helper | Purpose | Typical Install Command |
|--------|---------|-------------------------|
| rg | fast recursive content search | `apt install ripgrep` / `brew install ripgrep` / `winget install BurntSushi.ripgrep` |
| fd | fast file and directory discovery | `apt install fd-find` / `brew install fd` / `winget install sharkdp.fd` |
| jq | JSON inspection and transformation | `apt install jq` / `brew install jq` / `winget install jqlang.jq` |
| git | VCS operations | `apt install git` / `brew install git` / `winget install Git.Git` |
| curl | HTTP/API checks, downloads | `apt install curl` / `brew install curl` / built-in on modern Windows |
| pandoc | document conversion | `apt install pandoc` / `brew install pandoc` / `winget install JohnMacFarlane.Pandoc` |
| sqlite3 | lightweight SQL queries | `apt install sqlite3` / `brew install sqlite3` / `winget install SQLite.SQLite` |

## Detection

Before suggesting installation, verify what is already available.

```sh
# Linux / macOS
command -v rg && command -v fd && command -v jq && command -v git && command -v curl && command -v pandoc && command -v sqlite3

# Windows PowerShell
Get-Command rg -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command fd -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command jq -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command git -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command curl.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command pandoc -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command sqlite3 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
```

## Installation

### Linux
1. Detect package manager first: `which apt || which dnf || which pacman || which zypper || which apk`
2. Ask which helper(s) to install and confirm.
3. If `sudo` is required, ask separately. Never batch `sudo` commands without approval.
4. Example: `sudo apt update && sudo apt install -y ripgrep fd-find jq pandoc sqlite3`

### macOS
1. Check for Homebrew: `command -v brew`
2. If brew exists and user approves: `brew install ripgrep fd jq pandoc sqlite3`
3. If brew is missing, suggest installing it first and ask the user.

### Windows
1. Check for package manager: `winget --version` or `scoop` or `choco`
2. If winget exists and user approves, install the requested helpers.
3. If no package manager exists, explain that the user needs winget, scoop, or choco first.

## Verification After Install
- Re-run detection.
- If the helper still does not resolve, report failure and continue with available alternatives.
- Do not loop or retry without user instruction.

## Dismissing The Helper Reminder
- After all core helpers are installed and verified, set `helperSetup.dismissed` to `true` in `{APP_HOME}/config/config.json`.
- If the user wants to stop the reminder without installing the remaining helpers, also set `helperSetup.dismissed` to `true`.
- If the user wants to skip specific helpers permanently, add them to `helperSetup.declined`.

Example:

```json
"helperSetup": {
  "dismissed": true,
  "declined": ["fd"]
}
```

## Python Environment
- Python is not a host helper. It is a restricted runtime.
- If Python is truly necessary and `{APP_HOME}/scripts/venv/` does not exist yet, ask before creating it.
- Create it lazily with `python3 -m venv {APP_HOME}/scripts/venv`.
- All later Python usage must go through that venv.
