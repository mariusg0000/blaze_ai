# Session Decision Summary: telegram-image-intake

Date: 2026-06-30 11:29
Base commit: 355a196

## Context
The user wanted `analyze_image` to work from the Telegram transport, including the case where a user sends a photo without a caption and the case where a caption exists. The clarified rule was that the main LLM should always generate the effective `analyze_image` question itself from conversation context, while any Telegram caption is only additional guidance.

## Changes Made
- Extended Telegram update parsing to support both `message` and `channel_post`.
- Extended Telegram message models to support captions, photo arrays, and generic document attachments.
- Added Telegram Bot API support for `getFile` metadata lookup and file download.
- Added a new image intake layer that downloads Telegram-hosted images into `app_home/telegram/<instance>/attachments/` and builds a synthetic user turn that explicitly instructs the agent to use `analyze_image`.
- Wired the polling loop so text messages continue unchanged, while photo and image-document messages flow through the new synthetic-turn path.
- Updated the Telegram transport context and specs to document image handling, attachment persistence, and caption behavior.
- Added focused tests for text passthrough, photo download, image documents, non-image document rejection, channel post handling, and Bot API file download behavior.

## Decisions And Rationale
- Kept Telegram image handling inside the normal agent tool loop instead of creating a transport-only vision path. This preserves session history, tool activity rendering, and existing runtime behavior.
- Chose synthetic text turns that tell the main model to use `analyze_image` because this is the smallest change that reuses the new vision tool without bypassing the agent core.
- Stored downloaded Telegram images persistently in the instance folder instead of deleting them after one turn, because later follow-up turns may still rely on the local path preserved in session history.
- Treated captions as optional guidance only. This matches the user's clarified requirement that the LLM, not the transport, should synthesize the actual analyze question from context.
- Rejected non-image documents with a clear error instead of silently treating them as text or ignoring the MIME mismatch.

## Implementation Approach
- Added `internal/telegram/image_messages.go` to normalize inbound Telegram image messages, select the best photo variant or image document, fetch Telegram file metadata, download the file locally, and generate the synthetic `RunTurn` input.
- Extended `telegramClient` and `BotClient` in `internal/telegram/telegram.go` with `GetFile` and `DownloadFile` methods using the Telegram Bot API file endpoint.
- Updated polling to call a shared `updateMessage()` helper for `message` versus `channel_post`, then build either plain text input or image-backed synthetic input before calling `agent.RunTurn()`.
- Documented the behavior in `specs/14-telegram-bridge.md` and refreshed package docs in `internal/telegram/doc.go`.
- Validated with `gofmt` and `go test ./...`.

## Alternatives Considered
- Calling `analyze_image` directly from the Telegram transport was rejected because it would bypass the main model's planning and tool selection logic.
- Requiring a caption for every image was rejected because the user explicitly wanted the no-caption case supported via LLM-generated image questions.
- Temporary downloads deleted after one turn were rejected because session history would then retain dead paths for future follow-up analysis.

## Files Included
- `internal/telegram/image_messages.go`: new Telegram image intake and synthetic-turn builder.
- `internal/telegram/image_messages_test.go`: tests for image intake and Bot API download helpers.
- `internal/telegram/telegram.go`: update parsing, new bot client methods, transport context update, and polling integration.
- `internal/telegram/telegram_test.go`: mock client updates for the extended Telegram client interface.
- `internal/telegram/doc.go`: package description updated for image support.
- `specs/14-telegram-bridge.md`: Telegram image flow, attachment storage, and caption behavior documentation.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
