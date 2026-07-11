# Session Decision Summary: Remove Desktop, Prepare for Web Transport

Date: 2026-07-11 14:54

## Context

The desktop app existed in three incomplete layers: an archived Go WebView transport (`internal/desktop_old/`), an Electron backend (`internal/desktopbackend/`), and an Electron frontend (`desktop_electron/`). None were in active use — the Go WebView was Linux-only and CGo-heavy, the Electron backend was a subprocess protocol bridge, and the Electron frontend was stalled.

The user decided to replace the desktop app with a minimal web transport that mirrors the console output in a browser — terminal-like rows, SSE streaming, no chat bubbles.

## Changes Made

- **Deleted `internal/desktop_old/`** (23 files, ~370 KB): archived Go WebView transport with GTK3, tray, hotkey, and vendor JS assets
- **Deleted `internal/desktopbackend/`** (8 files): Electron desktop backend with stdio protocol server
- **Deleted `cmd/blazeai-desktop-backend/`** (12 files): backend entrypoint binary with embedded prompts and skills
- **Deleted `desktop_electron/`** (13 files): Electron frontend (renderer, preload, main, package.json, vendor JS/CSS)
- **Deleted `prompts/transport.desktop.md`**: desktop-specific transport prompt injected into runtime context
- **Deleted `plans/electron_migration.md`**: stale migration plan
- **Modified `specs.md`**: removed all desktop references (archived, Electron, webview), added `internal/web/` placeholder
- **Modified `.gitignore`**: removed `desktop_electron/node_modules/`, added `/blazeai-desktop-backend` binary ignore

Total: ~60 files removed, 2 files modified.

## Decisions And Rationale

- **Complete removal, not archive.** The `desktop_old/` was already archived with its own go.mod. The Electron stack was half-built and no longer the direction. Keeping either would create confusion and prompt bloat.
- **Deleted `transport.desktop.md` transport prompt.** The prompt builder loads these by name; a stale `transport.desktop.md` would never be used (no transport named "desktop" exists), but its presence in the embed would be misleading.
- **Updated `.gitignore`.** Removed the `desktop_electron/node_modules/` ignore since the entire `desktop_electron/` directory is gone. Added `/blazeai-desktop-backend` for the deleted binary's artifact.
- **Build and test pass clean.** `go build ./...` and `go test ./...` — all 15 packages pass. No code outside the deleted packages imported them.
