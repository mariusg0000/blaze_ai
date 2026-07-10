## Feature Description
Added a transport-agnostic system notification channel (`OnSystem`) to the handler interface, used to notify users when the runtime detects and applies a task switch. Also added a `purpose` parameter to `read_file` and `write_file` tools for user-visible operation labels.

## Rationale And Implementation
Previously task-switch compaction happened silently — users had no indication that old context was archived and a summary was injected. The `OnSystem` method was added to `runtime.Handler` and called immediately after `CompactByTaskSwitch` with the detection summary. Each transport renders it natively: console shows an orange `⚡ System:` line, Telegram sends a standalone notice, and desktop backends delegate to `AppendSystem`. The `purpose` parameter was added to `read_file` and `write_file` args structs, `FormatArgs`, and JSON schemas to match the existing `shell`/`replace_block` pattern where `FormatArgs` displays the purpose text directly when set, falling back to the file path when empty.

## Modified Files
- internal/runtime/runtime.go: added OnSystem to Handler interface and calls it after task-switch compaction
- internal/console/console.go: implements OnSystem with formatted orange console output
- internal/telegram/handler.go: implements OnSystem via sendNotice with bolt emoji prefix
- internal/desktopbackend/handler.go: implements OnSystem delegating to sink.AppendSystem
- internal/desktop_old/handler.go: implements OnSystem delegating to sink.AppendSystem
- internal/runtime/runtime_test.go: mockHandler updated with OnSystem + assertions in task-switch tests
- internal/console/console_test.go: mockHandler updated to satisfy interface
- internal/tools/read_file.go: added Purpose to args, FormatArgs, and Parameters schema
- internal/tools/write_file.go: added Purpose to args, FormatArgs, and Parameters schema
- internal/tools/read_file_test.go: added 3 tests for purpose behavior (direct, fallback, schema)
- internal/tools/write_file_test.go: added 3 tests for purpose behavior (direct, fallback, schema)
