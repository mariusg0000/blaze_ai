# Context Router

## Goal

Add a fast secondary LLM that manages skill and memory-bank activation automatically before each main LLM call.

## Flow

1. User sends a message.
2. Before forwarding it to the main LLM, the runtime collects roughly the last 10 user and assistant message pairs.
3. That transcript goes to a lightweight router model.
4. The router also sees all available skill and memory-bank descriptions plus the current active state.
5. The router returns structured load and unload decisions.
6. The runtime applies those decisions, then builds the final prompt for the main LLM.

## Design Notes

- The router never produces user-facing output.
- The main LLM still keeps manual `load_skill` and `load_memory_bank` access.
- The router model should be configurable in `config.json`, for example through a `routerModel` role.
- If no router model is configured, auto-management stays disabled and manual behavior remains unchanged.
- The router prompt should return strict JSON actions only.

## Naming Candidates

- Context Router
- Skill Manager
- Context Orchestrator
- Prelude Model
- Gatekeeper
