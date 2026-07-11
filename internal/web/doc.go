// doc.go — minimal web transport that mirrors the console output in a browser.
// Serves a single terminal-like HTML page, streams transcript blocks via SSE,
// and provides dropdown controls for mode and model selection.
// Layer: transport. Dependencies: internal/runtime, internal/config.
package web
