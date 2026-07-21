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

### Suggesting Features

Feature requests are welcome. Please open an issue and describe:

- The problem you're trying to solve
- Your proposed solution
- Any alternative solutions you've considered

### Pull Requests

1. Fork the repository and create your branch from `develop`
2. Make your changes
3. Add or update tests as needed
4. Ensure all tests pass (`make test-all`)
5. Update documentation if needed
6. Submit a pull request to the `develop` branch

## Development Setup

### Prerequisites

- Go 1.26+
- Node.js 22+
- npm 10+

### Getting Started

```bash
# Clone the repository
git clone https://github.com/songhuang/flowpartner.git
cd flowpartner

# Install frontend dependencies
cd frontend && npm install && cd ..

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

### Python (future)

- Use type annotations (parameters + return values)
- Use `pathlib.Path` for file paths
- Catch specific exceptions, never bare `except:`

## Commit Messages

We use the following format:

```
<type>(<scope>): <subject>
```

**Types:** `feat`, `fix`, `refactor`, `security`, `docs`, `test`

**Scopes:** `ts`, `py`, `go`, `proto`, `ui`, `agent`, `rag`

Example: `feat(go): add health check endpoint`

## Branch Strategy

- `main` — stable production code
- `develop` — integration branch for features
- Feature branches: `feature/<description>`
- Bug fix branches: `fix/<description>`

## Questions?

Feel free to open an issue if you have questions about contributing.
