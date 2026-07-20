# TODO: Remove read_file and write_file tools

## WHAT MUST BE DONE

Remove `read_file` and `write_file` native tools from the runtime. The agent should use `shell` for all file reading (cat, head, tail, sed, rg) and writing (heredoc, printf, cp). Retain `replace_block` for precise edits of existing files.

## WHY IT MUST BE DONE

- `write_file` is fully redundant with `shell` — heredoc and redirect operations cover all use cases.
- `read_file` can be replaced by `shell` commands (cat, rg, sed) which are more token-efficient for partial reads.
- Removing both tools eliminates 2 tool definitions from every LLM request, reducing token overhead.
- Simplifies the tool registry from 9 base tools to 7 base tools.
- Both tools require a mandatory `purpose` field (3 sentences) which adds unnecessary token overhead.
- `read_file` deduplication logic in `session.go` (`AppendReadFileResult`) can be removed.

## HOW IT MUST BE DONE

1. Delete `internal/tools/read_file.go` and `internal/tools/write_file_test.go`.
2. Delete `internal/tools/write_file.go` and `internal/tools/read_file_test.go`.
3. Remove `NewReadFileTool` and `NewWriteFileTool` registrations from `internal/runtime/runtime.go`.
4. Remove `AppendReadFileResult` from `internal/session/session.go` and its test from `internal/session/session_test.go`.
5. Remove the `read_file`-specific persistence branch in `internal/runtime/runtime.go` (line ~500).
6. Update agent definitions in `~/blazeai/agents/` — remove `read_file` and `write_file` from tool lists.
7. Update `specs/05-tools.md`, `specs/01-product-scope.md`, `specs/02-architecture.md`, `specs/15-runtime-core.md`, `specs/20-agent-orchestration.md` — remove file tools section and references.
8. Update `internal/console/console.go` and `internal/telegram/handler.go` — remove emoji cases for `read_file`/`write_file`.
9. Update embedded skills and prompts if they reference removed tools.
10. Update base tool count from 9 to 7 everywhere.
11. Validate: `go test ./...`, `go build ./...`, `git diff --check`.
