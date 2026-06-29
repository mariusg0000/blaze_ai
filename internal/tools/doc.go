// tools/doc.go — native tool implementations for the agent runtime.
// Implements shell, skill state tools, ask_a_friend, and block/file editing with
// OpenAI tool-calling format, multi-call per turn support,
// and per-call timeout (default 60s, returns "timeout <N>s exceeded").
// Layer: tool execution. Dependencies: internal/skills, internal/platform.
package tools
