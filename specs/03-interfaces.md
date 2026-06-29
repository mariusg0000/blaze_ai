# 03 - Interfaces

## Overview
- This spec defines the BlazeAI interface layer: the handler contract between the agent core and transports, the console implementation, and the postponed web transport.
- Product intent is defined in `01-product-scope.md`.
- Runtime mechanics are defined in `02-core-runtime.md`.
- Platform behavior is defined in `04-platform-ops.md`.

## Architecture Principle
- The agent core does not know about console or web.
- The agent core communicates with the active transport through a handler contract.
- Each transport implements the handler contract.
- Console is the first and complete transport.
- Web is a postponed transport that will implement the same contract over a browser connection.

## Handler Contract
- The agent core calls a handler interface during execution.
- The handler has three callbacks:
  1. `OnContent(delta)`: called for each streaming text chunk from the LLM.
  2. `OnToolCall(name, args)`: called before a tool is executed.
  3. `OnToolResult(name, status)`: called after a tool has finished.
- The handler is the only boundary between the agent core and the user-facing transport.
- Console implements this handler now.
- Web will implement this handler later, forwarding events over WebSocket or SSE.
- The contract may be extended only when a concrete need is proven by a real transport.

## Console Interface

### Console Principles
- The console is a simple interactive REPL.
- The console is not a full-screen TUI.
- The console should look clean, modern, and readable.
- The console should encourage Markdown-formatted LLM responses and render them properly.

### Console Detection
- The console is terminal-only (TTY). ANSI colors, bold labels, and visual separators are always active.
- Non-TTY fallback has been removed. The console requires a real terminal.

### Input

#### Prompt Label
- The input prompt label is `[USER/(provider/model)] >`.
- The `provider/model` part reflects the currently selected model.
- The label is colored and bold on TTY.

#### Single-Line Input
- By default, Enter sends the input.

#### Multiline Input
- If pasted text contains newlines, the console waits for an empty line before sending.
- This allows pasting multi-line content without premature submission.
- An empty line (just Enter) signals the end of a pasted block.

#### Slash Commands
- Slash commands start with `/`.
- Known commands are handled by the console before reaching the agent core.
- Known commands do not create conversation messages.
- Unknown slash commands are passed to the agent core as a normal user message.

#### `/exit`
- Closes the current session cleanly.
- Marks the session as cleanly closed in metadata.
- Prints a short goodbye message and exits.

#### `/model`
- Without argument: prints the list of favorite models from config.
- With argument: sets the current model in `provider/model_name` form.
- The model must exist in config; otherwise the runtime reports a clear error.

#### `/cd`
- Changes the current work folder.
- If the path is invalid, the runtime reports a clear error and keeps the current work folder.
- Changing the work folder affects tool execution and the `AGENTS.md` source in prompt build.

### Output

#### Labels
- All labels are colored and bold on TTY.
- `[USER/(provider/model)]`: user message marker, colored blue.
- `[TOOL CALL]`: tool invocation marker, colored green.
- `[TOOL RESPONSE]`: tool result marker, colored green on success, red on error.
- `[BLAZE]`: agent response marker, colored orange.
- Errors are printed in red.

#### Tool Display
- Tool calls and responses are displayed compactly on one line.
- Tool arguments are truncated to a fixed character limit.
- Tool calls and responses are grouped visually.
- Multiple consecutive tool calls appear as a block, separated from user and Blaze messages.

#### Visual Separators
- Visual separator lines are drawn between message types.
- User messages, tool blocks, and Blaze messages are visually distinct.
- Separator style is minimal and clean, not heavy box drawing.

#### Markdown Rendering
- LLM responses are rendered as terminal Markdown.
- Markdown rendering is incremental during streaming.
- Headings, bold, italic, code blocks, lists, tables, and links are rendered with terminal formatting.

#### Streaming
- Assistant text is displayed incrementally as it arrives from the LLM.
- The Blaze label is printed before the first content chunk.
- Content chunks are written directly to the console as they arrive.

## Web Transport

### Status
- Web transport is postponed.
- Web is not implemented in this phase.
- Web remains a valid architectural direction over the same handler contract.

### Future Behavior
- When implemented, web will expose a browser-based terminal-style interface.
- The web UI will imitate a terminal session, not a generic chat UI.
- The web transport will implement the same handler contract as the console.
- Events will be forwarded to the browser over WebSocket or SSE.
- The web UI will render Markdown to HTML on the client side.
- The web UI will display the same labels, separators, and tool grouping as the console.

## Error Display
- Runtime errors are printed as simple red text.
- Errors do not use a special visual format.
- After an error, the session continues if the error is recoverable.
- Fatal errors stop the session and print a clear message before exit.
