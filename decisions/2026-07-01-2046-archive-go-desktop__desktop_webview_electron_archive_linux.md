## Feature Description
Removes the Go WebView desktop transport from the active BlazeAI build and archives its code under `internal/desktop_old/`.
Keeps the old desktop UI, assets, and state logic available as reference for the planned Electron replacement.

## Rationale And Implementation
The Linux desktop transport depended on `webview_go` and WebKitGTK, which made the `JSC_SIGNAL_FOR_GC` signal conflict a deployment risk across systems. The implementation moved the desktop package and vendor assets into an archived module, removed the active desktop startup path and desktop-specific dependencies from the main Go module, and updated specs to describe the archived status.

## Modified Files
- `main.go`: removes the active desktop transport import and makes console the default active transport.
- `go.mod`: drops WebView, tray, and hotkey dependencies from the main module.
- `go.sum`: removes checksums for the dropped desktop-only dependencies.
- `internal/desktop_old/README.md`: marks the archived desktop code as reference-only.
- `internal/desktop_old/go.mod`: isolates the archived desktop code from the active module build.
- `internal/desktop_old/desktop.go`: preserves the old HTML/CSS/JS desktop UI for migration reference.
- `internal/desktop_old/assets/vendor/`: preserves the old desktop vendor assets for migration reference.
- `specs.md`: updates the project overview to describe the archived desktop transport.
- `specs/01-product-scope.md`: removes the desktop transport from the active product scope and notes the archive.
- `specs/02-architecture.md`: updates startup and package architecture to reflect the desktop archive.
- `specs/12-handler-contract.md`: removes desktop from the active Handler implementation list.
- `specs/17-platform.md`: removes the active desktop app-home folder from platform docs.
