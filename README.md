# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner is an AI agent desktop app built for non-technical users. People who don't have a computer background tend to trust AI too much — so the software itself has to be the safety gatekeeper, not the user.

## The core idea

Most AI tools assume the user knows what they're doing. FlowPartner assumes the opposite. Every design decision starts from the same question: *what happens if the user blindly trusts the AI?*

This leads to a few non-negotiables:

- **Fool-proof first.** If a design can lead the user into an unrecoverable state, it's rejected — no matter how elegant.
- **Safety over features.** Dangerous operations get blocked by default. The user can override, but they have to consciously choose to.
- **Always recoverable.** Before any file is modified or deleted, the system creates a backup. One click to undo.

## Current status

Early development. The project has a runnable Go backend, an Electron + React desktop frontend, with the Python Agent layer still to come.

**What's in the repo:**

- `backend/` — Go HTTP server: config loading, standard response format, health check, SPA serving
- `frontend/` — Electron + React + TypeScript + Tailwind: desktop app with system tray, native menu, and dual dev/production modes
- `proto/` — gRPC protocol definitions (placeholder, not yet populated)

**What's not here yet:**

- Python Agent orchestration layer
- Business logic and API endpoints
- WebSocket real-time communication
- Safety mechanisms (dangerous-op blacklist, auto-backup, operation logs)

## Project structure

```
flowpartner/
├── proto/              # gRPC proto definitions
├── frontend/           # Electron + React frontend (TypeScript + Vite + Tailwind)
├── backend/            # Go backend (HTTP server, safety layer)
├── agent/              # Python Agent orchestration (coming soon)
├── .github/            # CI workflow, issue templates, PR template
├── Makefile            # Build and test targets
├── LICENSE             # MIT License
├── SECURITY.md         # Security policy
└── README.md           # This file
```

## Running locally

### Prerequisites

- Go 1.26+
- Node.js 22+
- npm 10+

### Backend

```bash
cd backend && go run cmd/server/main.go
```

### Frontend (browser dev mode)

```bash
cd frontend && npm install && npm run dev
```

### Frontend (desktop dev mode)

```bash
# Terminal 1: Start Go backend
cd backend && FP_DEV_MODE=true go run cmd/server/main.go

# Terminal 2: Start Electron
cd frontend && npm run dev:electron
```

### Build for production

```bash
# Build frontend + compile Go binary + package installer
make build-electron
```

## Running tests

```bash
# All tests (backend + frontend)
make test-all

# Backend only
cd backend && go test ./...

# Frontend only
cd frontend && npm run test
```

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to contribute.

## Security

See [SECURITY.md](./SECURITY.md) for our security policy and how to report vulnerabilities.

## License

[MIT](./LICENSE) © 2026 SongHuang
