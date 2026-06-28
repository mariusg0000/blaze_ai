# Session Decision Summary: telegram-activity-bubble

Date: 2026-06-28 09:06
Base commit: bf697b8

## Context
The Telegram bridge UX was showing raw tool call/result bubbles and was editing the initial assistant bubble after tool activity, which pushed the final answer above the tool output. The deploy script also needed a stable NAS default target so the user would not have to re-enter `nas@192.168.0.104` for routine deploys.

## Changes Made
Refactored `internal/telegram/handler.go` so tool activity is rendered in one editable `🛠 Activity` bubble with emoji status markers instead of raw `[tool]` and `[tool result]` messages. Assistant content produced after tool activity now starts a new lower bubble, keeping the final answer below the activity block. Added focused tests for activity rendering, message ordering, and summarized tool errors. Updated the Telegram specs/docs to describe the new output behavior. Set `deploy_nas.sh` to default to `nas@192.168.0.104` while still allowing an override argument.

## Decisions And Rationale
The activity bubble keeps tool execution visible without flooding chat with raw logs. Freezing the earlier assistant bubble avoids the confusing Telegram ordering where the final answer ends up above the tool trail. The deploy script default was moved into the script itself because the NAS target is a stable project-specific constant and the user explicitly wanted it remembered.

## Implementation Approach
The handler now keeps a single activity message and updates it in place as tool results arrive. When tool work starts, any later assistant content is treated as a new message stream, so the final reply lands below the activity block. Tool results are reduced to short success/error/timeout summaries. The deploy script now uses `nas@192.168.0.104` when no SSH target is passed.

## Alternatives Considered
Rendering each tool call/result as its own bubble was rejected because it causes chat noise and pushes the final answer off-screen. A full tool-call ID correlation scheme was not added because that would require a broader runtime contract change.

## Files Included
- internal/telegram/handler.go: Telegram activity bubble behavior and post-tool message ordering.
- internal/telegram/handler_test.go: focused behavior tests for the new Telegram UX.
- deploy_nas.sh: default NAS deploy target for repeatable deploys.
- specs/06_telegram.md: spec updated for the activity bubble and final-answer placement.
- telegram.md: plan updated to match the Telegram UX change.
- decisions/2026-06-28-0906-telegram-activity-bubble.md: session record for this commit.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
