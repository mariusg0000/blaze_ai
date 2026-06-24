# Implementation Plan: Work Modes

## Overview
Add persistent "work modes" to BlazeAI. Each mode has a name, an assigned model, and an optional directive. The user cycles modes with Tab at the input prompt. The active mode's model drives the LLM. The active directive is injected into the last message sent to the LLM (volatile — never persisted in session.json). Modes are defined in `config.json` and managed via the `customize_me` skill.

Confirmed decisions: `lastMode` persists across sessions; Tab switches modes (no autocomplete); Option A — append directive to last message content; ignore AGENTS.md conflict for now; validation required; pre-create a "default" mode at first run; teach `customize_me` the new structure.

---

## 1. Config Structure

File: `internal/config/config.go`

### New types

```go
// Mode defines one work mode: a name, its assigned model, and an optional directive.
type Mode struct {
    Name      string `json:"name"`
    Model     string `json:"model"`
    Directive string `json:"directive,omitempty"`
}
```

### Config fields
Add to `Config`:
```go
Modes    []Mode `json:"modes,omitempty"`
LastMode string `json:"last_mode,omitempty"`
```

### Validation (in `Validate()`)
- At least one mode exists.
- Mode `name` values are unique (case-sensitive).
- Every mode `model` is valid `provider/model_name` and references an existing provider (reuse `validateModelFormat` + `validateModelProvider`).
- The `LastMode` (if non-empty) must reference an existing mode name. If it references a removed mode, reset to the first mode name (do not hard-error — graceful).

### First-run setup (firstrun.go)
After assigning the `default` role and favorite models, pre-create one mode:
```go
cfg.Modes = []config.Mode{{
    Name:  "default",
    Model: modelID, // the assigned default role model
}}
cfg.LastMode = "default"
```
This guarantees a valid starting state. No interactive prompt for modes on first run — the user creates more modes later via `customize_me`.

### Default() helper
Ensure `DefaultConfig()` does NOT pre-populate `Modes` (first-run is responsible). Keep `Modes` nil/empty as the zero state, and validation should only enforce modes when config is loaded via `LoadFrom` (not the zero default used for first-run scaffolding).

---

## 2. Runtime — Mode State and Switching

File: `internal/runtime/runtime.go`

### Agent fields
Add:
```go
CurrentMode *config.Mode
```
Pointer into `Config.Modes` (the active mode entry). Kept as a pointer so model updates mutate the slice entry in place.

### NewAgent changes
After `modelID` resolution from `cfg.LastModel`, initialize the active mode:
1. Find mode by `cfg.LastMode` in `cfg.Modes`.
2. If found and its model validates → use it: `agent.CurrentMode = &cfg.Modes[idx]`, and set `modelID = mode.Model` (the mode's model takes priority over `LastModel`).
3. If `LastMode` is empty or not found → fall back to first mode, or to `cfg.Roles.Default` if no modes exist.

This keeps backward compatibility: configs without `modes` still work (Agent runs without a mode; directive injection is skipped).

### New method: `SetMode(name string) error`
- Find mode by name in `cfg.Modes`. Error if not found.
- Set `agent.CurrentMode = &cfg.Modes[idx]`.
- Recreate provider client with `mode.Model` (reuse the logic from `SetModel`). Validation against config providers.
- Set `cfg.LastMode = name`, persist `cfg.Save()`.
- Update `agent.ModelID = mode.Model`.
Returns error if model invalid or provider cannot be created.

### SetModel extension
`SetModel(modelID)` currently switches the active model. Extend it:
- If `agent.CurrentMode != nil` → also update `agent.CurrentMode.Model = modelID` and persist (so the new model sticks to the active mode).
- Keep existing behavior otherwise (lastModel, provider recreation, save).

This handles "/model changes while in a mode → model sticks to mode" requirement.

### CycleMode() helper
Return the active mode's current mode name.
`NextMode() (*config.Mode, error)` — returns the next mode in `cfg.Modes` cyclically. Returns error if no modes exist. Used by the console Tab handler.

---

## 3. Directive Injection (Volatile)

File: `internal/runtime/runtime.go`

### Injection point
Inside `RunTurn`, in the tool-call loop, between `StripReasoningFromPayload` (line 184) and `Provider.Stream` (line 196):

```go
messages = a.Compactor.StripReasoningFromPayload(messages)

// Inject volatile mode directive into the last message (copy, never mutate session).
if a.CurrentMode != nil && strings.TrimSpace(a.CurrentMode.Directive) != "" {
    messages = injectDirective(messages, a.CurrentMode.Directive)
}

... a.Provider.Stream(ctx, messages, toolDefs, ...) ...
```

### injectDirective(messages, directive) function
```go
func injectDirective(messages []session.Message, directive string) []session.Message {
    if len(messages) == 0 {
        return messages
    }
    out := make([]session.Message, len(messages))
    copy(out, messages)
    last := &out[len(out)-1]
    content, _ := last.Content.(string)
    last.Content = content + "\n\n[MODE DIRECTIVE]\n" + directive
    return out
}
```
Notes:
- Copies the slice and mutates only the copy's last element — session.json stays intact.
- `prompt.json` debug write (line 187–192) happens AFTER injection — that's intentional so the debug file reflects exactly what was sent. Acceptable: prompt.json is a debug artifact, not the persisted session. (Alternative: write debug file before injection. Decide during implementation — either is fine; I'll write before injection to keep prompt.json matching the session, and avoid leaking directive into the debug artifact.)

### Determined order
1. Build → 2. Strip reasoning → 3. Write debug prompt.json → 4. Inject directive → 5. Stream.

So debug file matches session.json (no directive), and only the in-memory payload sent to the provider carries the directive.

Directive injected on every loop iteration (first turn = last message is user; subsequent iterations = last message is the most recent tool result). This matches the requirement: "injectată de fiecare dată la ultimul mesaj ... indiferent dacă este user sau tool response."

---

## 4. Console — Tab Cycle and Mode Display

Files: `internal/console/reader.go`, `internal/console/console.go`

### Tab detection (raw mode, x/term)
The current `Reader` uses `bufio.Scanner`, which reads whole lines and cannot detect Tab. We add a raw-mode input path used only when TTY, keeping `bufio.Scanner` for non-TTY/piped input.

New `ttyReader` (internal file `reader_tty.go` or in reader.go):
- On `NewReader` with `isTTY == true`, switch to raw mode (`term.MakeRaw(int(os.Stdin.Fd()))`), store old state for restore on `Close()`.
- `ReadLine()`: read bytes one at a time from `os.Stdin`.
  - `0x09` (Tab) → emit a special signal (return a sentinel error `ErrModeSwitch` or a sentinel string). Actually cleaner: expose a separate method `ReadLineOrEvent()` returning `input string, event Event` where `Event == ModeSwitch`. The console loop handles the event, then resumes reading.
  - `0x0a` / `0x0d` (Enter) → return current line.
  - `0x7f` / `0x08` (Backspace) → delete last byte, write `\b \b`.
  - Printable byte → append, echo.
  - `Ctrl-D` on empty line → return `io.EOF`.
- Minimal line editing: no arrow keys, no history (those are separate features). Keep it small per KISS.
- `Close()` → `term.Restore(fd, old)`.

Non-TTY: keep the current `bufio.Scanner` path unchanged (piped input).

### Console Run loop
Currently:
```go
inputs := c.startInputReader()
event, ok := <-inputs
```
Extend the `inputEvent` struct with an `event string` field ("mode_switch"). When the reader reports `mode_switch`:
- Call `c.Agent.NextMode()` → switch to next mode (also recreates provider + persists).
- Print a line showing the new mode + model, e.g. `[mode: planning | openai/gpt-4o]`.
- Re-print the prompt label (now showing the new mode) and continue reading (do not dispatch a turn).

### Prompt label update
`promptLabel()` currently: `[USER/openai/gpt-4o] >`. Extend to include mode name when a mode is active:
`[USER/openai/gpt-4o|planning] >` — keep `[USER/<model>]` shell when no mode (backward compat).
Color: keep current blue+bold.

### No autocomplete
Tab ONLY cycles modes. No completer function, no `/model <Tab>` completion. Simple, per requirement.

### Non-TTY
Tab detection disabled. Mode stays as loaded from `LastMode`. Acceptable: piped input doesn't need interactive mode cycling.

---

## 5. customize_me Skill

File: `skills/customize_me.md`

### Add to [DETAILS] a new section "## Work Modes":

```markdown
## Work Modes (config.json)
Modes are part of the runtime config at {APP_HOME}/config/config.json. Each mode
binds a model and an optional directive that is injected into the last message
sent to the LLM (volatile — not stored in session history).

### Structure
```json
"modes": [
  {
    "name": "default",
    "model": "provider/model_name",
    "directive": ""
  },
  {
    "name": "planning",
    "model": "openai/gpt-4o",
    "directive": "You are in planning mode. Use only read-only tools and propose a plan."
  }
],
"last_mode": "default"
```

### Rules
- `name`: unique, non-empty.
- `model`: must exist in favorite_models and reference a configured provider.
- `directive`: free text. Empty string = no directive injected.
- At least one mode must exist (the `default` mode is pre-created on first run).
- `last_mode`: persists the active mode between sessions; must match an existing mode name.

### Operations you can perform
- Create a new mode: append an entry to `modes` and persist config. Validate with the rules above.
- Edit a mode's directive or model: find by `name`, update, persist.
- Delete a mode: remove from `modes`. If it was `last_mode`, set `last_mode` to the first remaining mode. Never delete the last remaining mode.
- Switch active mode at runtime: use the `shell`-independent mechanism the runtime exposes — but at config level, set `last_mode` and save.
- After any edit, validate config integrity (unique names, valid models, provider references).

### Directive behavior
The directive is appended to the last message of the payload sent to the LLM on every LLM call while the mode is active. It is not stored in session.json. Use it to constrain agent behavior for the current task (e.g. read-only, quick/cheap, verbose, etc.). Keep directives short and imperative.
```

This teaches the skill to read/write `modes` and `last_mode`. The LLM, when asked to create a mode, edits config.json via shell and follows these rules. We do NOT add a tool for this — config edits go through the existing pattern (read config, modify JSON, save).

---

## 6. Validation Tests

File: `internal/config/config_test.go` (extend)

- `TestValidateModesEmpty` — missing `modes` is invalid only when not first-run scaffolding. (We'll allow empty Modes in Default() but require it in real loaded configs.) Decision: validation enforces modes when `len(Modes)==0` → error, EXCEPT accept nil during first-run flow. Simpler: `Validate()` returns error if modes empty. First-run always populates modes before validation. We'll make sure first-run sets modes before any validate call.
- `TestValidateDuplicateModeNames` — two modes with same name → error.
- `TestValidateModeModelBadFormat` — mode model not `provider/model_name` → error.
- `TestValidateModeModelUnknownProvider` — mode references missing provider → error.
- `TestValidateLastModeMissing` — `last_mode` references missing mode → error (or auto-reset to first — decided above to error loudly per "no fallbacks" spec mandate). Reconsider: spec says "no fallbacks on configuration errors, errors stop runtime with clear messages." So an invalid `last_mode` should be a hard validation error, not a silent reset. I'll implement validation error for `last_mode` referencing a non-existent mode.
- Round-trip save/load preserves modes.

Runtime tests (`internal/runtime/runtime_test.go`):
- `TestSetMode` — switch mode, model recreated, persisted.
- `TestSetModelUpdatesMode` — SetModel while mode active updates `CurrentMode.Model`.
- `TestDirectiveInjection` — `injectDirective` leaves original messages slice intact, appends to last in copy only.

Console tests:
- `TestTabSwitch` — simulate Tab event → `NextMode` called.
- `TestPromptLabelWithMode` — label contains mode name when mode active.

---

## 7. File-Affected Summary

| File | Change |
|---|---|
| `internal/config/config.go` | + `Mode` type, `Modes`/`LastMode` fields, validation |
| `internal/config/config_test.go` | + mode validation tests |
| `firstrun.go` | pre-create `default` mode + `last_mode` |
| `internal/runtime/runtime.go` | `CurrentMode` field, `SetMode`, `NextMode`, `injectDirective`, `SetModel` extension |
| `internal/runtime/runtime_test.go` | + mode tests |
| `internal/console/reader.go` | raw-mode Tab detection on TTY |
| `internal/console/console.go` | Tab event handling, mode display in prompt label, `Run()` extension |
| `internal/console/console_test.go` | + Tab/mode tests |
| `skills/customize_me.md` | + "Work Modes" section teaching modes structure |

New files (optional, if splitting):
- `internal/console/reader_tty.go` — raw-mode reader (only if reader.go gets too big; otherwise keep in reader.go).

---

## 8. Constraints

- Non-TTY: Tab disabled, bufio.Scanner path unchanged.
- No rollback of prompt caching considerations for modes (mode directive changes per turn, but directive is in the message body, not sysprompt — so sysprompt cache is unaffected).
- Backward compatibility: configs without `modes` continue to load but validation will error with a clear message prompting migration. First-run populates modes, so fresh installs are fine.
- Handler contract (`OnContent`/`OnToolCall`/`OnToolResult`) unchanged — modes are an input/config concern.

---

## 9. Validation Plan

1. `go build ./...`
2. `go test ./...`
3. Manual: fresh first-run → `config.json` has `modes: [{default, ...}]`, `last_mode: default`.
4. Manual: Tab in TTY → cycles to next mode, label updates, config `last_mode` persists.
5. Manual: `/model provider/x` while in a mode → mode `.model` updates, persists, next session uses it.
6. Manual: mode with non-empty directive → verify `session.json` does NOT contain directive; verify directive appears in the provider payload (use a debug print or `prompt.json` placement — see §3 decision). Confirm behavior.
7. Manual: non-TTY `echo "hi" | ./blazeai` → works, no Tab handling, mode from `last_mode`.

---

## 10. Open / Risk Items

- **prompt.json debug placement** (§3): writing debug file before injection keeps it matching session.json. Decide during implementation.
- **Raw-mode reader complexity** (§4): minimal implementation (no arrows/history). If line-editing needs grow later, revisit with a real library — but the user already rejected liner/readline, so keep this minimal.
- **`last_mode` validation strictness** (§6): hard error vs auto-reset. Plan implements hard error per "no fallbacks" mandate; easy to revisit.
- **Provider Mid-stream** — Tab-switch recreating the provider client while no turn is running is safe. Switching during an active turn is not supported (Tab only works at prompt). No concurrency concern.