// doc.go — one-shot secondary LLM calls routed by configured role.
// Provides a narrow helper for role-based expert consultation without tool recursion
// or hidden session persistence. Layer: secondary LLM calling. Dependencies:
// internal/config, internal/provider, internal/session, internal/tools.
package llmcall
