// console.go — terminal REPL transport implementing the handler contract.
// Implements OnContent, OnToolCall, OnToolResult. TTY-only: reads raw input for Tab mode cycling,
// renders Markdown with ANSI colors, and handles slash commands (/auth, /exit, /model, /cd).
// Layer: transport (console). Dependencies: internal/runtime, internal/config, internal/skills.
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
	"sync"
	"sync/atomic"
	"time"

	"blazeai/internal/config"
	"blazeai/internal/helpers"
	"blazeai/internal/provider"
	"blazeai/internal/runtime"
	"blazeai/internal/skills"
)

// ANSI color codes for TTY output.
const (
	dividerWidth     = 60
	colorReset       = "\033[0m"
	colorBold        = "\033[1m"
	colorItalic      = "\033[3m"
	colorRed         = "\033[1;31m"     // bold red
	colorGreen       = "\033[1;32m"     // bold green
	colorBrightGreen = "\033[1;32m"     // bold green (same as colorGreen, for checkmark)
	colorLightGray   = "\033[37m"       // standard white (subtle separators)
	colorBlue        = "\033[1;34m"     // bold blue
	colorPurple      = "\033[1;35m"     // bold magenta
	colorOrange      = "\033[1;33m"     // bold yellow
	colorBrightBlue  = "\033[1;34m"     // bold blue (same as colorBlue, for borders)
	colorReasoning   = "\033[38;5;244m" // medium gray
	colorCtx         = "\033[1;96m"
)

// slashCmd holds a slash command and its description for the startup splash.
type slashCmd struct {
	cmd  string
	desc string
}

// slashCommands lists all available slash commands for the startup splash.
var slashCommands = []slashCmd{
	{"/auth openai", "connect ChatGPT with browser OAuth"},
	{"/model [model]", "list or switch model (+/- fav)"},
	{"/cd <path>", "change working folder"},
	{"/clear", "clear current session"},
	{"/new", "start a clean session"},
	{"Ctrl+T", "toggle reasoning display"},
	{"/exit", "close session cleanly"},
}

var spinnerFrameInterval = 120 * time.Millisecond

var spinnerFrames = []string{"|", "/", "-", "\\"}

// Console is the console transport implementing runtime.Handler.
//
// WHAT:  The terminal REPL transport that renders LLM output and handles user input.
// WHY:   Console is the first and complete transport per spec.
// HOW:   Always uses raw-mode input for Tab mode cycling and ANSI colors for output.
type Console struct {
	Out    io.Writer
	In     io.Reader
	Agent  *runtime.Agent
	Reader *Reader

	outMu sync.Mutex

	contentStarted   bool
	contentBuffer    string
	inCodeBlock      bool
	lastPromptTokens int
	lineOpen         bool
	toolsStarted     bool
	turnAborting     atomic.Bool
	lastToolArgs     string
	reasoningStarted bool
	reasoningLines   int
	spinnerActive    bool
	spinnerVisible   bool
	spinnerFrame     int
	spinnerWidth     int
	spinnerLabel     string
	spinnerStop      chan struct{}
	spinnerDone      chan struct{}

	switchLineActive bool // true when a mode/model status line is present and can be overwritten
	switchLineWidth  int  // visible width of the current status line for reliable space-padding
}

// NewConsole creates a Console for terminal interaction.
//
// WHAT:  Constructs the console transport for TTY use.
// PARAMS: agent — the runtime agent.
// RETURNS: *Console — ready to run.
func NewConsole(agent *runtime.Agent) *Console {
	return &Console{
		Out:      os.Stdout,
		In:       os.Stdin,
		Agent:    agent,
		Reader:   NewReader(os.Stdin, true),
		lineOpen: false,
	}
}

// lockOutput stops the spinner and returns a held output mutex for direct console writes.
//
// WHAT:  Serializes all console output that can overlap with the spinner goroutine.
// WHY:   Spinner animation and runtime callbacks can otherwise interleave bytes on the TTY.
// HOW:   Takes the output mutex first, then clears any visible spinner line before writes.
func (c *Console) lockOutput() {
	c.outMu.Lock()
	c.stopSpinnerLocked()
}

// unlockOutput releases the console output mutex.
func (c *Console) unlockOutput() {
	c.outMu.Unlock()
}

// startSpinner begins animating a single status line until the next visible output arrives.
//
// WHAT:  Starts the console spinner for an active assistant turn.
// WHY:   Provider calls can stay silent for a while before the first chunk or next tool step.
// HOW:   Runs a ticker goroutine that rewrites one padded line in place under the output mutex.
// PARAMS: label — short status text such as "thinking...".
func (c *Console) startSpinner(label string) {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	c.startSpinnerLocked(label)
}

func (c *Console) startSpinnerLocked(label string) {
	if c.spinnerActive {
		c.spinnerLabel = label
		return
	}
	c.spinnerActive = true
	c.spinnerVisible = false
	c.spinnerFrame = 0
	c.spinnerWidth = 0
	c.spinnerLabel = label
	c.spinnerStop = make(chan struct{})
	c.spinnerDone = make(chan struct{})
	go c.runSpinner(c.spinnerStop, c.spinnerDone)
}

// updateSpinnerLabel changes the active spinner text without restarting the animation.
func (c *Console) updateSpinnerLabel(label string) {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	if !c.spinnerActive {
		return
	}
	c.spinnerLabel = label
}

// stopSpinner clears the spinner line and waits for the animation goroutine to exit.
func (c *Console) stopSpinner() {
	c.outMu.Lock()
	defer c.outMu.Unlock()
	c.stopSpinnerLocked()
}

func (c *Console) stopSpinnerLocked() {
	if !c.spinnerActive {
		return
	}
	stop := c.spinnerStop
	done := c.spinnerDone
	c.spinnerActive = false
	c.spinnerStop = nil
	c.spinnerDone = nil
	if stop != nil {
		close(stop)
	}
	if c.spinnerVisible {
		fmt.Fprintf(c.Out, "\r%s\r", strings.Repeat(" ", c.spinnerWidth))
		c.spinnerVisible = false
		c.spinnerWidth = 0
	}
	if done != nil {
		c.outMu.Unlock()
		<-done
		c.outMu.Lock()
	}
}

func (c *Console) runSpinner(stop <-chan struct{}, done chan<- struct{}) {
	ticker := time.NewTicker(spinnerFrameInterval)
	defer ticker.Stop()
	defer close(done)
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			c.outMu.Lock()
			if !c.spinnerActive {
				c.outMu.Unlock()
				return
			}
			frame := spinnerFrames[c.spinnerFrame%len(spinnerFrames)]
			c.spinnerFrame++
			line := frame + " " + c.spinnerLabel
			width := len(line)
			if c.spinnerWidth > width {
				width = c.spinnerWidth
			}
			fmt.Fprintf(c.Out, "\r%-*s\r%s", width, "", line)
			c.spinnerVisible = true
			c.spinnerWidth = len(line)
			c.outMu.Unlock()
		}
	}
}

// ensureLineBreakBeforeBlock closes the current inline content line before block output.
//
// WHAT:  Forces separators and tool markers onto a fresh line after streamed content.
func (c *Console) ensureLineBreakBeforeBlock() {
	c.flushPendingContent()
	if c.reasoningStarted {
		fmt.Fprintln(c.Out)
		fmt.Fprintln(c.Out) // blank line after reasoning
		c.reasoningStarted = false
		c.reasoningLines = 0
	}
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

// color wraps text with an ANSI color code.
//
// WHAT:  Applies ANSI color to text.
// PARAMS: c — color code; text — the text to colorize.
// RETURNS: string — ANSI-colored text.
func (c *Console) color(colorCode, text string) string {
	return colorCode + text + colorReset
}

// bold wraps text with bold ANSI code.
func (c *Console) bold(text string) string {
	return colorBold + text + colorReset
}

// responseSeparator prints the separator shown after the assistant finishes responding.
// Renders a boxed table with CTX tokens, current model, and work directory (tail-truncated).
//
// WHAT:  Prints a boxed table separator after the response.
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

	char := "─"
	vChar := "│"
	mChar := "┬"

	topLine := "┌" + strings.Repeat(char, w1) + mChar + strings.Repeat(char, w2) + mChar + strings.Repeat(char, w3) + "┐"
	botLine := "└" + strings.Repeat(char, w1) + "┴" + strings.Repeat(char, w2) + "┴" + strings.Repeat(char, w3) + "┘"

	blue := c.color(colorBrightBlue, vChar)
	styles := func(s string) string { return c.color(colorOrange, c.bold(s)) }

	fmt.Fprint(c.Out, c.color(colorBrightBlue, topLine))
	fmt.Fprintln(c.Out)
	fmt.Fprint(c.Out, blue, styles(cell1), blue, styles(cell2), blue, styles(cell3), blue)
	fmt.Fprintln(c.Out)
	fmt.Fprintln(c.Out, c.color(colorBrightBlue, botLine))
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

	// Keyboard shortcuts section.
	c.sectionLabel("Shortcuts", colorBlue)
	shortcuts := []struct {
		key  string
		desc string
	}{
		{"Tab", "cycle work mode"},
		{"Ctrl+\\", "cycle favorite model"},
		{"Ctrl+F", "add model to favorites"},
		{"Ctrl+R", "remove model from favorites"},
		{"Ctrl+T", "toggle reasoning display"},
		{"Ctrl+D", "exit (empty line)"},
	}
	maxKey := 0
	for _, s := range shortcuts {
		if len(s.key) > maxKey {
			maxKey = len(s.key)
		}
	}
	for _, s := range shortcuts {
		fmt.Fprintf(c.Out, "  %-*s  %s\n", maxKey, s.key, s.desc)
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

	// Shortcuts section.
	c.sectionLabel("Shortcuts", colorGreen)
	fmt.Fprintf(c.Out, "  %-8s  cycle work mode\n", c.bold("Tab"))
	fmt.Fprintf(c.Out, "  %-8s  cycle favorite model\n", c.bold("Ctrl+\\"))
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

// OnSystem displays a system-level notification to the user, such as a detected
// task switch with a summary of the archived task.
//
// WHAT:  Prints a formatted system notification.
// PARAMS: message — the system notification text.
func (c *Console) OnSystem(message string) {
	c.lockOutput()
	defer c.unlockOutput()
	c.ensureLineBreakBeforeBlock()
	fmt.Fprintln(c.Out, c.color(colorOrange, "⚡ System: "+message))
}

// OnMaintenanceCall renders an internal runtime operation using tool-style inline output.
func (c *Console) OnMaintenanceCall(name string, args string) {
	c.OnToolCall(name, args)
}

// OnMaintenanceResult renders the final internal operation status on the open tool line.
func (c *Console) OnMaintenanceResult(name string, result string) {
	c.renderToolResult(name, result, false, true)
}

// OnStreamPhase updates the waiting spinner label for transports that can show provider phases.
func (c *Console) OnStreamPhase(phase provider.StreamPhase) {
	switch phase {
	case provider.PhaseConnecting:
		c.updateSpinnerLabel("Connecting")
	case provider.PhaseWaitingFirstEvent:
		c.updateSpinnerLabel("Waiting")
	case provider.PhaseHiddenReasoning:
		c.updateSpinnerLabel("Thinking")
	case provider.PhaseStreaming:
		// The next visible output stops the spinner, so no label change is needed here.
	}
}

// OnUsage records the prompt token count from the latest provider response.
//
// WHAT:  Stores context size for end-of-turn separator rendering.
// PARAMS: promptTokens — provider-reported prompt tokens.
func (c *Console) OnUsage(promptTokens int) {
	c.lastPromptTokens = promptTokens
}

// OnReasoning is called for each streaming reasoning chunk from the LLM.
//
// WHAT:  Streams LLM reasoning/thinking blocks in muted color, truncated to ReasoningMaxHeight lines.
// PARAMS: delta — the reasoning text chunk from the LLM.
func (c *Console) OnReasoning(delta string) {
	if c.turnAborting.Load() {
		return
	}
	c.lockOutput()
	defer c.unlockOutput()
	maxHeight := c.Agent.Config.ReasoningMaxHeight
	if maxHeight > 0 && c.reasoningLines >= maxHeight {
		return
	}
	if !c.reasoningStarted {
		c.ensureLineBreakBeforeBlock()
		c.reasoningLines = 0
		fmt.Fprint(c.Out, c.color(colorReasoning, "🧠 "))
		c.reasoningStarted = true
	}
	newLines := strings.Count(delta, "\n")
	if maxHeight > 0 && c.reasoningLines+newLines > maxHeight {
		idx := 0
		for i := 0; i < maxHeight-c.reasoningLines; i++ {
			nl := strings.IndexByte(delta[idx:], '\n')
			if nl < 0 {
				break
			}
			idx += nl + 1
		}
		if idx > 0 {
			fmt.Fprint(c.Out, c.color(colorReasoning, delta[:idx]))
		}
		c.reasoningLines = maxHeight
		fmt.Fprint(c.Out, c.color(colorReasoning, "[...truncated]\n"))
		if bw, ok := c.Out.(*bufio.Writer); ok {
			bw.Flush()
		}
		return
	}
	fmt.Fprint(c.Out, c.color(colorReasoning, delta))
	c.reasoningLines += newLines
	if bw, ok := c.Out.(*bufio.Writer); ok {
		bw.Flush()
	}
}

// OnContent is called for each streaming text chunk from the LLM.
//
// WHAT:  Streams LLM text content to the console.
// PARAMS: delta — the text chunk from the LLM.
func (c *Console) OnContent(delta string) {
	if c.turnAborting.Load() {
		return
	}
	c.lockOutput()
	defer c.unlockOutput()
	if c.reasoningStarted {
		fmt.Fprintln(c.Out)
		fmt.Fprintln(c.Out) // blank line after reasoning
		c.reasoningStarted = false
		c.reasoningLines = 0
	}
	if !c.contentStarted {
		// Blank line after tool group before resuming content.
		if c.toolsStarted {
			fmt.Fprintln(c.Out)
			c.toolsStarted = false
		}
		c.contentStarted = true
		c.ensureLineBreakBeforeBlock()
		fmt.Fprint(c.Out, c.color(colorOrange, c.bold("[BLAZE]")))
		fmt.Fprintln(c.Out)
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
// Prints the tool purpose immediately and leaves the line open so OnToolResult can append
// the final status without redrawing wrapped terminal lines.
//
// WHAT:  Buffers tool call args and handles tool group header.
// PARAMS: name — tool name; args — formatted arguments (purpose text).
func (c *Console) OnToolCall(name string, args string) {
	if c.turnAborting.Load() {
		return
	}
	c.lockOutput()
	defer c.unlockOutput()
	c.ensureLineBreakBeforeBlock()
	c.toolsStarted = true

	// Insert blank line before the first tool in a group after content,
	// then reset contentStarted so consecutive tool calls stay compact.
	if c.contentStarted {
		fmt.Fprintln(c.Out)
		c.contentStarted = false
	}

	c.lastToolArgs = args

	if args != "" {
		fmt.Fprintf(c.Out, "%s %s …", c.color(colorGreen, toolEmoji(name)), c.color(colorCtx, args))
	}
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

// OnToolResult is called after a tool has finished.
// Prints a single line: tool emoji + purpose + status symbol.
// Appends the status to the pending line printed by OnToolCall to avoid broken output
// when the purpose wraps across multiple terminal lines.
// Success: ✔️. Error: ✖️ <message>. Timeout: ⏱ <message>.
//
// WHAT:  Displays tool result inline with the deferred tool call line.
// PARAMS: name — tool name; result — the raw tool output.
func (c *Console) OnToolResult(name string, result string) {
	c.renderToolResult(name, result, true, false)
}

func (c *Console) renderToolResult(name string, result string, showContext bool, showDetail bool) {
	if c.turnAborting.Load() {
		c.lastToolArgs = ""
		return
	}
	c.lockOutput()
	defer c.unlockOutput()
	badge, content, colorCode := parseToolResult(result)
	if showDetail && badge == "DONE" && strings.HasPrefix(strings.TrimSpace(result), "ok") {
		content = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(result), "ok"))
	}
	icon := c.color(colorGreen, toolEmoji(name))
	args := c.lastToolArgs
	c.lastToolArgs = ""

	switch badge {
	case "DONE":
		ctx := ""
		if showContext && c.lastPromptTokens > 0 {
			ctx = "  " + c.color(colorCtx, "CTX: "+formatCompactInt(c.lastPromptTokens))
		}
		detail := ""
		if showDetail && content != "" {
			detail = "  " + c.color(colorBrightGreen, content)
		}
		if args != "" {
			fmt.Fprintf(c.Out, " %s%s%s\n", c.color(colorBrightGreen, "✔️"), detail, ctx)
		} else {
			fmt.Fprintf(c.Out, "%s %s%s%s\n",
				icon,
				c.color(colorBrightGreen, "✔️"),
				detail,
				ctx,
			)
		}
	case "ERROR":
		if content != "" {
			content = strings.ReplaceAll(content, "\n", " ")
			if len(content) > 200 {
				content = content[:197] + "..."
			}
		}
		if args != "" && showDetail {
			fmt.Fprintf(c.Out, " %s %s\n", c.color(colorCode, "✖️"), c.color(colorCode, content))
		} else if args != "" {
			fmt.Fprintf(c.Out, " %s\n", c.color(colorCode, "✖️"))
			if content != "" {
				fmt.Fprintf(c.Out, "  %s\n", c.color(colorCode, content))
			}
		} else {
			fmt.Fprintf(c.Out, "%s %s %s\n",
				icon,
				c.color(colorCode, "✖️"),
				c.color(colorCode, content),
			)
		}
	case "TIMEOUT":
		if content != "" {
			content = strings.ReplaceAll(content, "\n", " ")
		}
		if args != "" && showDetail {
			fmt.Fprintf(c.Out, " %s %s\n", c.color(colorCode, "⏱"), c.color(colorCode, content))
		} else if args != "" {
			fmt.Fprintf(c.Out, " %s\n", c.color(colorCode, "⏱"))
			if content != "" {
				fmt.Fprintf(c.Out, "  %s\n", c.color(colorCode, content))
			}
		} else {
			fmt.Fprintf(c.Out, "%s %s %s\n",
				icon,
				c.color(colorCode, "⏱"),
				c.color(colorCode, content),
			)
		}
	}
	c.lineOpen = false
	c.startSpinnerLocked("Waiting")
}

// RequestSudoApproval prompts the user for confirmation before executing a sudo command.
// On approval, reads a hidden password. The password is never stored or echoed.
//
// WHAT:  Collects user approval and sudo password for a shell command.
// HOW:   Shows the command, asks Y/N, then reads hidden input.
// PARAMS: command — the shell command that contains sudo.
// RETURNS: approved — true if user confirmed; password — the sudo password, empty if declined.
func (c *Console) RequestSudoApproval(command string) (approved bool, password string) {
	c.lockOutput()
	defer c.unlockOutput()
	c.ensureLineBreakBeforeBlock()
	label := c.color(colorOrange, "sudo")
	fmt.Fprintf(c.Out, "\n%s: %s\n", label, command)
	fmt.Fprintf(c.Out, "%s [y/N] ", c.color(colorBrightGreen, "Execute with sudo?"))

	line, err := c.Reader.ReadLine()
	if err != nil {
		return false, ""
	}
	line = strings.TrimSpace(line)
	if line != "y" && line != "Y" {
		fmt.Fprintln(c.Out)
		return false, ""
	}

	pass, err := c.Reader.ReadHiddenInput("Sudo password: ")
	if err != nil || pass == "" {
		fmt.Fprintln(c.Out)
		return false, ""
	}
	return true, pass
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

// formatTurnError rewrites low-level runtime/provider failures into short
// console-facing messages when the underlying cause is known.
//
// WHAT:  Maps turn-level errors to concise user-visible console text.
// WHY:   Raw provider/network errors are noisy and hide the actionable reason.
// HOW:   Matches known timeout signatures and falls back to the original error.
// PARAMS: err — the error returned by runAgentTurn.
// RETURNS: string — formatted message prefixed with "error: ".
func formatTurnError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "provider stream idle timeout"):
		return "error: provider stream stalled for 3m with no events"
	case strings.Contains(msg, "timeout awaiting response headers"):
		return "error: provider timed out before starting the response"
	default:
		return "error: " + msg
	}
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
		rendered = c.color(colorBlue, c.bold(rendered))
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
		return c.bold(s)
	})
	text = reUnderscoreItalic.ReplaceAllStringFunc(text, func(match string) string {
		inner := match[1 : len(match)-1]
		return c.color(colorItalic, inner)
	})
	text = c.toggleDelimited(text, "*", func(s string) string {
		return c.color(colorItalic, s)
	})
	text = c.toggleDelimited(text, "`", func(s string) string {
		return c.color(colorOrange, s)
	})
	text = c.renderLinks(text)
	return text
}

// renderLinks replaces Markdown links with a terminal-friendly format.
//
// WHAT:  Converts [text](url) to text (url), coloring the URL portion.
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
		return label + " " + c.color(colorPurple, "("+url+")")
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
// HOW:   Uses raw-mode input for Tab mode cycling, loops reading input.
// RETURNS: error if a fatal error occurs.
func (c *Console) Run() error {
	c.showStartupSplash()
	return c.runTTY()
}

// runTTY runs the REPL loop with raw-mode input for Tab detection.
// No background goroutine — input is read directly at the prompt.
// During streaming, abort is via SIGINT only (no queued input).
//
// WHAT:  REPL with raw-mode Tab detection.
// WHY:   Tab key requires raw terminal mode to detect.
func (c *Console) runTTY() error {
	for {
		prompt := c.promptLabel()
		fmt.Fprint(c.Out, prompt)
		c.Reader.SetPrompt(prompt)
		line, event, err := c.Reader.ReadEvent()
		if err == io.EOF {
			fmt.Fprintln(c.Out)
			return nil
		}
		if err != nil {
			return fmt.Errorf("input error: %w", err)
		}

		// Handle mode switch event. Save partial input so the next
		// ReadEvent call can restore it via the prefill mechanism.
		// First switch: overwrites old prompt line with spaces then status.
		// Consecutive: goes up one line, overwrites previous status.
		if event == "mode_switch" {
			savedText := line
			if _, switchErr := c.Agent.NextMode(); switchErr != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("mode switch error: %v", switchErr)))
			} else {
				newStatus := fmt.Sprintf("[mode: %s | %s]", c.Agent.CurrentMode.Name, c.Agent.ModelID)
				c.writeSwitchStatus(newStatus)
			}
			c.Reader.prefill = savedText
			continue
		}

		// Handle model switch event (Ctrl+\). Same overwrite behavior.
		if event == "model_switch" {
			savedText := line
			if switchErr := c.Agent.NextFavoriteModel(); switchErr != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("model switch error: %v", switchErr)))
			} else {
				newStatus := fmt.Sprintf("[mode: %s | %s]", c.Agent.CurrentMode.Name, c.Agent.ModelID)
				c.writeSwitchStatus(newStatus)
			}
			c.Reader.prefill = savedText
			continue
		}

		// Handle reasoning display toggle (Ctrl+T).
		if event == "reasoning_switch" {
			savedText := line
			c.Agent.Config.ShowReasoning = !c.Agent.Config.ShowReasoning
			if err := c.Agent.Config.Save(); err != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("reasoning toggle error: %v", err)))
			} else {
				state := "disabled"
				if c.Agent.Config.ShowReasoning {
					state = "enabled"
				}
				newStatus := "[reasoning: " + state + "]"
				c.writeSwitchStatus(newStatus)
			}
			c.Reader.prefill = savedText
			continue
		}

		// Handle add/remove favorites (Ctrl+F / Ctrl+R).
		if event == "fav_add" {
			savedText := line
			if err := c.Agent.Config.AddFavorite(c.Agent.ModelID); err != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("add favorite error: %v", err)))
			} else if err := c.Agent.Config.Save(); err != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("save config error: %v", err)))
			} else {
				newStatus := "[favorite: " + c.Agent.ModelID + " +]"
				c.writeSwitchStatus(newStatus)
			}
			c.Reader.prefill = savedText
			continue
		}
		if event == "fav_remove" {
			savedText := line
			removed, err := c.Agent.Config.RemoveFavorite(c.Agent.ModelID)
			if err != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("remove favorite error: %v", err)))
			} else if !removed {
				newStatus := "[not in favorites: " + c.Agent.ModelID + "]"
				c.writeSwitchStatus(newStatus)
			} else if err := c.Agent.Config.Save(); err != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("save config error: %v", err)))
			} else {
				newStatus := "[favorite: " + c.Agent.ModelID + " -]"
				c.writeSwitchStatus(newStatus)
			}
			c.Reader.prefill = savedText
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

		// Reset the sticky switch status line: the user is now sending
		// a message, so the next switch will start with a fresh line.
		c.switchLineActive = false
		c.switchLineWidth = 0

		fmt.Fprintln(c.Out)

		c.resetTurnState()

		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)

		turnErr := c.runAgentTurn(input, interrupts)
		if turnErr != nil && !errors.Is(turnErr, runtime.ErrTurnAborted) {
			fmt.Fprintln(c.Out, c.color(colorRed, formatTurnError(turnErr)))
			c.lineOpen = false
		}
		signal.Stop(interrupts)
		c.flushPendingContent()
		fmt.Fprintln(c.Out)
		c.lineOpen = false
		c.responseSeparator()
	}
}

// writeSwitchStatus prints the mode/model status line, overwriting any
// previous switch line with exactly spaced padding to avoid ANSI artifacts.
//
// WHAT:  Reliably prints a switch status line using space-padding for clearing.
// WHY:   \033[K can leave artifacts when the old line contains ANSI color codes.
//
//	Spaces are universally reliable regardless of terminal.
//
// HOW:   If a switch line is already present, moves up with \033[F first.
//
//	Then pads with spaces matching the maximum of old and new line widths,
//	returns to column 0, and prints the new status.
//
// PARAMS: status — the formatted status line text (without trailing newline).
func (c *Console) writeSwitchStatus(status string) {
	if c.switchLineActive {
		fmt.Fprint(c.Out, "\033[F") // move up to previous status line
	}
	// Determine padding width: at least the new status, and at least the
	// previous status width (to cover any longer old content). Add 10 for
	// safety margin on the initial switch where the old prompt may be longer.
	width := len(status)
	if c.switchLineWidth > width {
		width = c.switchLineWidth
	}
	if !c.switchLineActive {
		width += 10 // first switch: old prompt + prefill may be longer than status
	}
	fmt.Fprintf(c.Out, "\r%s\r", strings.Repeat(" ", width))
	fmt.Fprintf(c.Out, "%s\n", status)
	c.switchLineWidth = len(status)
	c.switchLineActive = true
}

func (c *Console) resetTurnState() {
	c.contentStarted = false
	c.contentBuffer = ""
	c.inCodeBlock = false
	c.lastPromptTokens = 0
	c.lineOpen = false
	c.toolsStarted = false
	c.reasoningStarted = false
	c.reasoningLines = 0
}

// runAgentTurn executes one agent turn with SIGINT-only abort.
//
// WHAT:  Simplified turn execution for TTY mode.
// WHY:   TTY mode reads input directly at the prompt; no goroutine for queued input.
func (c *Console) runAgentTurn(input string, interrupts <-chan os.Signal) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.startSpinner("Connecting")
	defer c.stopSpinner()

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

// abortCurrentTurn stops visible turn activity and requests cancellation from the runtime.
func (c *Console) abortCurrentTurn(cancel context.CancelFunc) {
	c.turnAborting.Store(true)
	cancel()
	c.lockOutput()
	defer c.unlockOutput()
	c.contentBuffer = ""
	if c.lineOpen {
		fmt.Fprintln(c.Out)
		c.lineOpen = false
	}
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
	case "/auth":
		if arg != "openai" && arg != "chatgpt" {
			return true, false, fmt.Errorf("usage: /auth openai")
		}
		if err := c.authenticateChatGPT(); err != nil {
			return true, false, err
		}
		return true, false, nil
	case "/exit":
		if err := c.Agent.CloseSession(); err != nil {
			return true, true, fmt.Errorf("cannot close session: %w", err)
		}
		fmt.Fprintln(c.Out, "Goodbye.")
		return true, true, nil
	case "/model":
		if arg == "" {
			if err := c.interactiveSelectModel(); err != nil {
				fmt.Fprintln(c.Out, c.color(colorRed, err.Error()))
			}
			return true, false, nil
		}
		if arg == "+" {
			if err := c.Agent.Config.AddFavorite(c.Agent.ModelID); err != nil {
				return true, false, err
			}
			if err := c.Agent.Config.Save(); err != nil {
				return true, false, fmt.Errorf("cannot save config: %w", err)
			}
			fmt.Fprintf(c.Out, "Added to favorites: %s\n", c.Agent.ModelID)
			return true, false, nil
		}
		if arg == "-" {
			removed, err := c.Agent.Config.RemoveFavorite(c.Agent.ModelID)
			if err != nil {
				return true, false, err
			}
			if !removed {
				fmt.Fprintf(c.Out, "Not in favorites: %s\n", c.Agent.ModelID)
				return true, false, nil
			}
			if err := c.Agent.Config.Save(); err != nil {
				return true, false, fmt.Errorf("cannot save config: %w", err)
			}
			fmt.Fprintf(c.Out, "Removed from favorites: %s\n", c.Agent.ModelID)
			if len(c.Agent.Config.FavoriteModels) > 0 {
				if err := c.Agent.NextFavoriteModel(); err != nil {
					return true, false, err
				}
				fmt.Fprintf(c.Out, "Switched to: %s\n", c.Agent.ModelID)
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
// authenticateChatGPT connects the console to ChatGPT through browser OAuth.
//
// WHAT:  Performs OAuth, installs the provider, and persists the account's live models.
// WHY:   Provider integration belongs to the primary console transport.
// HOW:   Prints the authorization URL, waits for the localhost callback, then saves config.
// RETURNS: error if OAuth, provider installation, or config persistence fails.
func (c *Console) authenticateChatGPT() error {
	credential, err := provider.AuthenticateChatGPT(context.Background(), c.Out)
	if err != nil {
		return err
	}
	if err := provider.InstallChatGPTProvider(c.Agent.Config, credential); err != nil {
		return err
	}
	models, err := c.Agent.ListProviderModels(provider.ChatGPTOAuthProviderName)
	if err != nil {
		return fmt.Errorf("cannot retrieve ChatGPT models: %w", err)
	}
	if len(models) == 0 {
		return fmt.Errorf("ChatGPT provider returned no models")
	}
	for _, model := range models {
		modelID := provider.ChatGPTOAuthProviderName + "/" + model
		if err := c.Agent.Config.AddFavorite(modelID); err != nil {
			return fmt.Errorf("cannot save ChatGPT model %q: %w", modelID, err)
		}
	}
	if err := c.Agent.Config.Save(); err != nil {
		return fmt.Errorf("cannot save ChatGPT provider: %w", err)
	}
	fmt.Fprintln(c.Out, "ChatGPT provider connected.")
	fmt.Fprintln(c.Out, "Available models:")
	for _, model := range models {
		fmt.Fprintf(c.Out, "  %s/%s\n", provider.ChatGPTOAuthProviderName, model)
	}
	fmt.Fprintln(c.Out, "Use /model to select a ChatGPT model.")
	return nil
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
