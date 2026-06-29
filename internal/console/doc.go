// console/doc.go — terminal REPL transport implementing the handler contract.
// Implements OnContent, OnToolCall, OnToolResult. Always uses ANSI colors and raw-mode
// input for Tab mode cycling. Handles slash commands (/exit, /model, /cd).
// Renders Markdown incrementally during streaming. Terminal-only.
// Layer: transport (console). Dependencies: internal/runtime.
package console
