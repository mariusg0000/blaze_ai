# Build And Deploy

## Source Files

| File | Role |
|------|------|
| `go.mod` | Module definition, toolchain directive |
| `embed.go` | `//go:embed` directives for prompts and skills |
| `main.go` | Entry point with CLI flags |
| `internal/platform/platform.go` | App home, bootstrap, OS detection |
| `internal/platform/apphome_readmes.go` | Seeded READMEs via `//go:embed` |
| `internal/prompt/prompt.go` | Prompt builder with required transport prompt selection |
| `deploy_nas.sh` | Linux amd64 build, remote installer packaging, NAS deploy |

## Build Requirements

### Go Version

- Module: `go 1.25.0`
- Toolchain: `go 1.26.4` (auto-downloaded by `GOTOOLCHAIN=auto`)
- Bootstrap minimum: Go 1.21+ on the build system

The `toolchain` directive in `go.mod` enables automatic toolchain download when
building with an older Go. `GOTOOLCHAIN=auto` must be set or defaulted in the
build environment.

### Dependencies

```
require (
    golang.org/x/image v0.43.0   // direct — high-quality image resize for analyze_image
    golang.org/x/sys v0.46.0   // indirect — syscall wrappers (process group kill)
    golang.org/x/term v0.44.0   // indirect — raw terminal mode (hidden input)
)
```

Minimal external dependencies. All are `golang.org/x` subrepos (not third-party).
One small direct dependency is used for image resizing in multimodal vision input.

## Build Flags

Default release configuration:

```
CGO_ENABLED=0
```

`CGO_ENABLED=0` produces a fully static binary with no libc dependency. This is
required for cross-platform release and for running on minimal Linux systems.

## Embedded Assets

Two directories are embedded into the binary at compile time:

```
//go:embed prompts/*
var embeddedPrompts embed.FS

//go:embed skills
var embeddedBuiltinSkills embed.FS
```

### prompts/

| File | Purpose |
|------|---------|
| `sysprompt.md` | Universal system prompt |
| `sysprompt.darwin.md` | macOS-specific prompt additions |
| `sysprompt.linux.md` | Linux-specific prompt additions |
| `sysprompt.windows.md` | Windows-specific prompt additions |
| `transport.console.md` | Console-specific prompt rules |
| `transport.telegram.md` | Telegram-specific prompt rules |
| `transport.web.md` | Web-specific prompt rules |

Resolved at startup: `fs.Sub(embeddedPrompts, "prompts")` → passed to
`prompt.Builder`, which then requires `Builder.TransportName` to select the
matching `transport.<name>.md` file.

### skills/

| File | Purpose |
|------|---------|
| `skill-manager.md` | Manage skills listing, load, unload |
| `config-manager.md` | LLM-assisted configuration |
| `audit-manager.md` | Review recent sessions and synthesize workflow improvements |

Seeded to `app_home/skills/` at startup by `skills.SeedBuiltins()` if they
don't already exist (user customizations take priority).

## Release Targets

| Target | Build |
|--------|-------|
| `linux/amd64` | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build` |
| `linux/arm64` | `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build` |
| `darwin/amd64` | `GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build` |
| `darwin/arm64` | `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build` |
| `windows/amd64` | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build` |

### Linux Compatibility

Conservative build targets to minimize libc dependency. With `CGO_ENABLED=0`,
the binary has zero libc requirements. Validated on older glibc-based systems
to ensure forward compatibility.

No platform-specific build tags are required for the main binary — platform
divergence is handled at runtime through `platform.Detect()`.

## Binary Architecture

A single binary operates in two modes:

| Mode | Flag | Behavior |
|------|------|----------|
| Console REPL | (no flag) | Interactive terminal session |
| Telegram bridge | `--telegram <instance>` | Long-polling bot instance |

No separate `cmd/` subdirectories, no subcommand entry points. The Telegram
bridge is a CLI flag, not a separate binary.

## CLI Flags

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `-c` | bool | false | Continue last cleanly closed session |
| `-r` | bool | false | Resume most recent session (interrupted or clean) |
| `--telegram` | string | "" | Run Telegram bridge for named instance |

Flags `-c` and `-r` are mutually exclusive in intent (the code handles them as
separate cases in `openSession`).

## Startup Sequence (Console)

```
main()
  └─ run()
       ├─ Detect OS  → osType (linux/darwin/windows)
       ├─ Bootstrap  → app home + subfolders
       ├─ loadRuntimeConfig  → config.Load or firstRun
       ├─ prepareBuiltinAssets  → embedded prompts + seed skills
       ├─ openSession  → new / -c / -r resumed session
       │    ├─ session.Create(workDir)
       │    ├─ session.LastClean(workDir)
       │    └─ session.Last(workDir)
       ├─ runtime.NewAgent(cfg, sess, osType, promptsFS, workDir, handler, "console")
       ├─ agent.Handler = console
       └─ console.Run()
```

## Startup Sequence (Telegram)

```
main()
  └─ run()
       ├─ Detect OS
       ├─ Bootstrap
       ├─ loadRuntimeConfig
       ├─ prepareBuiltinAssets
       └─ telegram.Run(ctx, cfg, osType, promptsFS, instance)
```

The Telegram path diverges early — it does not call `openSession`,
`runConsole`, or interact with the work directory beyond what
`telegram.Run()` configures.

Inside `telegram.Run()`, agent construction uses `transportName="telegram"`,
which loads `transport.telegram.md` and applies the dynamic Telegram
`TransportContext` string.

## Deploy

### Simple Build And Run

```sh
CGO_ENABLED=0 go build -o blazeai .
./blazeai
```

### Cross-Compile For Target

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o blazeai-linux-amd64 .
```

### NAS Deploy Script

The repository includes `deploy_nas.sh` for the common Linux/NAS deployment path:

```sh
./deploy_nas.sh
./deploy_nas.sh user@host
```

Current behavior:

1. Builds `linux/amd64` with `CGO_ENABLED=0`
2. Writes the binary to `/tmp/blazeai-bin`
3. Base64-embeds that binary into `/tmp/blazeai_deploy/install.sh`
4. Copies the installer to `~/blazeai_installer/install.sh` on the remote host
5. Runs the remote installer over SSH

Default SSH target is `nas@192.168.0.104` when no argument is provided.

### Remote Installer Behavior

The generated remote installer:

- installs BlazeAI to `${HOME}/.local/bin/blazeai`
- removes any previous target binary before copying the new one
- ensures executable mode `0755`
- appends a guarded `${HOME}/.local/bin` PATH block to `${HOME}/.profile` when needed
- prints a final reminder to start a new shell or `source ~/.profile`

### Telegram Bridge Service

The Telegram bridge is typically run as a systemd service:

```
[Unit]
Description=BlazeAI Telegram Bridge (instance %i)
After=network-online.target

[Service]
ExecStart=/home/user/.local/bin/blazeai --telegram %i
Restart=always
User=user
Group=user

[Install]
WantedBy=default.target
```

`Restart=always` is recommended because the bridge has in-process retry for
transient polling failures, but systemd provides an additional recovery layer
for fatal process exits.
