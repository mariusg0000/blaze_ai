// config_test.go — tests for config loading, validation, saving, and first-run detection.
// Uses temp directories for file-based tests to avoid touching the real app home.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// validConfig returns a Config with all required fields populated and valid.
func validConfig() *Config {
	return &Config{
		Providers: []Provider{
			{Name: "openrouter", Endpoint: "https://openrouter.ai/api/v1", APIKey: "sk-test123"},
		},
		FavoriteModels: []string{"openrouter/deepseek-v4-flash"},
		AdapterCatalog: ModelAdapterCatalog{Adapters: map[string]ModelDefinition{
			"openrouter/deepseek-v4-flash": {
				Protocol:     ProtocolOpenAIChat,
				Capabilities: ModelCapabilities{Tools: true},
				OpenAIChat:   &OpenAIChatVariant{IncludeStreamUsage: true, IncludeReasoningContent: true},
			},
			"openrouter/gpt-4o": {
				Protocol:     ProtocolOpenAIChat,
				Capabilities: ModelCapabilities{Tools: true},
				OpenAIChat:   &OpenAIChatVariant{IncludeStreamUsage: true, IncludeReasoningContent: true},
			},
			"openrouter/gpt-4.1": {
				Protocol:     ProtocolOpenAIChat,
				Capabilities: ModelCapabilities{Tools: true},
				OpenAIChat:   &OpenAIChatVariant{IncludeStreamUsage: true, IncludeReasoningContent: true},
			},
		}},
		Roles: Roles{
			Default: "openrouter/deepseek-v4-flash",
			Vision:  "openrouter/gpt-4o",
			Advisor: "openrouter/gpt-4.1",
		},
		Compaction:     DefaultCompaction(),
		StripReasoning: DefaultStripReasoning(),
	}
}

// writeConfigToTemp writes a config JSON to a temp file and returns the path.
func writeConfigToTemp(t *testing.T, cfg any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("cannot marshal test config: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("cannot create temp config dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("cannot write temp config: %v", err)
	}
	if config, ok := cfg.(*Config); ok {
		catalogPath := filepath.Join(filepath.Dir(path), "model_adapters.json")
		catalogData, err := json.MarshalIndent(config.AdapterCatalog, "", "  ")
		if err != nil {
			t.Fatalf("cannot marshal temp model adapter catalog: %v", err)
		}
		if err := os.WriteFile(catalogPath, catalogData, 0600); err != nil {
			t.Fatalf("cannot write temp model adapter catalog: %v", err)
		}
	}
	return path
}

// TestLoadFromValid verifies that a valid config loads without errors.
func TestLoadFromValid(t *testing.T) {
	cfg := validConfig()
	path := writeConfigToTemp(t, cfg)
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() unexpected error: %v", err)
	}
	if loaded.Roles.Default != cfg.Roles.Default {
		t.Errorf("LoadFrom() default = %q, want %q", loaded.Roles.Default, cfg.Roles.Default)
	}
	if loaded.Roles.Advisor != cfg.Roles.Advisor {
		t.Errorf("LoadFrom() advisor = %q, want %q", loaded.Roles.Advisor, cfg.Roles.Advisor)
	}
	if len(loaded.Providers) != 1 {
		t.Errorf("LoadFrom() providers = %d, want 1", len(loaded.Providers))
	}
}

// TestLoadFromMissing verifies that a missing file returns ErrConfigMissing.
func TestLoadFromMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom() expected error for missing file, got nil")
	}
}

// TestLoadFromMalformed verifies that invalid JSON returns a parse error.
func TestLoadFromMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("cannot write malformed config: %v", err)
	}
	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom() expected error for malformed JSON, got nil")
	}
}

// TestValidateValid verifies that a complete valid config passes validation.
func TestValidateValid(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

// TestValidateMissingDefault verifies that an empty default role fails.
func TestValidateMissingDefault(t *testing.T) {
	cfg := validConfig()
	cfg.Roles.Default = ""
	err := cfg.Validate()
	if err != ErrDefaultRoleUnassigned {
		t.Errorf("Validate() err = %v, want ErrDefaultRoleUnassigned", err)
	}
}

// TestValidateInvalidModelFormat verifies that a malformed model ID fails.
func TestValidateInvalidModelFormat(t *testing.T) {
	cfg := validConfig()
	cfg.Roles.Default = "no-slash"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for invalid model format, got nil")
	}
}

// TestValidateInvalidModelTrailingSlash verifies that "provider/" fails.
func TestValidateInvalidModelTrailingSlash(t *testing.T) {
	cfg := validConfig()
	cfg.Roles.Default = "openrouter/"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for trailing slash model, got nil")
	}
}

// TestValidateProviderNotFound verifies that a model referencing a missing provider fails.
func TestValidateProviderNotFound(t *testing.T) {
	cfg := validConfig()
	cfg.Roles.Default = "nonexistent/model-x"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for missing provider, got nil")
	}
}

// TestValidateDuplicateProvider verifies that duplicate provider names fail.
func TestValidateDuplicateProvider(t *testing.T) {
	cfg := validConfig()
	cfg.Providers = append(cfg.Providers, Provider{
		Name: "openrouter", Endpoint: "https://other.com", APIKey: "sk-other",
	})
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for duplicate provider, got nil")
	}
}

// TestValidateEmptyProviderField verifies that an empty provider field fails.
func TestValidateEmptyProviderField(t *testing.T) {
	cfg := validConfig()
	cfg.Providers[0].APIKey = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for empty api_key, got nil")
	}
}

// TestValidateOAuthProvider verifies OAuth providers do not require an API key.
func TestValidateOAuthProvider(t *testing.T) {
	cfg := validConfig()
	cfg.Providers = append(cfg.Providers, Provider{
		Name:     "openai-chatgpt-oauth",
		Endpoint: "https://chatgpt.com/backend-api/codex/responses",
		AuthType: OAuthAuthType,
		OAuth:    &OAuthCredential{RefreshToken: "refresh-token"},
	})
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() OAuth provider error: %v", err)
	}
}

// TestValidateFavoriteModelBadProvider verifies favorite model with missing provider fails.
func TestValidateFavoriteModelBadProvider(t *testing.T) {
	cfg := validConfig()
	cfg.FavoriteModels = append(cfg.FavoriteModels, "ghost/model-y")
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for favorite model with missing provider, got nil")
	}
}

// TestValidateOAuthModelDefinition verifies that an OAuth Responses model entry is valid.
func TestValidateOAuthModelDefinition(t *testing.T) {
	cfg := validConfig()
	cfg.Providers = append(cfg.Providers, Provider{
		Name:     "chatgpt",
		Endpoint: "https://chatgpt.com/backend-api/codex/responses",
		AuthType: OAuthAuthType,
		OAuth:    &OAuthCredential{RefreshToken: "refresh-token"},
	})
	cfg.AdapterCatalog.Adapters["chatgpt/gpt-5.6-mini"] = ModelDefinition{
		Protocol:     ProtocolOpenAIResponses,
		Capabilities: ModelCapabilities{Tools: true},
		Responses:    &ResponsesVariant{Lite: true},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() OAuth catalog entry error: %v", err)
	}
}

// TestValidateOpenAIChatModelDefinition verifies that an OpenAI Chat model entry is valid.
func TestValidateOpenAIChatModelDefinition(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() OpenAI Chat catalog entry error: %v", err)
	}
}

// TestValidateModelCatalogValidEntries verifies valid standard and OAuth entries.
func TestValidateModelCatalogValidEntries(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		modelID  string
		model    ModelDefinition
	}{
		{
			name:     "OpenAI Chat",
			provider: Provider{Name: "standard", Endpoint: "https://api.example.com/v1", APIKey: "sk-test"},
			modelID:  "standard/chat-model",
			model: ModelDefinition{
				Protocol:     ProtocolOpenAIChat,
				Capabilities: ModelCapabilities{Tools: true},
				OpenAIChat:   &OpenAIChatVariant{IncludeStreamUsage: true, IncludeReasoningContent: true},
			},
		},
		{
			name:     "OAuth Responses",
			provider: Provider{Name: "oauth", Endpoint: "https://oauth.example.com", AuthType: OAuthAuthType, OAuth: &OAuthCredential{RefreshToken: "refresh-token"}},
			modelID:  "oauth/responses-model",
			model: ModelDefinition{
				Protocol:     ProtocolOpenAIResponses,
				Capabilities: ModelCapabilities{Tools: true},
				Responses:    &ResponsesVariant{Lite: true},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Providers = append(cfg.Providers, test.provider)
			cfg.AdapterCatalog.Adapters[test.modelID] = test.model
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error for valid catalog entry: %v", err)
			}
		})
	}
}

// TestValidateMissingCatalogReferences verifies all configured model references require catalog entries.
func TestValidateMissingCatalogReferences(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "favorite", mutate: func(cfg *Config) {
			cfg.FavoriteModels = append(cfg.FavoriteModels, "custom/missing-favorite")
			cfg.Providers = append(cfg.Providers, Provider{Name: "custom", Endpoint: "https://custom.example.com", APIKey: "sk-test"})
		}},
		{name: "default role", mutate: func(cfg *Config) {
			cfg.Roles.Default = "custom/missing-default"
			cfg.Providers = append(cfg.Providers, Provider{Name: "custom", Endpoint: "https://custom.example.com", APIKey: "sk-test"})
		}},
		{name: "vision role", mutate: func(cfg *Config) {
			cfg.Roles.Vision = "custom/missing-vision"
			cfg.Providers = append(cfg.Providers, Provider{Name: "custom", Endpoint: "https://custom.example.com", APIKey: "sk-test"})
		}},
		{name: "summarization role", mutate: func(cfg *Config) {
			cfg.Roles.Summarization = "custom/missing-summarization"
			cfg.Providers = append(cfg.Providers, Provider{Name: "custom", Endpoint: "https://custom.example.com", APIKey: "sk-test"})
		}},
		{name: "advisor role", mutate: func(cfg *Config) {
			cfg.Roles.Advisor = "custom/missing-advisor"
			cfg.Providers = append(cfg.Providers, Provider{Name: "custom", Endpoint: "https://custom.example.com", APIKey: "sk-test"})
		}},
		{name: "last model", mutate: func(cfg *Config) {
			cfg.LastModel = "custom/missing-last"
			cfg.Providers = append(cfg.Providers, Provider{Name: "custom", Endpoint: "https://custom.example.com", APIKey: "sk-test"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			test.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() expected missing catalog error for %s", test.name)
			}
			if !strings.Contains(err.Error(), "no model adapter matches") {
				t.Fatalf("Validate() error = %v, want no adapter match error", err)
			}
		})
	}
}

// TestValidateModelCatalogDefinitions verifies protocol and variant constraints.
func TestValidateModelCatalogDefinitions(t *testing.T) {
	tests := map[string]struct {
		mutate func(*Config)
		want   string
	}{
		"unknown protocol": {
			mutate: func(cfg *Config) {
				definition := cfg.AdapterCatalog.Adapters["openrouter/deepseek-v4-flash"]
				definition.Protocol = "unknown"
				cfg.AdapterCatalog.Adapters["openrouter/deepseek-v4-flash"] = definition
			},
			want: "unknown protocol",
		},
		"malformed key": {
			mutate: func(cfg *Config) {
				cfg.AdapterCatalog.Adapters["malformed"] = ModelDefinition{Protocol: ProtocolOpenAIChat, OpenAIChat: &OpenAIChatVariant{}}
			},
			want: ErrInvalidModelFormat.Error(),
		},
		"unknown provider": {
			mutate: func(cfg *Config) {
				cfg.AdapterCatalog.Adapters["ghost/model"] = ModelDefinition{Protocol: ProtocolOpenAIChat, OpenAIChat: &OpenAIChatVariant{}}
			},
			want: ErrProviderNotFound.Error(),
		},
		"protocol auth mismatch": {
			mutate: func(cfg *Config) {
				cfg.AdapterCatalog.Adapters["openrouter/oauth-model"] = ModelDefinition{
					Protocol:  ProtocolOpenAIResponses,
					Responses: &ResponsesVariant{},
				}
			},
			want: "openai-responses requires oauth provider",
		},
		"missing required variant": {
			mutate: func(cfg *Config) {
				definition := cfg.AdapterCatalog.Adapters["openrouter/deepseek-v4-flash"]
				definition.OpenAIChat = nil
				cfg.AdapterCatalog.Adapters["openrouter/deepseek-v4-flash"] = definition
			},
			want: "openai-chat requires openai_chat variant",
		},
		"prohibited other protocol variant": {
			mutate: func(cfg *Config) {
				definition := cfg.AdapterCatalog.Adapters["openrouter/deepseek-v4-flash"]
				definition.Responses = &ResponsesVariant{}
				cfg.AdapterCatalog.Adapters["openrouter/deepseek-v4-flash"] = definition
			},
			want: "openai-chat prohibits responses variant",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			test.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() expected catalog definition error for %s", name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

// TestSaveToAndLoadFrom verifies round-trip save and load.
func TestSaveToAndLoadFrom(t *testing.T) {
	cfg := validConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() unexpected error: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() after SaveTo() unexpected error: %v", err)
	}
	if loaded.Roles.Default != cfg.Roles.Default {
		t.Errorf("round-trip default = %q, want %q", loaded.Roles.Default, cfg.Roles.Default)
	}
	if loaded.Roles.Advisor != cfg.Roles.Advisor {
		t.Errorf("round-trip advisor = %q, want %q", loaded.Roles.Advisor, cfg.Roles.Advisor)
	}
	if loaded.Compaction.MaxContextTokens != cfg.Compaction.MaxContextTokens {
		t.Errorf("round-trip maxContextTokens = %d, want %d",
			loaded.Compaction.MaxContextTokens, cfg.Compaction.MaxContextTokens)
	}
	configData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read config.json: %v", err)
	}
	var configJSON map[string]json.RawMessage
	if err := json.Unmarshal(configData, &configJSON); err != nil {
		t.Fatalf("cannot parse config.json: %v", err)
	}
	if _, ok := configJSON["models"]; ok {
		t.Fatal("config.json contains legacy models key")
	}
	adapterPath := filepath.Join(filepath.Dir(path), "model_adapters.json")
	adapterData, err := os.ReadFile(adapterPath)
	if err != nil {
		t.Fatalf("cannot read model_adapters.json: %v", err)
	}
	var adapterJSON map[string]json.RawMessage
	if err := json.Unmarshal(adapterData, &adapterJSON); err != nil {
		t.Fatalf("cannot parse model_adapters.json: %v", err)
	}
	if len(adapterJSON) != 1 {
		t.Fatalf("model_adapters.json keys = %v, want exactly adapters", adapterJSON)
	}
	if _, ok := adapterJSON["adapters"]; !ok {
		t.Fatal("model_adapters.json has no adapters key")
	}
	resolved, err := loaded.ResolveModelAdapter("openrouter/gpt-4o")
	if err != nil {
		t.Fatalf("ResolveModelAdapter() after round trip failed: %v", err)
	}
	if !reflect.DeepEqual(resolved, cfg.AdapterCatalog.Adapters["openrouter/gpt-4o"]) {
		t.Errorf("resolved adapter = %#v, want %#v", resolved, cfg.AdapterCatalog.Adapters["openrouter/gpt-4o"])
	}
}

func TestResolveModelAdapterExact(t *testing.T) {
	cfg := validConfig()
	want := cfg.AdapterCatalog.Adapters["openrouter/gpt-4o"]
	got, err := cfg.ResolveModelAdapter("openrouter/gpt-4o")
	if err != nil {
		t.Fatalf("ResolveModelAdapter() error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ResolveModelAdapter() = %#v, want %#v", got, want)
	}
}

func TestResolveModelAdapterWildcard(t *testing.T) {
	cfg := validConfig()
	want := ModelDefinition{Protocol: ProtocolOpenAIChat, OpenAIChat: &OpenAIChatVariant{IncludeStreamUsage: true}}
	cfg.AdapterCatalog.Adapters = map[string]ModelDefinition{"openai/gpt-5.6-*": want}
	for _, modelID := range []string{"openai/gpt-5.6-sol", "openai/gpt-5.6-terra", "openai/gpt-5.6-luna"} {
		got, err := cfg.ResolveModelAdapter(modelID)
		if err != nil {
			t.Fatalf("ResolveModelAdapter(%q) error: %v", modelID, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ResolveModelAdapter(%q) = %#v, want %#v", modelID, got, want)
		}
	}
}

func TestResolveModelAdapterExactOverridesWildcard(t *testing.T) {
	cfg := validConfig()
	wildcard := ModelDefinition{Protocol: ProtocolOpenAIChat, OpenAIChat: &OpenAIChatVariant{IncludeStreamUsage: true}}
	exact := ModelDefinition{Protocol: ProtocolOpenAIChat, OpenAIChat: &OpenAIChatVariant{IncludeReasoningContent: true}}
	cfg.AdapterCatalog.Adapters = map[string]ModelDefinition{
		"openai/gpt-5.6-*":    wildcard,
		"openai/gpt-5.6-luna": exact,
	}
	got, err := cfg.ResolveModelAdapter("openai/gpt-5.6-luna")
	if err != nil {
		t.Fatalf("ResolveModelAdapter() error: %v", err)
	}
	if !reflect.DeepEqual(got, exact) {
		t.Errorf("ResolveModelAdapter() = %#v, want exact %#v", got, exact)
	}
}

func TestResolveModelAdapterRejectsInvalidRequestedIDs(t *testing.T) {
	cfg := validConfig()
	for _, test := range []struct {
		name    string
		modelID string
	}{
		{name: "wildcard suffix", modelID: "openai/gpt-5.6-*"},
		{name: "embedded star", modelID: "openai/g*pt-5.6"},
		{name: "multiple stars", modelID: "openai/gpt-5.6-**"},
		{name: "malformed separator count", modelID: "openai"},
		{name: "empty ID", modelID: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := cfg.ResolveModelAdapter(test.modelID); err == nil {
				t.Fatalf("ResolveModelAdapter(%q) expected error, got nil", test.modelID)
			}
		})
	}
}

func TestResolveModelAdapterBuiltinProviders(t *testing.T) {
	chatWithoutUsage := ModelDefinition{
		Protocol:     ProtocolOpenAIChat,
		Capabilities: ModelCapabilities{Tools: true, Reasoning: false},
		OpenAIChat:   &OpenAIChatVariant{IncludeStreamUsage: false, IncludeReasoningContent: false},
	}
	chatWithUsage := ModelDefinition{
		Protocol:     ProtocolOpenAIChat,
		Capabilities: ModelCapabilities{Tools: true, Reasoning: false},
		OpenAIChat:   &OpenAIChatVariant{IncludeStreamUsage: true, IncludeReasoningContent: true},
	}
	responses := ModelDefinition{
		Protocol:     ProtocolOpenAIResponses,
		Capabilities: ModelCapabilities{Tools: true, Reasoning: false},
		Responses:    &ResponsesVariant{Lite: false},
	}
	for _, test := range []struct {
		name     string
		provider Provider
		modelID  string
		want     ModelDefinition
	}{
		{name: "openrouter", provider: Provider{Name: "openrouter", Endpoint: "https://example.com", APIKey: "key"}, modelID: "openrouter/model", want: chatWithoutUsage},
		{name: "opencode-go", provider: Provider{Name: "opencode-go", Endpoint: "https://example.com", APIKey: "key"}, modelID: "opencode-go/model", want: chatWithUsage},
		{name: "opencode-zen", provider: Provider{Name: "opencode-zen", Endpoint: "https://example.com", APIKey: "key"}, modelID: "opencode-zen/model", want: chatWithUsage},
		{name: "google-gemini", provider: Provider{Name: "google-gemini", Endpoint: "https://example.com", APIKey: "key"}, modelID: "google-gemini/model", want: chatWithoutUsage},
		{name: "openai-chatgpt-oauth", provider: Provider{Name: "openai-chatgpt-oauth", Endpoint: "https://example.com", AuthType: OAuthAuthType, OAuth: &OAuthCredential{RefreshToken: "refresh"}}, modelID: "openai-chatgpt-oauth/model", want: responses},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{Providers: []Provider{test.provider}}
			got, err := cfg.ResolveModelAdapter(test.modelID)
			if err != nil {
				t.Fatalf("ResolveModelAdapter() error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("ResolveModelAdapter() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestResolveModelAdapterBuiltinOAuthModelOverride(t *testing.T) {
	cfg := &Config{Providers: []Provider{{
		Name:     "openai-chatgpt-oauth",
		Endpoint: "https://example.com",
		AuthType: OAuthAuthType,
		OAuth:    &OAuthCredential{RefreshToken: "refresh"},
	}}}
	lite, err := cfg.ResolveModelAdapter("openai-chatgpt-oauth/gpt-5.6-luna")
	if err != nil {
		t.Fatalf("ResolveModelAdapter() lite model error: %v", err)
	}
	if lite.Responses == nil || !lite.Responses.Lite {
		t.Errorf("gpt-5.6-luna Responses = %#v, want Lite true", lite.Responses)
	}
	standard, err := cfg.ResolveModelAdapter("openai-chatgpt-oauth/gpt-5.7-luna")
	if err != nil {
		t.Fatalf("ResolveModelAdapter() provider model error: %v", err)
	}
	if standard.Responses == nil || standard.Responses.Lite {
		t.Errorf("gpt-5.7-luna Responses = %#v, want Lite false", standard.Responses)
	}
}

func TestResolveModelAdapterExternalOverridesBuiltins(t *testing.T) {
	exact := ModelDefinition{Protocol: ProtocolOpenAIChat, OpenAIChat: &OpenAIChatVariant{IncludeStreamUsage: true}}
	wildcard := ModelDefinition{Protocol: ProtocolOpenAIChat, OpenAIChat: &OpenAIChatVariant{IncludeReasoningContent: true}}
	cfg := &Config{
		Providers: []Provider{{Name: "openrouter", Endpoint: "https://example.com", APIKey: "key"}},
		AdapterCatalog: ModelAdapterCatalog{Adapters: map[string]ModelDefinition{
			"openrouter/exact": exact,
			"openrouter/wild-*": wildcard,
		}},
	}
	for modelID, want := range map[string]ModelDefinition{
		"openrouter/exact":      exact,
		"openrouter/wild-model": wildcard,
	} {
		got, err := cfg.ResolveModelAdapter(modelID)
		if err != nil {
			t.Fatalf("ResolveModelAdapter(%q) error: %v", modelID, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ResolveModelAdapter(%q) = %#v, want %#v", modelID, got, want)
		}
	}
}

func TestResolveModelAdapterDoesNotMatchBuiltinByEndpoint(t *testing.T) {
	cfg := &Config{Providers: []Provider{{Name: "renamed", Endpoint: "https://openrouter.ai/api/v1", APIKey: "key"}}}
	_, err := cfg.ResolveModelAdapter("renamed/model")
	if err == nil || !strings.Contains(err.Error(), "no model adapter matches") {
		t.Fatalf("ResolveModelAdapter() error = %v, want no adapter match", err)
	}
}

func TestResolveModelAdapterBuiltinAuthMismatch(t *testing.T) {
	for _, test := range []struct {
		name     string
		provider Provider
		modelID  string
		want     string
	}{
		{name: "chat on oauth", provider: Provider{Name: "openrouter", Endpoint: "https://example.com", AuthType: OAuthAuthType, OAuth: &OAuthCredential{RefreshToken: "refresh"}}, modelID: "openrouter/model", want: "openai-chat requires non-oauth provider"},
		{name: "responses on api key", provider: Provider{Name: "openai-chatgpt-oauth", Endpoint: "https://example.com", APIKey: "key"}, modelID: "openai-chatgpt-oauth/model", want: "openai-responses requires oauth provider"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{Providers: []Provider{test.provider}}
			_, err := cfg.ResolveModelAdapter(test.modelID)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ResolveModelAdapter() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateModelAdapterPatterns(t *testing.T) {
	for _, test := range []struct {
		name    string
		pattern string
	}{
		{name: "wildcard provider", pattern: "*/model*"},
		{name: "no model prefix", pattern: "openrouter/*"},
		{name: "embedded star", pattern: "openrouter/g*pt"},
		{name: "multiple stars", pattern: "openrouter/g**"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.AdapterCatalog.Adapters = map[string]ModelDefinition{
				test.pattern: {
					Protocol:   ProtocolOpenAIChat,
					OpenAIChat: &OpenAIChatVariant{},
				},
			}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() accepted invalid adapter pattern %q", test.pattern)
			}
		})
	}
}

func TestLoadFromMigratesLegacyModels(t *testing.T) {
	cfg := validConfig()
	legacy := cfg.AdapterCatalog.Adapters
	cfg.AdapterCatalog = ModelAdapterCatalog{}
	cfg.LegacyModels = legacy
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("cannot create config dir: %v", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("cannot marshal legacy config: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("cannot write legacy config: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() migration error: %v", err)
	}
	if !reflect.DeepEqual(loaded.AdapterCatalog.Adapters, legacy) {
		t.Errorf("migrated adapters = %#v, want %#v", loaded.AdapterCatalog.Adapters, legacy)
	}
	if len(loaded.LegacyModels) != 0 {
		t.Errorf("LegacyModels = %#v, want empty", loaded.LegacyModels)
	}
	adapterPath := filepath.Join(filepath.Dir(path), "model_adapters.json")
	if _, err := os.Stat(adapterPath); err != nil {
		t.Fatalf("migration did not create adapter catalog: %v", err)
	}
	rewritten, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read migrated config: %v", err)
	}
	var rewrittenJSON map[string]json.RawMessage
	if err := json.Unmarshal(rewritten, &rewrittenJSON); err != nil {
		t.Fatalf("cannot parse migrated config: %v", err)
	}
	if _, ok := rewrittenJSON["models"]; ok {
		t.Fatal("migrated config still contains models key")
	}
}

func TestLoadFromLegacyAndAdapterConflict(t *testing.T) {
	cfg := validConfig()
	cfg.AdapterCatalog = ModelAdapterCatalog{}
	cfg.LegacyModels = validConfig().AdapterCatalog.Adapters
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("cannot create config dir: %v", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("cannot marshal conflict config: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("cannot write conflict config: %v", err)
	}
	adapterData, err := json.Marshal(ModelAdapterCatalog{Adapters: map[string]ModelDefinition{}})
	if err != nil {
		t.Fatalf("cannot marshal adapter catalog: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "model_adapters.json"), adapterData, 0600); err != nil {
		t.Fatalf("cannot write adapter catalog: %v", err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("LoadFrom() accepted legacy and adapter catalog conflict")
	}
}

func TestLoadFromMissingAdapterCatalog(t *testing.T) {
	cfg := validConfig()
	cfg.AdapterCatalog = ModelAdapterCatalog{}
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("cannot create config dir: %v", err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("cannot marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("cannot write config: %v", err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("LoadFrom() accepted config without adapter catalog")
	}
}

// TestReloadModelAdaptersPreservesNonModelConfig verifies that ReloadModelAdapters replaces only the catalog.
func TestReloadModelAdaptersPreservesNonModelConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := validConfig()
	cfg.AdapterCatalog.Adapters = map[string]ModelDefinition{
		"openrouter/deepseek-v4-flash": cfg.AdapterCatalog.Adapters["openrouter/deepseek-v4-flash"],
	}
	cfg.Roles = Roles{Default: "openrouter/deepseek-v4-flash"}
	cfg.FavoriteModels = []string{"openrouter/deepseek-v4-flash"}
	cfg.LastModel = "openrouter/deepseek-v4-flash"
	cfg.HelperSetup = HelperSetup{Dismissed: true, Declined: []string{"rg"}}
	cfg.DebugPrompt = true
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	wantProviders := append([]Provider(nil), cfg.Providers...)
	wantRoles := cfg.Roles
	wantFavorites := append([]string(nil), cfg.FavoriteModels...)
	wantLastModel := cfg.LastModel
	wantCompaction := cfg.Compaction
	wantHelperSetup := cfg.HelperSetup
	wantDebugPrompt := cfg.DebugPrompt
	wantAdapters := cfg.AdapterCatalog.Adapters
	cfg.AdapterCatalog.Adapters = map[string]ModelDefinition{
		"openrouter/in-memory-model": {
			Protocol:   ProtocolOpenAIChat,
			OpenAIChat: &OpenAIChatVariant{},
		},
	}

	if err := cfg.ReloadModelAdapters(); err != nil {
		t.Fatalf("ReloadModelAdapters() unexpected error: %v", err)
	}
	if !reflect.DeepEqual(cfg.AdapterCatalog.Adapters, wantAdapters) {
		t.Errorf("AdapterCatalog.Adapters = %#v, want saved catalog %#v", cfg.AdapterCatalog.Adapters, wantAdapters)
	}
	if !reflect.DeepEqual(cfg.Providers, wantProviders) {
		t.Errorf("Providers changed during reload: %#v", cfg.Providers)
	}
	if cfg.Roles != wantRoles {
		t.Errorf("Roles changed during reload: %#v", cfg.Roles)
	}
	if !reflect.DeepEqual(cfg.FavoriteModels, wantFavorites) {
		t.Errorf("FavoriteModels changed during reload: %#v", cfg.FavoriteModels)
	}
	if cfg.LastModel != wantLastModel {
		t.Errorf("LastModel changed during reload: %q", cfg.LastModel)
	}
	if cfg.Compaction != wantCompaction {
		t.Errorf("Compaction changed during reload: %#v", cfg.Compaction)
	}
	if !reflect.DeepEqual(cfg.HelperSetup, wantHelperSetup) {
		t.Errorf("HelperSetup changed during reload: %#v", cfg.HelperSetup)
	}
	if cfg.DebugPrompt != wantDebugPrompt {
		t.Errorf("DebugPrompt changed during reload: %t", cfg.DebugPrompt)
	}
}

// TestReloadModelAdaptersLoadFailure verifies that a strict load failure leaves the catalog unchanged.
func TestReloadModelAdaptersLoadFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := validConfig()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}
	cfg.AdapterCatalog.Adapters = map[string]ModelDefinition{
		"openrouter/in-memory-model": {
			Protocol:   ProtocolOpenAIChat,
			OpenAIChat: &OpenAIChatVariant{},
		},
	}
	wantAdapters := cfg.AdapterCatalog.Adapters
	configPath := filepath.Join(home, "blazeai", "config", "config.json")
	if err := os.WriteFile(configPath, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("cannot write invalid config: %v", err)
	}

	if err := cfg.ReloadModelAdapters(); err == nil {
		t.Fatal("ReloadModelAdapters() expected strict load error, got nil")
	}
	if !reflect.DeepEqual(cfg.AdapterCatalog.Adapters, wantAdapters) {
		t.Errorf("AdapterCatalog.Adapters changed after reload failure: %#v", cfg.AdapterCatalog.Adapters)
	}
}

// TestSaveToCreatesDir verifies that SaveTo creates parent directories.
func TestSaveToCreatesDir(t *testing.T) {
	cfg := validConfig()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() with nested dirs failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("SaveTo() did not create file: %v", err)
	}
}

// TestNeedsFirstRunAtMissing verifies that a missing file triggers first-run.
func TestNeedsFirstRunAtMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	needed, err := NeedsFirstRunAt(path)
	if err != nil {
		t.Fatalf("NeedsFirstRunAt() unexpected error: %v", err)
	}
	if !needed {
		t.Error("NeedsFirstRunAt() = false for missing file, want true")
	}
}

// TestNeedsFirstRunAtNoDefault verifies that an empty default role triggers first-run.
func TestNeedsFirstRunAtNoDefault(t *testing.T) {
	cfg := Default()
	cfg.Providers = []Provider{
		{Name: "test", Endpoint: "https://example.com", APIKey: "sk-test"},
	}
	path := writeConfigToTemp(t, cfg)
	needed, err := NeedsFirstRunAt(path)
	if err != nil {
		t.Fatalf("NeedsFirstRunAt() unexpected error: %v", err)
	}
	if !needed {
		t.Error("NeedsFirstRunAt() = false for empty default role, want true")
	}
}

// TestNeedsFirstRunAtConfigured verifies that a valid config with default role does not trigger.
func TestNeedsFirstRunAtConfigured(t *testing.T) {
	cfg := validConfig()
	path := writeConfigToTemp(t, cfg)
	needed, err := NeedsFirstRunAt(path)
	if err != nil {
		t.Fatalf("NeedsFirstRunAt() unexpected error: %v", err)
	}
	if needed {
		t.Error("NeedsFirstRunAt() = true for configured config, want false")
	}
}

// TestDefaultCompaction verifies spec default values.
func TestDefaultCompaction(t *testing.T) {
	c := DefaultCompaction()
	if c.MaxContextTokens != 100000 {
		t.Errorf("MaxContextTokens = %d, want 100000", c.MaxContextTokens)
	}
	if c.MinContextTokens != 50000 {
		t.Errorf("MinContextTokens = %d, want 50000", c.MinContextTokens)
	}
	if c.SummaryMaxTokens != 2000 {
		t.Errorf("SummaryMaxTokens = %d, want 2000", c.SummaryMaxTokens)
	}
	if c.MaxSummaryFiles != 10 {
		t.Errorf("MaxSummaryFiles = %d, want 10", c.MaxSummaryFiles)
	}
	if c.TokenCoefficient != 3.5 {
		t.Errorf("TokenCoefficient = %f, want 3.5", c.TokenCoefficient)
	}
	if c.MaxBackoffOffsetTokens != 25000 {
		t.Errorf("MaxBackoffOffsetTokens = %d, want 25000", c.MaxBackoffOffsetTokens)
	}
}

// TestDefaultStripReasoning verifies spec default values.
func TestDefaultStripReasoning(t *testing.T) {
	sr := DefaultStripReasoning()
	if !sr.Enable {
		t.Error("Enable = false, want true")
	}
	if sr.PreserveLast != 5 {
		t.Errorf("PreserveLast = %d, want 5", sr.PreserveLast)
	}
}

// TestDefault verifies that Default returns a config with populated defaults and empty roles.
func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.DebugPrompt {
		t.Error("Default() DebugPrompt = true, want false")
	}
	if cfg.Roles.Default != "" {
		t.Errorf("Default() Roles.Default = %q, want empty", cfg.Roles.Default)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("Default() Providers = %d, want 0", len(cfg.Providers))
	}
	if cfg.Compaction.MaxContextTokens != 100000 {
		t.Errorf("Default() Compaction.MaxContextTokens = %d, want 100000", cfg.Compaction.MaxContextTokens)
	}
	if !cfg.StripReasoning.Enable {
		t.Error("Default() StripReasoning.Enable = false, want true")
	}
}

// TestDebugPromptRoundTrip verifies the explicit prompt-debug configuration flag.
func TestDebugPromptRoundTrip(t *testing.T) {
	cfg := validConfig()
	cfg.DebugPrompt = true
	path := filepath.Join(t.TempDir(), "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() error: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if !loaded.DebugPrompt {
		t.Fatal("loaded DebugPrompt = false, want true")
	}

	data, err := json.Marshal(Default())
	if err != nil {
		t.Fatalf("Marshal(Default()) error: %v", err)
	}
	var decoded Config
	if err := json.Unmarshal(append(data[:len(data)-1], []byte(`,"debugPrompt":true}`)...), &decoded); err != nil {
		t.Fatalf("Unmarshal(debugPrompt) error: %v", err)
	}
	if !decoded.DebugPrompt {
		t.Fatal("decoded DebugPrompt = false, want true")
	}
}

// TestProviderByName verifies lookup by name.
func TestProviderByName(t *testing.T) {
	cfg := validConfig()
	p := cfg.ProviderByName("openrouter")
	if p == nil {
		t.Fatal("ProviderByName() returned nil for existing provider")
	}
	if p.Endpoint != "https://openrouter.ai/api/v1" {
		t.Errorf("ProviderByName() endpoint = %q, want correct URL", p.Endpoint)
	}
}

// TestProviderByNameNotFound verifies nil for a non-existent provider.
func TestProviderByNameNotFound(t *testing.T) {
	cfg := validConfig()
	p := cfg.ProviderByName("ghost")
	if p != nil {
		t.Error("ProviderByName() returned non-nil for missing provider")
	}
}

// TestModelForRole verifies configured role resolution.
func TestModelForRole(t *testing.T) {
	cfg := validConfig()
	modelID, err := cfg.ModelForRole("advisor")
	if err != nil {
		t.Fatalf("ModelForRole() error: %v", err)
	}
	if modelID != cfg.Roles.Advisor {
		t.Fatalf("ModelForRole() = %q, want %q", modelID, cfg.Roles.Advisor)
	}
}

// TestSplitModelID verifies that model IDs are split correctly.
func TestSplitModelID(t *testing.T) {
	provider, model := SplitModelID("openrouter/deepseek-v4-flash")
	if provider != "openrouter" {
		t.Errorf("provider = %q, want 'openrouter'", provider)
	}
	if model != "deepseek-v4-flash" {
		t.Errorf("model = %q, want 'deepseek-v4-flash'", model)
	}
}

// TestSplitModelIDNoSlash verifies behavior when there is no separator.
func TestSplitModelIDNoSlash(t *testing.T) {
	provider, model := SplitModelID("barename")
	if provider != "barename" {
		t.Errorf("provider = %q, want 'barename'", provider)
	}
	if model != "" {
		t.Errorf("model = %q, want empty", model)
	}
}

// TestDefaultHelperSetup verifies default HelperSetup values.
func TestDefaultHelperSetup(t *testing.T) {
	cfg := Default()
	if cfg.HelperSetup.Dismissed {
		t.Error("Default() HelperSetup.Dismissed = true, want false")
	}
	if cfg.HelperSetup.Declined == nil {
		t.Error("Default() HelperSetup.Declined = nil, want empty slice")
	}
}

// TestLoadFromWithoutHelperSetup verifies backward-compatibility:
// configs without helperSetup field load successfully with zero-value.
func TestLoadFromWithoutHelperSetup(t *testing.T) {
	raw := struct {
		Providers      []Provider                 `json:"providers"`
		FavoriteModels []string                   `json:"favorite_models"`
		Models         map[string]ModelDefinition `json:"models"`
		Roles          Roles                      `json:"roles"`
		Compaction     Compaction                 `json:"compaction"`
		StripReasoning StripReasoning             `json:"stripReasoning"`
	}{
		Providers: []Provider{
			{Name: "test", Endpoint: "https://example.com/v1", APIKey: "sk-test"},
		},
		FavoriteModels: []string{"test/model"},
		Models: map[string]ModelDefinition{
			"test/model": {
				Protocol:     ProtocolOpenAIChat,
				Capabilities: ModelCapabilities{Tools: true},
				OpenAIChat:   &OpenAIChatVariant{IncludeStreamUsage: true, IncludeReasoningContent: true},
			},
		},
		Roles: Roles{
			Default: "test/model",
		},
		Compaction:     DefaultCompaction(),
		StripReasoning: DefaultStripReasoning(),
	}
	path := writeConfigToTemp(t, raw)
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() without helperSetup failed: %v", err)
	}
	if loaded.HelperSetup.Dismissed {
		t.Error("HelperSetup.Dismissed = true for old config, want false (zero-value)")
	}
}

// TestSaveLoadHelperSetup verifies round-trip preservation of helper setup preferences.
func TestSaveLoadHelperSetup(t *testing.T) {
	cfg := validConfig()
	cfg.HelperSetup.Dismissed = true
	cfg.HelperSetup.Declined = []string{"rg", "fd"}
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.json")
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}
	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() after SaveTo() failed: %v", err)
	}
	if !loaded.HelperSetup.Dismissed {
		t.Error("HelperSetup.Dismissed = false, want true")
	}
	if len(loaded.HelperSetup.Declined) != 2 {
		t.Errorf("HelperSetup.Declined = %v, want [rg fd]", loaded.HelperSetup.Declined)
	}
}

// TestAddFavorite verifies adding a valid model to favorites.
func TestAddFavorite(t *testing.T) {
	cfg := validConfig()
	cfg.FavoriteModels = nil // start empty

	if err := cfg.AddFavorite("openrouter/gpt-4.1"); err != nil {
		t.Fatalf("AddFavorite() error: %v", err)
	}
	if len(cfg.FavoriteModels) != 1 {
		t.Fatalf("AddFavorite() len = %d, want 1", len(cfg.FavoriteModels))
	}
	if cfg.FavoriteModels[0] != "openrouter/gpt-4.1" {
		t.Errorf("AddFavorite() = %q, want openrouter/gpt-4.1", cfg.FavoriteModels[0])
	}
}

// TestAddFavoriteDuplicate verifies adding a duplicate is a silent no-op.
func TestAddFavoriteDuplicate(t *testing.T) {
	cfg := validConfig()
	cfg.FavoriteModels = []string{"openrouter/gpt-4.1"}

	if err := cfg.AddFavorite("openrouter/gpt-4.1"); err != nil {
		t.Fatalf("AddFavorite() error: %v", err)
	}
	if len(cfg.FavoriteModels) != 1 {
		t.Errorf("AddFavorite() duplicate len = %d, want 1", len(cfg.FavoriteModels))
	}
}

// TestAddFavoriteInvalidFormat verifies an invalid model format returns an error.
func TestAddFavoriteInvalidFormat(t *testing.T) {
	cfg := validConfig()
	err := cfg.AddFavorite("no-provider")
	if err == nil {
		t.Fatal("AddFavorite() expected error for invalid model format, got nil")
	}
}

// TestAddFavoriteMissingProvider verifies a model with an unknown provider returns an error.
func TestAddFavoriteMissingProvider(t *testing.T) {
	cfg := validConfig()
	err := cfg.AddFavorite("ghost/model-x")
	if err == nil {
		t.Fatal("AddFavorite() expected error for missing provider, got nil")
	}
}

// TestRemoveFavorite verifies removing an existing model from favorites.
func TestRemoveFavorite(t *testing.T) {
	cfg := validConfig()
	cfg.FavoriteModels = []string{"openrouter/a", "openrouter/b", "openrouter/c"}

	removed, err := cfg.RemoveFavorite("openrouter/b")
	if err != nil {
		t.Fatalf("RemoveFavorite() error: %v", err)
	}
	if !removed {
		t.Fatal("RemoveFavorite() returned false, want true")
	}
	if len(cfg.FavoriteModels) != 2 {
		t.Fatalf("RemoveFavorite() len = %d, want 2", len(cfg.FavoriteModels))
	}
	if cfg.FavoriteModels[0] != "openrouter/a" || cfg.FavoriteModels[1] != "openrouter/c" {
		t.Errorf("RemoveFavorite() = %v, want [openrouter/a openrouter/c]", cfg.FavoriteModels)
	}
}

// TestRemoveFavoriteNotFound verifies removing a non-existent model returns false.
func TestRemoveFavoriteNotFound(t *testing.T) {
	cfg := validConfig()
	cfg.FavoriteModels = []string{"openrouter/a"}

	removed, err := cfg.RemoveFavorite("openrouter/ghost")
	if err != nil {
		t.Fatalf("RemoveFavorite() error: %v", err)
	}
	if removed {
		t.Error("RemoveFavorite() returned true for missing model, want false")
	}
	if len(cfg.FavoriteModels) != 1 {
		t.Errorf("RemoveFavorite() len = %d, want 1 (unchanged)", len(cfg.FavoriteModels))
	}
}

// TestRemoveFavoriteLastItem verifies removing the only item leaves an empty list.
func TestRemoveFavoriteLastItem(t *testing.T) {
	cfg := validConfig()
	cfg.FavoriteModels = []string{"openrouter/solo"}

	removed, err := cfg.RemoveFavorite("openrouter/solo")
	if err != nil {
		t.Fatalf("RemoveFavorite() error: %v", err)
	}
	if !removed {
		t.Fatal("RemoveFavorite() returned false, want true")
	}
	if len(cfg.FavoriteModels) != 0 {
		t.Errorf("RemoveFavorite() len = %d, want 0", len(cfg.FavoriteModels))
	}
}
