# Backspace optimization — multiline paste flicker fix

- [ ] **reader.go** — Optimize backspace handler: local ANSI erase when cursor at end, keep redrawLine for mid-buffer only
- [ ] **Validate**: `go build ./...` + `go test ./...`
