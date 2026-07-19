# Security

FlowPartner is a safety-focused AI Agent desktop application. Our security model is different from traditional software: we're not just protecting against external threats — we're protecting users from their own AI's mistakes.

## Current Status

**No code is deployed yet.** This project is in the design and early development phase. There is no running service, no user data, and no attack surface to speak of.

## OurSecurity Philosophy

FlowPartner's core premise is that non-technical users tend to trust AI too much. The software must act as a safety gatekeeper. This means:

- **Dangerous operations are blocked by default.** File deletion, system configuration changes, privilege escalation — these require explicit user confirmation.
- **Every file operation is backed up.** Before a file is modified or deleted, the original is preserved. One click to undo.
- **All operations are logged.** Logs are append-only and cannot be deleted through the API.

## Reporting a Security Issue

Once the project reaches a deployable state, we will accept security reports. For now, if you've found a design vulnerability, please open a GitHub issue or contact the maintainer directly.

## Future Scope

As the project grows, we will cover:

- How to report vulnerabilities in the Agent execution pipeline
- How to report bugs in the safety blacklist or backup system
- Safe harbor for good-faith security research
