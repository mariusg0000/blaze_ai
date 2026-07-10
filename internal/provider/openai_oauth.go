// openai_oauth.go — browser OAuth flow and token refresh for the ChatGPT provider.
// Owns the loopback callback server, PKCE authorization, token exchange, and
// refresh requests used by the local desktop integration.
// Layer: external authentication. Dependencies: internal/config and net/http.
package provider

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"blazeai/internal/config"
)

const (
	// ChatGPTOAuthProviderName is the persisted provider id for ChatGPT OAuth.
	ChatGPTOAuthProviderName = "openai-chatgpt-oauth"
	chatGPTOAuthIssuer       = "https://auth.openai.com"
	chatGPTCodexEndpoint     = "https://chatgpt.com/backend-api/codex/responses"
	chatGPTOAuthClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	chatGPTOAuthPort         = 1455
	chatGPTOAuthTimeout      = 5 * time.Minute
)

// ChatGPTOAuthModels lists models exposed by the ChatGPT/Codex entitlement.
//
// WHAT:  Returns the known model identifiers accepted by the OAuth endpoint.
// WHY:   The private Codex endpoint does not expose the public /models listing.
// HOW:   Keeps the supported set explicit and avoids probing undocumented routes.
// PARAMS: none.
// RETURNS: []string — provider-qualified model identifiers.
func ChatGPTOAuthModels() []string {
	return []string{
		ChatGPTOAuthProviderName + "/gpt-5.5",
		ChatGPTOAuthProviderName + "/gpt-5.4",
		ChatGPTOAuthProviderName + "/gpt-5.4-mini",
	}
}

// OAuthFlowStatus identifies the current browser authorization state.
type OAuthFlowStatus string

const (
	OAuthFlowIdle    OAuthFlowStatus = "idle"
	OAuthFlowPending OAuthFlowStatus = "pending"
	OAuthFlowSuccess OAuthFlowStatus = "success"
	OAuthFlowError   OAuthFlowStatus = "error"
)

// OAuthFlowResult is the non-secret status returned to the desktop service.
// Credential is populated only for internal persistence after success.
type OAuthFlowResult struct {
	Status     OAuthFlowStatus
	Error      string
	Credential *config.OAuthCredential
}

// OAuthManager runs one browser authorization attempt on a loopback port.
//
// WHAT:  Coordinates one PKCE OAuth attempt and receives the browser callback.
// WHY:   Electron needs a URL immediately while the Go backend owns token handling.
// HOW:   A local HTTP listener validates state, exchanges the code, and keeps only the result in memory.
// PARAMS: N/A.
// RETURNS: N/A.
type OAuthManager struct {
	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	state    string
	status   OAuthFlowResult
	url      string
}

// NewOAuthManager creates an idle ChatGPT OAuth flow manager.
//
// WHAT:  Creates the state holder used by the desktop backend.
// WHY:   Each desktop backend owns at most one active login attempt.
// HOW:   Starts no listener until Begin is called.
// PARAMS: none.
// RETURNS: *OAuthManager — idle manager.
func NewOAuthManager() *OAuthManager {
	return &OAuthManager{status: OAuthFlowResult{Status: OAuthFlowIdle}}
}

// Begin starts browser OAuth and returns the authorization URL.
//
// WHAT:  Opens a loopback listener and prepares a PKCE authorization URL.
// WHY:   The browser must return the authorization code to the same local app.
// HOW:   Binds 127.0.0.1:1455, stores a cryptographic state value, and serves one callback.
// PARAMS: none.
// RETURNS: string — authorization URL; error if the listener cannot start.
func (m *OAuthManager) Begin() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.Status == OAuthFlowPending {
		return m.url, nil
	}

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", err
	}
	state, err := randomToken(32)
	if err != nil {
		return "", err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", chatGPTOAuthPort))
	if err != nil {
		return "", fmt.Errorf("cannot start OAuth callback listener: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", chatGPTOAuthPort)
	m.state = state
	m.status = OAuthFlowResult{Status: OAuthFlowPending}
	m.url = buildChatGPTAuthorizeURL(redirectURI, challenge, state)
	m.listener = listener
	m.server = &http.Server{Handler: m.callbackHandler(verifier, redirectURI)}
	server := m.server
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			m.finish(OAuthFlowResult{Status: OAuthFlowError, Error: serveErr.Error()})
		}
	}()
	return m.url, nil
}

// Status returns a snapshot of the current authorization state.
//
// WHAT:  Reads the browser flow result without exposing it through the UI.
// WHY:   Electron polls this small state while the browser is open.
// PARAMS: none.
// RETURNS: OAuthFlowResult — current status and, on success, credential data for backend use.
func (m *OAuthManager) Status() OAuthFlowResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := m.status
	if result.Credential != nil {
		credential := *result.Credential
		result.Credential = &credential
	}
	return result
}

// Close stops a pending loopback listener and clears its in-memory state.
//
// WHAT:  Cancels an active authorization attempt.
// WHY:   Shutdown must not leave a callback server listening after the desktop closes.
// HOW:   Closes the HTTP server and resets the manager to idle.
// PARAMS: none.
// RETURNS: none.
func (m *OAuthManager) Close() {
	m.mu.Lock()
	server := m.server
	m.server = nil
	m.listener = nil
	m.state = ""
	m.url = ""
	m.status = OAuthFlowResult{Status: OAuthFlowIdle}
	m.mu.Unlock()
	if server != nil {
		_ = server.Close()
	}
}

func (m *OAuthManager) callbackHandler(verifier, redirectURI string) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/auth/callback" {
			http.NotFound(response, request)
			return
		}
		if request.URL.Query().Get("state") != m.currentState() {
			m.finish(OAuthFlowResult{Status: OAuthFlowError, Error: "invalid OAuth state"})
			writeOAuthPage(response, false, "Invalid OAuth state")
			return
		}
		if oauthError := request.URL.Query().Get("error_description"); oauthError != "" {
			m.finish(OAuthFlowResult{Status: OAuthFlowError, Error: oauthError})
			writeOAuthPage(response, false, oauthError)
			return
		}
		code := request.URL.Query().Get("code")
		if code == "" {
			m.finish(OAuthFlowResult{Status: OAuthFlowError, Error: "missing authorization code"})
			writeOAuthPage(response, false, "Missing authorization code")
			return
		}

		ctx, cancel := context.WithTimeout(request.Context(), chatGPTOAuthTimeout)
		defer cancel()
		credential, err := exchangeChatGPTCode(ctx, code, redirectURI, verifier)
		if err != nil {
			m.finish(OAuthFlowResult{Status: OAuthFlowError, Error: err.Error()})
			writeOAuthPage(response, false, err.Error())
			return
		}
		m.finish(OAuthFlowResult{Status: OAuthFlowSuccess, Credential: &credential})
		writeOAuthPage(response, true, "BlazeAI is now connected to ChatGPT.")
	})
}

func (m *OAuthManager) currentState() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *OAuthManager) finish(result OAuthFlowResult) {
	m.mu.Lock()
	server := m.server
	m.status = result
	m.server = nil
	m.listener = nil
	m.state = ""
	m.mu.Unlock()
	if server != nil {
		_ = server.Close()
	}
}

type chatGPTTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	IDToken      string `json:"id_token"`
}

func exchangeChatGPTCode(ctx context.Context, code, redirectURI, verifier string) (config.OAuthCredential, error) {
	var token chatGPTTokenResponse
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {chatGPTOAuthClientID},
		"code_verifier": {verifier},
	}
	if err := oauthPost(ctx, chatGPTOAuthIssuer+"/oauth/token", form, &token); err != nil {
		return config.OAuthCredential{}, fmt.Errorf("OAuth token exchange failed: %w", err)
	}
	return credentialFromToken(token)
}

func refreshChatGPTCredential(ctx context.Context, current config.OAuthCredential) (config.OAuthCredential, error) {
	var token chatGPTTokenResponse
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {current.RefreshToken},
		"client_id":     {chatGPTOAuthClientID},
	}
	if err := oauthPost(ctx, chatGPTOAuthIssuer+"/oauth/token", form, &token); err != nil {
		return config.OAuthCredential{}, fmt.Errorf("OAuth token refresh failed: %w", err)
	}
	refreshed, err := credentialFromToken(token)
	if err != nil {
		return config.OAuthCredential{}, err
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = current.RefreshToken
	}
	if refreshed.AccountID == "" {
		refreshed.AccountID = current.AccountID
	}
	return refreshed, nil
}

func credentialFromToken(token chatGPTTokenResponse) (config.OAuthCredential, error) {
	if token.AccessToken == "" || token.RefreshToken == "" {
		return config.OAuthCredential{}, fmt.Errorf("OAuth response did not include required tokens")
	}
	expiresIn := token.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return config.OAuthCredential{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second).UnixMilli(),
		AccountID:    firstNonEmpty(extractChatGPTAccountID(token.IDToken), extractChatGPTAccountID(token.AccessToken)),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func oauthPost(ctx context.Context, endpoint string, form url.Values, target interface{}) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := newHTTPClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("server returned status %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("cannot parse OAuth response: %w", err)
	}
	return nil
}

func generatePKCE() (string, string, error) {
	verifier, err := randomToken(32)
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("cannot generate OAuth nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func buildChatGPTAuthorizeURL(redirectURI, challenge, state string) string {
	query := url.Values{
		"response_type":              {"code"},
		"client_id":                  {chatGPTOAuthClientID},
		"redirect_uri":               {redirectURI},
		"scope":                      {"openid profile email offline_access"},
		"code_challenge":             {challenge},
		"code_challenge_method":      {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"state":                      {state},
		"originator":                 {"blazeai"},
	}
	return chatGPTOAuthIssuer + "/oauth/authorize?" + query.Encode()
}

func extractChatGPTAccountID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
		Organizations    []struct {
			ID string `json:"id"`
		} `json:"organizations"`
		Auth struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if json.Unmarshal(data, &claims) != nil {
		return ""
	}
	if claims.ChatGPTAccountID != "" {
		return claims.ChatGPTAccountID
	}
	if claims.Auth.ChatGPTAccountID != "" {
		return claims.Auth.ChatGPTAccountID
	}
	if len(claims.Organizations) > 0 {
		return claims.Organizations[0].ID
	}
	return ""
}

func writeOAuthPage(response http.ResponseWriter, success bool, detail string) {
	status := http.StatusOK
	title := "Authorization failed"
	message := template.HTMLEscapeString(detail)
	if success {
		title = "Authorization successful"
		message = template.HTMLEscapeString(detail) + " You can close this window."
	} else {
		status = http.StatusBadRequest
	}
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(status)
	_, _ = fmt.Fprintf(response, "<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title></head><body><h1>%s</h1><p>%s</p></body></html>", title, title, message)
}
