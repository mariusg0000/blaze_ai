status: completed

# OpenCode-style model catalog and protocol adapters

## User outcome

Implement the provider/model normalization architecture used by OpenCode: saved BlazeAI configuration must define a model catalog with explicit per-model metadata; runtime must create one provider-neutral request; the selected model's protocol adapter must lower that request to validated protocol-native JSON. This foundation must support model-specific request variants, capabilities, and provider-native options such as reasoning levels without model-name guessing or silently sending invalid fields to GLM5.2 or another provider.

## Plan summary

BlazeAI currently treats every non-OAuth model as the same OpenAI Chat Completions API and serializes session data directly into one request body. OpenCode instead has a common `LLMRequest`, protocol adapters for API families, and model metadata that selects a route/protocol and compatibility variants. Its schemas are primarily **per protocol**, not independently written full schemas for every model; model entries carry the differences that affect lowering and compatibility.

BlazeAI will follow that shape in Go. A model definition, keyed by the existing full `provider/model` ID, will explicitly select a protocol and state model capabilities plus protocol-native request options. A provider-neutral request will carry messages, tools, generation, reasoning level, and namespaced native options. An adapter selected from the model definition will emit the native request type for its protocol. The adapter validates the selected model profile and only includes fields permitted by it. The initial implementation supplies the existing OpenAI Chat and ChatGPT OAuth Responses protocols, preserving their existing wire behavior through adapters; native Anthropic and Gemini adapters remain future additions.

This makes GLM5.2 configuration explicit: it can use the OpenAI Chat adapter with a model variant that disables fields its endpoint rejects. It does not depend on prefixes such as `glm-`, fallbacks, or a hidden provider capability table.

## Current state

- Standard clients in `internal/provider/provider.go` directly marshal a private `chatRequest` containing model, `[]session.Message`, OpenAI tools, `stream`, and unconditional `stream_options.include_usage`, then POST to `/chat/completions`.
- ChatGPT OAuth in `internal/provider/openai_responses.go` already converts messages and tools into a separate Responses API JSON structure and endpoint, but the conversion is not represented as a common protocol adapter.
- `config.Provider` has endpoint/auth credentials only; favorite models and role models are plain full ID strings. There is no saved model metadata, protocol selection, capability declaration, or per-model request variant.
- Session message marshaling currently exposes OpenAI-compatible fields including `reasoning_content`, encrypted reasoning, tool calls, and tool results. Tools are already converted to OpenAI function definitions by runtime.
- OpenCode separates: (a) common request/event schema, (b) protocol-level request body schema/lowerer and streaming parser, and (c) model catalog metadata/compatibility. OpenAI-compatible vendors reuse an OpenAI Chat protocol; model metadata supplies variants and compatibility.

## Supporting evidence

- `.agents/explorer/000001-opencode-provider-adaptation.md`
- `.agents/explorer/000002-blazeai-provider-implementation.md`

## Resolved design

### 1. Saved model catalog

Add `Config.Models map[string]ModelDefinition` with JSON key `models`. Keys are complete existing model IDs (`provider/model`), so this catalog does not change model selection syntax, roles, favourites, `agents.json`, or session data.

```go
type ModelDefinition struct {
    Protocol     string                 `json:"protocol"`
    Capabilities ModelCapabilities      `json:"capabilities"`
    OpenAIChat   *OpenAIChatVariant     `json:"openai_chat,omitempty"`
    Responses    *ResponsesVariant      `json:"responses,omitempty"`
}

type ModelCapabilities struct {
    Tools     bool `json:"tools"`
    Reasoning bool `json:"reasoning"`
}

type OpenAIChatVariant struct {
    IncludeStreamUsage      bool `json:"include_stream_usage"`
    IncludeReasoningContent bool `json:"include_reasoning_content"`
}

type ResponsesVariant struct {
    Lite bool `json:"lite"`
}
```

The catalog is the saved equivalent of OpenCode's model metadata, at a deliberately smaller scale. `Capabilities` is declarative input for request validation; it does not probe a provider or infer facts from a model name.

### 2. Protocol selection and validation

Define exactly two protocol identifiers in this tranche: `ProtocolOpenAIChat` with value `"openai-chat"` and `ProtocolOpenAIResponses` with value `"openai-responses"`.

- `openai-chat`: standard `/chat/completions` transport. Requires non-OAuth provider, a non-nil `OpenAIChatVariant`, and nil `ResponsesVariant`.
- `openai-responses`: existing ChatGPT OAuth `/responses` transport. Requires OAuth provider, a non-nil `ResponsesVariant`, and nil `OpenAIChatVariant`.

Every configured favourite, role model, and `last_model` must have a catalog entry. Missing entries, unknown protocol names, provider/auth mismatch, incompatible variant, or invalid model key are explicit validation errors. Active interactive-agent models live in separate `agents.json`, outside `Config.Validate`; client creation validates their selected full model ID against the catalog before any provider request. There is no fallback to the old implicit OpenAI Chat behavior.

### 3. Common internal request

Create provider-private common types, independent of JSON wire keys:

```go
type Request struct {
    Model           ModelReference
    Messages        []session.Message
    Tools           []tools.OpenAITool
    Stream          bool
    ProviderOptions map[string]any
}

type ModelReference struct {
    ID           string
    Name         string
    Definition   config.ModelDefinition
}
```

`Request` is produced once by `Client.StreamWithPhase`; adapters receive it and must not inspect global config. `ProviderOptions` is reserved for explicit namespaced protocol-native settings added later (for example `openai.reasoning_effort`); no unvalidated free-form config is accepted in this tranche. It deliberately contains no reasoning-level field yet, because this tranche does not define or persist a canonical reasoning type.

### 4. Adapter contract

```go
type Protocol interface {
    ID() string
    Validate(Request) error
    Lower(Request) (any, error)
}
```

`Lower` returns a concrete native Go request struct that is JSON-marshaled by the shared HTTP code. Protocols do not choose endpoint, credentials, or retry behavior. The existing parser functions stay in their matching transport implementation for this tranche; a later native protocol adds both a lowerer and parser.

### 5. OpenAI Chat adapter behavior

`OpenAIChatProtocol` lowers to a native body containing only model, messages, tools, `stream`, and optional stream options. It validates that the model's `OpenAIChatVariant` exists. It always emits model and stream; it emits tools only if supplied; it emits `stream_options` only if `IncludeStreamUsage` is true. Before serializing copied assistant history, it removes reasoning and encrypted reasoning fields when `IncludeReasoningContent` is false. Tools require `Capabilities.Tools`; supplied tools to a model declaring false are an error, not omission.

For GLM5.2, the saved entry can explicitly set both OpenAI Chat variant booleans false. This yields valid base Chat JSON without `stream_options` or reasoning-history fields.

### 6. Responses adapter behavior

`OpenAIResponsesProtocol` wraps the existing request/input conversion and selects its current normal or lite builder from `ResponsesVariant.Lite`. It validates OAuth/provider match and rejects supplied tools when `Capabilities.Tools` is false. It preserves all current Responses headers, encrypted reasoning continuation handling, tool conversion, `store:false`, and the lite `reasoning.context:"all_turns"` behavior.

Export the existing provider-private lite predicate as `provider.IsResponsesLiteModel(model string) bool`. First-run calls this exact predicate when it saves a `ResponsesVariant`; this is the single existing model-family decision and must not be duplicated.

### 7. Reasoning normalization, deferred implementation

This tranche does **not** reintroduce UI/persistence/cycling of reasoning levels. The first follow-up will add a strict canonical level to the established common request and map it in each protocol: OpenAI Chat `reasoning_effort`, Responses `reasoning.effort`, Anthropic thinking budget, and Gemini thinking config. An adapter must error if a selected level is unsupported by that model's declared `Capabilities.Reasoning`; it must not clamp or fall back.

## Rejected or deferred alternatives

- Separate complete JSON Schema documents per model are rejected. They duplicate protocol schemas and are not OpenCode's primary pattern. Concrete Go request structs are protocol schemas; model variants contain only model-specific differences.
- Automatic catalog download, model-prefix rules, endpoint probing, and capability guessing are rejected. The user must save authoritative values in configuration.
- Native Anthropic, Gemini, Bedrock, or other adapters are deferred. Each needs its own message/tool request representation and response parser.
- Free-form provider JSON patches are deferred: they bypass validation and recreate the invalid-field problem.
- Reasoning-level UI/configuration, tool-schema dialect projection, response parser refactoring, session format changes, and compaction changes are deferred.

## Exact write paths

- `internal/config/config.go`: modify
- `internal/config/config_test.go`: modify
- `firstrun.go`: modify
- `firstrun_test.go`: modify
- `internal/provider/openai_responses.go`: modify
- `internal/provider/protocol.go`: create
- `internal/provider/openai_chat_protocol.go`: create
- `internal/provider/openai_chat_protocol_test.go`: create
- `internal/provider/openai_responses_protocol.go`: create
- `internal/provider/openai_responses_protocol_test.go`: create
- `internal/provider/provider.go`: modify
- `internal/provider/provider_test.go`: modify
- `internal/provider/openai_responses.go`: modify
- `internal/provider/openai_responses_test.go`: modify
- `internal/runtime/runtime_test.go`: modify
- `internal/console/console_test.go`: modify
- `internal/telegram/commands_test.go`: modify

No other paths may change.

## Sequential delegation units

### Unit 1 — Explicit saved model metadata

**Goal:** require each selected full model ID to declare its protocol, capabilities, and protocol-specific variant.

**Write paths:** `internal/config/config.go`, `internal/config/config_test.go`, `firstrun.go`, `firstrun_test.go`

**Implementation recipe:**
1. Add exported `ModelDefinition`, `ModelCapabilities`, `OpenAIChatVariant`, `ResponsesVariant`, and the exact exported protocol constants `ProtocolOpenAIChat` and `ProtocolOpenAIResponses` exactly as declared in this task.
2. Add `Models map[string]ModelDefinition json:"models"` to `Config`; initialize it as an empty map in `Default`.
3. Add validation for each model key: it must pass existing full model-ID parsing, refer to a configured provider, use one of the two declared protocols, match the provider's auth type, have exactly its required variant, and have no variant for the other protocol.
4. Extend existing model-reference validation so every configured favourite, role value, and non-empty last model is present in `Models`. The client-resolution unit validates separately loaded active interactive-agent IDs before any provider request; do not make `Config` load `agents.json`.
5. In `internal/provider/openai_responses.go`, rename/export the existing `isResponsesLiteModel` predicate as `IsResponsesLiteModel(model string) bool` without changing its condition. Update its in-package call sites to the exported name.
6. In `firstrun.go`, add private `firstRunModelDefinition(protocol, modelName string) (config.ModelDefinition, error)`. For `ProtocolOpenAIChat`, return exactly the configured standard definition. For `ProtocolOpenAIResponses`, return exactly the OAuth definition and obtain `ResponsesVariant.Lite` solely from `provider.IsResponsesLiteModel(modelName)`. Any other protocol returns an error. This helper makes no network or config calls.
7. Update first-run standard and OAuth model setup to call `firstRunModelDefinition` and store its returned definition. Do not duplicate the definition literal or model-prefix decision at either call site.
6. Do not migrate old config, infer catalog entries, change ID syntax, or add defaults for absent metadata.

**Tests and assertions:** valid standard and OAuth catalog entries validate. Missing entry for a favourite, role, or last model fails. The client-routing unit tests separately assert that an interactive-agent model without a catalog entry fails before HTTP transport. Unknown protocol, malformed key, unknown provider, auth/protocol mismatch, missing required variant, and prohibited extra variant each fail with relevant errors. Tests call `firstRunModelDefinition` directly and assert its exact standard and OAuth definitions, including the latter's Lite value from `provider.IsResponsesLiteModel`; an unknown protocol errors. Existing provider tests continue to prove the exported lite predicate has its prior behavior.

**Verification:** `go test ./internal/config`; `go test .`

### Unit 2 — Common request and OpenAI Chat protocol

**Goal:** replace direct standard request serialization with an OpenCode-style common request lowered by a selected protocol.

**Write paths:** `internal/provider/protocol.go`, `internal/provider/openai_chat_protocol.go`, `internal/provider/openai_chat_protocol_test.go`

**Implementation recipe:**
1. Declare private `Request`, `ModelReference`, and `Protocol` exactly as described in the resolved design. `Request` keeps copied session messages, OpenAI tools, streaming intent, and reserved provider options; no JSON tags are placed on it.
2. Implement private `openAIChatProtocol` in `openai_chat_protocol.go`, including a concrete native request struct with current OpenAI Chat JSON names and the existing `streamOptions` type.
3. `Validate` requires `Protocol == openai-chat`, a non-nil `OpenAIChatVariant`, and errors if tools are supplied while `Capabilities.Tools` is false.
4. `Lower` copies messages, clears `ReasoningContent`, `ReasoningEncryptedContent`, and their presence state only when `IncludeReasoningContent` is false, and produces native JSON fields per the resolved behavior. It errors for an empty model name or failed validation.
5. Do not add an HTTP call, SSE parser, model detection, a public registry, or generic schema library.

**Tests and assertions:** an enabled variant serializes model, messages, tools, stream, stream usage, and reasoning fields. A GLM-style disabled variant serializes model/messages/tools/stream but not stream options or either reasoning field. Tools against `Capabilities.Tools:false`, empty model, and a malformed protocol/variant fail.

**Verification:** `go test ./internal/provider`

### Unit 3 — Responses protocol wrapper

**Goal:** represent the existing OAuth Responses path as a protocol adapter without changing its wire contract.

**Write paths:** `internal/provider/openai_responses_protocol.go`, `internal/provider/openai_responses_protocol_test.go`, `internal/provider/openai_responses.go`, `internal/provider/openai_responses_test.go`

**Implementation recipe:**
1. Implement private `openAIResponsesProtocol` satisfying `Protocol`.
2. Its `Validate` requires protocol `openai-responses`, non-nil `ResponsesVariant`, and tools only when capability allows them.
3. Its `Lower` delegates to the existing normal or lite Responses builder according to `ResponsesVariant.Lite`; change those builders only as needed to accept common `Request` input instead of independent model/messages/tools arguments.
4. Preserve every current native request field, headers, OAuth refresh behavior, encrypted reasoning continuation input, response event parsing, tool handling, and lite all-turns context.
5. Do not modify response parser semantics or add reasoning effort.

**Tests and assertions:** normal and lite variants serialize their existing request shapes, including Responses tools/input handling and lite `reasoning.context:"all_turns"`. Invalid variant and tools-disabled requests fail before builder output. Existing Responses streaming tests continue to pass unchanged in behavior.

**Verification:** `go test ./internal/provider`

### Unit 4 — Client routing through selected protocol

**Goal:** select the adapter from saved model metadata and route each existing transport through its matching protocol.

**Write paths:** `internal/provider/provider.go`, `internal/provider/provider_test.go`, `internal/runtime/runtime_test.go`, `internal/console/console_test.go`, `internal/telegram/commands_test.go`

**Implementation recipe:**
1. Extend `Client` with a private selected `ModelReference` and `Protocol`.
2. In `NewClient`, resolve the full configured model ID to its `ModelDefinition`, validate it through config, construct the matching private protocol, and retain the model suffix as the native model name. Return a relevant error for absent/invalid catalog metadata or unsupported protocol.
3. In `StreamWithPhase`, construct one `Request` from client metadata, incoming messages, incoming tools, and `Stream:true`; call the selected protocol's `Validate` and `Lower` before any HTTP work.
4. Standard OpenAI Chat transport JSON-marshals the lowered native body and preserves current endpoint, headers, retry, raw capture, phase, and SSE parser behavior. OAuth transport sends the lowered Responses native body through its existing HTTP flow.
5. Remove the direct inline standard `chatRequest` construction. Do not add fallback routing or alter model selection/persistence.
6. Update only the affected test fixtures identified in `.agents/explorer/000005-model-catalog-fixture-inventory.md`: provider, runtime, console, and Telegram API-key fixtures must declare the exact OpenAI Chat model definitions for every full model ID that reaches `NewClient`, with `Tools:true`, `Reasoning:false`, and both OpenAI Chat variant flags true. Add the required definition for a model used only by `SetModel` even when it is not a favourite. Do not modify llmcall fake-factory tests or the OAuth provider-only test path because they do not call `NewClient` with a model ID.

**Tests and assertions:** configured OpenAI Chat and Responses model definitions select their matching adapters. A GLM-style OpenAI Chat entry sends filtered JSON through an intercepted standard request and still parses the existing SSE result. Missing catalog metadata, protocol mismatch, and tools-disabled model fail before an HTTP request. Existing standard and OAuth happy-path tests retain their response behavior. Updated runtime, console, and Telegram fixtures preserve their existing behavioral assertions while supplying the explicit metadata required by their configured model IDs.

**Verification:** `go test ./internal/provider`

## Final integration verification

After all units: `go test ./...`

## Acceptance criteria

- Only declared paths change.
- Saved `models` metadata explicitly defines every selected model's protocol, capabilities, and valid protocol variant; no old implicit request contract remains.
- A private provider-neutral request is created once and lowered by the selected protocol before JSON encoding.
- OpenAI Chat and OAuth Responses are separate adapters selected from model metadata, while endpoint/auth/transport remain client responsibilities.
- A GLM5.2 OpenAI Chat variant can omit stream usage and historical reasoning fields with no model-name inference or fallback.
- Tools sent to a model declaring tools disabled produce an explicit error before HTTP transport.
- Existing successful OpenAI Chat and OAuth Responses behaviors are preserved by equivalent explicit catalog variants.
- All verification commands pass.

## Unresolved questions

None. Native protocol additions and a precise reasoning-level model option are intentionally follow-up work once this common-request/catalog/adapter foundation is accepted.

## Stop conditions

Stop for clarification if implementation requires inferring metadata from model names, adding a fallback for an absent catalog entry, changing `agents.json` or session persistence, adding a native provider protocol beyond OpenAI Chat/Responses, adding free-form JSON patching, changing response-parser semantics, or modifying an undeclared path.

## Completion

**Accepted outcome:** BlazeAI now uses explicit saved model definitions to select an OpenAI Chat or OpenAI Responses protocol adapter. `Client.StreamWithPhase` creates one neutral request, validates/lowers it before transport, and sends the lowered native body. A GLM-style OpenAI Chat model variant can explicitly omit stream usage and historical reasoning fields. Missing metadata and capability/protocol mismatches fail before HTTP work; no fallback or model-name protocol selection was added.

**Changed paths:**

- `internal/config/config.go`
- `internal/config/config_test.go`
- `firstrun.go`
- `firstrun_test.go`
- `internal/provider/protocol.go`
- `internal/provider/openai_chat_protocol.go`
- `internal/provider/openai_chat_protocol_test.go`
- `internal/provider/openai_responses_protocol.go`
- `internal/provider/openai_responses_protocol_test.go`
- `internal/provider/provider.go`
- `internal/provider/provider_test.go`
- `internal/provider/openai_responses.go`
- `internal/provider/openai_responses_test.go`
- `internal/runtime/runtime_test.go`
- `internal/console/console_test.go`
- `internal/telegram/commands_test.go`

**Verification:**

- `go test ./internal/config` — PASS
- `go test .` — PASS
- `go test ./internal/provider` — PASS
- `go test ./internal/runtime` — PASS
- `go test ./internal/console` — PASS
- `go test ./internal/telegram` — PASS
- Focused conformance audit `.agents/explorer/000006-protocol-routing-conformance.md` — PASS for all eight required facts.

**Remaining issues:** Native Anthropic/Gemini adapters, tool-schema dialect projections, and configurable canonical reasoning levels are intentionally deferred. No accepted implementation blocker remains.

## Post-completion verification correction

Running `go test ./...` after the focused checks exposed `internal/compaction` tests that constructed a raw provider client with no selected protocol. The test fixture must instead create its summarization client through `provider.NewClient` using the same explicit OpenAI Chat model definition required by the new catalog contract. This is a fixture-only correction; it does not change production behavior or relax protocol selection.
