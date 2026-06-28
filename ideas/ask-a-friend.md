# Ask A Friend

## Goal

Add a controlled one-shot delegation tool that lets the main BlazeAI model consult a secondary model for a focused subproblem.

This is not a full secondary agent and not a generic arbitrary LLM transport. It is a narrow consultation primitive.

## Why

Some tasks benefit from a second opinion or from a stronger model used only for one step:

- session learning review
- architecture review
- focused debugging analysis
- plan critique
- compact summarization

The main model should stay in control, but it should be able to ask for help in a structured way.

## Tool Shape

Suggested tool name:

- `ask_a_friend`

Suggested arguments:

```json
{
  "role": "friend",
  "purpose": "get a second opinion on a focused problem",
  "question": "What is the most likely cause of this failure pattern?",
  "context": "relevant extracted logs, code, or summaries",
  "output_format": "short markdown answer with findings and recommendation"
}
```

## Core Behavior

`ask_a_friend` should:

1. Resolve a configured model role.
2. Build a one-shot prompt.
3. Call the secondary model once.
4. Return plain text to the main model as a tool result.

It should not:

- run tool loops
- create a nested session
- access files by itself
- persist hidden memory
- choose arbitrary models freely at runtime

## Roles Instead Of Arbitrary Model Names

Avoid a generic tool like:

- `send_to_llm(model_name, text)`

That shape is too broad and too easy to abuse.

Prefer role-based routing, such as:

- `friend`
- `summarization`
- `vision` later if needed

Example config direction:

```json
"roles": {
  "default": "provider/model-default",
  "summarization": "provider/model-summarizer",
  "friend": "provider/model-strong"
}
```

If the requested role is not configured, the tool must fail clearly. No fallback.

## Prompt Model

The secondary call should be built as a strict one-shot request:

- `system`: explain that this model is an expert consultant with no tool use
- `user`: include purpose, question, context, and required output format

The secondary model should return only the requested answer.

## Safety And Scope Limits

`ask_a_friend` should be intentionally constrained:

- one request only
- no recursive delegation
- no tool calls in the secondary request
- bounded input size
- bounded output size
- separate timeout

Optional guardrails:

- reject empty `question`
- reject empty `context` when `purpose` requires evidence
- cap context length to avoid careless overuse

## Good Use Cases

- "Analyze this compact session transcript and identify missing skills."
- "Review this plan and list the main risks."
- "Summarize these 30 learning reports into a single improvement plan."
- "Give a second opinion on whether this should be a memory bank or a runnable skill."

## Bad Use Cases

- arbitrary chatting with another model
- hidden multi-step autonomous execution
- replacing the main runtime loop
- bypassing normal tool-based work for tasks that do not need delegation

## Relationship To Session Learning

This tool is a good fit for the planned session learning workflow.

Examples:

- per-session `learning.md` generation can use `role: summarization`
- cross-session meta-review can use `role: friend` or the current active model

That keeps the learning pipeline reusable without baking a special hardcoded LLM path for every future review feature.

## Open Questions

- Should `friend` be a first-class role in `config.json` or a separate delegation section?
- Should the tool allow only a fixed allowlist of roles?
- Should the secondary answer be persisted anywhere beyond the normal tool result in session history?
- Should the tool expose a `temperature`-like option later, or remain fully fixed for predictability?
