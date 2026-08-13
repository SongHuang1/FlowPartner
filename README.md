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

Early development. The project has three layers in place:

**What's in the repo:**

- `frontend/` — Electron + React + TypeScript + Tailwind: desktop app with system tray, native menu, activity bar, sidebar settings panel, chat area (WebSocket), and persistent data via REST
- `backend/` — Go: gRPC server, WebSocket server, bridge manager (WebSocket↔gRPC), atomic file storage, API Key encryption and memory management
- `agent/` — Python: gRPC client, ReAct agent loop, tool registry (read_file, write_file, list_directory)
- `proto/` — gRPC protocol definitions and generated code for both Go and Python

**Communication flow:** Electron (port discovery via IPC) → Frontend (bootstrap: fetchBackendPort → initApi) → WebSocket → Go bridge → gRPC bidirectional stream → Python agent → gRPC CallLLM → Go → WebSocket → Frontend

**Known issues:**

- The `CallLLM` handler returns mock responses — real LLM API integration is not yet connected.

**Not yet implemented:**

- Real LLM API integration (currently mocked)
- Safety mechanisms (dangerous-op blacklist, auto-backup, operation logs)
- Multi-conversation management
- Agent Skill system

## Project structure

```
FlowPartner/
├── .github/workflows/    # CI: ci.yml (Go + TS), release.yml (Electron build)
├── agent/                # Python Agent layer (uv)
│   ├── proto/            # proto file (sync with backend/proto/)
│   ├── src/agent/        # main.py, grpc_client.py, core/, tools/
│   ├── tests/
│   └── pyproject.toml
├── backend/              # Go backend
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── bridge/       # WebSocket ↔ gRPC bridge (core)
│   │   ├── handler/      # HTTP (legacy) + WebSocket/gRPC handlers
│   │   ├── config/
│   │   ├── crypto/       # API Key encryption/zeroing
│   │   ├── keystore/     # API Key memory management
│   │   ├── response/         # Standard response format
│   │   ├── sanitize/         # Error message sanitization (prevents credential leaking)
│   │   ├── server/           # Port discovery
│   │   └── storage/          # Atomic JSON writes (~/.flowpartner/)
│   └── proto/            # proto definitions + generated .pb.go
├── docs/
├── frontend/             # Electron + React + TypeScript + Tailwind
│   ├── electron/main.cjs
│   ├── electron/preload.cjs
│   ├── src/
│   │   ├── components/   # chat (ChatArea, ConnectionStatus, EventDetail), layout, settings, ui
│   │   ├── hooks/        # useConversation, useLock, useSettings, useWindowState, useWebSocket
│   │   ├── lib/          # api.ts (HTTP client + dynamic port init), utils.ts, validation.ts
│   │   └── types/
│   └── package.json
├── Makefile
├── CONTRIBUTING.md
├── SECURITY.md
├── README.md
└── README.zh.md
```

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to contribute.

## Security

See [SECURITY.md](./SECURITY.md) for our security policy and how to report vulnerabilities.
