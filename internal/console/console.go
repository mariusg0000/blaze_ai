// console.go — console REPL transport implementing the handler contract.
// Implements OnContent, OnToolCall, OnToolResult. Auto-detects TTY for colors, spinner, and
// visual separators. Handles slash commands (/exit, /model, /cd) before reaching the agent core.
// Renders Markdown incrementally during streaming. Non-TTY output is plain text.
// Layer: transport (console). Dependencies: internal/runtime, internal/config.
package console

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"blazeai/internal/runtime"
)

// ANSI color codes for TTY output.
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorOrange = "\033[33m"
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
		Out:      out,
		In:       in,
		IsTTY:    isTTY,
		Agent:    agent,
		Reader:   NewReader(in, isTTY),
		Spinner:  NewSpinner(out, isTTY),
		lineOpen: false,
	}
}

// ensureLineBreakBeforeBlock closes the current inline content line before block output.
//
// WHAT:  Forces separators and tool markers onto a fresh line after streamed content.
func (c *Console) ensureLineBreakBeforeBlock() {
	if c.lineOpen {
		fmt.Fprintln(c.Out)
		c.lineOpen = false
	}
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
	if !c.contentStarted {
		c.contentStarted = true
		c.Spinner.Stop()
		fmt.Fprint(c.Out, c.color(colorOrange, c.bold("[BLAZE] ")))
		c.lineOpen = true
	}
	fmt.Fprint(c.Out, delta)
	c.lineOpen = !strings.HasSuffix(delta, "\n")
}

// OnToolCall is called before a tool is executed.
// Stops the spinner on first event, prints a compact one-line tool call marker.
//
// WHAT:  Displays a tool call notification.
// PARAMS: name — tool name; args — raw JSON arguments.
func (c *Console) OnToolCall(name string, args json.RawMessage) {
	if !c.contentStarted {
		c.contentStarted = true
		c.Spinner.Stop()
	}
	c.ensureLineBreakBeforeBlock()
	c.separator()
	argStr := string(args)
	if len(argStr) > 80 {
		argStr = argStr[:77] + "..."
	}
	fmt.Fprintf(c.Out, "%s %s(%s)\n",
		c.color(colorGreen, c.bold("[TOOL CALL]")),
		c.bold(name),
		argStr,
	)
	c.lineOpen = false
}

// OnToolResult is called after a tool has finished.
// Prints a compact one-line tool result marker.
//
// WHAT:  Displays a tool result notification.
// PARAMS: name — tool name; result — the tool output.
func (c *Console) OnToolResult(name string, result string) {
	status := "ok"
	color := colorGreen
	if strings.HasPrefix(result, "error") || strings.HasPrefix(result, "timeout") {
		status = "error"
		color = colorRed
	}
	resultPreview := result
	if len(resultPreview) > 100 {
		resultPreview = resultPreview[:97] + "..."
	}
	resultPreview = strings.ReplaceAll(resultPreview, "\n", " ")
	fmt.Fprintf(c.Out, "%s %s [%s] %s\n",
		c.color(color, c.bold("[TOOL RESPONSE]")),
		c.bold(name),
		status,
		resultPreview,
	)
	c.lineOpen = false
	c.separator()
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
		c.lastPromptTokens = 0
		c.lineOpen = false
		c.Spinner.Start()

		// Run the agent turn.
		if err := c.Agent.RunTurn(input); err != nil {
			c.Spinner.Stop()
			fmt.Fprintln(c.Out, c.color(colorRed, fmt.Sprintf("error: %v", err)))
			c.lineOpen = false
		}
		fmt.Fprintln(c.Out)
		c.lineOpen = false
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
