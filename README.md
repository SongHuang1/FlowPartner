# FlowPartner

[English](README.md) | [中文](README.zh.md)

FlowPartner is an AI agent desktop app built for non-technical users. People who don't have a computer background tend to trust AI too much — so the software itself has to be the safety gatekeeper, not the user.

**No code yet.** We're still in the design and planning phase.

## The core idea

Most AI tools assume the user knows what they're doing. FlowPartner assumes the opposite. Every design decision starts from the same question: *what happens if the user blindly trusts the AI?*

This leads to a few non-negotiables:

- **Fool-proof first.** If a design can lead the user into an unrecoverable state, it's rejected — no matter how elegant.
- **Safety over features.** Dangerous operations get blocked by default. The user can override, but they have to consciously choose to.
- **Always recoverable.** Before any file is modified or deleted, the system creates a backup. One click to undo.

## Contributing

We're not ready for code contributions yet. Once we have something runnable, we'll add a contributing guide. For now, feel free to open an issue if you have thoughts on the design.

## License

[MIT](./LICENSE) © 2026 SongHuang
