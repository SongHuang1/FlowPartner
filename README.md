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

## Highlights

### Industrial-Grade Security Architecture

- **Dual-layer path validation**: All file operations pass through lexical checks + symlink resolution to prevent path traversal attacks (`path_guard.go`)
- **AES-256-GCM + Argon2id encryption**: API Keys use industry-standard encryption, decrypted form lives only in memory, explicitly zeroized on lock (`crypto/`, `keystore/`)
- **Rate limiting against brute force**: 5 failed password attempts trigger a 30-second lockout (`keystore.go`)
- **Error sanitization**: 7 regex patterns filter API Keys, Tokens, passwords from logs (`sanitize/`)
- **Delete protection**: Shell deletion commands (rm/del/Remove-Item, etc.) are intercepted and redirected to recoverable trash (`delete_guard.go`)

### High-Reliability Communication Pipeline

- **WebSocket + gRPC bidirectional streaming**: Frontend WebSocket ↔ Go bridge ↔ gRPC bidirectional stream ↔ Python Agent, fully async and non-blocking
- **Server-streaming LLM calls**: SSE stream parsing + idle timeout + auto-retry before first chunk, OpenAI-compatible API support (`llm/`)
- **Backpressure control**: WebSocket inbound queue returns overload errors when full, preventing memory exhaustion (`wsv2/router.go`)
- **Graceful shutdown**: gRPC GracefulStop → disconnect WebSocket → close HTTP, with 2s timeout fallback (`main.go`)

### Automatic Snapshots & Recoverability

- **Triple-trigger mechanism**: fsnotify file change debounce 60s + 15min periodic fallback + lock screen flush, ensuring no work state is lost (`snapshot/manager.go`)
- **Pre-restore snapshot**: Every restore automatically takes a snapshot first, making the restore operation itself reversible (`snapshot/restore.go`)
- **Atomic writes**: All file writes use "write temp file + rename" strategy, preventing data corruption from interrupted writes (`snapshot/capture.go`, `storage/storage.go`)
- **Retention policy**: 30-day / 5GB dual-dimension auto-cleanup, preventing unbounded storage growth (`snapshot/retain.go`)

### Multi-Agent Orchestration

- **Main-sub Agent architecture**: Main agent dispatches sub-agents via `agent__<name>` tools, with depth limits and event forwarding (`agent_def.go`, `thread/`)
- **Hot-reloadable definitions**: Agent definition changes broadcast `agents_changed` invalidation via gRPC, Python-side TTL cache auto-refreshes
- **Turn management**: Complete thread/turn lifecycle with interrupt and steer support (`thread/handlers.go`)

### Engineering Quality

- **Full CI/CD pipeline**: GitHub Actions for three languages (Go + TypeScript + Python) + Electron auto-build & release
- **Cross-platform compilation**: Windows / macOS / Linux × amd64 / arm64 one-command cross-compile (`Makefile`)
- **Zero dead code**: No unused imports, no commented-out legacy code, no dead-code tests

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
│   ├── proto/            # proto file (byte-identical with backend/proto/)
│   ├── src/agent/        # main.py, grpc_client.py, core/ (react_agent, subagent_runner, agent_registry), tools/
│   ├── tests/
│   └── pyproject.toml
├── backend/              # Go backend
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── wsv2/        # WebSocket v2 protocol (envelope, router, backpressure)
│   │   ├── thread/       # Thread/turn management (Manager, Handler, EventConverter)
│   │   ├── handler/      # HTTP handlers + WebSocket/gRPC handlers
│   │   ├── tools/        # Tool execution layer (read/write/bash/edit/trash/purge)
│   │   ├── snapshot/     # Workspace snapshot subsystem (auto capture + restore)
│   │   ├── config/       # Environment config
│   │   ├── crypto/       # API Key encryption/zeroing
│   │   ├── keystore/     # API Key memory management
│   │   ├── llm/          # LLM HTTP streaming client (SSE)
│   │   ├── response/     # Standard API response format
│   │   ├── sanitize/     # Error sanitization
│   │   ├── server/       # Port discovery
│   │   ├── static/       # Frontend static file server
│   │   └── storage/      # Atomic JSON writes (history, agent definitions)
│   └── proto/            # proto definitions + generated .pb.go
├── frontend/             # Electron + React + TypeScript + Tailwind
│   ├── electron/
│   │   ├── main.cjs      # Electron main process (spawns Go + Python)
│   │   └── preload.cjs   # IPC bridge to renderer
│   ├── src/              # React UI (chat, settings incl. agents/snapshots)
│   ├── electron-builder.yml
│   └── package.json
├── Makefile
└── README.md
```

## Tech stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Frontend | Electron + React + TypeScript + Tailwind | Desktop app UI |
| Backend | Go 1.26+ | Communication bridge, tool execution, security control |
| Agent | Python 3.12+ | AI orchestration, LLM calls, tool registry |
| Communication | WebSocket + gRPC bidirectional streaming | Frontend-backend + cross-language communication |
| Serialization | Protocol Buffers | Cross-language message definitions |
| Encryption | AES-256-GCM + Argon2id | API Key encrypted storage |
| Build | Makefile + GitHub Actions | CI/CD + cross-platform compilation |

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## Security

See [SECURITY.md](./SECURITY.md).
