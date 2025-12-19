# ProxyPilot (Windows)

English | [中文](README_CN.md)

ProxyPilot is a Windows “VibeProxy-style” desktop app for running and managing a local AI proxy (tray app + in-app dashboard), with extra compatibility for agentic CLIs (Factory/Droid, Codex CLI, Warp, etc.).

This repo started as a fork of **CLIProxyAPI**. For compatibility, some internal names (Go module path, `X-CLIProxyAPI-*` headers, legacy binary names) still exist, but the user-facing product is ProxyPilot.

Windows app packaging: `docs/proxypilot.md`

## Sponsor

[![z.ai](https://assets.router-for.me/english.png)](https://z.ai/subscribe?ic=8JVLJQFSKB)

This project is sponsored by Z.ai, supporting us with their GLM CODING PLAN.

GLM CODING PLAN is a subscription service designed for AI coding, starting at just $3/month. It provides access to their flagship GLM-4.6 model across 10+ popular AI coding tools (Claude Code, Cline, Roo Code, etc.), offering developers top-tier, fast, and stable coding experiences.

Get 10% OFF GLM CODING PLAN: https://z.ai/subscribe?ic=8JVLJQFSKB

## Overview

- OpenAI/Gemini/Claude compatible API endpoints for CLI models
- OpenAI Codex support (GPT models) via OAuth login
- Claude Code support via OAuth login
- Qwen Code support via OAuth login
- iFlow support via OAuth login
- Streaming and non-streaming responses
- Function calling/tools support
- Multimodal input support (text and images)
- Multiple accounts with round-robin load balancing (Gemini, OpenAI, Claude, Qwen and iFlow)
- Simple CLI authentication flows (Gemini, OpenAI, Claude, Qwen and iFlow)
- Generative Language API Key support
- AI Studio Build multi-account load balancing
- Gemini CLI multi-account load balancing
- Claude Code multi-account load balancing
- Qwen Code multi-account load balancing
- iFlow multi-account load balancing
- OpenAI Codex multi-account load balancing
- OpenAI-compatible upstream providers via config (e.g., OpenRouter)
- Reusable Go SDK for embedding the proxy (see `docs/sdk-usage.md`)

## Getting Started (Engine)

Upstream-style docs for the engine live here:

- Engine guides (upstream): https://help.router-for.me/
- Management API: https://help.router-for.me/management/api

## Cursor IDE

- Integration guide: `docs/cursor-ide.md`

## Compatibility

- Quick matrix: `docs/compat-matrix.md`

## SDK Docs

- Usage: `docs/sdk-usage.md`
- Advanced (executors & translators): `docs/sdk-advanced.md`
- Access: `docs/sdk-access.md`
- Watcher: `docs/sdk-watcher.md`
- Custom Provider Example: `examples/custom-provider`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Who is with us?

Those projects are based on CLIProxyAPI:

### [vibeproxy](https://github.com/automazeio/vibeproxy)

Native macOS menu bar app to use your Claude Code & ChatGPT subscriptions with AI coding tools - no API keys needed

### [Subtitle Translator](https://github.com/VjayC/SRT-Subtitle-Translator-Validator)

Browser-based tool to translate SRT subtitles using your Gemini subscription via CLIProxyAPI with automatic validation/error correction - no API keys needed

### [CCS (Claude Code Switch)](https://github.com/kaitranntt/ccs)

CLI wrapper for instant switching between multiple Claude accounts and alternative models (Gemini, Codex, Antigravity) via CLIProxyAPI OAuth - no API keys needed

### [ProxyPal](https://github.com/heyhuynhgiabuu/proxypal)

Native macOS GUI for managing CLIProxyAPI: configure providers, model mappings, and endpoints via OAuth - no API keys needed.

> [!NOTE]
> If you developed a project based on CLIProxyAPI, please open a PR to add it to this list.

## License

This project is licensed under the MIT License - see `LICENSE`.

## Disclaimer

This project is intended for educational and interoperability research purposes only. It interacts with various internal or undocumented APIs (such as Google's `v1internal`) to provide compatibility layers.

- **Use at your own risk.** The authors are not responsible for any consequences arising from the use of this software, including but not limited to account suspensions or service interruptions.
- This project is **not** affiliated with, endorsed by, or associated with Google, OpenAI, Anthropic, or any other service provider mentioned.
- Users are expected to comply with the Terms of Service of the respective platforms they connect to.
