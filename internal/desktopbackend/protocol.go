// protocol.go — JSON request/response protocol for the Electron desktop backend.
// Defines the stdio payload shapes used between the Electron main process and the
// Go backend. The protocol is synchronous request/response in phase 1.
// Layer: desktop backend protocol. Dependencies: standard library only.
package desktopbackend

import "encoding/json"

// Request is one inbound desktop backend RPC request.
//
// WHAT:  Carries one method call from Electron to the backend.
// WHY:   The backend reads stdin as a sequence of JSON RPC-like messages.
// PARAMS: ID — caller-generated correlation id; Method — backend method name; Params — raw JSON params.
type Request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response is one outbound desktop backend RPC response.
//
// WHAT:  Returns either a result payload or an error for one request.
// WHY:   Electron needs deterministic per-call completion.
// PARAMS: ID — request correlation id; OK — success flag; Result — success payload; Error — fatal message.
type Response struct {
	ID     string      `json:"id,omitempty"`
	OK     bool        `json:"ok"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// SendMessageParams is the request payload for send_message.
type SendMessageParams struct {
	Text string `json:"text"`
}

// ChangeModelParams is the request payload for change_model.
type ChangeModelParams struct {
	Model string `json:"model"`
}

// ChangeModeParams is the request payload for change_mode.
type ChangeModeParams struct {
	Name string `json:"name"`
}

// SetThemeParams is the request payload for set_theme.
type SetThemeParams struct {
	Theme string `json:"theme"`
}

// SetFontSizeParams is the request payload for set_font_size.
type SetFontSizeParams struct {
	FontSize float64 `json:"font_size"`
}

// SetWorkDirParams is the request payload for set_workdir.
type SetWorkDirParams struct {
	Path            string `json:"path"`
	ResumeLastClean bool   `json:"resume_last_clean"`
}

// SetWindowBoundsParams is the request payload for set_window_bounds.
type SetWindowBoundsParams struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// OpenAIAuthStartResponse is returned after the browser OAuth listener starts.
//
// WHAT:  Carries the URL Electron must open in the user's browser.
// WHY:   The Go backend owns the callback listener while Electron owns browser launching.
// PARAMS: URL — OpenAI authorization URL; Status — pending flow status.
// RETURNS: N/A.
type OpenAIAuthStartResponse struct {
	URL    string `json:"url"`
	Status string `json:"status"`
}

// OpenAIAuthStatusResponse is the poll result for a browser OAuth attempt.
//
// WHAT:  Exposes only non-secret OAuth state to the renderer.
// WHY:   Access and refresh tokens must never cross the Electron IPC boundary.
// PARAMS: Status — idle, pending, success, or error; Error — safe failure text.
// RETURNS: N/A.
type OpenAIAuthStatusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// BackendOptions configures one backend server instance.
//
// WHAT:  Carries startup-time desktop backend options resolved by the binary entrypoint.
// WHY:   Electron can request an explicit initial project path and resume policy.
// PARAMS: InitialWorkDir — explicit startup project path; ResumeLastClean — open the last clean session on first message.
type BackendOptions struct {
	InitialWorkDir  string
	ResumeLastClean bool
}
