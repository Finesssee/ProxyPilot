# ProxyPilot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)

**ProxyPilot** is an API proxy and translation server that bridges CLI-based AI models with standard AI API consumers. It enables seamless integration between popular CLI AI tools (Claude Code, Codex CLI, Gemini CLI, etc.) and applications expecting OpenAI-compatible, Anthropic, or Google Gemini API formats.

## Features

- **Multi-Provider Support** - Connect to multiple AI backends:
  - Claude (Anthropic) - via OAuth or API key
  - OpenAI/Codex - via OAuth or API key
  - Gemini (Google) - via OAuth (Gemini CLI, Vertex, AI Studio) or API key
  - Kiro (AWS CodeWhisperer) - via OAuth
  - Qwen - via cookie-based authentication
  - Custom OpenAI-compatible endpoints

- **Protocol Translation** - Automatically translates between API formats:
  - OpenAI Chat Completions API
  - OpenAI Responses API
  - Anthropic Messages API
  - Google Gemini API
  - Gemini CLI internal format

- **OAuth Authentication** - Browser-based OAuth flows for CLI tools with automatic token refresh

- **Streaming Support** - Full SSE streaming support with configurable keep-alives and chunking

- **Credential Management** - Smart credential rotation with:
  - Round-robin and fill-first routing strategies
  - Per-credential model exclusions
  - Automatic quota exceeded handling
  - Configurable retry logic

- **Advanced Configuration** - Per-credential settings including:
  - Custom base URLs and headers
  - Per-key proxy configuration
  - Model aliasing and mappings
  - Global model routing

- **Management Interface** - Built-in control panel for monitoring and configuration

## Quick Start

### Prerequisites

- Go 1.21 or higher
- A valid credential for at least one supported provider

### Installation

```bash
# Clone the repository
git clone https://github.com/router-for-me/ProxyPilot.git
cd ProxyPilot

# Build the server
go build -o proxypilot ./cmd/server

# Run the server
./proxypilot
```

### First Run

On first run, ProxyPilot will create a default `config.yaml` from the example configuration. You can also copy it manually:

```bash
cp config.example.yaml config.yaml
```

The server starts on port `8317` by default. Access the API at:
```
http://localhost:8317/v1/chat/completions
```

### Adding Credentials

ProxyPilot supports multiple authentication methods. Add your credentials to `config.yaml`:

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

For OAuth-based providers (Gemini CLI, Claude, Codex, Kiro), the server will open a browser window for authentication when credentials are needed. Tokens are automatically refreshed.

## Configuration

See [`config.example.yaml`](config.example.yaml) for the complete configuration reference. Key sections include:

| Section | Description |
|---------|-------------|
| `host`, `port`, `tls` | Server binding and HTTPS settings |
| `api-keys` | API keys for authenticating incoming requests |
| `remote-management` | Management API access control |
| `claude-api-key` | Claude/Anthropic API credentials |
| `codex-api-key` | OpenAI/Codex API credentials |
| `gemini-api-key` | Google Gemini API credentials |
| `kiro` | Kiro (AWS CodeWhisperer) credentials |
| `openai-compatibility` | Custom OpenAI-compatible providers |
| `routing` | Credential selection strategy |
| `quota-exceeded` | Automatic failover behavior |
| `streaming` | SSE keep-alive and chunking options |
| `global-model-mappings` | Route model aliases across providers |

### Environment Variables

- `CLIPROXY_HARNESS_ENABLED` - Enable/disable harness injection (default: true)
- `CLIPROXY_TOKEN_AWARE_ENABLED` - Enable token-based compression (default: true)
- `CLIPROXY_COMPRESSION_THRESHOLD` - Context compression threshold (default: 0.75)

## API Endpoints

ProxyPilot exposes standard AI API endpoints:

```
POST /v1/chat/completions     # OpenAI Chat Completions
POST /v1/responses            # OpenAI Responses API
POST /v1/messages             # Anthropic Messages API
POST /v1/models               # List available models
GET  /health                  # Health check
```

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Disclaimer

This project is intended for educational and interoperability research purposes only. It interacts with various internal or undocumented APIs (such as Google's `v1internal`) to provide compatibility layers.

- **Use at your own risk.** The authors are not responsible for any consequences arising from the use of this software, including but not limited to account suspensions or service interruptions.
- This project is **not** affiliated with, endorsed by, or associated with Google, OpenAI, Anthropic, Amazon, or any other service provider mentioned.
- Users are expected to comply with the Terms of Service of the respective platforms they connect to.
