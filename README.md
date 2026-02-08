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

- 🎯 **10 Auth Providers** - Claude, Codex (OpenAI), Gemini, Gemini CLI, Kiro (AWS), Amazon Q CLI, Qwen, Antigravity, MiniMax, Zhipu AI
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
- 🎨 **System Tray** - Native Windows tray app for quick access
- 🔄 **Auto-Updates** - Background update checking with one-click install
- ⏪ **Rollback Support** - Automatic crash detection with version recovery
- 📡 **60+ Management APIs** - Full control via REST endpoints

---

## Supported Providers

| Provider | Auth Method | Models |
|----------|-------------|--------|
| Claude (Anthropic) | OAuth2 / API Key | Claude Opus 4.5, Sonnet 4.5, Haiku 4.5 |
| Codex (OpenAI) | OAuth2 / API Key | GPT-5.2, GPT-5.2 Codex |
| Gemini | OAuth2 / API Key | Gemini 3 Pro, Gemini 3 Flash |
| Gemini CLI | OAuth2 | Cloud Code Assist (separate quota) |
| Kiro | OAuth2 + AWS SSO | AWS CodeWhisperer |
| Amazon Q CLI | Import from CLI | Amazon Q Developer |
| Qwen | OAuth2 | Qwen models |
| Antigravity | OAuth2 | Gemini via Antigravity (separate quota) |
| MiniMax | API Key | MiniMax M2, M2.1 models |
| Zhipu AI | API Key | GLM-4.5, GLM-4.6, GLM-4.7 |
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
# OAuth providers (opens browser)
./proxypilot --claude-login        # Claude
./proxypilot --codex-login         # OpenAI/Codex
./proxypilot --login               # Gemini
./proxypilot --kiro-login          # Kiro (Google OAuth)
./proxypilot --kiro-aws-login      # Kiro (AWS Builder ID)
./proxypilot --qwen-login          # Qwen
./proxypilot --antigravity-login   # Antigravity

# Import providers (from existing CLI tools)
./proxypilot --kiro-import         # Kiro IDE token

# API key providers (prompts for key)
./proxypilot --minimax-login       # MiniMax API key
./proxypilot --zhipu-login         # Zhipu AI API key
```

OAuth tokens are stored locally and auto-refreshed before expiry.

### Security Defaults (Auth + CORS)

- Proxy requests require API keys by default. To allow unauthenticated access (not recommended), set `allow-unauthenticated: true` in `config.yaml`.
- CORS is enabled for non-management endpoints by default (wildcard `*`). Management endpoints **do not** emit CORS headers unless you explicitly allow origins under `cors.management-allow-origins`.

Example:

```yaml
allow-unauthenticated: false
cors:
  allow-origins:
    - "http://localhost:5173"
  management-allow-origins:
    - "http://localhost:5173"
```

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

## Caching

ProxyPilot includes two caching layers to reduce latency and token usage.

### Response Cache

Caches full API responses for identical requests. Useful for repeated queries during development.

**Config** (`config.yaml`):
```yaml
response-cache:
  enabled: true           # Default: false
  max-size: 1000          # Max entries (default: 1000)
  max-bytes: 0            # Optional total cache size cap in bytes
  ttl-seconds: 300        # Cache TTL (default: 300 = 5 min)
  exclude-models:         # Models to skip (supports wildcards)
    - "*-thinking"
    - "o1-*"
```

### Prompt Cache

Synthetic prompt caching for providers without native support. Tracks repeated system prompts and estimates token savings.

**Config** (`config.yaml`):
```yaml
prompt-cache:
  enabled: true           # Default: false
  max-size: 500           # Max entries (default: 500)
  max-bytes: 0            # Optional total cache size cap in bytes
  ttl-seconds: 1800       # Cache TTL (default: 1800 = 30 min)
```

### Cache Management API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v0/management/cache/stats` | GET | Response cache stats (hits, misses, size) |
| `/v0/management/cache/clear` | POST | Clear response cache |
| `/v0/management/cache/enabled` | PUT | Enable/disable at runtime `{"enabled": true}` |
| `/v0/management/prompt-cache/stats` | GET | Prompt cache stats + estimated tokens saved |
| `/v0/management/prompt-cache/clear` | POST | Clear prompt cache |
| `/v0/management/prompt-cache/enabled` | PUT | Enable/disable at runtime |     
| `/v0/management/prompt-cache/top` | GET | Top 10 most-hit prompts |

---

## Lightweight Profile

For low‑memory or high‑throughput setups, you can disable heavier features:

```yaml
commercial-mode: true
usage-statistics-enabled: false
usage-sample-rate: 0.25
metrics-enabled: false
request-history-enabled: false
request-history-sample-rate: 0.25
agentic-harness-enabled: false
prompt-budget-enabled: false
request-log: false
response-cache:
  enabled: false
prompt-cache:
  enabled: false
```

---

## Auto-Updates

ProxyPilot includes a built-in auto-update system that checks for new releases and allows one-click installation.

### Configuration

**Config** (`config.yaml`):
```yaml
updates:
  auto-check: true              # Enable background update checking (default: true)
  check-interval-hours: 24      # How often to check (default: 24)
  notify-on-update: true        # Show toast notification when update available (default: true)
  channel: stable               # Update channel: "stable" or "prerelease" (default: stable)
```

### Features

- **Background Polling** - Checks for updates at configurable intervals
- **Dashboard Banner** - Proactive notification when update is available
- **One-Click Install** - Download, verify, and install from the tray menu
- **Session Dismissal** - Dismissed banners won't reappear until next session

### Tray Menu

When an update is available:
- **Settings → Download & Install vX.X.X** - One-click update flow
- **Settings → Check for Updates** - Manual check

---

## Rollback Support

ProxyPilot automatically backs up the previous version during updates and can restore it if something goes wrong.

### Automatic Recovery

- **Crash Detection** - Tracks rapid restarts within a 30-second window
- **Auto-Rollback** - After 3 rapid failures, automatically restores previous version
- **Health Marking** - After 30 seconds of stable operation, clears the failure counter

### Manual Rollback

From the tray menu:
- **Settings → Rollback to Previous Version** - Restore the previous version

### How It Works

1. During update, current binary is saved as `.old` backup
2. Rollback metadata stored in `%APPDATA%/ProxyPilot/updates/`
3. On crash loop detection, previous version is automatically restored
4. After successful startup (30s), backup can be cleaned up

---

## CLI Tools

| Binary | Description |
|--------|-------------|
| `proxypilot` | Main CLI with server and config switching |
| `proxypilot-tray` | System tray app |

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

## Author

ProxyPilot is developed and maintained by [@Finesssee](https://github.com/Finesssee). Some contributors shown in the git history are from upstream [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) merges. Direct contributors to ProxyPilot will be listed here as the project grows.

---

## Credits

ProxyPilot builds upon excellent work from the open-source community:

- **[CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)** - The original unified proxy server that inspired this project
- **[VibeProxy](https://github.com/automazeio/vibeproxy)** - Native macOS menu bar app showcasing clean proxy UX

Long-context features are inspired by research from the AI community:

- **[Factory.ai](https://factory.ai/news/evaluating-compression)** - Context compression techniques for long-running coding agents
- **[Anthropic](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)** - Effective harnesses for long-running agents

Special thanks to these teams for sharing their work and insights.

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
