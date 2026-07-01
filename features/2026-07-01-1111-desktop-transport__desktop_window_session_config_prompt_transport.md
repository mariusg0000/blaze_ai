## Feature Description
Adds a singleton desktop transport that opens BlazeAI in one dedicated embedded window and always reuses one fixed desktop session.
Startup now fails fast when desktop-local config or state is missing or invalid.

## Rationale And Implementation
The desktop companion had to behave like Telegram's single persistent chat, but as a real desktop app window instead of a browser tab. The implementation adds a new `-desktop` entrypoint, strict `app_home/desktop/config.json` and `state.json` loading, a fixed `app_home/desktop/session/` folder, local desktop commands, and an embedded `webview` UI bound directly to the runtime handler.

## Modified Files
- `main.go`: adds the `-desktop` startup flag and routes execution into the desktop transport.
- `internal/desktop/config.go`: loads and validates strict desktop-local config.
- `internal/desktop/state.go`: loads, validates, and saves strict desktop-local state.
- `internal/desktop/commands.go`: handles local desktop commands for model changes and fixed-session reset behavior.
- `internal/desktop/handler.go`: adapts runtime streaming callbacks to the desktop transcript.
- `internal/desktop/desktop.go`: owns singleton session startup, embedded window UI, and runtime integration.
- `internal/desktop/config_test.go`: verifies strict config and state validation.
- `internal/desktop/desktop_test.go`: verifies fixed desktop session creation and resume behavior.
- `prompts/transport.desktop.md`: defines desktop-specific prompt guidance.
- `specs.md`: updates the project map and runtime description for the new transport.
