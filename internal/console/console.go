// console.go — console REPL transport implementing the handler contract.
// Implements OnContent, OnToolCall, OnToolResult. Auto-detects TTY for colors, spinner, and
// visual separators. Handles slash commands (/exit, /model, /cd) before reaching the agent core.
// Renders Markdown incrementally during streaming. Non-TTY output is plain text.
// Layer: transport (console). Dependencies: internal/runtime, internal/config.
package console

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"blazeai/internal/config"
	"blazeai/internal/helpers"
	"blazeai/internal/runtime"
	"blazeai/internal/skills"
)

// ANSI color codes for TTY output.
const (
	dividerWidth     = 60
	colorReset       = "\033[0m"
	colorBold        = "\033[1m"
	colorItalic      = "\033[3m"
	colorRed         = "\033[31m"
	colorGreen       = "\033[32m"
	colorBrightGreen = "\033[1;32m"
	colorLightGray   = "\033[37m"
	colorGray        = "\033[90m"
	colorBlue        = "\033[34m"
	colorPurple      = "\033[35m"
	colorOrange      = "\033[33m"
	colorBrightBlue  = "\033[1;34m"
)

// slashCmd holds a slash command and its description for the startup splash.
type slashCmd struct {
	cmd  string
	desc string
}

// slashCommands lists all available slash commands for the startup splash.
var slashCommands = []slashCmd{
	{"/model [model]", "list or switch model"},
	{"/cd <path>", "change working folder"},
	{"/clear", "clear current session"},
	{"/new", "start a clean session"},
	{"/exit", "close session cleanly"},
}

// Console is the console transport implementing runtime.Handler.
//
// WHAT:  The REPL transport that renders LLM output and handles user input.
// WHY:   Console is the first and complete transport per spec.
// PARAMS: Out — output writer; In — input reader; IsTTY — whether output is a terminal;
//
//	Agent — the runtime agent; Reader — line reader for input.
type Console struct {
	Out     io.Writer
	In      io.Reader
	IsTTY   bool
	Agent   *runtime.Agent
	Reader  *Reader
	Spinner *Spinner

	contentStarted   bool
	contentBuffer    string
	inCodeBlock      bool
	inToolGroup      bool
	needContentLabel bool
	lastPromptTokens int
	lineOpen         bool
	turnAborting     atomic.Bool
	lastToolArgs     string
}

// inputEvent carries one console input line or a terminal read error.
type inputEvent struct {
	line  string
	err   error
	event string // "", "mode_switch"
}

// NewConsole creates a Console with TTY auto-detection.
//
// WHAT:  Constructs the console transport with automatic TTY detection.
// PARAMS: agent — the runtime agent.
// RETURNS: *Console — ready to run.
func NewConsole(agent *runtime.Agent) *Console {
	out := os.Stdout
	in := os.Stdin
	isTTY := isTerminal(out)
	return &Console{
		Out:              out,
		In:               in,
		IsTTY:            isTTY,
		Agent:            agent,
		Reader:           NewReader(in, isTTY),
		Spinner:          NewSpinner(out, isTTY),
		lineOpen:         false,
		inToolGroup:      false,
		needContentLabel: true,
	}
}

// ensureLineBreakBeforeBlock closes the current inline content line before block output.
//
// WHAT:  Forces separators and tool markers onto a fresh line after streamed content.
func (c *Console) ensureLineBreakBeforeBlock() {
	c.flushPendingContent()
	if c.lineOpen {
		fmt.Fprintln(c.Out)
		c.lineOpen = false
	}
}

// flushPendingContent renders any buffered assistant content that has not ended with a newline yet.
//
// WHAT:  Flushes the current partial Markdown line before non-content output or turn end.
func (c *Console) flushPendingContent() {
	if c.contentBuffer == "" {
		return
	}
	c.renderLine(c.contentBuffer, false)
	c.contentBuffer = ""
}

// openToolGroup prints the separator that starts a consecutive group of tool calls.
//
// WHAT:  Visually delimits the beginning of a tool batch.
func (c *Console) openToolGroup() {
	if c.inToolGroup {
		return
	}
	c.divider("tools", colorGreen, true)
	c.inToolGroup = true
}

// closeToolGroup prints the separator that ends a consecutive group of tool calls.
//
// WHAT:  Visually delimits the end of a tool batch.
func (c *Console) closeToolGroup() {
	if !c.inToolGroup {
		return
	}
	c.ctxSeparator(colorGreen)
	c.inToolGroup = false
}

// color wraps text with an ANSI color code if TTY, otherwise returns plain text.
//
// WHAT:  Applies ANSI color to text on TTY.
// PARAMS: c — color code; text — the text to colorize.
// RETURNS: string — colored text or plain text.
func (c *Console) color(colorCode, text string) string {
	if !c.IsTTY {
		return text
	}
	return colorCode + text + colorReset
}

// bold wraps text with bold ANSI code if TTY.
func (c *Console) bold(text string) string {
	if !c.IsTTY {
		return text
	}
	return colorBold + text + colorReset
}

// divider prints a unified console divider with an optional label.
//
// WHAT:  Renders the single visual separator style used across tools and context footer.
func (c *Console) divider(label, labelColor string, boldLabel bool) {
	c.ensureLineBreakBeforeBlock()
	char := "-"
	if c.IsTTY {
		char = "─"
	}
	if label == "" {
		line := strings.Repeat(char, dividerWidth)
		if c.IsTTY {
			lineCol := colorLightGray
			if labelColor != "" {
				lineCol = labelColor
			}
			fmt.Fprintln(c.Out, c.color(lineCol, line))
			return
		}
		fmt.Fprintln(c.Out, line)
		return
	}
	remainder := dividerWidth - len(label) - 1
	if remainder < 1 {
		remainder = 1
	}
	tail := strings.Repeat(char, remainder)
	if c.IsTTY {
		styledLabel := label
		if boldLabel {
			styledLabel = c.bold(styledLabel)
		}
		if labelColor != "" {
			styledLabel = c.color(labelColor, styledLabel)
		}
		lineColor := colorLightGray
		if labelColor != "" {
			lineColor = labelColor
		}
		fmt.Fprintf(c.Out, "%s %s\n", styledLabel, c.color(lineColor, tail))
		return
	}
	fmt.Fprintf(c.Out, "%s %s\n", label, tail)
}

// responseSeparator prints the separator shown after the assistant finishes responding.
// Renders a three-line ASCII table with CTX tokens, current model, and work directory (tail-truncated).
//
// WHAT:  Prints an ASCII table separator after the response.
func (c *Console) responseSeparator() {
	if c.lastPromptTokens <= 0 {
		return
	}
	c.ensureLineBreakBeforeBlock()

	ctxText := "CTX: " + formatCompactInt(c.lastPromptTokens)
	model := c.Agent.ModelID
	workDir := truncatePathTail(c.Agent.WorkDir, 30)

	cell1 := " " + ctxText + " "
	cell2 := " " + model + " "
	cell3 := " " + workDir + " "

	w1 := len(cell1)
	w2 := len(cell2)
	w3 := len(cell3)

	char := "-"
	vChar := "|"
	mChar := "+"
	if c.IsTTY {
		char = "─"
		vChar = "│"
		mChar = "┬"
	}

	topLine := "┌" + strings.Repeat(char, w1) + mChar + strings.Repeat(char, w2) + mChar + strings.Repeat(char, w3) + "┐"
	midLine := vChar + cell1 + vChar + cell2 + vChar + cell3 + vChar
	botLine := "└" + strings.Repeat(char, w1) + "┴" + strings.Repeat(char, w2) + "┴" + strings.Repeat(char, w3) + "┘"
	if !c.IsTTY {
		topLine = "+" + strings.Repeat(char, w1) + mChar + strings.Repeat(char, w2) + mChar + strings.Repeat(char, w3) + "+"
		botLine = "+" + strings.Repeat(char, w1) + "+" + strings.Repeat(char, w2) + "+" + strings.Repeat(char, w3) + "+"
	}

	if c.IsTTY {
		fmt.Fprintln(c.Out, c.color(colorBrightBlue, topLine))
		fmt.Fprint(c.Out, c.color(colorBrightBlue, vChar))
		fmt.Fprint(c.Out, c.color(colorOrange, cell1))
		fmt.Fprint(c.Out, c.color(colorBrightBlue, vChar))
		fmt.Fprint(c.Out, c.color(colorOrange, cell2))
		fmt.Fprint(c.Out, c.color(colorBrightBlue, vChar))
		fmt.Fprint(c.Out, c.color(colorOrange, cell3))
		fmt.Fprintln(c.Out, c.color(colorBrightBlue, vChar))
		fmt.Fprintln(c.Out, c.color(colorBrightBlue, botLine))
		return
	}
	fmt.Fprintln(c.Out, topLine)
	fmt.Fprintln(c.Out, midLine)
	fmt.Fprintln(c.Out, botLine)
}

// ctxSeparator prints the prompt-token separator using the requested color.
//
// WHAT:  Renders the same context-size label for tool-group and end-of-turn separators.
// PARAMS: color — ANSI color to use for the separator label and line.
func (c *Console) ctxSeparator(color string) {
	if c.lastPromptTokens <= 0 {
		return
	}
	c.divider("ctx "+formatCompactInt(c.lastPromptTokens), color, false)
}

// formatCompactInt returns a shorter human-readable token count such as 12.3k.
//
// WHAT:  Formats token counts compactly for separator display.
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
// If the path is longer than maxLen, prepends "..." to indicate truncation.
// The total result including "..." does not exceed maxLen.
//
// WHAT:  Truncates a path for compact display, keeping the tail.
// PARAMS: path — the full absolute path; maxLen — maximum output length including "...".
// RETURNS: string — the truncated path.
func truncatePathTail(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// showStartupSplash prints the welcome screen with title, commands, skills, model and work folder.
//
// WHAT:  Renders the startup welcome screen once at session start.
// WHY:   Gives the user an immediate overview of available commands, skills, and current state.
// HOW:   Boxed title, section labels with muted separators, columnar skill names, session info.
func (c *Console) showStartupSplash() {
	if !c.IsTTY {
		return
	}

	// Title box.
	title := "BlazeAI — blazing-fast AI terminal agent"
	width := len(title) + 2
	char := "─"
	topLine := "┌" + strings.Repeat(char, width) + "┐"
	botLine := "└" + strings.Repeat(char, width) + "┘"
	fmt.Fprintln(c.Out, c.color(colorBrightBlue, topLine))
	fmt.Fprint(c.Out, c.color(colorBrightBlue, "│ "))
	fmt.Fprint(c.Out, c.color(colorOrange, title))
	fmt.Fprintln(c.Out, c.color(colorBrightBlue, "   │"))
	fmt.Fprintln(c.Out, c.color(colorBrightBlue, botLine))
	fmt.Fprintln(c.Out)

	// Commands section.
	c.sectionLabel("Commands", colorBlue)
	maxCmd := 0
	for _, sc := range slashCommands {
		if len(sc.cmd) > maxCmd {
			maxCmd = len(sc.cmd)
		}
	}
	for _, sc := range slashCommands {
		fmt.Fprintf(c.Out, "  %-*s  %s\n", maxCmd, sc.cmd, sc.desc)
	}
	fmt.Fprintln(c.Out)

	// Skills section.
	c.sectionLabel("Skills", colorPurple)
	all, err := skills.DiscoverAll(c.Agent.WorkDir)
	if err != nil {
		fmt.Fprintf(c.Out, "  unavailable: %v\n", err)
	} else if len(all) == 0 {
		fmt.Fprintln(c.Out, "  (none)")
	} else {
		names := skills.SortedNames(all)
		displayNames := make([]string, len(names))
		maxName := 0
		for i, name := range names {
			displayNames[i] = formatSkillName(name)
			if len(displayNames[i]) > maxName {
				maxName = len(displayNames[i])
			}
		}
		colWidth := maxName + 3
		if colWidth < 30 {
			colWidth = 30
		}
		cols := 2
		for i, name := range displayNames {
			fmt.Fprintf(c.Out, "  %-*s", colWidth, name)
			if (i+1)%cols == 0 {
				fmt.Fprintln(c.Out)
			}
		}
		if len(displayNames)%cols != 0 {
			fmt.Fprintln(c.Out)
		}
	}
	fmt.Fprintln(c.Out)

	// Helpers section.
	c.sectionLabel("Helpers", colorOrange)
	helperStatuses := helpers.Detect(helpers.DefaultLookup)
	availableHelpers := helpers.Available(helperStatuses, c.Agent.WorkDir)
	if len(availableHelpers) == 0 {
		fmt.Fprintln(c.Out, "  (none)")
	} else {
		maxName := 0
		for _, helper := range availableHelpers {
			if len(helper.Name) > maxName {
				maxName = len(helper.Name)
			}
		}
		colWidth := maxName + 3
		if colWidth < 18 {
			colWidth = 18
		}
		cols := 3
		for i, helper := range availableHelpers {
			fmt.Fprintf(c.Out, "  %-*s", colWidth, helper.Name)
			if (i+1)%cols == 0 {
				fmt.Fprintln(c.Out)
			}
		}
		if len(availableHelpers)%cols != 0 {
			fmt.Fprintln(c.Out)
		}
	}
	fmt.Fprintln(c.Out)

	// Session section.
	c.sectionLabel("Session", colorGreen)
	fmt.Fprintf(c.Out, "  %-6s  %s\n", c.bold("Model"), c.Agent.ModelID)
	fmt.Fprintf(c.Out, "  %-6s %s\n", c.bold("Folder"), c.Agent.WorkDir)
	fmt.Fprintln(c.Out)
}

// sectionLabel prints a colored bold section label followed by a light gray dash separator to dividerWidth.
//
// WHAT:  Renders a section header with accent color on the label and subtle separator line.
// PARAMS: label — section name; labelColor — ANSI color for the label.
func (c *Console) sectionLabel(label string, labelColor string) {
	fill := strings.Repeat("─", dividerWidth-len(label)-1)
	fmt.Fprint(c.Out, c.color(labelColor, c.bold(label+" ")))
	fmt.Fprintln(c.Out, c.color(colorLightGray, fill))
}

// formatSkillName strips the scope prefix from a skill ID for display.
// global/name becomes name; project/name is kept as-is.
func formatSkillName(name string) string {
	return strings.TrimPrefix(name, "global/")
}

// OnUsage records the prompt token count from the latest provider response.
//
// WHAT:  Stores context size for end-of-turn separator rendering.
// PARAMS: promptTokens — provider-reported prompt tokens.
func (c *Console) OnUsage(promptTokens int) {
	c.lastPromptTokens = promptTokens
}

// OnContent is called for each streaming text chunk from the LLM.
// Stops the spinner on first chunk, then writes the delta to output.
//
// WHAT:  Streams LLM text content to the console.
// PARAMS: delta — the text chunk from the LLM.
func (c *Console) OnContent(delta string) {
	if c.turnAborting.Load() {
		return
	}
	if c.inToolGroup {
		c.closeToolGroup()
		c.needContentLabel = true
	}
	if !c.contentStarted {
		c.contentStarted = true
		c.Spinner.Stop()
	}
	if c.needContentLabel {
		fmt.Fprint(c.Out, c.color(colorOrange, c.bold("[BLAZE] ")))
		c.needContentLabel = false
	}
	c.contentBuffer += delta
	for {
		idx := strings.IndexByte(c.contentBuffer, '\n')
		if idx < 0 {
			break
		}
		line := c.contentBuffer[:idx]
		c.renderLine(line, true)
		c.contentBuffer = c.contentBuffer[idx+1:]
	}
	if c.contentBuffer != "" && !c.inCodeBlock && !shouldBufferMarkdownLine(c.contentBuffer) {
		c.writeRenderedLine(c.renderInline(c.contentBuffer), false)
		c.contentBuffer = ""
	}
}

// OnToolCall is called before a tool is executed.
// Stores args for deferred single-line display in OnToolResult.
//
// WHAT:  Buffers tool call args and handles tool group header.
// PARAMS: name — tool name; args — formatted arguments (purpose text).
func (c *Console) OnToolCall(name string, args string) {
	if c.turnAborting.Load() {
		return
	}
	if !c.contentStarted {
		c.contentStarted = true
		c.Spinner.Stop()
	}
	if !c.inToolGroup {
		c.openToolGroup()
	}
	c.lastToolArgs = args
}

// toolEmoji returns a tool-specific emoji for display in the console UI.
//
// WHAT:  Maps tool names to representative emoji characters.
// RETURNS: string — the emoji character for the given tool.
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
	default:
		return "🔧"
	}
}

// OnToolResult is called after a tool has finished.
// Prints a single line: tool emoji + purpose + status symbol.
// Success: ✓. Error: ✗ <message>. Timeout: ⏱ <message>.
//
// WHAT:  Displays tool result inline with the deferred tool call line.
// PARAMS: name — tool name; result — the raw tool output.
func (c *Console) OnToolResult(name string, result string) {
	if c.turnAborting.Load() {
		c.lastToolArgs = ""
		return
	}
	badge, content, colorCode := parseToolResult(result)
	icon := c.color(colorGreen, toolEmoji(name))
	args := c.lastToolArgs
	c.lastToolArgs = ""

	switch badge {
	case "DONE":
		if args != "" {
			fmt.Fprintf(c.Out, "%s %s %s\n",
				icon, args,
				c.color(colorBrightGreen, "✓"),
			)
		} else {
			fmt.Fprintf(c.Out, "%s %s\n",
				icon,
				c.color(colorBrightGreen, "✓"),
			)
		}
	case "ERROR":
		if content != "" {
			content = strings.ReplaceAll(content, "\n", " ")
			if len(content) > 200 {
				content = content[:197] + "..."
			}
		}
		if args != "" {
			fmt.Fprintf(c.Out, "%s %s %s %s\n",
				icon, args,
				c.color(colorCode, "✗"),
				c.color(colorCode, content),
			)
		} else {
			fmt.Fprintf(c.Out, "%s %s %s\n",
				icon,
				c.color(colorCode, "✗"),
				c.color(colorCode, content),
			)
		}
	case "TIMEOUT":
		if content != "" {
			content = strings.ReplaceAll(content, "\n", " ")
		}
		if args != "" {
			fmt.Fprintf(c.Out, "%s %s %s %s\n",
				icon, args,
				c.color(colorCode, "⏱"),
				c.color(colorCode, content),
			)
		} else {
			fmt.Fprintf(c.Out, "%s %s %s\n",
				icon,
				c.color(colorCode, "⏱"),
				c.color(colorCode, content),
			)
		}
	}
	c.lineOpen = false
}

// parseToolResult extracts a display badge, useful content, and color from raw tool output.
//
// WHAT:  Normalizes tool results into DONE/ERROR/TIMEOUT badges using prefix conventions.
// WHY:   Raw tool output follows conventions: ok/ok <msg>, error: <msg>, timeout <msg>.
// RETURNS: badge — DONE/ERROR/TIMEOUT; content — the most relevant output text; colorCode — ANSI color.
func parseToolResult(result string) (badge, content, colorCode string) {
	result = strings.TrimSpace(result)

	if strings.HasPrefix(result, "timeout") {
		return "TIMEOUT", strings.TrimSpace(strings.TrimPrefix(result, "timeout")), colorOrange
	}

	if strings.HasPrefix(result, "error:") {
		msg := strings.TrimSpace(strings.TrimPrefix(result, "error:"))
		if idx := strings.Index(msg, "\n"); idx >= 0 {
			msg = strings.TrimSpace(msg[:idx])
		}
		return "ERROR", msg, colorRed
	}

	if strings.HasPrefix(result, "exit_code:") {
		rest := strings.TrimSpace(strings.TrimPrefix(result, "exit_code:"))
		newlineIdx := strings.Index(rest, "\n")
		if newlineIdx < 0 {
			return "DONE", "", colorBrightGreen
		}

		exitCodeStr := strings.TrimSpace(rest[:newlineIdx])
		exitCode := 0
		fmt.Sscanf(exitCodeStr, "%d", &exitCode)
		remaining := rest[newlineIdx+1:]
		stdout := extractToolSection(remaining, "stdout:")
		stderr := extractToolSection(remaining, "stderr:")

		if exitCode != 0 {
			if stderr != "" {
				return "ERROR", stderr, colorRed
			}
			if stdout != "" {
				return "ERROR", stdout, colorRed
			}
			return "ERROR", "exit code " + exitCodeStr, colorRed
		}
		return "DONE", "", colorBrightGreen
	}

	if strings.HasPrefix(result, "ok") {
		return "DONE", "", colorBrightGreen
	}

	return "DONE", "", colorBrightGreen
}

// extractToolSection extracts the content of a labeled section such as "stdout:" or "stderr:".
//
// WHAT:  Pulls the text under a section label until the next known section label or EOF.
// PARAMS: text — tool output after the exit_code line; label — section label to extract.
// RETURNS: string — trimmed section content or empty if label not found.
func extractToolSection(text, label string) string {
	idx := strings.Index(text, label)
	if idx < 0 {
		return ""
	}
	after := text[idx+len(label):]
	after = strings.TrimPrefix(after, "\n")

	end := len(after)
	for _, other := range []string{"stdout:", "stderr:"} {
		if other == label {
			continue
		}
		if i := strings.Index(after, other); i >= 0 && i < end {
			end = i
		}
	}
	return strings.TrimSpace(after[:end])
}

// renderLine renders one Markdown line using a minimal terminal-friendly subset.
//
// WHAT:  Supports headings, bullets, numbered lists, code fences, and simple inline markers.
// WHY:   Full Markdown parsing is unnecessary for the console REPL, but raw Markdown reads poorly.
// PARAMS: line — one line without trailing newline; terminated — whether the source line ended with '\n'.
func (c *Console) renderLine(line string, terminated bool) {
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "```") {
		c.inCodeBlock = !c.inCodeBlock
		if terminated {
			c.writeRenderedLine("", true)
		}
		return
	}

	if c.inCodeBlock {
		c.writeRenderedLine("    "+line, terminated)
		return
	}

	if line == "" {
		c.writeRenderedLine("", terminated)
		return
	}

	if isTableSeparator(line) {
		if terminated {
			c.writeRenderedLine("", true)
		}
		return
	}

	if isTableRow(line) {
		cells := splitTableRow(line)
		for i, cell := range cells {
			cells[i] = c.renderInline(cell)
		}
		c.writeRenderedLine("  "+strings.Join(cells, "  -  "), terminated)
		return
	}

	if level, title, ok := parseHeading(line); ok {
		rendered := c.renderInline(title)
		if c.IsTTY {
			rendered = c.color(colorBlue, c.bold(rendered))
		}
		if level == 1 {
			rendered = strings.ToUpper(rendered)
		}
		c.writeRenderedLine(rendered, terminated)
		return
	}

	if item, ok := parseBullet(line); ok {
		c.writeRenderedLine("  - "+c.renderInline(item), terminated)
		return
	}

	if prefix, item, ok := parseNumbered(line); ok {
		c.writeRenderedLine("  "+prefix+" "+c.renderInline(item), terminated)
		return
	}

	c.writeRenderedLine(c.renderInline(line), terminated)
}

// writeRenderedLine writes one rendered line and updates line-open tracking.
func (c *Console) writeRenderedLine(text string, terminated bool) {
	if terminated {
		fmt.Fprintln(c.Out, text)
		c.lineOpen = false
		return
	}
	fmt.Fprint(c.Out, text)
	c.lineOpen = text != ""
}

// reLink matches Markdown links [text](url).
var reLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// reUnderscoreItalic matches _text_ only at word boundaries (space or line edge before/after _).
// Prevents false positives in code identifiers like task_write.
var reUnderscoreItalic = regexp.MustCompile(`(?:^|\s)_([^_]+)_(?:\s|$)`)

// renderInline strips or styles simple inline Markdown markers within a rendered line.
func (c *Console) renderInline(text string) string {
	text = c.toggleDelimited(text, "**", func(s string) string {
		if c.IsTTY {
			return c.bold(s)
		}
		return s
	})
	text = reUnderscoreItalic.ReplaceAllStringFunc(text, func(match string) string {
		inner := match[1 : len(match)-1]
		if c.IsTTY {
			return c.color(colorItalic, inner)
		}
		return inner
	})
	text = c.toggleDelimited(text, "*", func(s string) string {
		if c.IsTTY {
			return c.color(colorItalic, s)
		}
		return s
	})
	text = c.toggleDelimited(text, "`", func(s string) string {
		if c.IsTTY {
			return c.color(colorOrange, s)
		}
		return s
	})
	text = c.renderLinks(text)
	return text
}

// renderLinks replaces Markdown links with a terminal-friendly format.
//
// WHAT:  Converts [text](url) to text (url), coloring the URL portion on TTY.
// PARAMS: text — the line to process.
// RETURNS: string — the line with links rendered.
func (c *Console) renderLinks(text string) string {
	return reLink.ReplaceAllStringFunc(text, func(match string) string {
		parts := reLink.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		label := parts[1]
		url := parts[2]
		if c.IsTTY {
			return label + " " + c.color(colorPurple, "("+url+")")
		}
		return label + " (" + url + ")"
	})
}

// toggleDelimited replaces paired delimiters with styled or plain inner text.
func (c *Console) toggleDelimited(text, delim string, render func(string) string) string {
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
		b.WriteString(render(text[:end]))
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

// shouldBufferMarkdownLine reports whether a partial line should wait for completion.
func shouldBufferMarkdownLine(line string) bool {
	trimmedLeft := strings.TrimLeft(line, " ")
	if trimmedLeft == "" {
		return false
	}
	if strings.HasPrefix(trimmedLeft, "```") || strings.HasPrefix(trimmedLeft, "#") {
		return true
	}
	if strings.HasPrefix(trimmedLeft, "- ") || strings.HasPrefix(trimmedLeft, "* ") {
		return true
	}
	if trimmedLeft[0] >= '0' && trimmedLeft[0] <= '9' {
		return true
	}
	if strings.HasPrefix(trimmedLeft, "|") {
		return true
	}
	if strings.Contains(line, "**") || strings.Contains(line, "*") || strings.Contains(line, "`") {
		return true
	}
	if strings.Contains(line, "_") || strings.Contains(line, "[") {
		return true
	}
	return false
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

// promptLabel returns the colored input prompt label.
//
// WHAT:  Builds the [<mode> mode]> label.
// RETURNS: string — the formatted prompt label.
func (c *Console) promptLabel() string {
	if c.Agent.CurrentMode != nil {
		label := fmt.Sprintf("[%s mode]> ", c.Agent.CurrentMode.Name)
		return c.color(colorBlue, c.bold(label))
	}
	return c.color(colorBlue, c.bold("[default mode]> "))
}

// Run starts the REPL loop. Reads input, handles slash commands, and runs agent turns.
// Returns when the user types /exit or input ends.
//
// WHAT:  The main REPL loop.
// WHY:   This is the entrypoint for the console transport.
// HOW:   Loops reading input, dispatches slash commands or sends input to the agent.
// On TTY: uses raw-mode input for Tab detection (mode cycling).
// On non-TTY: uses buffered input with a background goroutine.
// RETURNS: error if a fatal error occurs.
func (c *Console) Run() error {
	c.showStartupSplash()
	if c.IsTTY {
		return c.runTTY()
	}
	return c.runNonTTY()
}

// runTTY runs the REPL loop with raw-mode input for Tab detection.
// No background goroutine — input is read directly at the prompt.
// During streaming, abort is via SIGINT only (no queued input).
//
// WHAT:  TTY-specific REPL with raw-mode Tab detection.
// WHY:   Tab key requires raw terminal mode to detect.
func (c *Console) runTTY() error {
	for {
		fmt.Fprint(c.Out, c.promptLabel())
		line, event, err := c.Reader.ReadEvent()
		if err == io.EOF {
			fmt.Fprintln(c.Out)
			return nil
		}
		if err != nil {
			return fmt.Errorf("input error: %w", err)
		}

		// Handle mode switch event.
		if event == "mode_switch" {
			if _, switchErr := c.Agent.NextMode(); switchErr != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("mode switch error: %v", switchErr)))
			} else {
				fmt.Fprintln(c.Out)
				fmt.Fprintf(c.Out, "[mode: %s | %s]\n", c.Agent.CurrentMode.Name, c.Agent.ModelID)
			}
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Handle slash commands.
		if strings.HasPrefix(input, "/") {
			handled, exit, cmdErr := c.handleCommand(input)
			if cmdErr != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, cmdErr.Error()))
				continue
			}
			if exit {
				return nil
			}
			if handled {
				continue
			}
		}

		fmt.Fprintln(c.Out)

		// Start spinner and reset content state before LLM call.
		c.contentStarted = false
		c.contentBuffer = ""
		c.inCodeBlock = false
		c.inToolGroup = false
		c.needContentLabel = true
		c.lastPromptTokens = 0
		c.lineOpen = false
		c.Spinner.Start()

		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)

		// Run agent turn without input events (abort via SIGINT only).
		turnErr := c.runAgentTurnTTY(input, interrupts)
		if turnErr != nil && !errors.Is(turnErr, runtime.ErrTurnAborted) {
			c.Spinner.Stop()
			fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("error: %v", turnErr)))
			c.lineOpen = false
		}
		signal.Stop(interrupts)
		c.flushPendingContent()
		fmt.Fprintln(c.Out)
		c.lineOpen = false
		c.closeToolGroup()
		c.responseSeparator()
	}
}

// runAgentTurnTTY executes one agent turn with SIGINT-only abort (no input events).
//
// WHAT:  Simplified turn execution for TTY mode.
// WHY:   TTY mode reads input directly at the prompt; no goroutine for queued input.
func (c *Console) runAgentTurnTTY(input string, interrupts <-chan os.Signal) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Agent.RunTurn(ctx, input)
	}()

	for {
		select {
		case err := <-errCh:
			c.turnAborting.Store(false)
			return err
		case <-interrupts:
			c.turnAborting.Store(true)
			cancel()
		}
	}
}

// runNonTTY runs the REPL loop with buffered input (existing goroutine approach).
// No Tab detection — modes are fixed from config's LastMode.
func (c *Console) runNonTTY() error {
	inputs := c.startInputReader()
	pending := ""
	promptShown := false

	for {
		if pending == "" {
			if !promptShown {
				fmt.Fprint(c.Out, c.promptLabel())
				promptShown = true
			}
			event, ok := <-inputs
			if !ok || event.err == io.EOF {
				fmt.Fprintln(c.Out)
				return nil
			}
			if event.err != nil {
				return fmt.Errorf("input error: %w", event.err)
			}
			pending = strings.TrimSpace(event.line)
		}

		input := pending
		pending = ""
		if input == "" {
			promptShown = false
			continue
		}
		promptShown = false

		// Handle slash commands.
		if strings.HasPrefix(input, "/") {
			handled, exit, err := c.handleCommand(input)
			if err != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, err.Error()))
				continue
			}
			if exit {
				return nil
			}
			if handled {
				continue
			}
		}

		fmt.Fprintln(c.Out)

		// Start spinner and reset content state before LLM call.
		c.contentStarted = false
		c.contentBuffer = ""
		c.inCodeBlock = false
		c.inToolGroup = false
		c.needContentLabel = true
		c.lastPromptTokens = 0
		c.lineOpen = false
		c.Spinner.Start()

		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)

		// Run the agent turn.
		nextInput, err := c.runAgentTurn(input, interrupts, inputs)
		if err != nil && !errors.Is(err, runtime.ErrTurnAborted) {
			c.Spinner.Stop()
			fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("error: %v", err)))
			c.lineOpen = false
		}
		signal.Stop(interrupts)
		c.flushPendingContent()
		fmt.Fprintln(c.Out)
		c.lineOpen = false
		c.closeToolGroup()
		c.responseSeparator()
		if nextInput != "" {
			pending = nextInput
		}
	}
}

// startInputReader continuously reads console input lines in the background.
func (c *Console) startInputReader() <-chan inputEvent {
	ch := make(chan inputEvent)
	go func() {
		defer close(ch)
		for {
			line, err := c.Reader.ReadLine()
			ch <- inputEvent{line: line, err: err}
			if err != nil {
				return
			}
		}
	}()
	return ch
}

// runAgentTurn executes one agent turn while listening for Ctrl+C abort requests.
func (c *Console) runAgentTurn(input string, interrupts <-chan os.Signal, inputs <-chan inputEvent) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Agent.RunTurn(ctx, input)
	}()

	nextInput := ""
	activeInputs := inputs

	for {
		select {
		case err := <-errCh:
			c.turnAborting.Store(false)
			return nextInput, err
		case event, ok := <-activeInputs:
			if !ok || event.err == io.EOF {
				activeInputs = nil
				c.turnAborting.Store(true)
				cancel()
				continue
			}
			if event.err != nil {
				c.turnAborting.Store(false)
				return "", fmt.Errorf("input error: %w", event.err)
			}
			trimmed := strings.TrimSpace(event.line)
			if trimmed == "" {
				continue
			}
			if nextInput == "" {
				nextInput = trimmed
			}
			if c.turnAborting.Load() {
				continue
			}
			c.abortCurrentTurn(cancel)
		case <-interrupts:
			if c.turnAborting.Load() {
				continue
			}
			c.abortCurrentTurn(cancel)
		}
	}
}

// abortCurrentTurn stops visible turn activity and requests cancellation from the runtime.
func (c *Console) abortCurrentTurn(cancel context.CancelFunc) {
	c.turnAborting.Store(true)
	c.Spinner.Stop()
	cancel()
	c.contentBuffer = ""
	if c.lineOpen {
		fmt.Fprintln(c.Out)
		c.lineOpen = false
	}
	c.closeToolGroup()
	fmt.Fprintln(c.Out, c.color(colorRed, c.bold("[ABORTED] current turn cancelled")))
}

// handleCommand processes a slash command. Returns (handled, shouldExit, error).
//
// WHAT:  Dispatches slash commands before they reach the agent core.
// PARAMS: input — the full input string starting with /.
// RETURNS: bool handled — whether the command was recognized; bool exit — whether to exit; error.
func (c *Console) handleCommand(input string) (bool, bool, error) {
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/exit":
		if err := c.Agent.CloseSession(); err != nil {
			return true, true, fmt.Errorf("cannot close session: %w", err)
		}
		fmt.Fprintln(c.Out, "Goodbye.")
		return true, true, nil
	case "/model":
		if arg == "" {
			if c.IsTTY {
				if err := c.interactiveSelectModel(); err != nil {
					fmt.Fprintln(c.Out, c.color(colorRed, err.Error()))
				}
			} else {
				c.listModels()
			}
			return true, false, nil
		}
		if err := c.Agent.SetModel(arg); err != nil {
			return true, false, err
		}
		fmt.Fprintf(c.Out, "Model set to: %s\n", arg)
		return true, false, nil
	case "/cd":
		if arg == "" {
			return true, false, fmt.Errorf("usage: /cd <path>")
		}
		if err := c.Agent.SetWorkDir(arg); err != nil {
			return true, false, err
		}
		fmt.Fprintf(c.Out, "Work folder: %s\n", arg)
		return true, false, nil
	case "/clear", "/new":
		if err := c.Agent.ResetConversation(); err != nil {
			return true, false, fmt.Errorf("cannot reset session: %w", err)
		}
		fmt.Fprintln(c.Out, "Session cleared.")
		return true, false, nil
	default:
		// Unknown slash command — pass to agent as normal message.
		return false, false, nil
	}
}

// listModels prints the favorite models from config.
//
// WHAT:  Displays the configured favorite models.
func (c *Console) listModels() {
	cfg := c.Agent.Config
	if len(cfg.FavoriteModels) == 0 {
		fmt.Fprintln(c.Out, "No favorite models configured.")
		return
	}
	fmt.Fprintln(c.Out, c.bold("Favorite models:"))
	for _, m := range cfg.FavoriteModels {
		marker := "  "
		if m == c.Agent.ModelID {
			marker = "> "
		}
		fmt.Fprintf(c.Out, "%s%s\n", marker, m)
	}
}

// interactiveSelectModel runs the interactive provider→model selection flow.
//
// WHAT:  Prompts user to select a provider, fetches its models, then selects one.
// WHY:   /model without args on TTY should let the user pick from live provider data.
// HOW:   Two-step numbered selection: providers → models from endpoint, then SetModel.
// RETURNS: error if cancelled or any step fails.
func (c *Console) interactiveSelectModel() error {
	providers := c.Agent.Config.Providers
	if len(providers) == 0 {
		return fmt.Errorf("no providers configured")
	}

	// Step 1: select provider.
	var selectedProvider config.Provider
	if len(providers) == 1 {
		selectedProvider = providers[0]
		fmt.Fprintf(c.Out, "\nProvider: %s (%s)\n", selectedProvider.Name, selectedProvider.Endpoint)
	} else {
		fmt.Fprintln(c.Out)
		fmt.Fprintln(c.Out, c.bold("Select provider:"))
		for i, p := range providers {
			marker := "  "
			fmt.Fprintf(c.Out, "%s%2d. %s (%s)\n", marker, i+1, p.Name, p.Endpoint)
		}
		fmt.Fprint(c.Out, "> ")

		num, err := c.readInteractiveNumber(1, len(providers))
		if err != nil {
			return err
		}
		selectedProvider = providers[num-1]
	}

	// Step 2: fetch models from the provider endpoint.
	fmt.Fprintln(c.Out)
	fmt.Fprintf(c.Out, "Fetching models from %s...\n", selectedProvider.Name)
	models, err := c.Agent.ListProviderModels(selectedProvider.Name)
	if err != nil {
		return fmt.Errorf("cannot list models: %w", err)
	}
	if len(models) == 0 {
		return fmt.Errorf("provider %s returned no models", selectedProvider.Name)
	}

	// Step 3: select model.
	fmt.Fprintln(c.Out, c.bold("Select model:"))
	padding := paddingWidth(len(models))
	for i, m := range models {
		marker := "  "
		if selectedProvider.Name+"/"+m == c.Agent.ModelID {
			marker = "> "
		}
		fmt.Fprintf(c.Out, "%s%*d. %s\n", marker, padding, i+1, m)
	}
	fmt.Fprint(c.Out, "> ")

	num, err := c.readInteractiveNumber(1, len(models))
	if err != nil {
		return err
	}
	modelID := selectedProvider.Name + "/" + models[num-1]

	// Step 4: set the model.
	if err := c.Agent.SetModel(modelID); err != nil {
		return err
	}
	fmt.Fprintf(c.Out, "Model set to: %s\n", modelID)
	return nil
}

// readInteractiveNumber reads a line from stdin and parses it as a number in [min, max].
//
// WHAT:  Prompts for and validates a numeric input within a range.
// RETURNS: int — the chosen number; error if input is empty/invalid/out of range.
func (c *Console) readInteractiveNumber(min, max int) (int, error) {
	line, err := c.readInteractiveLine()
	if err != nil {
		return 0, fmt.Errorf("cancelled")
	}
	num, convErr := strconv.Atoi(line)
	if convErr != nil || num < min || num > max {
		return 0, fmt.Errorf("invalid selection: enter %d-%d", min, max)
	}
	return num, nil
}

// readInteractiveLine reads a single trimmed line from stdin in cooked mode.
//
// WHAT:  Reads one line from os.Stdin (works between raw-mode ReadEvent calls).
// RETURNS: string — trimmed input; error if read fails or EOF.
func (c *Console) readInteractiveLine() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// paddingWidth returns the number of digits needed for the largest index.
func paddingWidth(count int) int {
	w := 1
	for n := count; n >= 10; n /= 10 {
		w++
	}
	return w
}
