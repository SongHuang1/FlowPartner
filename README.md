# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner is an AI agent desktop app built for non-technical users. People who don't have a computer background tend to trust AI too much — so the software itself has to be the safety gatekeeper, not the user.

## The core idea

Most AI tools assume the user knows what they're doing. FlowPartner assumes the opposite. Every design decision starts from the same question: *what happens if the user blindly trusts the AI?*

This leads to a few non-negotiables:

- **Fool-proof first.** If a design can lead the user into an unrecoverable state, it's rejected.
- **Safety over features.** Dangerous operations get blocked by default. The user can override, but they have to consciously choose to.
- **Always recoverable.** 

## Current status

Early development. The project has a runnable Go backend with data persistence, an Electron + React desktop frontend with a full UI shell, settings panel, and chat interface. The Python Agent layer is still to come.

**What's in the repo:**

- `backend/` — Go HTTP server: config loading, standard response format, health check, SPA serving, settings API, conversation API, JSON file storage with atomic writes
- `frontend/` — Electron + React + TypeScript + Tailwind: desktop app with system tray, native menu, activity bar, sidebar settings panel, chat area with empty/conversation state switching, and persistent data via REST API
- `proto/` — gRPC protocol definitions (placeholder, not yet populated)

**What's not here yet:**

- Python Agent orchestration layer
- Agent execution and real AI responses
- WebSocket real-time communication
- Safety mechanisms (dangerous-op blacklist, auto-backup, operation logs)
- Multi-conversation management



## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to contribute.

## Security

See [SECURITY.md](./SECURITY.md) for our security policy and how to report vulnerabilities.
