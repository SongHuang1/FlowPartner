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

Early development. The project has a runnable Go backend and a React frontend, with the Python Agent layer still to come.

**What's in the repo:**

- `backend/` — Go HTTP server: config loading, standard response format, health check, SPA serving
- `frontend/` — React + TypeScript + Tailwind CSS: desktop-style UI shell (title bar, activity bar, sidebar, chat area, status bar)
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
├── frontend/           # TypeScript frontend (React + Vite + Tailwind)
├── backend/            # Go backend (HTTP server, safety layer)
├── agent/              # Python Agent orchestration (coming soon)
├── docs/               # Design documents (not committed)
└── Makefile
```

## Running locally

```bash
# Backend
cd backend && go run cmd/server/main.go

# Frontend
cd frontend && npm install && npm run dev
```

## Contributing

We're in early development. Once the architecture stabilizes, we'll add a contributing guide. For now, feel free to open an issue if you have thoughts on the design.

## License

[MIT](./LICENSE) © 2026 SongHuang
