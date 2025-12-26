<p align="center">
  <img src="./static/icon.png" width="128" height="128" alt="ProxyPilot Logo">
</p>

<h1 align="center">ProxyPilot</h1>

<p align="center">
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://x.com/Finessse377721"><img src="https://img.shields.io/badge/Follow-x%2F%40Finessse377721-1DA1F2?logo=x&logoColor=white" alt="Follow on X"></a>
</p>

<p align="center">
  <strong>Stop juggling API keys.</strong> ProxyPilot is a powerful API proxy that lets you use your existing Claude Code, Codex, Gemini, Kiro, and Qwen subscriptions with any AI coding tool – one unified endpoint for all your AI providers.
</p>

---

## Features

### Core Proxy

- **7 Auth Providers** - Claude, Codex (OpenAI), Gemini, Gemini CLI, Kiro (AWS CodeWhisperer), Qwen, iFlow
- **Universal API Translation** - Auto-converts between OpenAI, Anthropic, and Gemini formats
- **OAuth Integration** - Browser-based login with automatic token refresh
- **Multi-Account Support** - Round-robin distribution across credentials
- **Smart Routing** - Per-credential model exclusions, quota handling, automatic failover
- **Streaming Support** - Full SSE with configurable keep-alives

### Long-Context Features

- **Context Compression** - LLM-based summarization when approaching context limits (Factory.ai research)
- **Agentic Harness** - Guided workflow for long-running coding sessions (Anthropic research)
- **Session Memory** - Persistent event storage across conversation turns
- **Semantic Search** - Ollama-powered embeddings for memory retrieval

### Desktop App (Windows)

- **Control Center** - Native WebView2 app with start/stop, live logs, diagnostics
- **One-Click OAuth** - Login buttons for all providers
- **Agent Detection** - Auto-detects Claude Code, Codex CLI, Factory Droid, Gemini CLI
- **Model Mappings** - Visual UI for custom aliases
- **System Tray** - Quick access from taskbar

### Management API

- 60+ REST endpoints for configuration, credentials, routing, memory, and diagnostics
- Built-in web dashboard at `/proxypilot.html`
- Real-time log streaming
- Usage statistics

---

## Supported Providers

| Provider | Auth Method | Models |
|----------|-------------|--------|
| Claude (Anthropic) | OAuth2 / API Key | Claude Opus 4.5, Sonnet 4.5, Haiku |
| Codex (OpenAI) | OAuth2 / API Key | GPT-5.1, GPT-5.1 Codex, GPT-4.5 |
| Gemini | OAuth2 / API Key | Gemini 2.5 Pro, 2.5 Flash |
| Gemini CLI | OAuth2 | Cloud Code Assist models |
| Kiro | OAuth2 + AWS SSO | AWS CodeWhisperer |
| Qwen | OAuth2 | Qwen models |
| iFlow | Cookie-based | Alibaba models |
| Custom | API Key | Any OpenAI-compatible endpoint |

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

## Installation

### Download Release

1. Go to [Releases](https://github.com/Finesssee/ProxyPilot/releases)
2. Download binary for your platform
3. Run `./proxypilot`

### Build from Source

```bash
git clone https://github.com/Finesssee/ProxyPilot.git
cd ProxyPilot
go build -o proxypilot ./cmd/server
./proxypilot
```

---

## Quick Start

1. Copy config: `cp config.example.yaml config.yaml`
2. Run: `./proxypilot`
3. Server starts on `http://localhost:8317`
4. Open dashboard: `http://localhost:8317/proxypilot.html`

### OAuth Login

```bash
./proxypilot --claude-login    # Claude
./proxypilot --codex-login     # OpenAI/Codex
./proxypilot --login           # Gemini
./proxypilot --kiro-login      # Kiro/AWS
```

### Configure Your Tool

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

---

## Configuration

See [`config.example.yaml`](config.example.yaml) for full reference.

| Section | Description |
|---------|-------------|
| `host`, `port`, `tls` | Server binding |
| `api-keys` | Keys for authenticating requests |
| `claude-api-key` | Claude credentials |
| `codex-api-key` | OpenAI/Codex credentials |
| `gemini-api-key` | Gemini credentials |
| `kiro` | AWS CodeWhisperer credentials |
| `openai-compatibility` | Custom providers |
| `routing` | Credential selection strategy |
| `global-model-mappings` | Model aliases |

---

## Requirements

- Go 1.24+ (build from source)
- Windows, macOS, or Linux

---

## License

MIT License - see [LICENSE](LICENSE)

---

## Disclaimer

This project is for educational and interoperability research purposes. It interacts with various APIs to provide compatibility layers.

- **Use at your own risk.** Authors are not responsible for account suspensions or service interruptions.
- **Not affiliated** with Google, OpenAI, Anthropic, Amazon, or any other provider.
- Users must comply with the Terms of Service of connected platforms.
