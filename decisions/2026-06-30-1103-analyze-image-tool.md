# Session Decision Summary: analyze-image-tool

Date: 2026-06-30 11:03
Base commit: 1baeb8b

## Context
The user wanted image delegation split out from `ask_a_friend` to avoid confusion and to keep the interface minimal: a dedicated tool with only `input_file` and `question`. The same session also required local image preprocessing before upload, strict no-fallback behavior, updated prompt/spec documentation, a commit, and deployment to the NAS target.

## Changes Made
- Added a new `analyze_image` native tool with required `input_file` and `question` parameters.
- Added shared local image preprocessing that detects the real MIME type, decodes supported images, resizes them to a maximum long side of 1200px, flattens transparency, JPEG-encodes the result, and produces a base64 data URL for multimodal provider requests.
- Extended `llmcall` request building so one-shot secondary calls can send either plain text or multimodal text plus image content.
- Wired `analyze_image` through the runtime to always use the configured `vision` role with generated metadata context and a fixed output-shape instruction.
- Updated `ask_a_friend` so `input_file` remains text-only and image files fail clearly with guidance to use `analyze_image`.
- Added or updated focused tests for the new tool, multimodal request construction, runtime registration, emoji mappings, and provider stream callback correctness discovered during validation.
- Updated prompt and spec files to document the new tool split, the text-only nature of `ask_a_friend`, the multimodal behavior of `analyze_image`, and the existing `deploy_nas.sh` workflow.
- Added `golang.org/x/image` as a direct dependency for high-quality resizing.

## Decisions And Rationale
- Chose a separate `analyze_image` tool instead of overloading `ask_a_friend` because the user wanted a simpler interface and a clearer tool-selection boundary.
- Forced `analyze_image` to the `vision` role so the main model no longer has to choose roles correctly for image requests.
- Kept `ask_a_friend` image rejection explicit instead of silently treating images as text or attempting OCR/compression fallbacks, matching the project's no-fallback rule.
- Used local resize plus JPEG output to keep multimodal payloads compact and provider-friendly while preserving enough detail for screenshots and UI captures.
- Added `golang.org/x/image/draw` rather than writing a custom scaler because the dependency is small, pure Go, and directly solves the needed resize quality problem with less custom code.
- Updated the stale deploy spec because the repo already contains `deploy_nas.sh`, so leaving the old statement would keep specs inaccurate after this session.

## Implementation Approach
- Implemented image helpers in `internal/tools/image_input.go` for MIME sniffing, image decoding, resize, alpha flattening, JPEG encoding, and data URL creation.
- Implemented the new tool in `internal/tools/analyze_image.go` and passed a prepared request object into runtime wiring.
- Extended `internal/llmcall/llmcall.go` so `Request` can carry an optional `ImageDataURL`, with user content emitted either as plain text or as OpenAI-compatible multimodal content blocks.
- Registered `analyze_image` in `internal/runtime/runtime.go`, generating deterministic metadata context and delegating through the existing one-shot caller.
- Updated prompt and spec files in the same change set so runtime behavior and documentation stay aligned.
- Ran `go mod tidy`, formatted touched Go files, and validated with `go test ./...`.

## Alternatives Considered
- Reusing `ask_a_friend` with `role="vision"` and image-aware branching was rejected because it keeps tool selection ambiguous and mixes text-only and multimodal flows in one tool contract.
- Shell-based OCR or external image preprocessing tools were rejected because they introduce fallback-like behavior, more moving parts, and cross-platform fragility.
- Sending raw image files without local resize was rejected because the user explicitly wanted bounded long-side resizing and because compact multimodal payloads are more efficient.

## Files Included
- `internal/tools/analyze_image.go`: new image delegation tool.
- `internal/tools/image_input.go`: shared local image preprocessing helpers.
- `internal/tools/analyze_image_test.go`: tests for image preprocessing and tool behavior.
- `internal/tools/ask_friend.go`: explicit text-only contract and image rejection.
- `internal/tools/ask_friend_test.go`: coverage for image rejection.
- `internal/llmcall/llmcall.go`: multimodal request support.
- `internal/llmcall/llmcall_test.go`: multimodal request test coverage.
- `internal/runtime/runtime.go`: analyze_image registration and vision-role wiring.
- `internal/runtime/runtime_test.go`: runtime registration coverage.
- `internal/console/console.go`: analyze_image emoji.
- `internal/console/console_test.go`: console emoji test update.
- `internal/telegram/handler.go`: analyze_image emoji.
- `internal/telegram/handler_test.go`: Telegram emoji test update.
- `internal/provider/provider_test.go`: corrected content-callback argument order found during validation.
- `go.mod`: direct resize dependency.
- `go.sum`: checksums for the resize dependency.
- `prompts/sysprompt.md`: tool guidance for text vs image delegation.
- `specs.md`: project-level native tool list update.
- `specs/01-product-scope.md`: native tool count and list update.
- `specs/04-prompts.md`: prompt section description update.
- `specs/05-tools.md`: analyze_image tool documentation.
- `specs/08-cross-model-delegation.md`: combined text and image delegation spec rewrite.
- `specs/15-runtime-core.md`: runtime registry update.
- `specs/19-build-deploy.md`: deploy script and build dependency update.

## Commit Linkage
This summary is committed together with the implementation changes to keep rationale linked to code history.
