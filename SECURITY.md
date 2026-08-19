# Security

FlowPartner is a safety-focused AI Agent desktop application. Our security model is different from traditional software: we're not just protecting against external threats — we're protecting users from their own AI's mistakes.

## Current Status

Early development. Some security mechanisms are already implemented; others are planned.

**Already implemented:**
- API Key encrypted at rest (AES-GCM with Argon2id key derivation)
- API Key zeroed from memory after use
- Unlock rate limiting (progressive lockout after failed attempts)
- Internal/private network URL blocking for LLM base URL
- Input validation on all API endpoints
- Atomic file writes (temp + rename) to prevent data corruption
- Path traversal protection on file operations
- Error message sanitization (prevents credential/token leaking in error responses)
- Dual-layer path validation (lexical + symlink resolution)
- Permission approval flow for out-of-workspace operations
- Tool execution via Go proxy (read, write, bash, edit)
- 30s timeout for shell execution
- File size limits (10MB for read operations)

**Planned:**
- Dangerous operation blacklist (file deletion, system config changes, privilege escalation)
- Automatic file backup before modification or deletion
- Append-only operation logs
- Per-session operation audit trail

## Our Security Philosophy

FlowPartner's core premise is that non-technical users tend to trust AI too much. The software must act as a safety gatekeeper. This means:

- **Dangerous operations are blocked by default.** File deletion, system configuration changes, privilege escalation — these require explicit user confirmation.
- **Every file operation is backed up.** Before a file is modified or deleted, the original is preserved. One click to undo.
- **All operations are logged.** Logs are append-only and cannot be deleted through the API.

## Reporting a Security Issue

If you find a security vulnerability, please use the **"Report a vulnerability"** button on the Issues page. This creates a private issue that only you and the maintainers can see.

## Future Scope

As the project grows, we will cover:

- How to report vulnerabilities in the Agent execution pipeline
- How to report bugs in the safety blacklist or backup system
- Safe harbor for good-faith security research
