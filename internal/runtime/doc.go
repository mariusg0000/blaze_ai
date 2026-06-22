// runtime/doc.go — agent core orchestration loop.
// Drives the LLM call cycle: build prompt, send to provider, handle tool calls, persist messages,
// and trigger compaction when thresholds are met. Communicates with transports only via the Handler contract.
// Layer: agent core. Dependencies: internal/config, internal/provider, internal/prompt, internal/session,
// internal/tools, internal/skills.
package runtime
