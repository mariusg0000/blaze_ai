# TODO: Implement session retrospective

## WHAT MUST BE DONE
Implement an explicit review-only workflow that analyzes recent BlazeAI sessions, creates missing per-session `review.md` reports, and produces a cross-session improvement plan covering skills, memories, tool use, and workflow inefficiencies.

## WHY IT MUST BE DONE
Real session evidence can reveal missing or weak skills and inefficient tool patterns more reliably than speculative improvement work.

## HOW IT MUST BE DONE
Scan terminal and Telegram session sources, prefer newest sessions, summarize compact transcripts with the summarization model, write concise reports, synthesize up to 30 reports, present recommendations, and stop for user discussion. Do not auto-create or modify skills, memories, docs, or code; stop clearly when required configuration or source files are unavailable.
