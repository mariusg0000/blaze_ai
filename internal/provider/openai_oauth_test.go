// openai_oauth_test.go — tests for console-owned ChatGPT OAuth configuration.
package provider

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"blazeai/internal/config"
)

// TestInstallChatGPTProvider verifies OAuth credentials and model favorites are installed together.
func TestInstallChatGPTProvider(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.Provider{
			{Name: "openrouter", Endpoint: "https://openrouter.ai/api/v1", APIKey: "sk-test"},
		},
		FavoriteModels: []string{"openrouter/test-model"},
	}
	credential := config.OAuthCredential{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
	}

	if err := InstallChatGPTProvider(cfg, credential); err != nil {
		t.Fatalf("InstallChatGPTProvider() error: %v", err)
	}

	installed := cfg.ProviderByName(ChatGPTOAuthProviderName)
	if installed == nil {
		t.Fatal("ChatGPT OAuth provider was not installed")
	}
	if installed.AuthType != config.OAuthAuthType {
		t.Errorf("AuthType = %q, want %q", installed.AuthType, config.OAuthAuthType)
	}
	if installed.OAuth == nil || installed.OAuth.RefreshToken != credential.RefreshToken {
		t.Errorf("OAuth credential = %+v, want refresh token %q", installed.OAuth, credential.RefreshToken)
	}
	if !reflect.DeepEqual(cfg.FavoriteModels, []string{"openrouter/test-model"}) {
		t.Errorf("FavoriteModels = %v, want existing favorites unchanged", cfg.FavoriteModels)
	}
}

func TestListChatGPTModelsUsesLiveAccountCatalog(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/backend-api/codex/models" {
			t.Errorf("request = %s %s, want GET /backend-api/codex/models", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("client_version") != chatGPTCodexClientVersion {
			t.Errorf("client_version = %q, want %q", r.URL.Query().Get("client_version"), chatGPTCodexClientVersion)
		}
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("ChatGPT-Account-ID") != "acct_1" {
			t.Errorf("ChatGPT-Account-ID = %q", r.Header.Get("ChatGPT-Account-ID"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"slug":"gpt-live-a"},{"slug":""},{"slug":"gpt-live-b"}]}`))
	})}}
	server.Start()
	defer server.Close()

	client := &Client{
		Endpoint: server.URL + "/backend-api/codex/responses",
		AuthType: config.OAuthAuthType,
		OAuth: &config.OAuthCredential{
			AccessToken: "access-token",
			RefreshToken: "refresh-token",
			ExpiresAt:   time.Now().Add(time.Hour).UnixMilli(),
			AccountID:   "acct_1",
		},
		HTTP: server.Client(),
	}

	models, err := client.ListModels()
	if err != nil {
		t.Fatalf("ListModels() error: %v", err)
	}
	want := []string{"gpt-live-a", "gpt-live-b"}
	if !reflect.DeepEqual(models, want) {
		t.Errorf("ListModels() = %v, want %v", models, want)
	}
}

func TestExchangeChatGPTCodeUsesCodexTokenExchange(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("grant_type") == "authorization_code" {
			if r.Form.Get("code_verifier") != "verifier" {
				t.Errorf("code_verifier = %q, want verifier", r.Form.Get("code_verifier"))
			}
			_, _ = w.Write([]byte(`{"id_token":"header.eyJodHRwczovL2FwaS5vcGVuYWkuY29tL2F1dGgiOnsiY2hhdGdwdF9hY2NvdW50X2lkIjoiYWNjdF8xIn19.sig","access_token":"access","refresh_token":"refresh","expires_in":3600}`))
			return
		}
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:token-exchange" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("requested_token") != "openai-api-key" {
			t.Errorf("requested_token = %q", r.Form.Get("requested_token"))
		}
		if r.Form.Get("subject_token_type") != "urn:ietf:params:oauth:token-type:id_token" {
			t.Errorf("subject_token_type = %q", r.Form.Get("subject_token_type"))
		}
		_, _ = w.Write([]byte(`{"access_token":"api-key"}`))
	})}}
	server.Start()
	defer server.Close()

	credential, err := exchangeChatGPTCodeAt(t.Context(), server.URL, "code", "http://localhost:1455/auth/callback", "verifier")
	if err != nil {
		t.Fatalf("exchangeChatGPTCodeAt() error = %v", err)
	}
	if credential.IDToken == "" || credential.APIKey != "api-key" || credential.AccountID != "acct_1" {
		t.Fatalf("credential = %+v, want id token, api key, and account id", credential)
	}
}

func TestExchangeChatGPTCodeAllowsOptionalAPIKeyExchangeFailure(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("grant_type") == "authorization_code" {
			_, _ = w.Write([]byte(`{"id_token":"header.eyJodHRwOi8vYXBpLm9wZW5haS5jb20vYXV0aCI6eyJjaGF0Z3B0X2FjY291bnRfaWQiOiJhY2N0XzEifX0.sig","access_token":"access","refresh_token":"refresh","expires_in":3600}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})}}
	server.Start()
	defer server.Close()

	credential, err := exchangeChatGPTCodeAt(t.Context(), server.URL, "code", "http://localhost:1455/auth/callback", "verifier")
	if err != nil {
		t.Fatalf("exchangeChatGPTCodeAt() error = %v", err)
	}
	if credential.AccessToken != "access" || credential.RefreshToken != "refresh" {
		t.Fatalf("credential = %+v, want OAuth tokens despite optional API key exchange failure", credential)
	}
	if credential.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty after optional exchange failure", credential.APIKey)
	}
}

func TestBuildChatGPTAuthorizeURLUsesCodexScopes(t *testing.T) {
	authorizeURL := buildChatGPTAuthorizeURL("http://localhost:1455/auth/callback", "challenge", "state")
	parsed, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := parsed.Query().Get("scope"); got != "openid profile email offline_access api.connectors.read api.connectors.invoke" {
		t.Fatalf("scope = %q", got)
	}
}

// TestNewClientForProviderOAuth verifies provider-level model listing retains OAuth credentials.
func TestNewClientForProviderOAuth(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.Provider{ChatGPTProvider(config.OAuthCredential{RefreshToken: "refresh-token"})},
	}

	client, err := NewClientForProvider(cfg, ChatGPTOAuthProviderName)
	if err != nil {
		t.Fatalf("NewClientForProvider() error: %v", err)
	}
	if client.OAuth == nil || client.OAuth.RefreshToken != "refresh-token" {
		t.Errorf("client OAuth credential = %+v", client.OAuth)
	}
}
