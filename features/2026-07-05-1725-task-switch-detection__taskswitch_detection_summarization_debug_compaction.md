## Feature Description
Parallel task-switch detection that uses the summarization LLM to detect user topic changes and automatically prune old context while preserving summaries.

## Rationale And Implementation
The user requested automatic detection of task changes — when they switch topics mid-session, old messages should be compacted with a summary. The detection runs in a goroutine parallel to the main LLM call, sends a compact transcript (reasoning stripped, tool calls/results truncated) to the summarization model, and applies cleanup only when a switch is detected. Debug files (prompt, response, parsed result) are written to the session folder for troubleshooting.

## Modified Files
- internal/compaction/taskswitch.go: new file — DetectTaskSwitch, buildDetectionTranscript, detectionSystemPrompt, parseDetectionResponse with flexible index parsing (int, string "5", "user N", "user_NN"), extractIndex and parseDigits helpers
- internal/compaction/taskswitch_test.go: new file — 15+ unit tests for transcript building, parsing all response formats, CompactByTaskSwitch, and userIndexToSessionIndex
- internal/compaction/compaction.go: added CompactByTaskSwitch (applies detection result), userIndexToSessionIndex (converts user index to session array index), and LoadSummariesText (public wrapper for detection goroutine)
- internal/runtime/runtime.go: restructured RunTurn — parallel detection goroutine runs alongside main LLM call, detection result applied after normal completion; fixed summarization client creation to always create when configured (removed model-equality check that silently disabled detection); debug file writes for taskswitch
- internal/runtime/runtime_test.go: 3 integration tests — switch applied, no switch, abort skips detection
