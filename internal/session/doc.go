// session/doc.go — file-based session persistence in app_home/sessions/.
// Creates random-named session folders, persists the complete message JSON array exactly as sent
// to the LLM, tracks closed_cleanly status, and supports -c continuation of the last clean session.
// Layer: session storage. Dependencies: internal/platform (app home path resolution).
package session
