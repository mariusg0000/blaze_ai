# TODO: Implement server-centric clients

## WHAT MUST BE DONE
Rework BlazeAI into a central server that owns LLM connections, prompt building, sessions, compaction, and orchestration, while connected clients provide UI and execute tools locally according to explicit capabilities.

## WHY IT MUST BE DONE
A server-centric model would support console, web, desktop, Android, and Telegram clients without duplicating the full runtime, while making the server easier to isolate and deploy in Docker.

## HOW IT MUST BE DONE
Define secure persistent client connections with authentication, streaming, tool dispatch and result correlation, approvals, cancellation, reconnect and heartbeat. Start with one ConsoleAI client and core tools, fail clearly when capabilities are missing, validate the protocol, then add Docker and other clients incrementally.
