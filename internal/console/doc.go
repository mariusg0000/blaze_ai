// console/doc.go — console REPL transport implementing the handler contract.
// Implements OnContent, OnToolCall, OnToolResult. Auto-detects TTY for colors, spinner, and
// visual separators. Handles slash commands (/exit, /model, /cd) before reaching the agent core.
// Renders Markdown incrementally during streaming. Non-TTY output is plain text.
// Layer: transport (console). Dependencies: internal/runtime.
package console
