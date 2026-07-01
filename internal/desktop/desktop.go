// desktop.go — desktop companion startup, fixed-session wiring, and embedded window UI.
// Boots the singleton desktop transport from app_home/desktop, resumes the same
// session folder on every start, and hosts one local chat page inside a native
// desktop window with no browser chrome.
// Layer: transport runtime. Dependencies: internal/config, internal/platform,
// internal/runtime, internal/session.
package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	webview "github.com/webview/webview_go"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	runtimecore "blazeai/internal/runtime"
	"blazeai/internal/session"
)

const desktopPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>BlazeAI Desktop</title>
  <style>
    :root {
      color-scheme: dark;
      --bg: #111318;
      --panel: #171a21;
      --panel-2: #1e2430;
      --border: #2a3344;
      --text: #e6edf7;
      --muted: #96a2b5;
      --accent: #5aa3ff;
      --accent-2: #7bc96f;
      --danger: #ff6b6b;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font: 14px/1.45 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: var(--bg);
      color: var(--text);
    }
    .app {
      height: 100vh;
      display: grid;
      grid-template-rows: auto 1fr auto;
    }
    .topbar, .footer {
      display: flex;
      gap: 12px;
      align-items: center;
      padding: 12px 16px;
      background: var(--panel);
      border-bottom: 1px solid var(--border);
    }
    .footer {
      border-top: 1px solid var(--border);
      border-bottom: none;
      flex-wrap: wrap;
    }
    .grow { flex: 1; min-width: 0; }
    .muted { color: var(--muted); }
    .transcript {
      margin: 0;
      padding: 18px 16px 120px;
      overflow: auto;
      white-space: pre-wrap;
      background: linear-gradient(180deg, #111318 0%, #131720 100%);
      font: 14px/1.5 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    }
    textarea, select, button {
      border-radius: 10px;
      border: 1px solid var(--border);
      background: var(--panel-2);
      color: var(--text);
      font: inherit;
    }
    textarea {
      width: 100%;
      min-height: 120px;
      resize: vertical;
      padding: 12px;
    }
    select, button {
      padding: 9px 12px;
    }
    button {
      cursor: pointer;
    }
    button.primary {
      background: var(--accent);
      color: #06111f;
      border-color: transparent;
      font-weight: 600;
    }
    button.warn {
      color: var(--danger);
    }
    .composer {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 12px;
      width: 100%;
    }
    .status {
      color: var(--accent-2);
      font-weight: 600;
    }
    @media (max-width: 800px) {
      .topbar, .footer, .composer {
        display: block;
      }
      .topbar > *, .footer > *, .composer > * {
        margin-bottom: 10px;
      }
    }
  </style>
</head>
<body>
  <div class="app">
    <div class="topbar">
      <strong>BlazeAI Desktop</strong>
      <span id="workdir" class="grow muted"></span>
      <label>
        <span class="muted">Model</span>
        <select id="model"></select>
      </label>
      <button id="clear">Clear</button>
      <button id="closeSession">Close Session</button>
      <button id="quit" class="warn">Quit</button>
    </div>
    <pre id="transcript" class="transcript"></pre>
    <div class="footer">
      <div id="status" class="status grow">Ready</div>
      <form id="composer" class="composer">
        <textarea id="input" placeholder="Type a message or /help"></textarea>
        <button id="send" class="primary" type="submit">Send</button>
      </form>
    </div>
  </div>
  <script>
    const transcriptEl = document.getElementById('transcript');
    const statusEl = document.getElementById('status');
    const workdirEl = document.getElementById('workdir');
    const inputEl = document.getElementById('input');
    const sendEl = document.getElementById('send');
    const modelEl = document.getElementById('model');
    const composerEl = document.getElementById('composer');
    const clearEl = document.getElementById('clear');
    const closeSessionEl = document.getElementById('closeSession');
    const quitEl = document.getElementById('quit');

    let lastTranscript = '';
    let lastModel = '';
    let requestInFlight = false;

    function applyState(state) {
      if (state.transcript !== lastTranscript) {
        transcriptEl.textContent = state.transcript;
        transcriptEl.scrollTop = transcriptEl.scrollHeight;
        lastTranscript = state.transcript;
      }
      statusEl.textContent = state.status;
      workdirEl.textContent = state.workdir;
      inputEl.disabled = state.busy;
      sendEl.disabled = state.busy;
      clearEl.disabled = state.busy;
      closeSessionEl.disabled = state.busy;

      if (JSON.stringify(Array.from(modelEl.options).map(o => o.value)) !== JSON.stringify(state.models)) {
        modelEl.innerHTML = '';
        for (const model of state.models) {
          const option = document.createElement('option');
          option.value = model;
          option.textContent = model;
          modelEl.appendChild(option);
        }
      }
      if (state.model !== lastModel) {
        modelEl.value = state.model;
        lastModel = state.model;
      }
    }

    async function refresh() {
      if (requestInFlight) {
        return;
      }
      requestInFlight = true;
      try {
	        applyState(await getState());
      } catch (err) {
        statusEl.textContent = 'Error: ' + err.message;
      } finally {
        requestInFlight = false;
      }
    }

    composerEl.addEventListener('submit', async (event) => {
      event.preventDefault();
      const text = inputEl.value.trim();
      if (!text) {
        return;
      }
      inputEl.value = '';
      try {
	        applyState(await sendMessage(text));
      } catch (err) {
        statusEl.textContent = 'Error: ' + err.message;
      }
    });

    clearEl.addEventListener('click', async () => {
      try {
	        applyState(await clearSession());
      } catch (err) {
        statusEl.textContent = 'Error: ' + err.message;
      }
    });

    closeSessionEl.addEventListener('click', async () => {
      try {
	        applyState(await closeSession());
      } catch (err) {
        statusEl.textContent = 'Error: ' + err.message;
      }
    });

    quitEl.addEventListener('click', async () => {
      try {
	        applyState(await quitApp());
      } catch (err) {
        statusEl.textContent = 'Error: ' + err.message;
      }
    });

    modelEl.addEventListener('change', async () => {
      try {
	        applyState(await changeModel(modelEl.value));
      } catch (err) {
        statusEl.textContent = 'Error: ' + err.message;
      }
    });

    inputEl.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        composerEl.requestSubmit();
      }
    });

    refresh();
    setInterval(refresh, 700);
  </script>
</body>
</html>
`

const (
	defaultDesktopWindowWidth  = 1100
	defaultDesktopWindowHeight = 760
	windowStateWriteDelay      = 350 * time.Millisecond
)

type stateResponse struct {
	Transcript string   `json:"transcript"`
	Status     string   `json:"status"`
	Busy       bool     `json:"busy"`
	Model      string   `json:"model"`
	Models     []string `json:"models"`
	WorkDir    string   `json:"workdir"`
}

// Run starts the singleton desktop companion transport and blocks until the app exits.
//
// WHAT:  Boots one embedded desktop chat window over the shared runtime core.
// WHY:   The desktop transport provides a persistent singleton conversation like Telegram.
// PARAMS: ctx — process lifetime context; cfg — loaded global runtime config; osType — detected OS;
// promptsFS — embedded prompt filesystem.
// RETURNS: error if startup or embedded window wiring fails.
func Run(ctx context.Context, cfg *config.Config, osType platform.OS, promptsFS fs.FS) error {
	desktopCfg, _, err := LoadConfig()
	if err != nil {
		return err
	}
	state, statePath, err := LoadState(cfg)
	if err != nil {
		return err
	}
	instanceDir, err := InstanceDir()
	if err != nil {
		return err
	}
	sessDir := filepath.Join(instanceDir, "session")
	sess, resumed, err := openDesktopSession(sessDir)
	if err != nil {
		return err
	}
	agent, err := runtimecore.NewAgent(cfg, sess, osType, promptsFS, desktopCfg.WorkDir, nil, "desktop")
	if err != nil {
		return fmt.Errorf("cannot create desktop agent: %w", err)
	}
	agent.Builder.TransportContext = strings.TrimSpace(`Desktop companion transport is active.
One singleton desktop app owns this transport.
Exactly one fixed session is resumed on every start.
Do not create or refer to multiple desktop conversations.
Replies are shown in one dedicated desktop window, not a terminal.
Keep replies readable in plain text and avoid unnecessary tool chatter.`)
	if resumed && agent.Compactor != nil {
		if err := agent.Compactor.RebuildForResume(sess); err != nil {
			return fmt.Errorf("cannot rebuild summaries for desktop resume: %w", err)
		}
	}
	if err := agent.SetModelLocal(state.SelectedModelValue()); err != nil {
		return fmt.Errorf("cannot apply desktop model: %w", err)
	}

	ui := newDesktopUI(agent, cfg, state, statePath)
	ui.loadSessionTranscript()
	view := webview.New(false)
	if view == nil {
		return fmt.Errorf("cannot create desktop window")
	}
	destroyed := false
	defer func() {
		if !destroyed {
			view.Destroy()
		}
	}()
	view.SetTitle("BlazeAI Desktop")
	initialBounds := WindowBounds{Width: defaultDesktopWindowWidth, Height: defaultDesktopWindowHeight}
	if storedBounds, ok := state.WindowBoundsValue(); ok {
		initialBounds.Width = storedBounds.Width
		initialBounds.Height = storedBounds.Height
	}
	view.SetSize(initialBounds.Width, initialBounds.Height, webview.HintNone)
	view.SetHtml(desktopPage)
	if err := ui.attach(view); err != nil {
		return err
	}
	platformUI, err := startDesktopPlatform(view, ui, desktopCfg, osType)
	if err != nil {
		return err
	}
	defer platformUI.Shutdown()

	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		<-ctx.Done()
		view.Terminate()
	}()
	go func() {
		<-ui.quitCh
		view.Terminate()
	}()
	view.Run()
	view.Destroy()
	destroyed = true
	if err := ui.flushState(); err != nil {
		return err
	}
	return nil
}

func openDesktopSession(sessionDir string) (*session.Session, bool, error) {
	sess, err := session.Load(sessionDir)
	if err == nil {
		return sess, true, nil
	}
	if !errors.Is(err, session.ErrSessionNotFound) {
		return nil, false, fmt.Errorf("cannot load desktop session: %w", err)
	}
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, false, fmt.Errorf("cannot create desktop session folder: %w", err)
	}
	sess = &session.Session{
		Messages:      []session.Message{},
		ClosedCleanly: false,
		Folder:        sessionDir,
	}
	if err := sess.Save(); err != nil {
		return nil, false, fmt.Errorf("cannot create desktop session: %w", err)
	}
	return sess, false, nil
}

type desktopUI struct {
	agent     *runtimecore.Agent
	cfg       *config.Config
	state     *State
	statePath string
	handler   *Handler
	view      webview.WebView
	quitCh    chan struct{}
	quitOnce  sync.Once

	mu             sync.Mutex
	transcriptText string
	status         string
	busy           bool
	saveTimer      *time.Timer
}

func newDesktopUI(agent *runtimecore.Agent, cfg *config.Config, state *State, statePath string) *desktopUI {
	ui := &desktopUI{
		agent:     agent,
		cfg:       cfg,
		state:     state,
		statePath: statePath,
		quitCh:    make(chan struct{}),
		status:    "Ready",
	}
	ui.handler = NewHandler(ui)
	agent.Handler = ui.handler
	return ui
}

func (ui *desktopUI) attach(view webview.WebView) error {
	ui.view = view
	if err := view.Bind("getState", ui.getState); err != nil {
		return fmt.Errorf("cannot bind getState: %w", err)
	}
	if err := view.Bind("sendMessage", ui.sendMessage); err != nil {
		return fmt.Errorf("cannot bind sendMessage: %w", err)
	}
	if err := view.Bind("changeModel", ui.changeModel); err != nil {
		return fmt.Errorf("cannot bind changeModel: %w", err)
	}
	if err := view.Bind("clearSession", ui.clearSession); err != nil {
		return fmt.Errorf("cannot bind clearSession: %w", err)
	}
	if err := view.Bind("closeSession", ui.closeSession); err != nil {
		return fmt.Errorf("cannot bind closeSession: %w", err)
	}
	if err := view.Bind("quitApp", ui.quitApp); err != nil {
		return fmt.Errorf("cannot bind quitApp: %w", err)
	}
	return nil
}

func (ui *desktopUI) getState() stateResponse {
	return ui.snapshot()
}

func (ui *desktopUI) sendMessage(text string) (stateResponse, error) {
	if err := ui.submitInput(strings.TrimSpace(text)); err != nil {
		return stateResponse{}, err
	}
	return ui.snapshot(), nil
}

func (ui *desktopUI) changeModel(modelID string) (stateResponse, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return stateResponse{}, fmt.Errorf("model is required")
	}
	if ui.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before changing the model")
	}
	if err := ui.agent.SetModelLocal(modelID); err != nil {
		return stateResponse{}, err
	}
	ui.state.SetSelectedModel(modelID)
	if err := ui.flushState(); err != nil {
		return stateResponse{}, err
	}
	ui.AppendSystem("Model set to: " + modelID)
	ui.SetStatus("Ready")
	return ui.snapshot(), nil
}

func (ui *desktopUI) clearSession() (stateResponse, error) {
	if ui.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before clearing the fixed session")
	}
	if err := ui.agent.ResetConversation(); err != nil {
		return stateResponse{}, err
	}
	ui.resetTranscript()
	ui.AppendSystem("Session cleared.")
	return ui.snapshot(), nil
}

func (ui *desktopUI) closeSession() (stateResponse, error) {
	if ui.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before closing the session")
	}
	if err := ui.agent.CloseSession(); err != nil {
		return stateResponse{}, err
	}
	ui.AppendSystem("Session closed cleanly. Desktop app stays online.")
	return ui.snapshot(), nil
}

func (ui *desktopUI) quitApp() (stateResponse, error) {
	if ui.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before quitting")
	}
	ui.requestQuit("Desktop app is shutting down.")
	return ui.snapshot(), nil
}

func (ui *desktopUI) submitInput(text string) error {
	if text == "" {
		return fmt.Errorf("message is empty")
	}
	if ui.isBusy() {
		return fmt.Errorf("wait for the current turn to finish before sending another message")
	}
	ui.AppendUser(text)

	if strings.HasPrefix(text, "/") {
		handled, response, err := HandleCommand(text, ui.agent, ui.cfg, ui.state, ui.statePath)
		if err != nil {
			return err
		}
		if handled {
			if strings.HasPrefix(text, "/clear") || strings.HasPrefix(text, "/new") {
				ui.resetTranscript()
			}
			if response != "" {
				ui.AppendSystem(response)
			}
			return nil
		}
	}

	ui.handler.BeginTurn()
	go func(input string) {
		err := ui.agent.RunTurn(context.Background(), input)
		ui.handler.FinishTurn(err)
		if err != nil {
			ui.AppendSystem("Error: " + err.Error())
		}
	}(text)
	return nil
}

func (ui *desktopUI) loadSessionTranscript() {
	ui.resetTranscript()
	for _, msg := range ui.agent.Session.Messages {
		switch msg.Role {
		case "user":
			if text := messageText(msg.Content); strings.TrimSpace(text) != "" {
				ui.AppendUser(text)
			}
		case "assistant":
			if text := messageText(msg.Content); strings.TrimSpace(text) != "" {
				ui.StartAssistant()
				ui.AppendAssistant(text)
				ui.FinishAssistant()
			}
		case "tool":
			text := strings.TrimSpace(messageText(msg.Content))
			if text == "" {
				text = "Tool completed."
			}
			if msg.Name != "" {
				text = msg.Name + "\n" + text
			}
			ui.AppendTool(text)
		}
	}
	ui.SetStatus("Ready")
}

func (ui *desktopUI) snapshot() stateResponse {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	return stateResponse{
		Transcript: ui.transcriptText,
		Status:     ui.status,
		Busy:       ui.busy,
		Model:      ui.agent.ModelID,
		Models:     modelOptions(ui.cfg, ui.agent.ModelID),
		WorkDir:    truncatePath(ui.agent.WorkDir, 72),
	}
}

func (ui *desktopUI) resetTranscript() {
	ui.mu.Lock()
	ui.transcriptText = ""
	ui.mu.Unlock()
}

func (ui *desktopUI) isBusy() bool {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	return ui.busy
}

func (ui *desktopUI) appendSection(prefix string, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	ui.mu.Lock()
	if ui.transcriptText != "" {
		ui.transcriptText += "\n\n"
	}
	ui.transcriptText += prefix + ":\n" + text
	ui.mu.Unlock()
}

// AppendUser adds one user message block to the transcript.
func (ui *desktopUI) AppendUser(text string) {
	ui.appendSection("You", text)
}

// AppendSystem adds one local desktop status block to the transcript.
func (ui *desktopUI) AppendSystem(text string) {
	ui.appendSection("System", text)
}

// AppendTool adds one tool activity block to the transcript.
func (ui *desktopUI) AppendTool(text string) {
	ui.appendSection("Tool", text)
}

// StartAssistant opens a new assistant block if one is not active yet.
func (ui *desktopUI) StartAssistant() {
	ui.mu.Lock()
	if ui.transcriptText != "" {
		ui.transcriptText += "\n\n"
	}
	ui.transcriptText += "BlazeAI:\n"
	ui.mu.Unlock()
}

// AppendAssistant appends streamed assistant text to the active assistant block.
func (ui *desktopUI) AppendAssistant(delta string) {
	ui.mu.Lock()
	ui.transcriptText += delta
	ui.mu.Unlock()
}

// FinishAssistant currently keeps the last assistant block open as plain text.
func (ui *desktopUI) FinishAssistant() {}

// SetStatus updates the footer status line.
func (ui *desktopUI) SetStatus(text string) {
	ui.mu.Lock()
	ui.status = text
	ui.mu.Unlock()
}

// SetBusy toggles whether the desktop transport accepts another turn.
func (ui *desktopUI) SetBusy(active bool) {
	ui.mu.Lock()
	ui.busy = active
	ui.mu.Unlock()
}

func (ui *desktopUI) rememberWindowBounds(bounds WindowBounds) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if !ui.state.UpdateWindowBounds(bounds) {
		return
	}
	ui.mu.Lock()
	if ui.saveTimer == nil {
		ui.saveTimer = time.AfterFunc(windowStateWriteDelay, func() {
			_ = ui.flushState()
		})
	} else {
		ui.saveTimer.Reset(windowStateWriteDelay)
	}
	ui.mu.Unlock()
}

func (ui *desktopUI) flushState() error {
	ui.mu.Lock()
	if ui.saveTimer != nil {
		ui.saveTimer.Stop()
	}
	ui.mu.Unlock()
	if err := ui.state.SaveTo(ui.statePath, ui.cfg); err != nil {
		return fmt.Errorf("cannot persist desktop state: %w", err)
	}
	return nil
}

func (ui *desktopUI) requestQuit(message string) {
	if strings.TrimSpace(message) != "" {
		ui.AppendSystem(message)
	}
	ui.quitOnce.Do(func() { close(ui.quitCh) })
}

func modelOptions(cfg *config.Config, selected string) []string {
	seen := map[string]struct{}{}
	options := make([]string, 0, len(cfg.FavoriteModels)+1)
	for _, modelID := range cfg.FavoriteModels {
		if strings.TrimSpace(modelID) == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		options = append(options, modelID)
	}
	if strings.TrimSpace(selected) != "" {
		if _, ok := seen[selected]; !ok {
			options = append(options, selected)
		}
	}
	return options
}

func truncatePath(path string, max int) string {
	if max <= 0 || len(path) <= max {
		return path
	}
	if max <= 3 {
		return path[:max]
	}
	return "..." + path[len(path)-max+3:]
}

func messageText(content interface{}) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := messageTextPart(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprint(v)
	}
}

func messageTextPart(item interface{}) string {
	m, ok := item.(map[string]interface{})
	if !ok {
		return fmt.Sprint(item)
	}
	typeName, _ := m["type"].(string)
	switch typeName {
	case "text":
		text, _ := m["text"].(string)
		return text
	case "input_text":
		text, _ := m["text"].(string)
		return text
	case "image_url", "input_image":
		return "[image omitted]"
	default:
		return ""
	}
}
