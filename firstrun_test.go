// firstrun_test.go — tests for the first-run interactive setup.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"blazeai/internal/config"
	providerpkg "blazeai/internal/provider"
)

// newBufReader wraps a string in a bufio.Reader for test input.
func newBufReader(s string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(s))
}

// overrideAppHome sets a temp directory as the app home for config.Save().
// This works because config.Save() calls platform.AppHome() which calls os.UserHomeDir().
// Since we can't easily override os.UserHomeDir, we use HOME env var on Linux/macOS.
func overrideAppHome(t *testing.T, tmpDir string) {
	t.Helper()
	t.Setenv("HOME", tmpDir)
}

// TestSelectProviderKnown verifies selection of a known provider by number.
func TestSelectProviderKnown(t *testing.T) {
	var out bytes.Buffer
	provider, err := selectProvider(&out, newBufReader("1\n"))
	if err != nil {
		t.Fatalf("selectProvider() error: %v", err)
	}
	if provider.Name != "openrouter" {
		t.Errorf("provider name = %q, want 'openrouter'", provider.Name)
	}
	if provider.Endpoint == "" {
		t.Error("endpoint is empty")
	}
	if provider.APIKey != "" {
		t.Error("APIKey should be empty before key entry")
	}
}

// TestSelectProviderCustom verifies custom provider entry.
func TestSelectProviderCustom(t *testing.T) {
	var out bytes.Buffer
	input := fmt.Sprintf("%d\nmyprov\nhttps://api.example.com/v1\n", len(knownProviders)+2)
	provider, err := selectProvider(&out, newBufReader(input))
	if err != nil {
		t.Fatalf("selectProvider() error: %v", err)
	}
	if provider.Name != "myprov" {
		t.Errorf("provider name = %q, want 'myprov'", provider.Name)
	}
	if provider.Endpoint != "https://api.example.com/v1" {
		t.Errorf("endpoint = %q, want 'https://api.example.com/v1'", provider.Endpoint)
	}
}

// TestSelectProviderChatGPTOAuth verifies the console OAuth provider choice.
func TestSelectProviderChatGPTOAuth(t *testing.T) {
	var out bytes.Buffer
	provider, err := selectProvider(&out, newBufReader(fmt.Sprintf("%d\n", len(knownProviders)+1)))
	if err != nil {
		t.Fatalf("selectProvider() error: %v", err)
	}
	if provider.Name != providerpkg.ChatGPTOAuthProviderName {
		t.Errorf("provider name = %q, want %q", provider.Name, providerpkg.ChatGPTOAuthProviderName)
	}
	if provider.AuthType != config.OAuthAuthType {
		t.Errorf("auth type = %q, want %q", provider.AuthType, config.OAuthAuthType)
	}
}

// TestSelectProviderOutOfRange verifies error on out-of-range number.
func TestSelectProviderOutOfRange(t *testing.T) {
	var out bytes.Buffer
	_, err := selectProvider(&out, newBufReader("999\n"))
	if err == nil {
		t.Error("selectProvider() expected error for out-of-range, got nil")
	}
}

// TestSelectProviderInvalid verifies error on non-numeric input.
func TestSelectProviderInvalid(t *testing.T) {
	var out bytes.Buffer
	_, err := selectProvider(&out, newBufReader("abc\n"))
	if err == nil {
		t.Error("selectProvider() expected error for non-numeric, got nil")
	}
}

func TestFirstRunModelDefinition(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		model    string
		want     config.ModelDefinition
		wantErr  bool
	}{
		{
			name:     "standard",
			protocol: config.ProtocolOpenAIChat,
			model:    "gpt-4",
			want: config.ModelDefinition{
				Protocol:     config.ProtocolOpenAIChat,
				Capabilities: config.ModelCapabilities{Tools: true, Reasoning: false},
				OpenAIChat:   &config.OpenAIChatVariant{IncludeStreamUsage: true, IncludeReasoningContent: true},
			},
		},
		{
			name:     "OAuth Responses",
			protocol: config.ProtocolOpenAIResponses,
			model:    "gpt-5.6-mini",
			want: config.ModelDefinition{
				Protocol:     config.ProtocolOpenAIResponses,
				Capabilities: config.ModelCapabilities{Tools: true, Reasoning: false},
				Responses:    &config.ResponsesVariant{Lite: providerpkg.IsResponsesLiteModel("gpt-5.6-mini")},
			},
		},
		{name: "unknown protocol", protocol: "unknown", model: "model", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition, err := firstRunModelDefinition(test.protocol, test.model)
			if test.wantErr {
				if err == nil {
					t.Fatal("firstRunModelDefinition() expected error for unknown protocol, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("firstRunModelDefinition() error: %v", err)
			}
			if !reflect.DeepEqual(definition, test.want) {
				t.Errorf("firstRunModelDefinition() = %+v, want %+v", definition, test.want)
			}
		})
	}
}

// TestFirstRunFullFlow verifies the complete setup with a mock model endpoint.
func TestFirstRunFullFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			fmt.Fprint(w, `{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalProviders := knownProviders
	knownProviders = []config.Provider{
		{Name: "testprov", Endpoint: server.URL},
	}
	defer func() { knownProviders = originalProviders }()

	tmpHome := t.TempDir()
	overrideAppHome(t, tmpHome)

	var out bytes.Buffer
	// Select provider 1, API key, model 1, skip vision, skip summarization, skip advisor.
	cfg, err := firstRun(&out, newBufReader("1\ntest-key\n1\nn\nn\nn\n"))
	if err != nil {
		t.Fatalf("firstRun() error: %v", err)
	}
	if cfg.Roles.Default != "testprov/gpt-3.5-turbo" {
		t.Errorf("default role = %q, want 'testprov/gpt-3.5-turbo'", cfg.Roles.Default)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].APIKey != "test-key" {
		t.Errorf("provider not configured correctly: %+v", cfg.Providers)
	}
	wantDefinition := config.ModelDefinition{
		Protocol:     config.ProtocolOpenAIChat,
		Capabilities: config.ModelCapabilities{Tools: true, Reasoning: false},
		OpenAIChat:   &config.OpenAIChatVariant{IncludeStreamUsage: true, IncludeReasoningContent: true},
	}
	if definition := cfg.Models[cfg.Roles.Default]; !reflect.DeepEqual(definition, wantDefinition) {
		t.Errorf("default model definition = %+v, want %+v", definition, wantDefinition)
	}

	// Verify config was saved to disk.
	configPath := filepath.Join(tmpHome, "blazeai", "config", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config.json not saved at %s: %v", configPath, err)
	}

	// Verify modes.json was NOT created.
	modesPath := filepath.Join(tmpHome, "blazeai", "config", "modes.json")
	if _, err := os.Stat(modesPath); !os.IsNotExist(err) {
		t.Errorf("modes.json should not exist after first run, but found at %s", modesPath)
	}
}

// TestFirstRunNoModels verifies error when provider returns no models.
func TestFirstRunNoModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	originalProviders := knownProviders
	knownProviders = []config.Provider{
		{Name: "testprov", Endpoint: server.URL},
	}
	defer func() { knownProviders = originalProviders }()

	tmpHome := t.TempDir()
	overrideAppHome(t, tmpHome)

	var out bytes.Buffer
	_, err := firstRun(&out, newBufReader("1\ntest-key\n"))
	if err == nil {
		t.Fatal("firstRun() expected error for no models, got nil")
	}
}

// TestFirstRunModelRetrievalFailure verifies error when the /models endpoint fails.
func TestFirstRunModelRetrievalFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid key"}`)
	}))
	defer server.Close()

	originalProviders := knownProviders
	knownProviders = []config.Provider{
		{Name: "testprov", Endpoint: server.URL},
	}
	defer func() { knownProviders = originalProviders }()

	tmpHome := t.TempDir()
	overrideAppHome(t, tmpHome)

	var out bytes.Buffer
	_, err := firstRun(&out, newBufReader("1\ntest-key\n"))
	if err == nil {
		t.Fatal("firstRun() expected error for failed model retrieval, got nil")
	}
}

// TestFirstRunOptionalRoles verifies vision, summarization, and advisor role assignment.
func TestFirstRunOptionalRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"}]}`)
	}))
	defer server.Close()

	originalProviders := knownProviders
	knownProviders = []config.Provider{
		{Name: "testprov", Endpoint: server.URL},
	}
	defer func() { knownProviders = originalProviders }()

	tmpHome := t.TempDir()
	overrideAppHome(t, tmpHome)

	var out bytes.Buffer
	// Provider 1, key, model 1, yes vision + model 2, yes summarization + model 2, yes advisor + model 1.
	cfg, err := firstRun(&out, newBufReader("1\ntest-key\n1\ny\n2\ny\n2\ny\n1\n"))
	if err != nil {
		t.Fatalf("firstRun() error: %v", err)
	}
	if cfg.Roles.Vision != "testprov/gpt-4" {
		t.Errorf("vision role = %q, want 'testprov/gpt-4'", cfg.Roles.Vision)
	}
	if cfg.Roles.Summarization != "testprov/gpt-4" {
		t.Errorf("summarization role = %q, want 'testprov/gpt-4'", cfg.Roles.Summarization)
	}
	if cfg.Roles.Advisor != "testprov/gpt-3.5-turbo" {
		t.Errorf("advisor role = %q, want 'testprov/gpt-3.5-turbo'", cfg.Roles.Advisor)
	}
	for _, modelID := range cfg.FavoriteModels {
		if definition := cfg.Models[modelID]; definition.Protocol != config.ProtocolOpenAIChat {
			t.Errorf("model %q protocol = %q, want %q", modelID, definition.Protocol, config.ProtocolOpenAIChat)
		}
	}
}

// TestKnownProvidersCount verifies the curated list does not exceed 15.
func TestKnownProvidersCount(t *testing.T) {
	if len(knownProviders) > 15 {
		t.Errorf("knownProviders has %d entries, max 15 per spec", len(knownProviders))
	}
}

// TestPlatformOS verifies OS detection works.
func TestPlatformOS(t *testing.T) {
	osType, err := platformOS()
	if err != nil {
		t.Fatalf("platformOS() error: %v", err)
	}
	if osType == "" {
		t.Error("osType is empty")
	}
}

// TestFirstRunDoesNotCreateLegacyModes verifies that first-run setup does not
// write config/modes.json. Legacy active-mode creation has been removed.
//
// WHAT: First-run must not create modes.json.
// HOW: Runs firstRun with a mock endpoint and asserts modes.json does not exist.
func TestFirstRunDoesNotCreateLegacyModes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			fmt.Fprint(w, `{"data":[{"id":"gpt-4"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalProviders := knownProviders
	knownProviders = []config.Provider{
		{Name: "testprov", Endpoint: server.URL},
	}
	defer func() { knownProviders = originalProviders }()

	tmpHome := t.TempDir()
	overrideAppHome(t, tmpHome)

	var out bytes.Buffer
	_, err := firstRun(&out, newBufReader("1\ntest-key\n1\nn\nn\nn\n"))
	if err != nil {
		t.Fatalf("firstRun() error: %v", err)
	}

	modesPath := filepath.Join(tmpHome, "blazeai", "config", "modes.json")
	if _, statErr := os.Stat(modesPath); !os.IsNotExist(statErr) {
		t.Errorf("modes.json must not be created by first run, but found at %s", modesPath)
	}
}
