[DESCRIPTION]
Diagnose and optionally help install cross-platform host helper utilities such as rg, fd, jq, git, and curl.

[DETAILS]
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

## Detection

Before suggesting installation, verify what is already available:

```sh
# Linux / macOS
command -v rg && command -v fd && command -v jq && command -v git && command -v curl

# Windows PowerShell
Get-Command rg -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command fd -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command jq -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command git -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
Get-Command curl.exe -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Source
```

## Installation

### Linux
1. Detect package manager first: `which apt || which dnf || which pacman || which zypper || which apk`
2. Ask user which helper(s) to install and confirm.
3. If sudo is required, ask separately — never batch sudo commands without approval.
4. Example: `sudo apt update && sudo apt install -y ripgrep fd-find jq`

### macOS
1. Check for Homebrew: `command -v brew`
2. If brew exists and user approves: `brew install ripgrep fd jq`
3. If brew missing, suggest installing brew first (ask user).

### Windows
1. Check for package manager: `winget --version` or `scoop` or `choco`
2. If winget exists and user approves: `winget install --id BurntSushi.ripgrep`
3. If no package manager, explain that user needs winget/scoop/choco first.

## Verification After Install
- Re-run the detection command.
- If the helper still does not resolve, report failure and continue with available alternatives.
- Do not loop or retry without user instruction.

## Config Preferences (Optional)
- Only update `helperSetup.dismissed` or `helperSetup.declined` in `{APP_HOME}/config/config.json` if the user explicitly asks you to remember their preference.
- Example config snippet for declined helpers:
```json
"helperSetup": {
  "dismissed": false,
  "declined": ["fd"]
}
```
- `dismissed: true` suppresses all future automatic helper installation suggestions.

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
