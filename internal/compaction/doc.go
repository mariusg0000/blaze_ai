// compaction/doc.go — context compaction for long sessions.
// Triggers on provider-reported prompt_tokens reaching maxContextTokens, prunes old messages
// with tool-boundary safety, summarizes pruned segments, stores summary chunks, and strips
// reasoning parts from the LLM payload while keeping session JSON intact on disk.
// Layer: context management. Dependencies: internal/session, internal/provider, internal/prompt.
package compaction
