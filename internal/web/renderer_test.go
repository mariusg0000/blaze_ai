// renderer_test.go — tests for HTML output rendering.
// Purpose: Verify agent tool line rendering with CTX token display.
// Layer: transport output. Direct dependency: none (pure string transformation).
package web

import (
	"strings"
	"testing"
)

// TestAgentToolLineHTMLWithCTX verifies that agentToolLineHTML renders CTX
// when ctxTokens > 0, consistent with main-runtime tool result formatting.
// WHAT: CTX span must appear with compact token count after the badge.
// HOW: Passes non-zero ctxTokens and checks HTML output for CTX content.
func TestAgentToolLineHTMLWithCTX(t *testing.T) {
	// formatCompactInt(12500) = "12k" (>=10000 range uses integer formatting).
	html := agentToolLineHTML("[code_abc]", "shell", "", "✔️", 12500)
	if !strings.Contains(html, "CTX: 12k") {
		t.Errorf("agentToolLineHTML missing CTX: %q", html)
	}
	if !strings.Contains(html, "✔️") {
		t.Errorf("agentToolLineHTML missing badge: %q", html)
	}
	// The agent prefix wraps the input in Agent [...], so the literal name
	// "[code_abc]" renders as "Agent [[code_abc]]".
	if !strings.Contains(html, "Agent [[code_abc]]") {
		t.Errorf("agentToolLineHTML missing agent prefix: %q", html)
	}
}

// TestAgentToolLineHTMLWithoutCTX verifies that agentToolLineHTML omits CTX
// when ctxTokens is zero, avoiding noise on tool_call or missing-data lines.
// WHAT: No CTX span when token count is zero.
// HOW: Passes zero ctxTokens and checks HTML output lacks CTX.
func TestAgentToolLineHTMLWithoutCTX(t *testing.T) {
	html := agentToolLineHTML("[code_abc]", "shell", "ls -la", "", 0)
	if strings.Contains(html, "CTX:") {
		t.Errorf("agentToolLineHTML should not contain CTX when tokens=0: %q", html)
	}
	if !strings.Contains(html, "ls -la") {
		t.Errorf("agentToolLineHTML missing args: %q", html)
	}
}

// TestAgentToolLineHTMLAlwaysRendersCTXWhenTokensSet verifies that the renderer
// is badge-agnostic: CTX appears whenever ctxTokens > 0 regardless of badge.
// WHAT: Renderer always renders CTX when tokens are provided.
// HOW: Passes non-zero tokens with ERROR badge and confirms CTX presence.
// NOTE: The handler decides when to pass tokens (DONE only); the renderer
// faithfully renders what it receives.
func TestAgentToolLineHTMLAlwaysRendersCTXWhenTokensSet(t *testing.T) {
	html := agentToolLineHTML("[code_abc]", "shell", "", "✖️", 10000)
	if !strings.Contains(html, "CTX: 10k") {
		t.Errorf("agentToolLineHTML should contain CTX when tokens > 0: %q", html)
	}
}
