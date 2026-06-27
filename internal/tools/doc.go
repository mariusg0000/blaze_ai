// tools/doc.go — native tool implementations for the agent runtime.
// Implements shell, load_skill, unload_skill, run_skill, and replace_block with OpenAI
// tool-calling format, multi-call per turn support, and per-call timeout (default 60s,
// returns "timeout <N>s exceeded").
// Layer: tool execution. Dependencies: internal/skills, internal/platform.
package tools
