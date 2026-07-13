package console

import (
	"strings"
	"testing"
)

// TestToolGroupVisual verifies the visual output of content→tool group→content flow.
// Runs three consecutive tool calls between content blocks and checks blank line placement.
func TestToolGroupVisual(t *testing.T) {
	c, out := newConsole(mockAgent(t))
	c.OnUsage(11186, 0, 11186)

	// First content block.
	c.OnContent("Sure, let me read that file.\n")

	// Tool group: three read_file calls, all succeed.
	c.OnToolCall("read_file", "Read /tmp/a.md for inspection")
	c.OnToolResult("read_file", "exit_code: 0\nstdout:\na\n")
	c.OnToolCall("read_file", "Read /tmp/b.md for cross-reference")
	c.OnToolResult("read_file", "exit_code: 0\nstdout:\nb\n")
	c.OnToolCall("read_file", "Read /tmp/c.md for context")
	c.OnToolResult("read_file", "exit_code: 0\nstdout:\nc\n")

	// Content resumes after tool group.
	c.OnContent("Here is what I found.\n")

	plain := stripANSICodes(out.String())

	t.Logf("=== FULL OUTPUT ===\n%s=== END ===", plain)

	// Each line we expect, in order.
	lines := strings.Split(plain, "\n")

	// Build index of key lines.
	findLine := func(sub string) int {
		for i, l := range lines {
			if strings.Contains(l, sub) {
				return i
			}
		}
		return -1
	}

	bzIdx := findLine("[BLAZE]")
	contentIdx := findLine("Sure, let me read that file.")
	tool1Idx := findLine("/tmp/a.md")
	tool2Idx := findLine("/tmp/b.md")
	tool3Idx := findLine("/tmp/c.md")
	bz2Idx := findLine("Here is what I found.")

	if bzIdx < 0 {
		t.Fatal("missing first [BLAZE]")
	}
	if contentIdx < 0 {
		t.Fatal("missing content line")
	}
	if tool1Idx < 0 || tool2Idx < 0 || tool3Idx < 0 {
		t.Fatal("missing one or more tool lines")
	}
	if bz2Idx < 0 {
		t.Fatal("missing second [BLAZE]")
	}

	// Order check.
	if !(bzIdx < contentIdx && contentIdx < tool1Idx && tool1Idx < tool2Idx && tool2Idx < tool3Idx && tool3Idx < bz2Idx) {
		t.Fatalf("order wrong: [BLAZE]=%d content=%d tool1=%d tool2=%d tool3=%d resume=%d",
			bzIdx, contentIdx, tool1Idx, tool2Idx, tool3Idx, bz2Idx)
	}

	// Blank line after content, before first tool.
	if tool1Idx != contentIdx+2 {
		t.Errorf("expected blank line between content and tool1 (got gap of %d lines, want 2)", tool1Idx-contentIdx)
	}

	// No blank line between consecutive tools.
	gap12 := tool2Idx - tool1Idx
	if gap12 != 1 {
		t.Errorf("expected no blank line between tool1 and tool2 (got gap %d, want 1)", gap12)
	}
	gap23 := tool3Idx - tool2Idx
	if gap23 != 1 {
		t.Errorf("expected no blank line between tool2 and tool3 (got gap %d, want 1)", gap23)
	}

	// Blank line after tool group: tool3 → blank → [BLAZE] → Here.
	// So the blank line is at tool3Idx+1 and [BLAZE] content resumes at tool3Idx+3.
	// Find second [BLAZE]; the first one at bzIdx is the initial content block.
	bz2LineIdx := -1
	for i := bzIdx + 1; i < len(lines); i++ {
		if strings.Contains(lines[i], "[BLAZE]") {
			bz2LineIdx = i
			break
		}
	}
	if bz2LineIdx < 0 {
		t.Fatal("missing second [BLAZE] after tool group")
	}
	if bz2LineIdx != tool3Idx+2 {
		t.Errorf("expected [BLAZE] at tool3+2 (tool3=%d, blank=%d, [BLAZE]=%d, got gap %d, want 2)",
			tool3Idx, tool3Idx+1, bz2LineIdx, bz2LineIdx-tool3Idx)
	}
}
