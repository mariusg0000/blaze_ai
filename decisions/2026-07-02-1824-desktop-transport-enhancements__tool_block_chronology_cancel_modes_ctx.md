## Feature Description
Desktop transport enhancements: streaming tool/reasoning blocks now render chronologically, send button doubles as stop/cancel during LLM processing, mode and model selectors added to footer, and context usage shown as a graphical bar.

## Rationale And Implementation
User reported that tool blocks appeared above reasoning blocks even when reasoning came first chronologically, and new tool calls kept updating old blocks at the top. Root cause: handler used per-turn booleans (`assistantStarted`, `reasoningStarted`) that did not detect segment switches (tool→reasoning→tool), so the tool activity accumulator was never reset on type change. Fix: replaced booleans with an `activeSegment` field that tracks the current streamed segment type and resets the tool accumulator on every switch. This ensures each tool/reasoning/assistant segment creates a new block at the bottom when the type changes, while consecutive same-type events still accumulate in the same block.

## Modified Files
- internal/desktopbackend/handler.go: replaced per-turn booleans with activeSegment tracking; OnToolCall, OnToolResult, OnReasoning, and OnContent all check activeSegment and reset tool accumulator on type switch
- internal/desktopbackend/service.go: added cancel/mode/quit-while-busy support, AppendReasoning resets activeTool on new block creation, replay transcript resets tool accumulator after reasoning
- internal/desktopbackend/config.go: removed hotkey configuration
- internal/desktopbackend/hotkey.go: deleted
- internal/desktopbackend/protocol.go: added ChangeModeParams and next_mode/cancel RPCs
- desktop_electron/src/main/main.js: added desktop:cancel handler
- desktop_electron/src/preload/preload.js: exposed cancel() via IPC bridge
- desktop_electron/src/renderer/app.js: stop-overlay toggle, mode selector combo, ctx bar width, ApplySource resets on segment switch
- desktop_electron/src/renderer/index.html: status line split (mode/model/status left, ctx right), stop button, mode/model selects in footer
- desktop_electron/src/renderer/styles.css: .stop-overlay, .ctx-track/fill, .status-select, mode/model styling
- plans/electron_migration.md: removed hotkey from migration plan
