# Decision Summary: Protocol adapter catalog and repair workflow

Date: 2026-07-31 08:38

## Context

The originating goal was to complete protocol adapters, structured repair diagnostics, a separate model adapter catalog, wildcard resolution, and builtin provider contracts. The changes span provider lowering, configuration persistence and migration, first-run setup, runtime model reload, console repair context, the config-manager skill, and the corresponding tests and specifications.

The implementation must preserve the project directive that missing or unsupported metadata stops explicitly rather than using a fallback. Validation already performed for this change is `go test ./...` and `go build -o /tmp/blazeai .`, both successful. Historical rationale is supplied: protocol and capabilities must not be inferred from model names; user adapters must be separate from credentials/configuration; exact and longest-prefix resolution enables explicit progressive repair; builtin identities must be exact; and provider diagnostics must be treated as untrusted and only trigger repair context for 400/422 failures before assistant content.

## Changes Made

- Added provider-neutral protocol selection and lowering for OpenAI Chat and Responses requests, with structured missing-catalog and request-rejected errors.
- Added the embedded builtin provider/model adapter catalog and user `model_adapters.json` persistence, exact/longest-prefix wildcard resolution, validation, one-shot legacy migration, and reload support.
- Updated first-run model registration and runtime model switching to use explicit adapter metadata, including one catalog reload retry.
- Added one-shot console repair context for missing catalog commands and eligible provider failures, plus config-manager repair guidance.
- Updated affected tests and specifications to document the active catalog, builtin contracts, resolver precedence, migration, adapter reload, and repair diagnostics.

## Decisions And Rationale

- Keep adapter metadata out of `config.json` and persist it in a dedicated `model_adapters.json`; this separates model protocol/profile edits from provider credentials and makes repair narrowly scoped.
- Resolve user exact entries before user wildcards, then builtin model overrides and exact builtin provider identities; this preserves explicit user overrides while providing only declared builtin contracts, never endpoint or alias matching.
- Permit only terminal wildcards and select the longest literal prefix; this supports progressive family/version/exact repair without introducing an implicit fallback.
- Require protocol variants and capabilities to be explicit and reject missing or contradictory metadata; the selected solution intentionally avoids model-name heuristics and generic defaults.
- Reload only the adapter catalog and retry once when model switching reports a missing entry; this allows an in-process repair retry while preserving the current model/provider state on failure.
- Capture only 400/422 provider rejections that occur before assistant content, wrap provider text as untrusted diagnostic data, and consume the context once; unrelated authorization, rate-limit, server, network, and post-content failures must not create repair instructions.
- Preserve the existing Responses builders while placing protocol validation/lowering before HTTP transport; this minimizes wire behavior changes while adding explicit adapter selection.
- The specs resync updated `specs.md`, `specs/02-architecture.md`, `specs/03-config-schema.md`, `specs/13-console-ui.md`, and `specs/15-runtime-core.md` for the concrete active behavior established by this diff.
