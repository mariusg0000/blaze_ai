## Feature Description
Added `reasoning_max_height` config entry (default 150) that limits reasoning block display height in the desktop Electron renderer with custom thin scrollbar and autoscroll-to-bottom, and truncates reasoning output in the console terminal transport.

## Rationale And Implementation
The old Go WebView desktop had `reasoning_max_height` as CSS max-height with `overflow-y: auto` and `content.scrollTop = content.scrollHeight` autoscroll in the embedded JS. These were lost during the desktop_old → desktopbackend + Electron migration. The user requested restoring both the config entry and the autoscroll behavior. The main config struct now carries the value; the console uses it to truncate reasoning lines; the desktopbackend forwards it to Electron; the Electron renderer applies it as CSS with thin integrated scrollbar and autoscrolls reasoning content.

## Modified Files
- internal/config/config.go: added ReasoningMaxHeight int field with default 150 and validation
- internal/console/console.go: added reasoningLines counter and line-truncation logic in OnReasoning
- internal/desktopbackend/config.go: changed defaultReasoningMaxHeightPx from 100 to 150
- desktop_electron/src/renderer/styles.css: added thin custom scrollbar styling for .row-reasoning .content
- desktop_electron/src/renderer/app.js: added per-reasoning-block autoscroll after renderBlocks
