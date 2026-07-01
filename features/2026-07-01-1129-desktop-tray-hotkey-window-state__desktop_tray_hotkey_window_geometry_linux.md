## Feature Description
Desktop transport now hides to tray on close, restores from tray or one explicit global hotkey, and persists its window bounds across runs.
These native desktop extras fail fast outside Linux instead of silently degrading.

## Rationale And Implementation
The user asked for all three desktop upgrades in sequence: tray behavior, a global show/hide shortcut, and persisted window icon, size, and position. The implementation adds strict optional hotkey config in `app_home/desktop/config.json`, stores window geometry in `app_home/desktop/state.json`, wires the embedded `webview` window into GTK callbacks for hide/show and geometry tracking, and uses tray and hotkey libraries only where the Linux desktop transport can support them natively.

## Modified Files
- `internal/desktop/desktop.go`: restores saved bounds, flushes desktop-local state, and coordinates quit and geometry persistence.
- `internal/desktop/platform_linux.go`: implements GTK window hooks, tray integration, close-to-tray behavior, and Linux native show/hide handling.
- `internal/desktop/platform_other.go`: fails explicitly on non-Linux instead of pretending the native extras exist.
- `internal/desktop/hotkey_linux.go`: validates and parses explicit Linux desktop hotkey config.
- `internal/desktop/hotkey_other.go`: rejects unsupported hotkey config on non-Linux platforms.
- `internal/desktop/state.go`: persists window bounds together with the desktop-local selected model.
- `internal/desktop/config.go`: adds strict optional hotkey config validation.
- `internal/desktop/icon.go`: generates the desktop window and tray icon in-process.
- `internal/desktop/commands.go`: updates model persistence through the synchronized desktop state API.
- `internal/desktop/config_test.go`: covers hotkey validation, normalized shortcuts, and window geometry validation.
- `go.mod`: adds tray and hotkey dependencies required by the Linux desktop transport.
- `go.sum`: records the new module checksums.
- `specs.md`: documents tray integration, optional hotkey support, and persisted desktop geometry.
