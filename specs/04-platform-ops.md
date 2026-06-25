# 04 - Platform And Operations

## Overview
- This spec defines BlazeAI platform support, build strategy, shell behavior, safety rules, first-run setup, and environment configuration.
- Product intent is defined in `01-product-scope.md`.
- Runtime mechanics are defined in `02-core-runtime.md`.
- Interface details are defined in `03-interfaces.md`.

## Supported Platforms
- Linux
- macOS
- Windows

## Build Toolchain

### Go Toolchain Policy
- The project uses the Go `toolchain` directive in `go.mod`.
- `go.mod` declares the minimum Go version and the desired toolchain version.
- `GOTOOLCHAIN=auto` is the expected developer setting.
- The correct toolchain is downloaded automatically on first build.
- No `.tools/` directory is committed to the repository.
- No system Go installation beyond a bootstrap minimum (Go 1.21+) is required.

### Build Strategy
- `CGO_ENABLED=0` is the default release strategy.
- Release builds are single static binaries per target platform.
- Dependencies that require CGO or platform-native libraries are avoided unless a concrete requirement justifies reduced portability.

### Release Targets
- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`

### Linux Portability
- Linux compatibility is a primary constraint.
- Release builds should prefer conservative build targets to maximize runtime portability on older systems.
- Avoid design choices that require a newer system libc than necessary for supported Linux targets.
- Linux release validation should include at least one system older or more conservative than the main development machine.

## Shell Behavior

### Shell Selection Per Platform
- Linux: prefer `bash`, fallback to `sh`.
- macOS: prefer `bash`, fallback to `sh`. Explicit `zsh` support is optional and not required in this phase.
- Windows: prefer `pwsh`, fallback to `powershell.exe`, final fallback to `cmd.exe`.

### Shell Awareness
- The runtime and prompt must remain aware of platform differences in:
  - quoting rules
  - path syntax and separators
  - environment variable syntax
  - command availability
  - script formats

### Script Execution
- Inline shell commands are preferred for simple tasks.
- OS-native scripts are preferred for complex tasks.
- Scripts are stored under `app_home/scripts/`.
- Bash scripts on Linux and macOS.
- PowerShell scripts on Windows.

### Python Fallback
- Python is a last resort only, when shell and OS-native scripts are insufficient.
- Python usage is suggested in the system prompt as a last resort, not as a default path.
- When Python is necessary, a virtual environment is used under `app_home/scripts/venv`.
- The virtual environment is created lazily, only when Python is first needed.
- The runtime does not pre-create the venv at startup.

## Environment Configuration

### API Keys
- API keys are stored directly in `config.json` at the provider level.
- `config.json` lives in `app_home/config/` and is not committed to version control.
- There is no separate `.env` file in this phase.

### Config File
- Runtime configuration lives in `app_home/config/config.json`.
- Config contains providers, models, role assignments, and API keys.
- Config is the single source of truth for runtime configuration.

## First-Run Setup

### Trigger
- First-run setup triggers when `config.json` is missing or the `default` model role is not assigned.

### Provider Selection
- The setup is interactive in the console.
- The user is presented with a curated list of well-known OpenAI-compatible providers, maximum 15 entries.
- The list includes providers such as OpenRouter, DeepSeek, OpenAI, OpenCode-Go, and similar major providers.
- The user can select a provider by number from the list.
- The user can choose a custom option to define a provider manually by entering `name`, `endpoint`, and `api_key`.

### API Key Entry
- After provider selection or custom definition, the user is prompted for the API key.
- The API key is stored in `config.json` at the provider level.
- The key is mapped to the selected provider by name.

### Model Retrieving
- After the API key is entered, the runtime attempts to retrieve the list of available models from the provider endpoint.
- If model retrieval fails, the runtime stops with a clear error message. No fallback.
- The user is presented with the retrieved model list and selects by number.

### Role Assignment
- After model selection, the user assigns the `default` role.
- `vision` and `summarization` roles can be assigned during setup or skipped for later.

### Completion
- After setup, `config.json` is written to `app_home/config/`.
- The runtime proceeds to start a new session.

## Safety Rules

### Destructive Commands
- Destructive commands are treated with extreme care.
- The agent prefers narrow commands, verifies targets first, and avoids irreversible actions unless clearly requested.

### Backups
- Backups are a decision of the LLM, guided by the system prompt.
- The runtime does not enforce automatic backups before tool execution.
- When the agent decides a backup is appropriate, it stores backups under `app_home/backups/`.

### Privilege Elevation
- On Linux and macOS: `sudo` is used only after explicit user approval.
  - The user must enter the password interactively in the terminal.
  - The password is never revealed in chat or stored in session history.
- On Windows: `Run as Administrator` elevation follows the same principle.
  - Elevation is used only after explicit user approval.
  - The user handles the elevation prompt interactively.

### Sensitive Data
- The agent must avoid exposing sensitive file contents in logs or responses.
- API keys and secrets are never included in session JSON or prompt text.

## App Home Layout
- `app_home/` is created at first start under the OS home directory.
- Standard subfolders:
  - `skills/`: custom skills
  - `scripts/`: task scripts and Python venv
  - `scripts/venv/`: Python virtual environment, created lazily
  - `backups/`: agent-created backups
  - `projects/`: per-project session folders with JSON message history
  - `config/`: runtime configuration

## High-Risk Areas
- Local shell execution remains the main product risk.
- Cross-platform shell behavior differs materially across Linux, macOS, and Windows.
- Build compatibility on older Linux systems can fail if release artifacts are built in an environment that is too new or depends on native system libraries.
- Python venv portability across platforms is not guaranteed and is treated as a best-effort fallback only.
