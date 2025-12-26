<p align="center">
  <img src="https://raw.githubusercontent.com/Finesssee/ProxyPilot/main/static/icon.png" width="128" height="128" alt="ProxyPilot Logo">
</p>

<h1 align="center">ProxyPilot</h1>

<p align="center">
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://github.com/Finesssee/ProxyPilot/stargazers"><img src="https://img.shields.io/github/stars/Finesssee/ProxyPilot?style=social" alt="GitHub Stars"></a>
</p>

<p align="center">
  <strong>Stop juggling API keys.</strong> ProxyPilot is a powerful API proxy server that lets you use your existing Claude Code, Codex, Gemini, Kiro, and Qwen subscriptions with any AI coding tool – one unified endpoint for all your AI providers.
</p>

<p align="center">
  Built on Go, it handles OAuth authentication, token management, and API translation automatically. One server to route them all.
</p>

---

> 📣 **Latest models supported:** Claude Opus 4.5 / Sonnet 4.5 with extended thinking, GPT-5.1 / GPT-5.1 Codex, Gemini 2.5 Pro/Flash, and Kiro (AWS CodeWhisperer)! 🚀

---

## Features

- 🎯 **Multi-Provider Support** - Claude, OpenAI/Codex, Gemini, Kiro, Qwen, and custom endpoints
- 🔄 **Protocol Translation** - Auto-converts between OpenAI, Anthropic, and Gemini API formats
- 🔐 **OAuth Integration** - Browser-based OAuth flows with automatic token refresh
- 👥 **Multi-Account Support** - Round-robin distribution and automatic failover
- 📊 **Real-Time Streaming** - Full SSE support with configurable keep-alives
- ⚡ **Smart Routing** - Per-credential model exclusions and quota handling
- 🎨 **Management UI** - Built-in control panel for monitoring and configuration
- 💾 **Self-Contained** - Single binary, no external dependencies

## Installation

### Download Pre-built Release (Recommended)

1. Go to [Releases](https://github.com/Finesssee/ProxyPilot/releases)
2. Download the latest binary for your platform
3. Run the server

### Build from Source

```bash
# Clone the repository
git clone https://github.com/Finesssee/ProxyPilot.git
cd ProxyPilot

# Build the server
go build -o proxypilot ./cmd/server

# Run the server
./proxypilot
```

## Usage

### First Launch

1. Copy the example config: `cp config.example.yaml config.yaml`
2. Run ProxyPilot: `./proxypilot`
3. Server starts on `http://localhost:8317`
4. Configure your AI tool to use the ProxyPilot endpoint

### Authentication

**API Keys (Claude, OpenAI, Gemini):**
```yaml
claude-api-key:
  - api-key: "sk-ant-..."

codex-api-key:
  - api-key: "sk-..."

gemini-api-key:
  - api-key: "AIzaSy..."
```

**OAuth Authentication:**

For OAuth providers (Gemini CLI, Claude, Codex, Kiro), run the login command:
```bash
./proxypilot --claude-login    # Claude OAuth
./proxypilot --codex-login     # OpenAI/Codex OAuth
./proxypilot --login           # Gemini OAuth
./proxypilot --kiro-login      # Kiro/AWS OAuth
```

Browser opens automatically. Tokens are stored and auto-refreshed.

### API Endpoints

```
POST /v1/chat/completions     # OpenAI Chat Completions
POST /v1/responses            # OpenAI Responses API
POST /v1/messages             # Anthropic Messages API
GET  /v1/models               # List available models
GET  /healthz                 # Health check
```

## Configuration

See [`config.example.yaml`](config.example.yaml) for the complete reference.

| Section | Description |
|---------|-------------|
| `host`, `port`, `tls` | Server binding and HTTPS settings |
| `api-keys` | API keys for authenticating incoming requests |
| `claude-api-key` | Claude/Anthropic credentials |
| `codex-api-key` | OpenAI/Codex credentials |
| `gemini-api-key` | Google Gemini credentials |
| `kiro` | Kiro (AWS CodeWhisperer) credentials |
| `openai-compatibility` | Custom OpenAI-compatible providers |
| `routing` | Credential selection strategy (round-robin, fill-first) |
| `global-model-mappings` | Route model aliases across providers |

## Requirements

- Go 1.24+ (for building from source)
- macOS, Linux, or Windows

## Credits

ProxyPilot builds upon the excellent work of the open-source community. Special thanks to all contributors.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- **Report Issues:** [GitHub Issues](https://github.com/Finesssee/ProxyPilot/issues)
- **Contribute:** See [CONTRIBUTING.md](CONTRIBUTING.md)

---

## Disclaimer

This project is intended for educational and interoperability research purposes only. It interacts with various internal or undocumented APIs to provide compatibility layers.

- **Use at your own risk.** The authors are not responsible for any consequences arising from the use of this software, including but not limited to account suspensions or service interruptions.
- This project is **not** affiliated with, endorsed by, or associated with Google, OpenAI, Anthropic, Amazon, or any other service provider mentioned.
- Users are expected to comply with the Terms of Service of the respective platforms they connect to.
