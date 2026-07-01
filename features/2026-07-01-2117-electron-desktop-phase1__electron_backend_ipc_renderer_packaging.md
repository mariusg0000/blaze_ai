## Feature Description
Added the first active Electron desktop path for BlazeAI by pairing a new Go desktop backend with a minimal Electron shell that renders the archived desktop UI. The desktop backend preserves transcript, local state, and runtime integration without reviving WebView or GTK dependencies.

## Rationale And Implementation
The archived Go WebView transport had already been removed, but the replacement desktop path still needed a real executable foundation that kept the Go runtime and failed fast on config or backend errors. This phase extracted the reusable desktop transport logic into `internal/desktopbackend/`, added a dedicated `cmd/blazeai-desktop-backend/` stdio backend binary, embedded prompt and skill assets for that binary, and created `desktop_electron/` with backend spawn, preload IPC, and a renderer port of the old desktop UI.

## Modified Files
- `cmd/blazeai-desktop-backend/main.go`: boots the desktop backend binary with runtime config, embedded assets, and stdio serving.
- `cmd/blazeai-desktop-backend/embed.go`: embeds prompts and builtin skills into the desktop backend binary.
- `cmd/blazeai-desktop-backend/resources/prompts/`: embedded prompt templates needed by the backend binary.
- `cmd/blazeai-desktop-backend/resources/skills/`: embedded builtin skill templates needed by the backend binary.
- `internal/desktopbackend/config.go`: active desktop-local config loading and validation.
- `internal/desktopbackend/state.go`: active desktop-local state persistence for model, theme, font, and window geometry.
- `internal/desktopbackend/service.go`: core desktop backend state machine, session lifecycle, transcript state, and RPC handlers.
- `internal/desktopbackend/server.go`: stdio JSON request loop used by Electron.
- `internal/desktopbackend/protocol.go`: protocol payloads shared across backend methods.
- `internal/desktopbackend/handler.go`: runtime handler adapter for transcript, reasoning, tool activity, and status streaming.
- `internal/desktopbackend/commands.go`: deterministic desktop-local slash command handling.
- `internal/desktopbackend/tool_activity.go`: compact tool activity formatting reused from the old desktop flow.
- `internal/desktopbackend/hotkey.go`: config-side hotkey validation now that native hotkey ownership moves to Electron.
- `desktop_electron/package.json`: Electron app definition, scripts, and packaging config.
- `desktop_electron/scripts/build-backend.mjs`: builds the Go backend into the Electron app resources.
- `desktop_electron/src/main/main.js`: Electron main process lifecycle, backend spawn, IPC bridge, window restore, and native directory picker.
- `desktop_electron/src/preload/preload.js`: isolated renderer bridge for backend calls.
- `desktop_electron/src/renderer/index.html`: desktop renderer shell.
- `desktop_electron/src/renderer/styles.css`: ported desktop UI styling.
- `desktop_electron/src/renderer/app.js`: renderer polling, actions, and transcript rendering.
- `desktop_electron/src/renderer/vendor/`: Markdown, sanitization, syntax highlighting, and CSS vendor assets ported from the archive.
- `plans/electron_migration.md`: migration plan that matches the delivered phase structure and responsibilities.
