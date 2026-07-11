// server.go — minimal HTTP server with SSE streaming for the web transport.
// Serves one HTML page, streams transcript blocks via SSE, and accepts
// input + control commands over POST endpoints.
// Layer: transport runtime. Dependencies: internal/runtime, internal/config.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"blazeai/internal/config"
	"blazeai/internal/runtime"
)

// sseEvent is one event sent to all connected SSE clients.
type sseEvent struct {
	Event string
	Data  string
}

// blockPayload is the JSON payload for a transcript block event.
type blockPayload struct {
	Type string `json:"type"`
	HTML string `json:"html"`
}

// configPayload carries the full UI state for initial sync and after changes.
type configPayload struct {
	Modes     []string `json:"modes"`
	Mode      string   `json:"mode"`
	Model     string   `json:"model"`
	Favorites []string `json:"favorites"`
	WorkDir   string   `json:"workdir"`
	Reasoning bool     `json:"reasoning"`
}

// statusPayload carries the footer status line state.
type statusPayload struct {
	Busy    bool   `json:"busy"`
	Text    string `json:"text"`
	Model   string `json:"model"`
	WorkDir string `json:"workdir"`
}

// transcriptBlock is one stored row in the server-side transcript.
type transcriptBlock struct {
	Type string
	HTML string
}

// Server is the web transport HTTP server hosting one agent session.
type Server struct {
	agent  *runtime.Agent
	hub    *sseHub
	server *http.Server

	mu     sync.Mutex
	blocks []transcriptBlock
	status string
	busy   bool
}

// sseHub manages broadcast to all connected SSE clients.
type sseHub struct {
	mu      sync.Mutex
	clients map[chan sseEvent]struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{clients: make(map[chan sseEvent]struct{})}
}

// broadcast sends an event to all connected clients. Slow clients are dropped.
func (h *sseHub) broadcast(ev sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
			// Drop slow client.
			delete(h.clients, ch)
		}
	}
}

// subscribe registers a new SSE client channel.
func (h *sseHub) subscribe(ch chan sseEvent) {
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
}

// unsubscribe removes an SSE client channel.
func (h *sseHub) unsubscribe(ch chan sseEvent) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

// NewServer creates a web transport server with the given agent and listen address.
// The agent.Handler is replaced with a web Handler bound to this server.
func NewServer(agent *runtime.Agent, addr string) *Server {
	s := &Server{
		agent:  agent,
		hub:    newSSEHub(),
		status: "Ready",
	}
	agent.Handler = NewHandler(s)
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handlePage)
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/input", s.handleInput)
	mux.HandleFunc("/mode", s.handleMode)
	mux.HandleFunc("/model", s.handleModel)
	mux.HandleFunc("/toggle-favorite", s.handleToggleFavorite)
	mux.HandleFunc("/toggle-reasoning", s.handleToggleReasoning)
	mux.HandleFunc("/clear", s.handleClear)
	s.server = &http.Server{Addr: addr, Handler: mux}
	return s
}

// Start begins listening and blocks until the server stops.
func (s *Server) Start() error {
	log.Printf("Web transport listening on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// sendBlock appends or replaces a block in the transcript and broadcasts it.
// streaming=false creates a new block; streaming=true replaces the last block of the same type.
func (s *Server) sendBlock(blockType, html string, streaming bool) {
	s.mu.Lock()
	if streaming {
		// Find and replace the last block of matching type.
		replaced := false
		for i := len(s.blocks) - 1; i >= 0; i-- {
			if s.blocks[i].Type == blockType {
				s.blocks[i].HTML = html
				replaced = true
				break
			}
		}
		if !replaced {
			s.blocks = append(s.blocks, transcriptBlock{Type: blockType, HTML: html})
		}
	} else {
		s.blocks = append(s.blocks, transcriptBlock{Type: blockType, HTML: html})
	}
	s.mu.Unlock()

	payload, _ := json.Marshal(blockPayload{Type: blockType, HTML: html})
	s.hub.broadcast(sseEvent{Event: "block", Data: string(payload)})
}

// SetBusy toggles whether the transport accepts another turn.
func (s *Server) SetBusy(active bool) {
	s.mu.Lock()
	s.busy = active
	s.mu.Unlock()
	s.broadcastStatus()
}

// SetStatus updates the footer status line.
func (s *Server) SetStatus(text string) {
	s.mu.Lock()
	s.status = text
	s.mu.Unlock()
	s.broadcastStatus()
}

// broadcastStatus sends the current status to all SSE clients.
func (s *Server) broadcastStatus() {
	s.mu.Lock()
	model := ""
	workDir := ""
	if s.agent != nil {
		model = s.agent.ModelID
		workDir = s.agent.WorkDir
	}
	payload, _ := json.Marshal(statusPayload{
		Busy:    s.busy,
		Text:    s.status,
		Model:   model,
		WorkDir: workDir,
	})
	s.mu.Unlock()
	s.hub.broadcast(sseEvent{Event: "status", Data: string(payload)})
}

// broadcastConfig sends the full UI configuration to all SSE clients.
func (s *Server) broadcastConfig() {
	if s.agent == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.agent.Config
	modesCfg := s.agent.Modes
	modeNames := make([]string, 0)
	currentMode := ""
	if modesCfg != nil {
		for _, m := range modesCfg.Modes {
			modeNames = append(modeNames, m.Name)
		}
		if s.agent.CurrentMode != nil {
			currentMode = s.agent.CurrentMode.Name
		}
	}

	payload, _ := json.Marshal(configPayload{
		Modes:     modeNames,
		Mode:      currentMode,
		Model:     s.agent.ModelID,
		Favorites: cfg.FavoriteModels,
		WorkDir:   s.agent.WorkDir,
		Reasoning: cfg.ShowReasoning,
	})
	s.hub.broadcast(sseEvent{Event: "config", Data: string(payload)})
}

// replayTranscript sends all stored blocks to a newly connected SSE client.
func (s *Server) replayTranscript(ch chan sseEvent) {
	s.mu.Lock()
	blocks := make([]transcriptBlock, len(s.blocks))
	copy(blocks, s.blocks)
	s.mu.Unlock()
	for _, b := range blocks {
		payload, _ := json.Marshal(blockPayload{Type: b.Type, HTML: b.HTML})
		ch <- sseEvent{Event: "block", Data: string(payload)}
	}
}

// handlePage serves the single-page HTML application.
func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(webPageHTML))
}

// handleEvents upgrades to SSE and streams blocks, status, and config events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ch := make(chan sseEvent, 32)
	s.hub.subscribe(ch)
	defer s.hub.unsubscribe(ch)

	// Send initial config and replay transcript.
	s.broadcastConfig()
	s.replayTranscript(ch)
	s.broadcastStatus()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
			flusher.Flush()
		}
	}
}

// handleInput processes a user message and runs the agent turn.
func (s *Server) handleInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		http.Error(w, "empty input", http.StatusBadRequest)
		return
	}

	if s.agent == nil {
		http.Error(w, "no active session", http.StatusServiceUnavailable)
		return
	}

	// Add user message as a block.
	html := `<span class="user-text">` + escapeHTML(text) + `</span>`
	s.sendBlock("user", html, false)

	// Run agent turn asynchronously. The Handler (set via NewServer)
	// receives all streaming callbacks and produces SSE events.
	go func() {
		h, _ := s.agent.Handler.(*Handler)
		if h != nil {
			h.BeginTurn()
		}
		turnErr := s.agent.RunTurn(context.Background(), text)
		if turnErr != nil {
			s.SetStatus("Error: " + turnErr.Error())
		}
		if h != nil {
			h.FinishTurn(turnErr)
		}
		// Re-broadcast config in case model/mode changed during the turn.
		s.broadcastConfig()
	}()

	w.WriteHeader(http.StatusOK)
}

// handleMode changes the work mode.
func (s *Server) handleMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.agent == nil {
		http.Error(w, "no active session", http.StatusServiceUnavailable)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "mode name required", http.StatusBadRequest)
		return
	}

	if err := s.agent.SetMode(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	html := `<span class="orange">⚡ Mode: ` + escapeHTML(name) + `</span>`
	s.sendBlock("system", html, false)
	s.broadcastConfig()
	w.WriteHeader(http.StatusOK)
}

// handleModel changes the active model.
func (s *Server) handleModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.agent == nil {
		http.Error(w, "no active session", http.StatusServiceUnavailable)
		return
	}

	modelID := strings.TrimSpace(r.FormValue("id"))
	if modelID == "" {
		http.Error(w, "model id required", http.StatusBadRequest)
		return
	}

	if err := s.agent.SetModel(modelID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	html := `<span class="orange">⚡ Model: ` + escapeHTML(modelID) + `</span>`
	s.sendBlock("system", html, false)
	s.broadcastConfig()
	w.WriteHeader(http.StatusOK)
}

// handleToggleFavorite adds or removes the current model from favorites.
func (s *Server) handleToggleFavorite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.agent == nil {
		http.Error(w, "no active session", http.StatusServiceUnavailable)
		return
	}

	cfg := s.agent.Config
	modelID := s.agent.ModelID

	if checkFavorite(cfg, modelID) {
		removed, err := cfg.RemoveFavorite(modelID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if removed {
			_ = cfg.Save()
		}
	} else {
		if err := cfg.AddFavorite(modelID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = cfg.Save()
	}

	s.broadcastConfig()
	w.WriteHeader(http.StatusOK)
}

// handleToggleReasoning toggles the reasoning display setting.
func (s *Server) handleToggleReasoning(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.agent == nil {
		http.Error(w, "no active session", http.StatusServiceUnavailable)
		return
	}

	s.agent.Config.ShowReasoning = !s.agent.Config.ShowReasoning
	s.agent.Config.Save()

	s.broadcastConfig()
	w.WriteHeader(http.StatusOK)
}

// handleClear resets the current session.
func (s *Server) handleClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.agent == nil {
		http.Error(w, "no active session", http.StatusServiceUnavailable)
		return
	}

	if err := s.agent.ResetConversation(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.blocks = nil
	s.status = "Ready"
	s.mu.Unlock()

	// Signal the frontend to clear its transcript.
	payload, _ := json.Marshal(map[string]interface{}{
		"modes":     []string{},
		"mode":      "",
		"model":     "",
		"favorites": []string{},
		"workdir":   "",
		"reasoning": false,
		"_clear":    true,
	})
	s.hub.broadcast(sseEvent{Event: "config", Data: string(payload)})
	// Then send real config without _clear flag.
	s.broadcastConfig()
	w.WriteHeader(http.StatusOK)
}

// checkFavorite reports whether a model ID is in the favorites list.
func checkFavorite(cfg *config.Config, modelID string) bool {
	for _, m := range cfg.FavoriteModels {
		if m == modelID {
			return true
		}
	}
	return false
}
