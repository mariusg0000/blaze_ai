// platform/doc.go — OS detection, shell selection, and platform-aware path handling.
// Detects linux, darwin, or windows at runtime. Selects the appropriate shell chain
// (bash->sh on Linux/macOS, pwsh->powershell->cmd on Windows). Provides platform-aware
// quoting, path separators, and environment variable syntax helpers.
// Layer: platform abstraction. Dependencies: none (standard library only).
package platform
