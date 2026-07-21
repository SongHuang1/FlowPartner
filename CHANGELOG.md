# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Electron desktop shell with system tray and Chinese-localized native menu
- Port negotiation between Electron main process and Go backend (production mode)
- `__FP_BACKEND_READY__` stderr signal for backend readiness detection
- Auto-scroll to bottom in MessageList when new messages appear
- Sidebar expand/collapse animation (w-64 ↔ w-0 transition)
- `electron-builder.yml` for Windows NSIS installer packaging
- `build-go-binary` and `build-electron` Makefile targets
- `@types/node` added for Electron API type support
- IPC handler for async app version retrieval (sandbox-compatible)

### Fixed
- `go mod tidy` to correctly mark `github.com/google/uuid` as direct dependency
- Sidebar animation using conditional rendering (now always rendered with width toggle)
- Preload script crash in sandbox mode (replaced `app.getVersion()` with IPC invoke)
- Go command name portability (use `go` instead of `go.exe` on Windows)

### Security
- BrowserWindow configured with `contextIsolation: true`, `nodeIntegration: false`, `sandbox: true`
- Preload uses `contextBridge` to safely expose minimal API to renderer

## [0.1.0] - 2026-07-22

### Added
- Initial Go backend scaffold with `/health` endpoint and config loading
- React + TypeScript frontend with desktop-style UI shell
- CI pipeline with GitHub Actions
- Standard response format with `code`, `message`, `data`, `timestamp`, `request_id`
- Initial test suite (backend + frontend)
