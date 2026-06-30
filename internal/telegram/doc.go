// telegram/doc.go — Telegram bridge transport package.
// Loads Telegram instance config/state, starts long polling, enforces one allowed chat,
// accepts text plus image messages, and adapts runtime.Handler streaming into Telegram messages.
// Layer: transport. Dependencies: internal/config, internal/platform, internal/runtime, internal/session.
package telegram
