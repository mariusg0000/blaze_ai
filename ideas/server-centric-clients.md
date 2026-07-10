# Server-Centric BlazeAI Clients

## Goal

Rework BlazeAI into a central server that owns the full agent runtime, while separate client apps provide the UI transport and execute tool calls on the local client device.

## Core Model

The BlazeAI server runs on a trusted host such as a NAS or home server. It keeps the LLM connections, prompt building, compaction, session persistence, configuration, and the main orchestration loop.

Each client app connects to that server through a secure persistent connection. The client handles the user interface and exposes a local execution environment. When the LLM requests a tool, the server decides whether that tool belongs to the connected client and forwards the tool call there.

The BlazeAI server does not run user-facing tools locally in this model. It only brokers tool calls to connected clients and waits for their results.

## Client Types

- `ConsoleAI` runs in a local terminal and executes shell and file tools on that machine.
- `WebApp` provides a browser UI and can expose a reduced or delegated local tool set.
- `Android` provides a mobile UI and can execute phone-local tools.
- `Desktop App` provides a native desktop UI and can execute desktop-local tools.
- `Telegram` stays as a remote chat transport with its own restricted capability set.

## Tool Execution Rule

Tool execution location depends on the connected client, never on the BlazeAI server.

Examples:

- `ConsoleAI` shell commands run on the console client host.
- Android-specific tools run on the Android phone.
- Web tools run only if the Web client exposes that capability.
- Telegram exposes only the tools explicitly allowed for that transport.

If a required client capability is missing or the client is disconnected, the server must fail with a clear error. It must not silently run the tool on the server as a fallback.

This means the server acts as an orchestration node only, not as an execution target for shell, file, or device tools.

## Prompt And Tool Exposure

Prompt content and available tools should change per connected client.

The server should inject:

- active client type
- client OS
- client working directory when applicable
- client capability list
- explicit statement of where tools run

Only tools available for the active client should be exposed to the LLM.

## Protocol Shape

The server and client need a transport protocol that supports:

- session start and authentication
- bidirectional streaming assistant output
- tool call dispatch
- tool result return
- approval requests
- cancellation
- reconnect and heartbeat

## Design Notes

- The server remains the single source of truth for sessions and orchestration.
- Clients are execution targets with transport-specific UX and capabilities.
- Tool names should stay location-aware, for example `console_shell`, `android_shell`, or `desktop_shell`.
- This architecture makes it possible to support console, web, Android, desktop, and Telegram without duplicating the full runtime in every frontend.

## Deployment Implication

Because the server no longer needs local shell or filesystem tool execution, it becomes much easier to package and run in Docker.

- The container can focus on network access, persistent session storage, config, and outbound LLM connectivity.
- Client-side tool execution removes the need to grant the server container broad host-machine access.
- Docker becomes a clean deployment target for NAS or home-server environments because the server is only the agent runtime and orchestration layer.

## Evaluation

This looks like a strong long-term direction for BlazeAI. It turns the project from a local terminal agent with multiple transports into a central agent runtime with multiple local execution clients.

Main strengths:

- The server becomes stable, easier to deploy, and easier to isolate in Docker.
- Clients become explicit local execution targets instead of thin display layers.
- The model fits the no-fallback rule well because missing client capabilities can fail fast and clearly.
- The same central runtime can serve console, desktop, Android, web, and Telegram-facing experiences without duplicating the full agent core.

Main risks:

- This is a significant architecture change, not just a transport refactor.
- The protocol becomes a critical subsystem because it must handle auth, reconnect, cancel, timeout, approvals, and tool-call correlation.
- Distributed tool execution is harder to debug than the current in-process local tool model.
- Web and Telegram are weaker execution environments, so their capability model must stay explicit and constrained.

Recommended rollout:

1. Build `BlazeAI Server` plus `ConsoleAI` first.
2. Support one connected client per session in the first version.
3. Start with the core client-side tools only: shell, read file, write file, replace block.
4. Validate the remote tool-call protocol before adding more transports.
5. Add Docker packaging for the server once the console client flow is stable.
6. Add desktop, Android, web, and Telegram incrementally after the protocol and capability model are proven.

Overall verdict: the idea is strong and worth pursuing, but only through an incremental rollout that keeps the first distributed version narrow and easy to reason about.
