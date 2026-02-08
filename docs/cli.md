# ProxyPilot CLI

**proxypilot** is the unified command-line interface for the ProxyPilot local AI proxy. It provides both **server** functionality (running the proxy server) and **switch** functionality (toggling AI agents between proxy mode and native mode).

## Installation

### Via bun (recommended)

```bash
bun install -g proxypilot
```

### Via npm

```bash
npm install -g proxypilot
```

### Via Go

```bash
go install github.com/Finesssee/ProxyPilot/cmd/server@latest
```

### Manual Download

Download pre-built binaries from [GitHub Releases](https://github.com/Finesssee/ProxyPilot/releases).

Available binaries:
- `proxypilot-darwin-amd64` - macOS Intel
- `proxypilot-darwin-arm64` - macOS Apple Silicon
- `proxypilot-linux-amd64` - Linux x64
- `proxypilot-linux-arm64` - Linux ARM64
- `proxypilot-windows-amd64.exe` - Windows x64

## Server Mode

Run ProxyPilot as a local proxy server:

```bash
proxypilot                           # Start server on default port (8317)
proxypilot -config /path/to/config   # Use custom config file
```

### Server Features

- Routes AI requests through configured providers
- Automatic model aliasing and translation
- OAuth login helpers for various providers
- Agent configuration management

### Login Commands

```bash
proxypilot --login                   # Login to Google Account (Gemini)
proxypilot --codex-login             # Login to Codex using OAuth
proxypilot --claude-login            # Login to Claude using OAuth
proxypilot --qwen-login              # Login to Qwen using OAuth
proxypilot --antigravity-login       # Login to Antigravity using OAuth
proxypilot --kiro-login              # Login to Kiro using Google OAuth
proxypilot --kiro-aws-login          # Login to Kiro using AWS Builder ID
proxypilot --minimax-login           # Add MiniMax API key
proxypilot --zhipu-login             # Add Zhipu AI API key
```

### Login Options

```bash
--no-browser                         # Don't open browser automatically for OAuth
--incognito                          # Use incognito/private mode (useful for multiple accounts)
--no-incognito                       # Force disable incognito mode
```

### Import Commands

```bash
proxypilot --vertex-import <file>    # Import Vertex service account key JSON
proxypilot --kiro-import             # Import Kiro token from Kiro IDE
```

## Agent Configuration

ProxyPilot can automatically configure AI CLI tools to use the proxy.

### Detect Installed Agents

```bash
proxypilot --detect-agents
```

Example output:
```
Detecting installed CLI agents...

  Claude Code:    [+] Installed
                  Binary: /usr/local/bin/claude
                  Config: ~/.claude/settings.json
  Codex CLI:      [+] Installed
                  Binary: /usr/local/bin/codex
                  Config: ~/.codex
  Gemini CLI:     [-] Not found
```

### Setup Commands

```bash
proxypilot --setup-all               # Configure all detected agents
proxypilot --setup-claude            # Configure Claude Code
proxypilot --setup-codex             # Configure Codex CLI
proxypilot --setup-droid             # Configure Factory Droid
proxypilot --setup-opencode          # Configure OpenCode
proxypilot --setup-gemini            # Configure Gemini CLI
proxypilot --setup-cursor            # Configure Cursor
proxypilot --setup-kilo              # Configure Kilo Code CLI
proxypilot --setup-roocode           # Configure RooCode (VS Code)
```

## Switch Mode

Switch AI agents between proxy mode (through ProxyPilot) and native mode (direct API access). Think of it like `nvm` for Node versions, but for AI agent configurations.

### Usage

```bash
proxypilot --switch <agent>                    # Show status of specific agent
proxypilot --switch <agent> --mode proxy       # Switch agent to proxy mode
proxypilot --switch <agent> --mode native      # Switch agent to native mode
proxypilot --switch <agent> --mode status      # Show current mode (default)
```

### Supported Agents

| Agent | CLI Tool | Config Location |
|-------|----------|-----------------|
| `claude` | Claude Code | `~/.claude/settings.json` |
| `gemini` | Gemini CLI | `~/.gemini/settings.json` |
| `codex` | Codex CLI | `~/.codex/config.toml` |
| `opencode` | OpenCode | `~/.opencode/config.json` |
| `droid` | Factory Droid | `~/.factory/settings.json` |
| `cursor` | Cursor IDE | `~/.cursor/config.json` |
| `kilo` | Kilo Code* | VS Code settings |
| `roocode` | RooCode* | VS Code settings |

*VS Code extensions require manual configuration - proxypilot will display instructions.

### Modes

| Mode | Description |
|------|-------------|
| `proxy` | Route through ProxyPilot (`http://127.0.0.1:8317`) |
| `native` | Use direct API access (restore original config) |
| `status` | Show current mode (default) |

### Examples

#### Check single agent status

```bash
$ proxypilot --switch claude
Claude Code: proxy
Config: ~/.claude/settings.json
```

#### Switch Claude to proxy mode

```bash
$ proxypilot --switch claude --mode proxy
Switched Claude Code to proxy mode
Config: ~/.claude/settings.json
Backup: ~/.claude/settings.json.native.json
```

#### Switch Gemini back to native mode

```bash
$ proxypilot --switch gemini --mode native
Switched Gemini CLI to native mode
Restored from: ~/.gemini/settings.json.native.json
```

### How Switch Works

#### Switching to proxy mode

1. Backs up current config to `<config>.native.json`
2. Updates config to point to `http://127.0.0.1:8317`
3. Preserves all other settings (API keys, preferences, etc.)

#### Switching to native mode

1. Restores config from `<config>.native.json` backup
2. Removes backup file after successful restore
3. Original API keys and settings are restored

#### Config Detection

ProxyPilot automatically detects which mode an agent is in by checking:
- If config contains `127.0.0.1:8317` -> proxy mode
- If backup file exists -> was previously in proxy mode
- Otherwise -> native mode

## Agent-Specific Notes

### Claude Code

Config location: `~/.claude/settings.json`

Proxy mode sets:
```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8317"
  }
}
```

### Codex CLI

Config location: `~/.codex/config.toml`

Proxy mode sets:
```toml
[openai]
api_base_url = "http://127.0.0.1:8317"
```

### Gemini CLI

Config location: `~/.gemini/settings.json`

Proxy mode sets the API endpoint to ProxyPilot.

### Kilo Code & RooCode

These are VS Code extensions that require manual configuration:

1. Open VS Code Settings (Ctrl+,)
2. Search for the extension settings
3. Update the API base URL to `http://127.0.0.1:8317`

ProxyPilot will display these instructions when you run:
```bash
proxypilot --setup-kilo
proxypilot --setup-roocode
```

## Backup System

Before modifying any configuration file, ProxyPilot creates a timestamped backup:

```
~/.claude/.proxypilot-backups/
  settings.json.20251228-221853.bak

~/.codex/.proxypilot-backups/
  config.toml.20251228-221853.bak
```

### Restoring Backups

```bash
# List backups
ls ~/.claude/.proxypilot-backups/

# Restore
cp ~/.claude/.proxypilot-backups/settings.json.<timestamp>.bak ~/.claude/settings.json
```

## Troubleshooting

### "Config not found"

The agent's config file doesn't exist. Run the agent once to create it, or create the config manually.

### "Failed to backup config"

Check file permissions on the config directory.

### "Already in proxy/native mode"

The agent is already configured for the requested mode. No changes made.

### CLI not detected

If a CLI tool is installed but not detected:
1. Ensure the binary is in your `PATH`
2. Try running the tool directly to verify it's installed
3. Check if the config directory exists

### Configuration not applied

1. Restart the CLI tool after running setup
2. Verify ProxyPilot is running on port 8317
3. Check the backup directory for the original config

## Full Usage Reference

```bash
proxypilot -h, --help                # Show help
proxypilot -v, --version             # Show version

# Server
proxypilot                           # Start proxy server
proxypilot -config <path>            # Use custom config file

# OAuth Logins
proxypilot --login                   # Google/Gemini login
proxypilot --codex-login             # Codex OAuth
proxypilot --claude-login            # Claude OAuth
proxypilot --qwen-login              # Qwen OAuth
proxypilot --antigravity-login       # Antigravity OAuth
proxypilot --kiro-login              # Kiro Google OAuth
proxypilot --kiro-aws-login          # Kiro AWS Builder ID

# Agent Detection & Setup
proxypilot --detect-agents           # Detect installed agents
proxypilot --setup-all               # Configure all agents
proxypilot --setup-<agent>           # Configure specific agent

# Agent Switching
proxypilot --switch <agent>          # Show agent status
proxypilot --switch <agent> --mode proxy   # Switch to proxy
proxypilot --switch <agent> --mode native  # Switch to native
```

## Links

- [npm package](https://www.npmjs.com/package/proxypilot)
- [GitHub Releases](https://github.com/Finesssee/ProxyPilot/releases)
- [Source Code](https://github.com/Finesssee/ProxyPilot)
