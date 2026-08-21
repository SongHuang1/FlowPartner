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
- Tool execution via Go proxy (read, write, bash, edit, trash, purge)
- 30s timeout for shell execution
- File size limits (10MB for read operations)
- Shell deletion-command blacklist (`rm`, `del`, `Remove-Item`, etc. are intercepted and routed to the recoverable `trash` tool)
- Recoverable deletion: files are moved to a recycle-bin directory via `trash`; permanent `purge` always requires explicit user approval and writes an audit log
- Automatic workspace snapshots: change-triggered (60s debounce) plus periodic fallback (15min), sensitive files excluded by default, restore always takes a pre-snapshot first

**Planned:**
- Append-only operation logs (currently only purge/agent-definition changes are audited; no global append-only log API yet)

## Our Security Philosophy

FlowPartner's core premise is that non-technical users tend to trust AI too much. The software must act as a safety gatekeeper. This means:

- **Dangerous operations are blocked by default.** File deletion, system configuration changes, privilege escalation — these require explicit user confirmation.
- **Recoverable by default.** Deletions go to a recycle bin; the workspace is snapshotted automatically so state can be rolled back (single files are backed up via snapshots on a best-effort schedule rather than before every individual write).
- **All operations are logged.** Logs are append-only and cannot be deleted through the API.

## Reporting a Security Issue

If you find a security vulnerability, please use the **"Report a vulnerability"** button on the Issues page. This creates a private issue that only you and the maintainers can see.

## Future Scope

As the project grows, we will cover:

- How to report vulnerabilities in the Agent execution pipeline
- How to report bugs in the safety blacklist or backup system
- Safe harbor for good-faith security research
