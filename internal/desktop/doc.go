// doc.go — desktop companion transport with one fixed local session.
// Loads one singleton desktop instance from app_home/desktop, resumes the same
// session folder on every start, and exposes the shared runtime through a small
// embedded desktop window.
// Layer: transport. Dependencies: webview, internal/config, internal/platform,
// internal/runtime, internal/session.
package desktop
