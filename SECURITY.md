# Security

FlowPartner is a safety-focused AI Agent desktop application. Our security model is different from traditional software: we're not just protecting against external threats — we're protecting users from their own AI's mistakes.

## Current Status

This project is in the design and early development phase. 

## OurSecurity Philosophy

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
