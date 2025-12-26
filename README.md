<p align="center">
  <img src="./static/icon.png" width="128" height="128" alt="ProxyPilot Logo">
</p>

<h1 align="center">ProxyPilot</h1>

<p align="center">
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-28a745" alt="MIT License"></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://x.com/Finessse377721"><img src="https://img.shields.io/badge/Follow-%F0%9D%95%8F%2F%40Finessse377721-1c9bf0" alt="Follow on 𝕏"></a>
  <a href="https://github.com/Finesssee/ProxyPilot"><img src="https://img.shields.io/github/stars/Finesssee/ProxyPilot.svg?style=social&label=Star%20this%20repo" alt="Star this repo"></a>
</p>

<p align="center">
  <strong>Stop juggling API keys.</strong> ProxyPilot is a powerful local API proxy that lets you use your existing Claude Code, Codex, Gemini, Kiro, and Qwen subscriptions with any AI coding tool – no separate API keys required.
</p>

<p align="center">
  Built in Go, it handles OAuth authentication, token management, and API translation automatically. One server to route them all.
</p>

---

> [!TIP]
> 📣 **Latest models supported:**
> Claude Opus 4.5 / Sonnet 4.5 with extended thinking, GPT-5.2 / GPT-5.2 Codex, Gemini 3 Pro/Flash, and Kiro (AWS CodeWhisperer)! 🚀

**Setup Guides:**
- [Claude Code Setup →](docs/claude-code-local-proxy.md)
- [Cursor IDE Setup →](docs/cursor-ide.md)

---

## Features

- 🎯 **7 Auth Providers** - Claude, Codex (OpenAI), Gemini, Gemini CLI, Kiro (AWS), Qwen, iFlow
- 🔄 **Universal API Translation** - Auto-converts between OpenAI, Anthropic, and Gemini formats
- 🔧 **Tool Calling Repair** - Fixes tool/function call mismatches between providers automatically
- 🧠 **Extended Thinking** - Full support for Claude and Gemini thinking models
- 🔐 **OAuth Integration** - Browser-based login with automatic token refresh
- 👥 **Multi-Account Support** - Round-robin distribution with automatic failover
- ⚡ **Quota Auto-Switch** - Automatically switches to backup project/model when quota exceeded
- 📊 **Usage Statistics** - Track requests, tokens, and errors per provider/model
- 🧩 **Context Compression** - LLM-based summarization for long sessions (Factory.ai research)
- 🤖 **Agentic Harness** - Guided workflow for coding agents (Anthropic research)
- 💾 **Session Memory** - Persistent storage across conversation turns
- 🎨 **Desktop App** - Native Windows control center with system tray
- 📡 **60+ Management APIs** - Full control via REST endpoints

---

## Supported Providers

| Provider | Auth Method | Models |
|----------|-------------|--------|
| Claude (Anthropic) | OAuth2 / API Key | Claude Opus 4.5, Sonnet 4.5, Haiku 4.5 |
| Codex (OpenAI) | OAuth2 / API Key | GPT-5.2, GPT-5.2 Codex |
| Gemini | OAuth2 / API Key | Gemini 3 Pro, Gemini 3 Flash |
| Gemini CLI | OAuth2 | Cloud Code Assist models |
| Kiro | OAuth2 + AWS SSO | AWS CodeWhisperer |
| Qwen | OAuth2 | Qwen models |
| iFlow | Cookie-based | Alibaba models |
| Custom | API Key | Any OpenAI-compatible endpoint |

---

## Installation

### Download Pre-built Release (Recommended)

1. Go to the [**Releases**](https://github.com/Finesssee/ProxyPilot/releases) page
2. Download the latest binary for your platform
3. Run `./proxypilot`

### Build from Source

```bash
git clone https://github.com/Finesssee/ProxyPilot.git
cd ProxyPilot
go build -o proxypilot ./cmd/server
./proxypilot
```

---

## Usage

### First Launch

1. Copy config: `cp config.example.yaml config.yaml`
2. Run: `./proxypilot`
3. Server starts on `http://localhost:8317`
4. Open dashboard: `http://localhost:8317/proxypilot.html`

### Authentication

Run OAuth login for your provider:

```bash
./proxypilot --claude-login    # Claude
./proxypilot --codex-login     # OpenAI/Codex
./proxypilot --login           # Gemini
./proxypilot --kiro-login      # Kiro/AWS
```

Your browser opens automatically. Tokens are stored and auto-refreshed.

### Configure Your Tools

**Claude Code** (`~/.claude/settings.json`):
```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8317",
    "ANTHROPIC_AUTH_TOKEN": "your-api-key"
  }
}
```

**Codex CLI** (`~/.codex/config.toml`):
```toml
[openai]
api_base_url = "http://127.0.0.1:8317"
```

**Factory Droid** (`~/.factory/settings.json`):
```json
{
  "customModels": [{
    "name": "ProxyPilot",
    "baseUrl": "http://127.0.0.1:8317"
  }]
}
```

---

## API Endpoints

```
POST /v1/chat/completions     # OpenAI Chat Completions
POST /v1/responses            # OpenAI Responses API
POST /v1/messages             # Anthropic Messages API
GET  /v1/models               # List available models
GET  /healthz                 # Health check
```

All endpoints auto-translate between formats based on the target provider.

---

## CLI Tools

| Binary | Description |
|--------|-------------|
| `proxypilot` | Main API proxy server |
| `proxypilotui` | Desktop control center (Windows) |
| `cliproxytray` | System tray application (Windows) |
| `proxypilotpack` | Build and packaging tool |

---

## Tool Integrations

Works with these AI coding tools:

- **Claude Code** - Auto-configure via settings.json
- **Codex CLI** - Auto-configure via config.toml
- **Factory Droid** - Auto-configure via settings.json
- **Cursor IDE** - Manual endpoint configuration
- **Continue** - Manual endpoint configuration

---

## Requirements

- macOS, Linux, or Windows
- Go 1.24+ (for building from source)

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Support

- **Report Issues**: [GitHub Issues](https://github.com/Finesssee/ProxyPilot/issues)

---

## Disclaimer

This project is for educational and interoperability research purposes. It interacts with various APIs to provide compatibility layers.

- **Use at your own risk.** Authors are not responsible for account suspensions or service interruptions.
- **Not affiliated** with Google, OpenAI, Anthropic, Amazon, or any other provider.
- Users must comply with the Terms of Service of connected platforms.
