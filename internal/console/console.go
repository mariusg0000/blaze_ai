// console.go — console REPL transport implementing the handler contract.
// Implements OnContent, OnToolCall, OnToolResult. Auto-detects TTY for colors, spinner, and
// visual separators. Handles slash commands (/exit, /model, /cd) before reaching the agent core.
// Renders Markdown incrementally during streaming. Non-TTY output is plain text.
// Layer: transport (console). Dependencies: internal/runtime, internal/config.
package console

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"blazeai/internal/runtime"
)

// ANSI color codes for TTY output.
const (
	colorReset       = "\033[0m"
	colorBold        = "\033[1m"
	colorRed         = "\033[31m"
	colorGreen       = "\033[32m"
	colorBrightGreen = "\033[1;32m"
	colorBlue        = "\033[34m"
	colorPurple      = "\033[35m"
	colorOrange      = "\033[33m"
)

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
	lastPromptTokens int
	lineOpen         bool
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
		Out:         out,
		In:          in,
		IsTTY:       isTTY,
		Agent:       agent,
		Reader:      NewReader(in, isTTY),
		Spinner:     NewSpinner(out, isTTY),
		lineOpen:    false,
		inToolGroup: false,
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
	c.ensureLineBreakBeforeBlock()
	c.separator()
	c.inToolGroup = true
}

// closeToolGroup prints the separator that ends a consecutive group of tool calls.
//
// WHAT:  Visually delimits the end of a tool batch.
func (c *Console) closeToolGroup() {
	if !c.inToolGroup {
		return
	}
	c.separator()
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

// separator prints a visual separator line.
//
// WHAT:  Prints a minimal separator between message types.
func (c *Console) separator() {
	if c.IsTTY {
		fmt.Fprintln(c.Out, strings.Repeat("─", 60))
	} else {
		fmt.Fprintln(c.Out, strings.Repeat("-", 60))
	}
}

// userSeparator prints the separator shown between user input and the model response.
//
// WHAT:  Prints a bold purple separator before the assistant starts responding.
func (c *Console) userSeparator() {
	c.ensureLineBreakBeforeBlock()
	line := strings.Repeat("-", 60)
	if c.IsTTY {
		fmt.Fprintln(c.Out, c.color(colorPurple, c.bold(line)))
		return
	}
	fmt.Fprintln(c.Out, line)
}

// responseSeparator prints the separator shown after the assistant finishes responding.
// If provider usage is available, the separator embeds the prompt token count.
//
// WHAT:  Prints a separator with context size after the response.
func (c *Console) responseSeparator() {
	c.ensureLineBreakBeforeBlock()
	line := strings.Repeat("-", 60)
	if c.lastPromptTokens <= 0 {
		c.userSeparator()
		return
	}
	label := fmt.Sprintf(" CTX: %s ", formatCompactInt(c.lastPromptTokens))
	if len(label) >= len(line) {
		fmt.Fprintln(c.Out, label)
		return
	}
	left := (len(line) - len(label)) / 2
	right := len(line) - len(label) - left
	decorated := strings.Repeat("-", left) + label + strings.Repeat("-", right)
	if c.IsTTY {
		fmt.Fprintln(c.Out, c.color(colorPurple, c.bold(decorated)))
		return
	}
	fmt.Fprintln(c.Out, decorated)
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
	if c.inToolGroup {
		c.closeToolGroup()
	}
	if !c.contentStarted {
		c.contentStarted = true
		c.Spinner.Stop()
		fmt.Fprint(c.Out, c.color(colorOrange, c.bold("[BLAZE] ")))
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
// Stops the spinner on first event, prints a compact one-line tool call marker.
//
// WHAT:  Displays a tool call notification.
// PARAMS: name — tool name; args — raw JSON arguments.
func (c *Console) OnToolCall(name string, args string) {
	if !c.contentStarted {
		c.contentStarted = true
		c.Spinner.Stop()
	}
	if !c.inToolGroup {
		c.openToolGroup()
	}
	argStr := args
	if len(argStr) > 80 {
		argStr = argStr[:77] + "..."
	}
	if argStr != "" {
		fmt.Fprintf(c.Out, "%s %s - %s\n",
			c.color(colorGreen, c.bold("[TOOL CALL]")),
			c.bold(name),
			argStr,
		)
	} else {
		fmt.Fprintf(c.Out, "%s %s\n",
			c.color(colorGreen, c.bold("[TOOL CALL]")),
			c.bold(name),
		)
	}
	c.lineOpen = false
}

// OnToolResult is called after a tool has finished.
// Prints a compact one-line tool result marker with a status badge and useful preview.
//
// WHAT:  Displays tool result status and the most relevant output line.
// PARAMS: name — tool name; result — the raw tool output.
func (c *Console) OnToolResult(name string, result string) {
	badge, content, colorCode := parseToolResult(result)
	if content != "" {
		content = strings.ReplaceAll(content, "\n", " ")
		if len(content) > 100 {
			content = content[:97] + "..."
		}
	}
	if content != "" {
		fmt.Fprintf(c.Out, "%s %s [%s] - %s\n",
			c.color(colorGreen, c.bold("[TOOL RESPONSE]")),
			c.bold(name),
			c.color(colorCode, c.bold(badge)),
			content,
		)
	} else {
		fmt.Fprintf(c.Out, "%s %s [%s]\n",
			c.color(colorGreen, c.bold("[TOOL RESPONSE]")),
			c.bold(name),
			c.color(colorCode, c.bold(badge)),
		)
	}
	c.lineOpen = false
}

// parseToolResult extracts a display badge, useful content, and color from raw tool output.
//
// WHAT:  Normalizes shell and generic tool results into a compact status badge.
// WHY:   Raw tool output contains redundant labels like "exit_code:" and "stdout:".
// RETURNS: badge — OK/ERROR/TIMEOUT; content — the most relevant output text; colorCode — ANSI color.
func parseToolResult(result string) (badge, content, colorCode string) {
	result = strings.TrimSpace(result)

	if strings.HasPrefix(result, "timeout") {
		return "TIMEOUT", strings.TrimSpace(strings.TrimPrefix(result, "timeout")), colorOrange
	}

	if strings.HasPrefix(result, "exit_code:") {
		rest := strings.TrimSpace(strings.TrimPrefix(result, "exit_code:"))
		newlineIdx := strings.Index(rest, "\n")
		if newlineIdx < 0 {
			return "OK", rest, colorBrightGreen
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
		if stdout != "" {
			return "OK", stdout, colorBrightGreen
		}
		return "OK", "", colorBrightGreen
	}

	if strings.HasPrefix(result, "error:") {
		msg := strings.TrimSpace(strings.TrimPrefix(result, "error:"))
		if idx := strings.Index(msg, "\n"); idx >= 0 {
			msg = strings.TrimSpace(msg[:idx])
		}
		return "ERROR", msg, colorRed
	}

	return "OK", result, colorBrightGreen
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

// renderInline strips or styles simple inline Markdown markers within a rendered line.
func (c *Console) renderInline(text string) string {
	text = c.toggleDelimited(text, "**", func(s string) string {
		if c.IsTTY {
			return c.bold(s)
		}
		return s
	})
	text = c.toggleDelimited(text, "`", func(s string) string {
		if c.IsTTY {
			return c.color(colorOrange, s)
		}
		return s
	})
	return text
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
	if strings.Contains(line, "**") || strings.Contains(line, "`") {
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
// WHAT:  Builds the [USER/(provider/model)] > label.
// RETURNS: string — the formatted prompt label.
func (c *Console) promptLabel() string {
	label := fmt.Sprintf("[USER/%s] > ", c.Agent.ModelID)
	return c.color(colorBlue, c.bold(label))
}

// Run starts the REPL loop. Reads input, handles slash commands, and runs agent turns.
// Returns when the user types /exit or input ends.
//
// WHAT:  The main REPL loop.
// WHY:   This is the entrypoint for the console transport.
// HOW:   Loops reading input, dispatches slash commands or sends input to the agent.
// RETURNS: error if a fatal error occurs.
func (c *Console) Run() error {
	for {
		fmt.Fprint(c.Out, c.promptLabel())
		input, err := c.Reader.ReadLine()
		if err == io.EOF {
			fmt.Fprintln(c.Out)
			return nil
		}
		if err != nil {
			return fmt.Errorf("input error: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

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

		c.userSeparator()

		// Start spinner and reset content state before LLM call.
		c.contentStarted = false
		c.contentBuffer = ""
		c.inCodeBlock = false
		c.inToolGroup = false
		c.lastPromptTokens = 0
		c.lineOpen = false
		c.Spinner.Start()

		// Run the agent turn.
		if err := c.Agent.RunTurn(input); err != nil {
			c.Spinner.Stop()
			fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("error: %v", err)))
			c.lineOpen = false
		}
		c.flushPendingContent()
		fmt.Fprintln(c.Out)
		c.lineOpen = false
		c.closeToolGroup()
		c.responseSeparator()
	}
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
			c.listModels()
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
