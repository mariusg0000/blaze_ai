# Platform

## Source Files

| File | Role |
|------|------|
| `internal/platform/platform.go` | OS detection, shell chain, app home, project dirs, bootstrap, OSInfo |
| `internal/platform/platform_test.go` | Unit tests |
| `internal/platform/doc.go` | Package docs |
| `internal/platform/apphome_readmes.go` | Builtin README seeding for app home subfolders |
| `internal/platform/apphome_readmes/` | Embedded README files (skills, scripts, backups, projects, config, telegram) |

## Overview

The platform package is the lowest layer — no internal dependencies, only Go
standard library. It provides OS detection, shell chain selection, app home
resolution, and standard subfolder creation.

It has zero dependencies on any other BlazeAI package.

## OS Detection

```go
type OS string

const (
    Linux   OS = "linux"
    Darwin  OS = "darwin"
    Windows OS = "windows"
)
```

`Detect()` maps `runtime.GOOS` to the OS type. Returns `ErrUnsupportedOS` for
any value other than `linux`, `darwin`, or `windows`.

## Shell Chain

| OS | Shell Preference |
|----|-----------------|
| Linux | `bash` → `sh` |
| Darwin | `bash` → `sh` |
| Windows | `pwsh` → `powershell.exe` → `cmd.exe` |

`ShellChain(os)` returns the ordered list of shell binary names. No filesystem
check — pure static lookup.

`SelectShell(os)` iterates the chain and returns the first binary found via
`exec.LookPath`. Returns `ErrNoShell` with the full chain as context if none
found.

## App Home

Resolved as `$HOME/blazeai` on all platforms.

```go
func AppHome() (string, error)
```

Uses `os.UserHomeDir()` and appends `blazeai`. Error if home directory cannot
be resolved (unusual on desktop, may happen in minimal containers).

### Standard Subfolders

```
$HOME/blazeai/
  config/     — config.json, modes.json
  skills/     — global user-installed skills
  scripts/    — scripts for tools
  scripts/venv/  — LAZY: created only when Python is first used
  projects/   — per-project session and skill storage
  backups/    — LLM-created backups
  telegram/   — Telegram bridge instance storage
```

`scripts/venv` is excluded from `Bootstrap()` — created lazily by the shell
tool when Python execution is needed.

### Bootstrap

```go
func Bootstrap() error
```

1. Creates app home dir (`os.MkdirAll`, 0755)
2. Removes legacy `sessions/` directory if present (migrated to `projects/`)
3. Creates each standard subfolder (0755)
4. Seeds README files into subfolders if missing

### Legacy Migration

The old `app_home/sessions/` directory (from an earlier architecture) is
removed if found during bootstrap. Sessions now live under
`app_home/projects/<project>/sessions/`.

## Project Directories

Each working directory gets a sanitized folder name under
`app_home/projects/<sanitized_path>/`:

```go
func ProjectFolderName(workDir string) string
```

Sanitization:
1. `filepath.ToSlash(workDir)` — normalize to forward slashes
2. Replace `/` and `:` with `_`
3. Lowercase
4. Trim leading/trailing `_`

Example: `/mnt/data/work/ai/projects/blazeai` → `mnt_data_work_ai_projects_blazeai`

### Project Structure

```
app_home/projects/<project>/
  sessions/   — session folders (YYYYMMDD-HHMMSS-<hex>)
  skills/     — project-scoped skills (<name>/skill.md)
```

`EnsureProjectDir(workDir)` creates both subdirectories (0755) and returns the
sessions path.

## OSInfo

Provides a human-readable OS description for prompt injection. Used by
`{OS_INFO}` variable in prompt building.

| OS | Source | Fallback |
|----|--------|----------|
| Linux | `/etc/os-release` → `PRETTY_NAME` | `"Linux"` |
| Darwin | `sw_vers -productName` + `sw_vers -productVersion` | `"macOS"` |
| Windows | `cmd /c ver` | `"Windows"` |

All OS-specific detection uses defensive coding — any error falls back to the
generic OS name. No hard errors from `OSInfo()`.

## App Home READMEs

README files are embedded via `//go:embed` in `apphome_readmes.go` and seeded
into app home subfolders during bootstrap. Existing files are never overwritten.

One README per subfolder:
- `skills/README.md` — how to add global skills
- `scripts/README.md` — script management
- `backups/README.md` — backup format and usage
- `projects/README.md` — how project storage works
- `config/README.md` — configuration file reference
- `telegram/README.md` — Telegram bridge setup guide

## Error Constants

| Error | Cause |
|-------|-------|
| `ErrUnsupportedOS` | `runtime.GOOS` is not linux/darwin/windows |
| `ErrNoShell` | No shell binary from the chain is on PATH |
