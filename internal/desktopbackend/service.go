// service.go — active desktop backend state, transcript, and protocol methods.
// Rehosts the old desktop transport behavior without any GUI bindings so an
// Electron shell can drive the same runtime and transcript lifecycle.
// Layer: desktop backend runtime. Dependencies: config, platform, runtime, session.
package desktopbackend

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"

	"blazeai/internal/config"
	"blazeai/internal/platform"
	runtimecore "blazeai/internal/runtime"
	"blazeai/internal/session"
)

// transcriptBlock is one chronological row in the desktop transcript.
//
// WHAT:  Carries a typed unit of conversation content.
// WHY:   The renderer rebuilds the transcript from typed rows instead of one flat string.
// PARAMS: Type — row kind; Prefix — short label; Text — raw markdown content.
type transcriptBlock struct {
	Type   string
	Prefix string
	Text   string
}

// Block type ids used by transcript rows.
const (
	blockUser      = "user"
	blockAssistant = "assistant"
	blockSystem    = "system"
	blockTool      = "tool"
	blockReasoning = "reasoning"
)

// blockPayload is one transcript block serialized to Electron.
//
// WHAT:  JSON view of one transcript row.
// WHY:   The renderer applies per-type layout and markdown rules.
// PARAMS: Type — row kind; Prefix — label; Text — raw markdown content.
type blockPayload struct {
	Type   string `json:"type"`
	Prefix string `json:"prefix"`
	Text   string `json:"text"`
}

// stateResponse is the full desktop UI state payload.
//
// WHAT:  Carries transcript rows plus current status, model, and visual preferences.
// WHY:   The renderer fully rebuilds its state from this payload.
// PARAMS: Blocks — transcript rows; Status — footer text; Busy — input lock;
//
//	Model/Models — current and selectable model ids; WorkDir — visible work directory;
//	Theme/FontSize — local UI preferences; ReasoningMaxHeight — per-theme reasoning pane height.
type stateResponse struct {
	Blocks             []blockPayload `json:"blocks"`
	Status             string         `json:"status"`
	Busy               bool           `json:"busy"`
	Model              string         `json:"model"`
	Models             []string       `json:"models"`
	WorkDir            string         `json:"workdir"`
	WorkDirFull        string         `json:"workdir_full"`
	Theme              string         `json:"theme"`
	FontSize           float64        `json:"font_size"`
	ReasoningMaxHeight float64        `json:"reasoning_max_height"`
	Window             windowPayload  `json:"window"`
}

// windowPayload is the serialized persisted desktop window geometry.
//
// WHAT:  Exposes the last saved window bounds to Electron.
// WHY:   The desktop shell should reopen where the user left it.
// PARAMS: Initialized — whether stored bounds are valid; X/Y/Width/Height — saved geometry.
type windowPayload struct {
	Initialized bool `json:"initialized"`
	X           int  `json:"x"`
	Y           int  `json:"y"`
	Width       int  `json:"width"`
	Height      int  `json:"height"`
}

// Service owns the active desktop backend state.
//
// WHAT:  Holds desktop-local config, transcript state, and the lazily-created runtime agent.
// WHY:   Electron requests need one shared backend object that preserves desktop transport behavior.
// PARAMS: Config — global runtime config; desktopCfg/state — desktop-local persisted state;
//
//	agent — current runtime agent; handler — runtime transcript adapter.
type Service struct {
	cfg        *config.Config
	desktopCfg *Config
	state      *State
	statePath  string
	configPath string
	osType     platform.OS
	promptsFS  fs.FS

	agent                  *runtimecore.Agent
	handler                *Handler
	pendingWorkDir         string
	pendingResumeLastClean bool
	quitRequested          bool

	mu              sync.Mutex
	blocks          []transcriptBlock
	status          string
	busy            bool
	activeTool      int
	activeReply     int
	activeReasoning int
}

// NewService loads desktop-local config/state and returns a ready backend service.
//
// WHAT:  Creates one backend service with persisted desktop state loaded.
// WHY:   Startup should fail before protocol serving if desktop-local state is broken.
// PARAMS: cfg — loaded runtime config; osType — detected OS; promptsFS — prompt templates;
//
//	opts — initial workdir and first-message resume policy.
//
// RETURNS: *Service — ready backend state holder; error on missing or invalid config/state.
func NewService(cfg *config.Config, osType platform.OS, promptsFS fs.FS, opts BackendOptions) (*Service, error) {
	desktopCfg, configPath, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	state, statePath, err := LoadState(cfg)
	if err != nil {
		return nil, err
	}
	service := &Service{
		cfg:                    cfg,
		desktopCfg:             desktopCfg,
		state:                  state,
		statePath:              statePath,
		configPath:             configPath,
		osType:                 osType,
		promptsFS:              promptsFS,
		pendingWorkDir:         strings.TrimSpace(opts.InitialWorkDir),
		pendingResumeLastClean: opts.ResumeLastClean,
		status:                 "Ready",
		activeTool:             -1,
		activeReply:            -1,
		activeReasoning:        -1,
	}
	service.handler = NewHandler(service)
	if service.pendingWorkDir != "" {
		service.desktopCfg.WorkDir = service.pendingWorkDir
		if err := service.desktopCfg.SaveTo(service.configPath); err != nil {
			return nil, fmt.Errorf("cannot persist initial desktop workdir: %w", err)
		}
	}
	return service, nil
}

func (s *Service) handleRequest(req Request) (Response, bool, error) {
	switch req.Method {
	case "get_state":
		return Response{ID: req.ID, OK: true, Result: s.snapshot()}, false, nil
	case "send_message":
		var params SendMessageParams
		if err := decodeParams(req.Params, &params); err != nil {
			return Response{}, false, err
		}
		state, err := s.sendMessage(params.Text)
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, false, nil
	case "change_model":
		var params ChangeModelParams
		if err := decodeParams(req.Params, &params); err != nil {
			return Response{}, false, err
		}
		state, err := s.changeModel(params.Model)
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, false, nil
	case "clear_session":
		state, err := s.clearSession()
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, false, nil
	case "close_session":
		state, err := s.closeSession()
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, false, nil
	case "set_theme":
		var params SetThemeParams
		if err := decodeParams(req.Params, &params); err != nil {
			return Response{}, false, err
		}
		state, err := s.setTheme(params.Theme)
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, false, nil
	case "set_font_size":
		var params SetFontSizeParams
		if err := decodeParams(req.Params, &params); err != nil {
			return Response{}, false, err
		}
		state, err := s.setFontSize(params.FontSize)
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, false, nil
	case "set_workdir":
		var params SetWorkDirParams
		if err := decodeParams(req.Params, &params); err != nil {
			return Response{}, false, err
		}
		state, err := s.setWorkDir(params.Path, params.ResumeLastClean)
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, false, nil
	case "set_window_bounds":
		var params SetWindowBoundsParams
		if err := decodeParams(req.Params, &params); err != nil {
			return Response{}, false, err
		}
		state, err := s.setWindowBounds(WindowBounds{X: params.X, Y: params.Y, Width: params.Width, Height: params.Height})
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, false, nil
	case "quit":
		state, err := s.quit()
		return Response{ID: req.ID, OK: err == nil, Result: state, Error: errorText(err)}, err == nil, nil
	default:
		return Response{}, false, fmt.Errorf("unknown desktop backend method: %s", req.Method)
	}
}

func (s *Service) sendMessage(text string) (stateResponse, error) {
	if err := s.submitInput(strings.TrimSpace(text)); err != nil {
		return stateResponse{}, err
	}
	return s.snapshot(), nil
}

func (s *Service) changeModel(modelID string) (stateResponse, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return stateResponse{}, fmt.Errorf("model is required")
	}
	if s.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before changing the model")
	}
	s.state.SetSelectedModel(modelID)
	if err := s.flushState(); err != nil {
		return stateResponse{}, err
	}
	if s.agent != nil {
		if err := s.agent.SetModelLocal(modelID); err != nil {
			return stateResponse{}, err
		}
	}
	s.AppendSystem("Model set to: " + modelID)
	s.SetStatus("Ready")
	return s.snapshot(), nil
}

func (s *Service) setTheme(theme string) (stateResponse, error) {
	theme = strings.TrimSpace(theme)
	if theme == "" {
		return stateResponse{}, fmt.Errorf("theme is required")
	}
	if err := validateTheme(theme); err != nil {
		return stateResponse{}, err
	}
	s.state.SetTheme(theme)
	if err := s.flushState(); err != nil {
		return stateResponse{}, err
	}
	return s.snapshot(), nil
}

func (s *Service) setFontSize(fontSize float64) (stateResponse, error) {
	if fontSize <= 0 {
		return stateResponse{}, fmt.Errorf("font size must be greater than zero")
	}
	if err := validateFontSize(fontSize); err != nil {
		return stateResponse{}, err
	}
	s.state.SetFontSize(fontSize)
	if err := s.flushState(); err != nil {
		return stateResponse{}, err
	}
	return s.snapshot(), nil
}

func (s *Service) setWorkDir(path string, resumeLastClean bool) (stateResponse, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return stateResponse{}, fmt.Errorf("workdir path is required")
	}
	if !isDir(path) {
		return stateResponse{}, fmt.Errorf("workdir is not a directory: %s", path)
	}
	if s.agent != nil {
		if s.isBusy() {
			return stateResponse{}, fmt.Errorf("wait for the current turn to finish before switching workdir")
		}
		if err := s.agent.CloseSession(); err != nil {
			return stateResponse{}, fmt.Errorf("cannot close current session before switching workdir: %w", err)
		}
		s.agent = nil
		s.resetTranscript()
	}
	s.pendingWorkDir = path
	s.pendingResumeLastClean = resumeLastClean
	s.desktopCfg.WorkDir = path
	if err := s.desktopCfg.SaveTo(s.configPath); err != nil {
		return stateResponse{}, err
	}
	return s.snapshot(), nil
}

func (s *Service) setWindowBounds(bounds WindowBounds) (stateResponse, error) {
	if bounds.Width <= 0 {
		return stateResponse{}, fmt.Errorf("window width must be greater than zero")
	}
	if bounds.Height <= 0 {
		return stateResponse{}, fmt.Errorf("window height must be greater than zero")
	}
	s.state.UpdateWindowBounds(bounds)
	if err := s.flushState(); err != nil {
		return stateResponse{}, err
	}
	return s.snapshot(), nil
}

func (s *Service) clearSession() (stateResponse, error) {
	if s.agent == nil {
		s.resetTranscript()
		return s.snapshot(), nil
	}
	if s.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before clearing the session")
	}
	if err := s.agent.ResetConversation(); err != nil {
		return stateResponse{}, err
	}
	s.resetTranscript()
	s.AppendSystem("Session cleared.")
	return s.snapshot(), nil
}

func (s *Service) closeSession() (stateResponse, error) {
	if s.agent == nil {
		return s.snapshot(), nil
	}
	if s.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before closing the session")
	}
	if err := s.agent.CloseSession(); err != nil {
		return stateResponse{}, err
	}
	s.agent = nil
	s.resetTranscript()
	s.AppendSystem("Session closed cleanly. Desktop app stays online.")
	return s.snapshot(), nil
}

func (s *Service) quit() (stateResponse, error) {
	if s.isBusy() {
		return stateResponse{}, fmt.Errorf("wait for the current turn to finish before quitting")
	}
	s.quitRequested = true
	s.AppendSystem("Desktop app is shutting down.")
	return s.snapshot(), nil
}

func (s *Service) submitInput(text string) error {
	if text == "" {
		return fmt.Errorf("message is empty")
	}
	if s.isBusy() {
		return fmt.Errorf("wait for the current turn to finish before sending another message")
	}
	if s.agent == nil {
		if err := s.ensureSession(context.Background()); err != nil {
			return err
		}
	}
	s.AppendUser(text)
	if strings.HasPrefix(text, "/") {
		handled, response, err := HandleCommand(text, s.agent, s.cfg, s.state, s.statePath)
		if err != nil {
			return err
		}
		if handled {
			if strings.HasPrefix(text, "/clear") || strings.HasPrefix(text, "/new") {
				s.resetTranscript()
			}
			if response != "" {
				s.AppendSystem(response)
			}
			return nil
		}
	}
	s.handler.BeginTurn()
	go func(input string) {
		err := s.agent.RunTurn(context.Background(), input)
		s.handler.FinishTurn(err)
		if err != nil {
			s.AppendSystem("Error: " + err.Error())
		}
	}(text)
	return nil
}

func (s *Service) ensureSession(ctx context.Context) error {
	workDir := s.pendingWorkDir
	if workDir == "" {
		workDir = s.desktopCfg.WorkDir
	}
	if !isDir(workDir) {
		return fmt.Errorf("desktop workdir is not a directory: %s", workDir)
	}
	var (
		sess    *session.Session
		resumed bool
		err     error
	)
	if s.pendingResumeLastClean {
		sess, err = session.LastClean(workDir)
		if err != nil {
			return fmt.Errorf("cannot resume last clean desktop session in %s: %w", workDir, err)
		}
		resumed = true
		s.pendingResumeLastClean = false
	} else {
		sess, err = session.Create(workDir)
		if err != nil {
			return fmt.Errorf("cannot create session in %s: %w", workDir, err)
		}
	}
	agent, err := runtimecore.NewAgent(s.cfg, sess, s.osType, s.promptsFS, workDir, nil, "desktop")
	if err != nil {
		return fmt.Errorf("cannot create desktop agent: %w", err)
	}
	agent.Builder.TransportContext = strings.TrimSpace(`Electron desktop transport is active.
Exactly one desktop window owns this conversation UI.
Replies are shown in a desktop renderer, not in a terminal.
The renderer supports full Markdown including headings, lists, tables, fenced code blocks, blockquotes, links, and inline images.
Use structured formatting freely when it improves clarity.`)
	if resumed && agent.Compactor != nil {
		if err := agent.Compactor.RebuildForResume(sess); err != nil {
			return fmt.Errorf("cannot rebuild summaries for resume: %w", err)
		}
	}
	if err := agent.SetModelLocal(s.state.SelectedModelValue()); err != nil {
		return fmt.Errorf("cannot apply desktop model: %w", err)
	}
	agent.Handler = s.handler
	s.agent = agent
	if resumed {
		s.loadSessionTranscript()
	}
	return nil
}

func (s *Service) loadSessionTranscript() {
	if s.agent == nil {
		return
	}
	s.resetTranscript()
	activity := &toolActivity{}
	for _, msg := range s.agent.Session.Messages {
		switch msg.Role {
		case "user":
			if text := messageText(msg.Content); strings.TrimSpace(text) != "" {
				s.AppendUser(text)
				activity.Reset()
			}
		case "assistant":
			if reasoning := strings.TrimSpace(msg.Reasoning); reasoning != "" {
				s.StartReasoning()
				s.AppendReasoning(reasoning)
				s.FinishReasoning()
			}
			if text := messageText(msg.Content); strings.TrimSpace(text) != "" {
				s.StartAssistant()
				s.AppendAssistant(text)
				s.FinishAssistant()
				activity.Reset()
			}
			for _, call := range decodeReplayToolCalls(msg.ToolCalls, s.agent.Tools) {
				activity.AddCall(call.callID, call.name, call.args)
				s.SetToolActivity(activity.Render())
			}
		case "tool":
			activity.ApplyResult(msg.ToolCallID, msg.Name, messageText(msg.Content))
			s.SetToolActivity(activity.Render())
		}
	}
	s.SetStatus("Ready")
}

func (s *Service) snapshot() stateResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	blocks := make([]blockPayload, 0, len(s.blocks))
	for _, b := range s.blocks {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		blocks = append(blocks, blockPayload{Type: b.Type, Prefix: b.Prefix, Text: text})
	}
	modelID := ""
	workDir := s.pendingWorkDir
	if workDir == "" {
		workDir = s.desktopCfg.WorkDir
	}
	if s.agent != nil {
		modelID = s.agent.ModelID
		workDir = s.agent.WorkDir
	}
	bounds, ok := s.state.WindowBoundsValue()
	window := windowPayload{}
	if ok {
		window = windowPayload{Initialized: true, X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height}
	}
	return stateResponse{
		Blocks:             blocks,
		Status:             s.status,
		Busy:               s.busy,
		Model:              modelID,
		Models:             modelOptions(s.cfg, modelID),
		WorkDir:            truncatePath(workDir, 72),
		WorkDirFull:        workDir,
		Theme:              s.state.ThemeValue(),
		FontSize:           s.state.FontSizeValue(),
		ReasoningMaxHeight: s.desktopCfg.ReasoningMaxHeightValue(),
		Window:             window,
	}
}

// AppendUser adds one user message block to the transcript.
func (s *Service) AppendUser(text string) {
	s.appendSection(blockUser, "You", text)
}

// AppendSystem adds one local backend status block to the transcript.
func (s *Service) AppendSystem(text string) {
	s.appendSection(blockSystem, "System", text)
}

// StartAssistant opens a new assistant block if one is not active yet.
func (s *Service) StartAssistant() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeReply >= 0 {
		return
	}
	if s.activeReasoning >= 0 {
		s.activeReasoning = -1
	}
	s.activeTool = -1
	s.blocks = append(s.blocks, transcriptBlock{Type: blockAssistant, Prefix: "BlazeAI"})
	s.activeReply = len(s.blocks) - 1
}

// AppendAssistant appends streamed assistant text to the active assistant block.
func (s *Service) AppendAssistant(delta string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeReply < 0 {
		s.blocks = append(s.blocks, transcriptBlock{Type: blockAssistant, Prefix: "BlazeAI"})
		s.activeReply = len(s.blocks) - 1
	}
	s.blocks[s.activeReply].Text += delta
}

// FinishAssistant closes the current streamed assistant block.
func (s *Service) FinishAssistant() {
	s.mu.Lock()
	s.activeReply = -1
	s.mu.Unlock()
}

// StartReasoning opens a new reasoning block if one is not active yet.
func (s *Service) StartReasoning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeReasoning >= 0 {
		return
	}
	s.activeTool = -1
	s.activeReply = -1
	s.blocks = append(s.blocks, transcriptBlock{Type: blockReasoning, Prefix: "Reasoning"})
	s.activeReasoning = len(s.blocks) - 1
}

// AppendReasoning appends streamed reasoning text to the active reasoning block.
func (s *Service) AppendReasoning(delta string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeReasoning < 0 {
		s.blocks = append(s.blocks, transcriptBlock{Type: blockReasoning, Prefix: "Reasoning"})
		s.activeReasoning = len(s.blocks) - 1
	}
	s.blocks[s.activeReasoning].Text += delta
}

// FinishReasoning closes the current streamed reasoning block.
func (s *Service) FinishReasoning() {
	s.mu.Lock()
	s.activeReasoning = -1
	s.mu.Unlock()
}

// SetToolActivity replaces or creates the current editable tool activity block.
func (s *Service) SetToolActivity(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeReply = -1
	s.activeReasoning = -1
	if s.activeTool >= 0 && s.activeTool < len(s.blocks) {
		s.blocks[s.activeTool].Text = text
		return
	}
	s.blocks = append(s.blocks, transcriptBlock{Type: blockTool, Prefix: "Tool", Text: text})
	s.activeTool = len(s.blocks) - 1
}

// SetStatus updates the footer status line.
func (s *Service) SetStatus(text string) {
	s.mu.Lock()
	s.status = text
	s.mu.Unlock()
}

// SetBusy toggles whether the desktop backend accepts another turn.
func (s *Service) SetBusy(active bool) {
	s.mu.Lock()
	s.busy = active
	s.mu.Unlock()
}

func (s *Service) appendSection(blockType, prefix, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.mu.Lock()
	s.activeTool = -1
	s.activeReply = -1
	s.activeReasoning = -1
	s.blocks = append(s.blocks, transcriptBlock{Type: blockType, Prefix: prefix, Text: text})
	s.mu.Unlock()
}

func (s *Service) resetTranscript() {
	s.mu.Lock()
	s.blocks = nil
	s.activeTool = -1
	s.activeReply = -1
	s.activeReasoning = -1
	s.mu.Unlock()
}

func (s *Service) isBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy
}

func (s *Service) flushState() error {
	if err := s.state.SaveTo(s.statePath, s.cfg); err != nil {
		return fmt.Errorf("cannot persist desktop state: %w", err)
	}
	return nil
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
	case "text", "input_text":
		text, _ := m["text"].(string)
		return text
	case "image_url", "input_image":
		return "[image omitted]"
	default:
		return ""
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
