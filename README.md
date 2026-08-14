# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner is an AI agent desktop app built for non-technical users. People who don't have a computer background tend to trust AI too much — so the software itself has to be the safety gatekeeper, not the user.

## The core idea

Most AI tools assume the user knows what they're doing. FlowPartner assumes the opposite. Every design decision starts from the same question: *what happens if the user blindly trusts the AI?*

This leads to a few non-negotiables:

- **Fool-proof first.** If a design can lead the user into an unrecoverable state, it's rejected.
- **Safety over features.** Dangerous operations get blocked by default. The user can override, but they have to consciously choose to.
- **Always recoverable.**

## Build from Source

This section explains how to create a distributable installer from source. The final output is a single installable package (`.exe` / `.dmg` / `.deb`) — end users do not need Go, Node.js, Python, or any other tool installed.

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.26+ | Compile the backend binary |
| Node.js | 26+ | Build the frontend and package the Electron app |
| Python | 3.12+ | Compile the agent to a standalone executable |
| uv | latest | Python dependency management |
| protoc | 3.21+ | gRPC code generation |
| protoc-gen-go | latest | Go protobuf stubs |
| protoc-gen-go-grpc | latest | Go gRPC stubs |

Install protoc plugins:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Step 1: Clone

```bash
git clone https://github.com/SongHuang1/FlowPartner.git
cd FlowPartner
```

### Step 2: Build the Go backend

```bash
cd backend
go build -o flowpartner-backend ./cmd/server/
```

Output: `backend/flowpartner-backend` (or `.exe` on Windows).

In production, Electron spawns this binary from `resources/bin/`. The binary prints `__FP_BACKEND_READY__ HTTP=:PORT gRPC=:PORT` when ready.

### Step 3: Build the Python agent as a standalone executable

The agent must be packaged as a standalone executable so end users don't need Python installed.

```bash
cd agent
uv sync --frozen
```

Then compile to a standalone binary (requires PyInstaller):

```bash
cd agent
uv run pyinstaller --onefile --name flowpartner-agent src/agent/main.py
```

Output: `agent/dist/flowpartner-agent` (or `.exe` on Windows).

**Note:** If you're cross-compiling for a different OS, run PyInstaller on that target OS (or use a CI runner for that platform).

### Step 4: Build the frontend

```bash
cd frontend
npm ci
npm run build
```

### Step 5: Assemble everything into one installer

Copy the compiled binaries into `frontend/bin/`:

```bash
# From repo root
cp backend/flowpartner-frontend/bin/
cp agent/dist/flowpartner-agent frontend/bin/   # Same for all platforms
```

Update `frontend/electron-builder.yml` to include both binaries in `extraResources`:

```yaml
extraResources:
  - from: bin/
    to: bin/
    filter:
      - flowpartner-backend*
      - flowpartner-agent*
```

Then build the Electron installer:

```bash
cd frontend
npm run build:electron
```

Output:
- Windows: `frontend/dist-electron/FlowPartner-Windows-{version}-{arch}.exe`
- macOS: `frontend/dist-electron/FlowPartner-macOS-{version}-{arch}.dmg`
- Linux: `frontend/dist-electron/FlowPartner-Linux-{version}-{arch}.AppImage`

### Quick build (Makefile)

```bash
make build-go-binary          # Compile Go backend → frontend/bin/
make build-agent              # Compile Python agent → frontend/bin/
make build-electron           # Build frontend + package everything into one installer
make cross-build-all          # Cross-compile Go for 6 platform/arch combos
```

### Regenerate protobuf stubs

```bash
# Go
cd backend && protoc --go_out=. --go-grpc_out=. proto/agent.proto

# Python
cd agent && uv run python -m grpc_tools.protoc -I ../backend/proto --python_out=src/agent --grpc_python_out=src/agent agent.proto
```

**Important:** `backend/proto/agent.proto` and `agent/proto/agent.proto` must be kept in sync. The Go version has `go_package`; the Python version does not.

### Development workflow

For local development (hot reload, no packaging):

```bash
# Terminal 1: Backend
cd backend && go run cmd/server/main.go

# Terminal 2: Agent
cd agent && uv run python -m src.agent.main

# Terminal 3: Frontend
cd frontend && npm run dev

# Terminal 4: Electron (optional)
cd frontend && npm run dev:electron
```

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
│   │   ├── crypto/       # API Key encryption/zeroing
│   │   ├── keystore/     # API Key memory management
│   │   ├── llm/          # LLM HTTP streaming client (SSE)
│   │   ├── sanitize/     # Error sanitization
│   │   ├── server/       # Port discovery
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
