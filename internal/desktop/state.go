// state.go — desktop companion state loading and saving.
// Loads app_home/desktop/state.json, validates the selected model against the
// global provider config, and persists desktop-local model changes plus window geometry.
// Layer: transport state. Dependencies: internal/config.
package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"blazeai/internal/config"
)

const stateFileName = "state.json"

// State holds mutable local desktop transport state.
//
// WHAT:  Stores the selected model and persistent window geometry for the desktop transport.
// WHY:   Desktop-local model changes and window layout should survive restarts.
// PARAMS: SelectedModel — active provider/model_name for the desktop app;
// Window — last persisted window coordinates and size.
type State struct {
	SelectedModel string      `json:"selected_model"`
	Window        WindowState `json:"window"`

	mu     sync.Mutex `json:"-"`
	saveMu sync.Mutex `json:"-"`
}

// WindowState holds the persisted desktop window geometry.
//
// WHAT:  Tracks the last known desktop window position and size.
// WHY:   The desktop companion should reopen where the user left it.
// PARAMS: Initialized — whether bounds have been captured before; X/Y — top-left coordinates;
// Width/Height — saved window size in pixels.
type WindowState struct {
	Initialized bool `json:"initialized"`
	X           int  `json:"x"`
	Y           int  `json:"y"`
	Width       int  `json:"width"`
	Height      int  `json:"height"`
}

// WindowBounds is the in-memory window geometry passed to the native layer.
//
// WHAT:  Carries window coordinates and size without JSON concerns.
// WHY:   Native callbacks should use one small transport-local struct.
// PARAMS: X/Y — top-left coordinates; Width/Height — size in pixels.
type WindowBounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

// LoadState loads and validates the singleton desktop state.
//
// WHAT:  Reads app_home/desktop/state.json.
// WHY:   Desktop startup must fail fast on missing or invalid local state.
// PARAMS: cfg — loaded global runtime config used to validate providers.
// RETURNS: *State — validated state; string — state path; error on failure.
func LoadState(cfg *config.Config) (*State, string, error) {
	dir, err := InstanceDir()
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, stateFileName)
	state, err := LoadStateFrom(path, cfg)
	if err != nil {
		return nil, path, err
	}
	return state, path, nil
}

// LoadStateFrom loads and validates desktop state from an explicit file path.
//
// WHAT:  Reads one desktop state file from disk.
// WHY:   Tests and startup share the same validation logic.
// PARAMS: path — absolute state file path; cfg — loaded global runtime config.
// RETURNS: *State — validated state; error on read, parse, or validation failure.
func LoadStateFrom(path string, cfg *config.Config) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("desktop state file missing: %s", path)
		}
		return nil, fmt.Errorf("cannot read desktop state file %s: %w", path, err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("cannot parse desktop state file %s: %w", path, err)
	}
	if err := state.Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid desktop state file %s: %w", path, err)
	}
	return &state, nil
}

// Validate checks the selected model against global providers.
//
// WHAT:  Validates the desktop-local selected model and optional stored window geometry.
// WHY:   The transport must stop instead of silently falling back to another model or broken bounds.
// PARAMS: cfg — loaded global runtime config.
// RETURNS: error if the selected model is missing or invalid.
func (s *State) Validate(cfg *config.Config) error {
	snapshot := s.snapshot()
	return snapshot.validate(cfg)
}

func (s *State) validate(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("global config is required")
	}
	modelID := strings.TrimSpace(s.SelectedModel)
	if modelID == "" {
		return fmt.Errorf("selected_model is required")
	}
	providerName, modelName := config.SplitModelID(modelID)
	if providerName == "" || modelName == "" || strings.Contains(modelName, "/") {
		return fmt.Errorf("selected_model must be in provider/model_name format")
	}
	if cfg.ProviderByName(providerName) == nil {
		return fmt.Errorf("selected_model provider not found: %s", providerName)
	}
	if s.Window.Initialized {
		if s.Window.Width <= 0 {
			return fmt.Errorf("window.width must be greater than zero")
		}
		if s.Window.Height <= 0 {
			return fmt.Errorf("window.height must be greater than zero")
		}
	}
	return nil
}

// SelectedModelValue returns the active desktop-local model safely.
func (s *State) SelectedModelValue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.SelectedModel
}

// SetSelectedModel updates the active desktop-local model safely.
func (s *State) SetSelectedModel(modelID string) {
	s.mu.Lock()
	s.SelectedModel = modelID
	s.mu.Unlock()
}

// WindowBoundsValue returns the persisted bounds and whether they are initialized.
func (s *State) WindowBoundsValue() (WindowBounds, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Window.Initialized {
		return WindowBounds{}, false
	}
	return WindowBounds{X: s.Window.X, Y: s.Window.Y, Width: s.Window.Width, Height: s.Window.Height}, true
}

// UpdateWindowBounds stores the latest window geometry and reports whether it changed.
func (s *State) UpdateWindowBounds(bounds WindowBounds) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := !s.Window.Initialized ||
		s.Window.X != bounds.X ||
		s.Window.Y != bounds.Y ||
		s.Window.Width != bounds.Width ||
		s.Window.Height != bounds.Height
	s.Window = WindowState{
		Initialized: true,
		X:           bounds.X,
		Y:           bounds.Y,
		Width:       bounds.Width,
		Height:      bounds.Height,
	}
	return changed
}

func (s *State) snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return State{SelectedModel: s.SelectedModel, Window: s.Window}
}

// SaveTo writes state.json atomically to an explicit path.
//
// WHAT:  Persists desktop-local selected model changes and window geometry.
// WHY:   UI model switches and native window changes must survive restarts without touching global config.
// PARAMS: path — absolute state file path; cfg — loaded global runtime config.
// RETURNS: error if validation or persistence fails.
func (s *State) SaveTo(path string, cfg *config.Config) error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	snapshot := s.snapshot()
	if err := snapshot.validate(cfg); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create desktop state directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal desktop state: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("cannot write temp desktop state %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("cannot commit desktop state %s: %w", path, err)
	}
	return nil
}
