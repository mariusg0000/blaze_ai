# Session Decision Summary: Scan-full-history sanitizer

Date: 2026-06-22 17:40
Base commit: db8722a

## Context
- `-r` resume still failed with 400 despite previous sanitize implementation
- Session had: `assistant.tool_calls` incomplete → `user: continua` → `user: continua`
- Old Sanitize only checked the tail of the message array, so `user` messages after the incomplete assistant hid it

## Fix
- Rewrote `Sanitize()` to scan forward from index 0 instead of backward from end
- For each `assistant` with `tool_calls`, count consecutive `tool` messages that follow
- If `actualResults < expectedResults`, truncate the session **from that assistant onward**
- This removes not just the incomplete assistant but any subsequent user messages built on broken history

## Files
- `internal/session/session.go`: rewritten Sanitize, improved docs
- `internal/session/session_test.go`: new regression test for the exact scenario
