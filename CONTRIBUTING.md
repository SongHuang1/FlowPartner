# Contributing to FlowPartner

Thank you for your interest in contributing to FlowPartner! This document outlines the guidelines for contributing to this project.

## Code of Conduct

This project adheres to a code of conduct. By participating, you are expected to uphold this code. Please report unacceptable behavior.

## How to Contribute

### Reporting Bugs

Before creating a bug report, please check the existing issues to avoid duplicates. When filing an issue, include:

- A clear, descriptive title
- Steps to reproduce the problem
- Expected behavior vs actual behavior
- Your environment (OS, Go version, Node version)
- Any relevant logs or screenshots
- **Attention** please! Your issue will not be processed if the issue is made by AI Agents.

### Suggesting Features

Feature requests are welcome. Please open an issue and describe:

- The problem you're trying to solve
- Your proposed solution
- Any alternative solutions you've considered

### Pull Requests

1. Fork the repository and create your branch from `main`
2. Make your changes
3. Add or update tests as needed
4. Ensure all tests pass (`make test-all`)
5. Check if CI/CD (if needed) pass
6. Update documentation if needed
7. Submit a pull request to the `main` branch

## Development Setup

### Prerequisites

- Go 1.26+
- Node.js 26+
- npm 10+
- Python 3.12+ (with uv)
- protoc + `protoc-gen-go` / `protoc-gen-go-grpc` / `grpc_tools.protoc` (only needed when changing proto files)

### Getting Started

```bash
# Clone the repository
git clone https://github.com/SongHuang1/FlowPartner.git
cd FlowPartner

# Install frontend dependencies
cd frontend && npm install && cd ..

# Install Python agent dependencies
cd agent && uv sync --frozen && cd ..

# Run tests to verify setup
make test-all
```

## Coding Standards

### Go

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` for formatting
- Write tests for new functionality
- Document public functions with doc comments

### TypeScript

- Use ES Module syntax (import/export)
- Use functional components with Hooks (no class components)
- Wrap async operations in try-catch
- Add TypeScript types to all function parameters and return values

### Python

- Use type annotations (parameters + return values)
- Use `pathlib.Path` for file paths, never `os.path`
- Catch specific exceptions, never bare `except:`
- Package manager is uv: `uv sync --frozen` to install, `uv run` to execute

## Proto / gRPC

Proto definitions are duplicated in two locations and must stay **byte-identical** (both contain the `go_package` option):
- `backend/proto/agent.proto` (source for Go codegen)
- `agent/proto/agent.proto` (source for Python codegen)

**Any proto change must be applied to both files**, then regenerate via `make gen-proto`:
- Go: `agent.pb.go`, `agent_grpc.pb.go`
- Python: `agent_pb2.py`, `agent_pb2_grpc.py`

Never manually edit generated files.

## Commit Messages

We use the following format:

```
<type>(<scope>): <subject>
```

**Types:** `feat`, `fix`, `refactor`, `security`, `docs`, `test`, `chore`

**Scopes:** `ts`, `py`, `go`, `proto`, `ui`, `agent`, `rag`, `crypto`, `keystore`, `llm`

Example: `feat(go): add health check endpoint`

## Branch Strategy

- `main` — stable production code, PRs merge here
- Feature branches: `feature/<description>`
- Bug fix branches: `fix/<description>`

## Questions?

Feel free to open an issue if you have questions about contributing.
