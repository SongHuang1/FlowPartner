# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner is an AI agent desktop app built for non-technical users. People who don't have a computer background tend to trust AI too much — so the software itself has to be the safety gatekeeper, not the user.

## The core idea

Most AI tools assume the user knows what they're doing. FlowPartner assumes the opposite. Every design decision starts from the same question: *what happens if the user blindly trusts the AI?*

This leads to a few non-negotiables:

- **Fool-proof first.** If a design can lead the user into an unrecoverable state, it's rejected.
- **Safety over features.** Dangerous operations get blocked by default. The user can override, but they have to consciously choose to.
- **Always recoverable.**


---

## Current status

Early development. The project has three layers in place:

- `frontend/` — Electron + React + TypeScript + Tailwind
- `backend/` — Go: gRPC, WebSocket, bridge manager, API Key encryption, LLM HTTP streaming client
- `agent/` — Python: gRPC client, ReAct loop, tool registry

**Communication flow:** Electron (port discovery) → Frontend (bootstrap) → WebSocket → Go bridge → gRPC bidirectional stream → Python agent → gRPC CallLLM (server-streaming) → Go LLM client → OpenAI-compatible API → SSE chunks → stream back to Frontend

## Project structure

```
FlowPartner/
├── .github/workflows/    # CI: ci.yml, release.yml
├── agent/                # Python Agent layer
│   ├── proto/            # proto file (sync with backend/proto/)
│   ├── src/agent/        # main.py, grpc_client.py, core/, tools/
│   └── pyproject.toml
├── backend/              # Go backend
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── bridge/       # WebSocket ↔ gRPC bridge
│   │   ├── handler/      # HTTP handlers + WebSocket/gRPC handlers
│   │   ├── tools/        # Tool execution layer (read/write/bash/edit)
│   │   ├── crypto/       # API Key encryption/zeroing
│   │   ├── keystore/     # API Key memory management
│   │   ├── llm/          # LLM HTTP streaming client (SSE)
│   │   ├── sanitize/     # Error sanitization
│   │   ├── server/       # Port discovery
│   │   ├── static/       # Frontend static file server
│   │   └── storage/      # Atomic JSON writes
│   └── proto/            # proto definitions + generated .pb.go
├── frontend/             # Electron + React + TypeScript + Tailwind
│   ├── electron/
│   │   ├── main.cjs      # Electron main process (spawns Go + Python)
│   │   └── preload.cjs   # IPC bridge to renderer
│   ├── src/              # React UI
│   ├── electron-builder.yml
│   └── package.json
├── Makefile
└── README.md
```

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## Security

See [SECURITY.md](./SECURITY.md).
