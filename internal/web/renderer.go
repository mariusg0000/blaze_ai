// renderer.go — HTML output rendering mirroring the console ANSI renderer.
// Produces inline HTML with CSS class names matching console color semantics:
// orange, blue, green, purple, red, ctx (cyan), bright-blue, bright-green, reasoning.
// Layer: transport output. Dependencies: none (pure string transformation).
package web

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// reLink matches Markdown links [text](url).
var reLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// reUnderscoreItalic matches _text_ at word boundaries.
var reUnderscoreItalic = regexp.MustCompile(`(?:^|\s)_([^_]+)_(?:\s|$)`)

// toolEmoji returns a tool-specific emoji for display in the web UI.
func toolEmoji(name string) string {
	switch name {
	case "shell":
		return "💻"
	case "task_write":
		return "📋"
	case "task_read":
		return "📖"
	case "load_skill":
		return "📥"
	case "unload_skill":
		return "📤"
	case "replace_block":
		return "📝"
	case "run_skill":
		return "🚀"
	case "ask_a_friend":
		return "🤝"
	case "analyze_image":
		return "🖼"
	case "compaction":
		return "🗜️"
	case "read_file":
		return "📄"
	case "write_file":
		return "💾"
	default:
		return "🔧"
	}
}

// escapeHTML escapes &, <, >, ", ' for safe HTML embedding.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// formatCompactInt returns a shorter human-readable number such as 12.3k.
func formatCompactInt(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	if n < 10000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%dk", n/1000)
}

// truncatePathTail returns the last maxLen characters of an absolute path.
func truncatePathTail(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// renderLineHTML renders one Markdown line as HTML with CSS class names.
// Mirrors the console ANSI renderLine function.
func renderLineHTML(line string, inCodeBlock *bool) string {
	trimmed := strings.TrimSpace(line)

	// Code fence toggle.
	if strings.HasPrefix(trimmed, "```") {
		*inCodeBlock = !*inCodeBlock
		return ""
	}

	if *inCodeBlock {
		return `<span class="code">    ` + escapeHTML(line) + `</span><br>`
	}

	if line == "" {
		return `<br>`
	}

	// Table separator line — skip.
	if isTableSeparator(line) {
		return ""
	}

	// Table data row.
	if isTableRow(line) {
		cells := splitTableRow(line)
		rendered := make([]string, len(cells))
		for i, cell := range cells {
			rendered[i] = renderInlineHTML(cell)
		}
		return `  ` + strings.Join(rendered, `  -  `) + `<br>`
	}

	// Headings.
	if level, title, ok := parseHeading(line); ok {
		rendered := renderInlineHTML(title)
		switch {
		case level == 1:
			return `<span class="blue bold upper">` + rendered + `</span><br>`
		case level == 2:
			return `<span class="blue bold">` + rendered + `</span><br>`
		default:
			return `<span class="blue">` + rendered + `</span><br>`
		}
	}

	// Bullet list.
	if item, ok := parseBullet(line); ok {
		return `  - ` + renderInlineHTML(item) + `<br>`
	}

	// Numbered list.
	if prefix, item, ok := parseNumbered(line); ok {
		return `  ` + prefix + ` ` + renderInlineHTML(item) + `<br>`
	}

	return renderInlineHTML(line) + `<br>`
}

// renderInlineHTML applies inline Markdown formatting and produces HTML with CSS classes.
func renderInlineHTML(text string) string {
	// Escape HTML first; Markdown markers (*, _, `, [) survive.
	text = escapeHTML(text)

	// Bold **text**
	text = toggleDelimitedHTML(text, "**", "strong")

	// Italic _text_ (word boundaries)
	text = reUnderscoreItalic.ReplaceAllStringFunc(text, func(match string) string {
		inner := match[1 : len(match)-1]
		return `<em>` + inner + `</em>`
	})

	// Italic *text*
	text = toggleDelimitedHTML(text, "*", "em")

	// Inline code `text`
	text = toggleDelimitedHTML(text, "`", "code")

	// Links [text](url) → text (url) with styled URL.
	text = reLink.ReplaceAllStringFunc(text, func(match string) string {
		parts := reLink.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		return parts[1] + ` <span class="purple">(` + parts[2] + `)</span>`
	})

	return text
}

// toggleDelimitedHTML replaces paired delimiters with HTML tags on already-escaped text.
func toggleDelimitedHTML(text, delim, tag string) string {
	var b strings.Builder
	for {
		start := strings.Index(text, delim)
		if start < 0 {
			b.WriteString(text)
			return b.String()
		}
		b.WriteString(text[:start])
		text = text[start+len(delim):]
		end := strings.Index(text, delim)
		if end < 0 {
			b.WriteString(delim)
			b.WriteString(text)
			return b.String()
		}
		b.WriteString(`<` + tag + `>`)
		b.WriteString(text[:end])
		b.WriteString(`</` + tag + `>`)
		text = text[end+len(delim):]
	}
}

// parseHeading extracts ATX headings (#, ##, ###...) from a line.
func parseHeading(line string) (int, string, bool) {
	trimmedLeft := strings.TrimLeft(line, " ")
	level := 0
	for level < len(trimmedLeft) && trimmedLeft[level] == '#' {
		level++
	}
	if level == 0 || level >= len(trimmedLeft) || trimmedLeft[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(trimmedLeft[level+1:]), true
}

// parseBullet extracts unordered list items from a line.
func parseBullet(line string) (string, bool) {
	trimmedLeft := strings.TrimLeft(line, " ")
	if strings.HasPrefix(trimmedLeft, "- ") || strings.HasPrefix(trimmedLeft, "* ") {
		return strings.TrimSpace(trimmedLeft[2:]), true
	}
	return "", false
}

// parseNumbered extracts numbered list items from a line.
func parseNumbered(line string) (string, string, bool) {
	trimmedLeft := strings.TrimLeft(line, " ")
	idx := 0
	for idx < len(trimmedLeft) && trimmedLeft[idx] >= '0' && trimmedLeft[idx] <= '9' {
		idx++
	}
	if idx == 0 || idx+1 >= len(trimmedLeft) || trimmedLeft[idx] != '.' || trimmedLeft[idx+1] != ' ' {
		return "", "", false
	}
	return trimmedLeft[:idx+1], strings.TrimSpace(trimmedLeft[idx+2:]), true
}

// isTableSeparator detects Markdown table separator lines like |---|---|.
func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return false
	}
	stripped := strings.ReplaceAll(trimmed, "|", "")
	stripped = strings.ReplaceAll(stripped, "-", "")
	stripped = strings.ReplaceAll(stripped, " ", "")
	return stripped == ""
}

// isTableRow detects Markdown table data lines starting and ending with |.
func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") && !isTableSeparator(line)
}

// splitTableRow extracts cell texts from a | a | b | c | table row.
func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cell := strings.TrimSpace(p)
		if cell != "" {
			cells = append(cells, cell)
		}
	}
	return cells
}

// parseToolResult extracts a badge and content from raw tool output.
func parseToolResult(result string) (badge, content, colorClass string) {
	result = strings.TrimSpace(result)

	if strings.HasPrefix(result, "timeout") {
		return "TIMEOUT", strings.TrimSpace(strings.TrimPrefix(result, "timeout")), "orange"
	}

	if strings.HasPrefix(result, "error:") {
		msg := strings.TrimSpace(strings.TrimPrefix(result, "error:"))
		if idx := strings.Index(msg, "\n"); idx >= 0 {
			msg = strings.TrimSpace(msg[:idx])
		}
		return "ERROR", msg, "red"
	}

	if strings.HasPrefix(result, "exit_code:") {
		rest := strings.TrimSpace(strings.TrimPrefix(result, "exit_code:"))
		newlineIdx := strings.Index(rest, "\n")
		if newlineIdx < 0 {
			return "DONE", "", "bright-green"
		}
		exitCodeStr := strings.TrimSpace(rest[:newlineIdx])
		exitCode := 0
		fmt.Sscanf(exitCodeStr, "%d", &exitCode)
		if exitCode != 0 {
			return "ERROR", "exit code " + exitCodeStr, "red"
		}
		return "DONE", "", "bright-green"
	}

	return "DONE", "", "bright-green"
}

// agentToolLineHTML renders a child tool line with Agent identity and main tool styling.
func agentToolLineHTML(agent, name, args, badge string) string {
	return `<span class="orange">Agent [` + escapeHTML(agent) + `]</span> ` + toolLineHTML(name, args, badge)
}

// toolLineHTML renders a single tool activity line with emoji, args, and badge.
func toolLineHTML(name, args, badge string) string {
	emoji := toolEmoji(name)
	parts := []string{`<span class="green">` + emoji + `</span>`}
	if args != "" {
		parts = append(parts, `<span class="ctx">`+escapeHTML(args)+`</span>`)
	}
	switch badge {
	case "✔️":
		parts = append(parts, `<span class="bright-green">✔️</span>`)
	case "✖️":
		parts = append(parts, `<span class="red">✖️</span>`)
	case "⏱":
		parts = append(parts, `<span class="orange">⏱</span>`)
	}
	return strings.Join(parts, " ")
}

// separatorHTML builds the boxed table separator line shown after a response.
func separatorHTML(ctxTokens int, model, workDir string) string {
	if ctxTokens <= 0 {
		return ""
	}
	ctxText := "CTX: " + formatCompactInt(ctxTokens)
	wd := truncatePathTail(workDir, 30)

	cell1 := " " + ctxText + " "
	cell2 := " " + model + " "
	cell3 := " " + wd + " "

	w1 := len(cell1)
	w2 := len(cell2)
	w3 := len(cell3)

	char := "─"

	v := `<span class="bright-blue">` + "│" + `</span>`
	c1 := `<span class="orange bold">` + cell1 + `</span>`
	c2 := `<span class="orange bold">` + cell2 + `</span>`
	c3 := `<span class="orange bold">` + cell3 + `</span>`

	top := `<span class="bright-blue">` + "┌" + strings.Repeat(char, w1) + "┬" + strings.Repeat(char, w2) + "┬" + strings.Repeat(char, w3) + "┐" + `</span><br>`
	mid := v + c1 + v + c2 + v + c3 + v + `<br>`
	bot := `<span class="bright-blue">` + "└" + strings.Repeat(char, w1) + "┴" + strings.Repeat(char, w2) + "┴" + strings.Repeat(char, w3) + "┘" + `</span>`
	return top + mid + bot
}

// assistantContentHTML renders the full assistant reply from accumulated lines.
func assistantContentHTML(content string) string {
	var b strings.Builder
	inCodeBlock := false
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		b.WriteString(renderLineHTML(line, &inCodeBlock))
	}
	// Close unclosed code block.
	if inCodeBlock {
		b.WriteString(`</span>`)
	}
	return b.String()
}
