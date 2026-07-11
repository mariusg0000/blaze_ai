## Feature Description
Improved the Blaze context-compaction and TaskSwitcher prompts using append-only memory rules, delta-focused summaries, explicit historical-context boundaries, and strict TaskSwitcher output contracts.

## Rationale And Implementation
The summarizer must describe only the newly pruned span instead of rewriting global session state or historical summaries. The TaskSwitcher must classify only clear task changes, preserve the exact `[user N]` boundary, summarize only the preceding span, and return exactly `null` or the required JSON object.

## Modified Files
- internal/compaction/compaction.go: replaced the generic summary prompt with structured append-only instructions
- internal/compaction/taskswitch.go: replaced the detector prompt with stricter classification and output rules
- internal/compaction/compaction_test.go: added summary-prompt contract assertions
- internal/compaction/taskswitch_test.go: added TaskSwitcher prompt contract assertions
- tasks.md: recorded completed implementation and validation tasks
