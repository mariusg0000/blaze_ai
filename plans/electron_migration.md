# Electron Migration Plan

## Goal

Replace the removed Go WebView desktop transport with an Electron desktop app while preserving the current desktop UI format and behavior as closely as possible.

Required outcomes:
- no `webview_go`, WebKitGTK, or `JSC_SIGNAL_FOR_GC` in the active desktop path
- keep the current desktop UI look and layout: topbar, buttons, theme toggle, font size control, transcript styling, tool activity rows, reasoning panel, model picker, and workdir picker flow
- keep the runtime core, tools, provider, sessions, compaction, prompts, and config logic in Go
- fail fast on missing backend, bad config, broken IPC, or invalid desktop state

## Current State

- Active transports: console and Telegram in the root Go module
- Archived desktop reference: `internal/desktop_old/`
- Archived desktop UI source: `internal/desktop_old/desktop.go`
- Archived desktop state/config logic: `internal/desktop_old/state.go`, `internal/desktop_old/config.go`
- Archived desktop vendor assets: `internal/desktop_old/assets/vendor/`
- Active runtime boundary: `internal/runtime/runtime.go` via `runtime.Handler`

## Recommended Target Architecture

### 1. Split responsibility

- Go backend owns agent execution, sessions, tools, config, prompt build, compaction, and provider streaming.
- Electron owns windowing, renderer UI, native dialogs, packaging, and desktop process lifecycle.

### 2. New components

- `cmd/blazeai-desktop-backend/`
  - dedicated Go desktop backend binary
  - no import of `internal/desktop_old`
  - no WebKitGTK, no WebView, no GTK tray code
- `desktop_electron/`
  - Electron app root
  - `main` process for app lifecycle, dialog, backend spawn
  - `preload` bridge for safe renderer API
  - `renderer` app that reproduces the archived desktop UI

### 3. Communication model

- transport protocol: stdio JSON lines or JSON-RPC over stdin/stdout
- no localhost HTTP server
- stdout reserved for protocol only
- stderr reserved for logs/errors only
- backend exit is fatal to the Electron app

## Non-Goals

- no full runtime rewrite in Node.js
- no change to console or Telegram behavior
- no silent fallback to console if Electron/backend fails
- no redesign of the desktop UI in phase 1

## Source Reference Mapping

These archived pieces should be ported almost 1:1:

- `internal/desktop_old/desktop.go`
  - HTML template
  - CSS theme variables
  - renderer JavaScript behavior
  - transcript row structure
  - model/theme/font/workdir controls
- `internal/desktop_old/assets/vendor/`
  - `marked.min.js`
  - `highlight.min.js`
  - `dompurify.min.js`
  - highlight CSS themes
- `internal/desktop_old/state.go`
  - selected model, theme, font size, window geometry model
- `internal/desktop_old/config.go`
  - desktop config contract: `workdir`, reasoning max height
- `internal/desktop_old/handler.go`
  - streaming semantics for content, reasoning, tool activity, usage, and busy state

## Target File Layout

Recommended structure:

```text
cmd/
  blazeai-desktop-backend/
    main.go

internal/
  desktopbackend/
    server.go
    protocol.go
    session.go
    state.go
    handler.go
    commands.go

desktop_electron/
  package.json
  electron-builder.yml
  src/
    main/
      main.ts
      backend.ts
      dialogs.ts
    preload/
      index.ts
    renderer/
      index.html
      index.ts
      styles.css
      vendor/
```

Notes:
- `internal/desktop_old/` remains untouched as archive/reference.
- `internal/desktopbackend/` contains only active Go desktop backend logic.

## Protocol Plan

## Request/Response shape

```json
{"id":"1","method":"get_state","params":{}}
{"id":"1","ok":true,"result":{}}
{"id":"2","ok":false,"error":"message"}
```

## Initial backend methods

- `get_state`
- `send_message`
- `change_model`
- `clear_session`
- `close_session`
- `set_theme`
- `set_font_size`
- `set_workdir`
- `pick_last_session_mode`
- `quit`

## Optional async events

- `state_changed`
- `fatal_error`
- `backend_ready`

Phase 1 can work with polling plus request/response if that is simpler. Phase 2 should move to push events for streaming responsiveness.

## State Ownership

### Go backend owns

- session lifecycle
- `runtime.Agent`
- transcript block state
- tool activity formatting
- busy/status flags
- model list resolution
- desktop-local config/state persistence rules

### Electron owns

- window creation and geometry application
- native directory picker
- native confirm dialogs
- renderer DOM updates

### Shared serialized state

Use a backend payload equivalent to archived `stateResponse`:

```json
{
  "blocks": [],
  "status": "Ready",
  "busy": false,
  "model": "provider/model",
  "models": ["provider/model"],
  "workdir": "/abs/path",
  "theme": "dark",
  "font_size": 13.5,
  "reasoning_max_height": 100
}
```

## Migration Phases

## Phase 1: Extract a Go Desktop Backend

Goal: create a desktop backend with no GUI dependency.

Steps:
- create `cmd/blazeai-desktop-backend/main.go`
- create `internal/desktopbackend/`
- move reusable logic from `internal/desktop_old/` into active backend code without importing WebView-specific types
- replace `view.Bind(...)` methods with backend RPC handlers
- preserve transcript, tool activity, slash command, session, and state behavior

Likely changed files:
- new `cmd/blazeai-desktop-backend/main.go`
- new `internal/desktopbackend/*.go`
- possibly `internal/runtime/runtime.go` only if a small helper is needed, but avoid runtime changes if possible

Validation:
- `go build ./...`
- `go test ./...`
- manual backend boot with invalid config should hard-fail
- manual backend boot with valid config should return `backend_ready`

## Phase 2: Build the Electron Shell

Goal: start the backend as a subprocess and render the existing UI in Electron.

Steps:
- create `desktop_electron/package.json`
- add Electron main/preload/renderer structure
- spawn backend from Electron main process
- implement stdio protocol client
- port archived HTML/CSS/JS into renderer files
- wire renderer actions to preload API instead of WebView `Bind`

UI porting rules:
- keep CSS variables, spacing, colors, scrollbar styling, and typography from `desktop_old`
- keep current transcript row types and Markdown rendering behavior
- keep current controls and labels unless a technical blocker requires a change

Validation:
- `npm install`
- `npm run build` or equivalent
- Electron launches backend and renders initial state
- killing backend causes visible fatal error and app shutdown

## Phase 3: Streaming and UX Hardening

Goal: match or improve the archived desktop responsiveness.

Steps:
- switch from polling-only to backend push events if needed
- debounce or batch transcript re-renders
- ensure reasoning panel autoscroll behavior matches the archived UI
- ensure assistant/tool streaming does not freeze the renderer

Validation:
- long streamed replies stay responsive
- tool activity updates incrementally
- reasoning block behavior matches archived semantics

## Phase 4: Packaging and Cutover

Goal: ship Electron as the desktop app and keep the root Go module clean.

Steps:
- add Electron packaging config for Linux first
- document backend binary discovery rules
- define release artifact layout
- update root docs/specs to describe Electron desktop as the active desktop transport
- keep `internal/desktop_old/` archived until Electron stabilizes

Validation:
- packaged app starts on a clean Linux machine with valid config
- missing backend binary fails with explicit error
- no WebKitGTK dependency remains in the desktop shipping path

## Implementation Details To Preserve

These behaviors should be preserved unless explicitly dropped later:

- singleton desktop session model
- local desktop slash commands
- theme persistence
- font size persistence
- workdir persistence
- reasoning max height setting
- model picker behavior
- transcript blocks with typed rows: user, assistant, system, tool, reasoning
- compact tool activity block behavior

## Risks

- IPC protocol drift between Electron and Go backend
- stdout contamination if backend writes logs to stdout
- accidental reuse of archived desktop code that still imports WebView types
- packaging complexity for backend binary distribution
- renderer regressions if the HTML/CSS port is rewritten instead of copied incrementally

## Open Questions

- Should the desktop backend reuse `app_home/desktop/` paths exactly, or move to a new `app_home/electron/` namespace?
- Should phase 1 preserve the old singleton fixed desktop session, or switch immediately to project-scoped desktop sessions?
- Is sudo approval still unsupported on desktop in phase 1, or should Electron add a secure approval dialog before release?

## Recommended Execution Order

1. build `internal/desktopbackend/` with no GUI code
2. build Electron shell with static renderer port from `desktop_old`
3. harden streaming and packaging
4. update docs/specs and promote Electron to active desktop transport

## Definition Of Done

- Electron desktop app launches successfully on Linux without WebKitGTK
- archived UI is preserved closely enough that the current topbar, controls, fonts, colors, and transcript behavior are recognizable and functionally equivalent
- root Go module builds and tests cleanly without GUI dependencies
- desktop backend and Electron app fail fast on configuration, protocol, and process errors
- `internal/desktop_old/` remains available as reference until the Electron path is stable
