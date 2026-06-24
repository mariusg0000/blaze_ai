# IDEAS.md — Future Feature Concepts

## 1. Memory-Bank

**Goal:** On-demand contextual knowledge bases, loaded into the prompt only when relevant.

A Memory-Bank is a structured knowledge file with two sections, similar to skills:

```
[DESCRIPTION]
Short summary of what this memory bank contains.

[DETAILS]
Full knowledge block injected into the prompt when active.
```

**Examples:**
- `my-network` — all IPs, servers, roles, and topology for the local network.
- `project-deploy` — deployment targets, credentials references, CI/CD pipeline details.
- `client-acme` — client-specific infrastructure, contacts, conventions.

**Behavior:**
- Stored in `app_home/memory-banks/<name>/memory-bank.md`.
- Discovered alongside skills (builtin + custom).
- `[DESCRIPTION]` listed in the prompt as "Available Memory Banks".
- `[DETAILS]` injected only when explicitly loaded or auto-loaded by the router.
- Loaded/unloaded via `load_memory_bank` / `unload_memory_bank` tools (mirrors the skill mechanism).
- The main LLM can still call `load_memory_bank` manually.

---

## 2. Context Router (Secondary LLM)

**Goal:** A fast, lightweight LLM that manages skill and memory-bank activation automatically, before each main LLM call.

**How it works:**
1. User sends a message.
2. Before forwarding to the main LLM, the runtime takes the last ~10 user/assistant message pairs.
3. These are sent to a **Context Router** — a fast secondary model (e.g., a small local model or a cheap cloud endpoint).
4. The router knows:
   - All available skills (`[DESCRIPTION]` blocks).
   - All available memory banks (`[DESCRIPTION]` blocks).
   - Currently active skills and memory banks.
5. The router returns a decision: which skills/memory-banks to load or unload based on the conversation direction.
6. The runtime applies the router's decisions, then builds the final prompt and sends it to the main LLM.

**Design notes:**
- The router does **not** generate user-facing output — it only manages context.
- The main LLM retains the ability to call `load_skill` / `load_memory_bank` manually (no rights revoked).
- The router model is configurable in `config.json` (e.g., `routerModel` role).
- Fallback: if no router model is configured, auto-management is disabled; everything works as today (manual only).
- The router prompt includes the skill/memory-bank descriptions and the recent conversation transcript, and returns a structured JSON response with load/unload actions.

**Naming candidates:**
- Context Router
- Skill Manager
- Context Orchestrator
- Prelude Model
- Gatekeeper

---

## 3. Task-Focused Summarization

**Goal:** Replace fixed pruning + summarization with task-aware summarization that keeps only the last active task in focus.

**Idea:**
- When context compaction happens, identify the last task that is still active.
- Summarize only the work that has finished.
- Keep the active task itself uncompressed and in view.
- Treat completed tasks as closed history, not as always-on background context.

**Why it may help:**
- Reduces noise from unrelated completed work.
- Keeps the current task sharper in the prompt.
- Matches the way users usually think: one active task, older tasks summarized behind it.
