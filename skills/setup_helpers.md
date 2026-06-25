[DESCRIPTION]
Load when host tools are missing or the user asks about installing helpers. Use for `rg`, `fd`, `jq`, `git`, `curl`, `pandoc`, `sqlite3`, and approval-safe install guidance.

[BEHAVIOR]
# Setup Helpers

## Scope
- Configure helper utilities only when the user asks or when a missing helper would materially improve current work.
- NEVER install anything without explicit user approval.
- NEVER run `sudo` without explicit user approval per command.
- After installation, verify that the helper is usable before the next prompt build.

## Core Cross-Platform Helpers

| Helper | Purpose | Typical Install Command (varies by OS) |
|--------|---------|----------------------------------------|
| rg     | fast recursive content search | `apt install ripgrep` / `brew install ripgrep` / `winget install BurntSushi.ripgrep` |
| fd     | fast file and directory discovery | `apt install fd-find` / `brew install fd` / `winget install sharkdp.fd` |
| jq     | JSON inspection and transformation | `apt install jq` / `brew install jq` / `winget install jqlang.jq` |
| git    | VCS operations | `apt install git` / `brew install git` / `winget install Git.Git` |
| curl   | HTTP/API checks, downloads | `apt install curl` / `brew install curl` / (built-in on modern Windows) |
| pandoc | document conversion (MD, HTML, PDF, DOCX, LaTeX) | `apt install pandoc` / `brew install pandoc` / `winget install JohnMacFarlane.Pandoc` |
| sqlite3| lightweight SQL database queries | `apt install sqlite3` / `brew install sqlite3` / `winget install SQLite.SQLite` |

## Detection

Before suggesting installation, verify what is already available:

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
2. Ask user which helper(s) to install and confirm.
3. If sudo is required, ask separately — never batch sudo commands without approval.
4. Example: `sudo apt update && sudo apt install -y ripgrep fd-find jq pandoc sqlite3`

### macOS
1. Check for Homebrew: `command -v brew`
2. If brew exists and user approves: `brew install ripgrep fd jq pandoc sqlite3`
3. If brew missing, suggest installing brew first (ask user).

### Windows
1. Check for package manager: `winget --version` or `scoop` or `choco`
2. If winget exists and user approves:
   - `winget install --id BurntSushi.ripgrep`
   - `winget install --id JohnMacFarlane.Pandoc`
   - `winget install --id SQLite.SQLite`
3. If no package manager, explain that user needs winget/scoop/choco first.

## Verification After Install
- Re-run the detection command.
- If the helper still does not resolve, report failure and continue with available alternatives.
- Do not loop or retry without user instruction.

## Dismissing The Helper Reminder
- After all core helpers are installed and verified, edit `{APP_HOME}/config/config.json` and set `helperSetup.dismissed` to `true`.
- If the user says they do not need remaining helpers and want to stop the reminder, also set `helperSetup.dismissed` to `true`.
- If the user wants to skip specific helpers permanently, add them to `helperSetup.declined` instead.
- Example when done or user declines:
```json
"helperSetup": {
  "dismissed": true,
  "declined": ["fd"]
}
```
- After `dismissed` is set to `true`, the advisory reminder will not appear in future prompts.

## Python Environment
- Python is NOT a host helper — it is a restricted runtime.
- If Python is truly necessary and the BlazeAI venv does not exist yet at `{APP_HOME}/scripts/venv/`:
  - Ask the user before creating the venv.
  - Create it lazily with the system Python: `python3 -m venv {APP_HOME}/scripts/venv`
  - All subsequent Python usage MUST go through this venv:
    - `{APP_HOME}/scripts/venv/bin/python ...` (Linux/macOS)
    - `{APP_HOME}/scripts/venv/Scripts/python.exe ...` (Windows)
  - All packages MUST be installed into this venv:
    - `{APP_HOME}/scripts/venv/bin/python -m pip install ...` (Linux/macOS)
    - `{APP_HOME}/scripts/venv/Scripts/python.exe -m pip install ...` (Windows)
- NEVER use system `python`, `python3`, or global `pip` directly.
