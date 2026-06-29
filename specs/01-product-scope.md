# 01 - Product Scope

## Purpose
- BlazeAI is a fast, cross-platform AI terminal agent for experienced users.
- The product is optimized for direct command execution, low interaction overhead, and maximum practical flexibility.

## Product Identity
- BlazeAI should feel like a sharp terminal assistant rather than a generic chatbot.
- The primary interaction model is command-driven, shell-native, and short-path to execution.
- The product should favor explicit control over hidden automation.

## Core Goals
- Keep the tool set as small as possible while still covering real work.
- Prefer one strong execution path over many special-purpose abstractions.
- Run consistently on Linux, macOS, and Windows.
- Keep the runtime simple enough to inspect, reason about, and repair quickly.
- Use prompt behavior as a major source of product personality and control.

## Target Users
- The primary users are technical and experienced.
- The interface should assume familiarity with terminals, shell commands, local files, and LLM-assisted workflows.
- The product should not depend on heavy guided flows or beginner-friendly wizards.

## Interaction Model
- The main interface is a simple CLI.
- The web interface should imitate a terminal session, not replace it with a generic chat UI.
- Both interfaces should behave like views over the same agent core.
- The console experience should include Markdown rendering, colored and bold labels, clear visual separation between message blocks, and streaming output.

## Execution Model
- The main tool is `shell`.
- Shell execution should prefer OS-native behavior over abstracted cross-platform tricks.
- Bash scripts and OS-specific scripts are preferred when a task needs structure beyond inline shell.
- Python is a last resort only.
- When Python is necessary, it should run inside a virtual environment under the app home tree.

## App Home Bootstrap
- At application start, BlazeAI resolves the current operating system home directory.
- The app home is the `blazeai` folder inside that OS home directory.
- If the `blazeai` folder does not exist, the runtime creates it.
- The app home is the canonical storage root for BlazeAI runtime data.
- Standard subfolders are created under app home: `skills`, `scripts`, `backups`, `projects`, `config`.
- The app home layout must be portable across supported operating systems.

## Session Model
- BlazeAI does not use a database for session storage.
- Sessions are stored on disk in the app home `sessions` folder.
- Session state should be represented with plain text and JSON files.
- The session model should preserve enough context to resume or inspect prior activity without adding database complexity.
- The storage layout should remain simple and explicit.

## Commands And Controls
- The CLI should support short slash commands for common session controls.
- Expected commands include `/exit`, `/model`, and `/cd`.
- Additional command actions may be added later, but only if they support fast terminal workflow.
- Commands should remain lightweight and discoverable.

## Prompt Ownership
- Most agent behavior should be shaped by system prompt files.
- The prompt layer is responsible for guiding execution style, tool preference, safety discipline, and interface tone.
- Prompt text should reinforce the product goals instead of compensating for a complicated runtime.

## Non-Goals
- BlazeAI is not a database-backed assistant platform.
- BlazeAI is not a full-screen TUI project.
- BlazeAI is not Python-first.
- BlazeAI is not intended to expose a large tool catalog.
- BlazeAI should not add orchestration layers that do not directly improve user outcomes.

## Product Priorities
- Speed of interaction comes first.
- Simplicity of execution comes second.
- Cross-platform behavior comes third.
- Visual polish should support clarity, not distract from the terminal workflow.
- Any new feature must justify its cost in complexity.

## Scope Boundary
- This file defines product intent, user model, interface shape, and runtime direction.
- Detailed runtime mechanics belong in the core runtime spec.
- Interface implementation details belong in the interface spec.
- Platform-specific behavior belongs in the platform and operations spec.
