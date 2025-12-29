# ppswitch - ProxyPilot Config Switcher

**ppswitch** is a lightweight CLI tool that switches AI coding agents between proxy mode (through ProxyPilot) and native mode (direct API access). Think of it like `nvm` for Node versions, but for AI agent configurations.

## Installation

### Via bun (recommended)

```bash
bun install -g ppswitch
```

### Via npm

```bash
npm install -g ppswitch
```

### Via Go

```bash
go install github.com/Finesssee/ProxyPilot/cmd/ppswitch@latest
```

### Manual Download

Download pre-built binaries from [GitHub Releases](https://github.com/Finesssee/ProxyPilot/releases?q=ppswitch).

Available binaries:
- `ppswitch-darwin-amd64` - macOS Intel
- `ppswitch-darwin-arm64` - macOS Apple Silicon
- `ppswitch-linux-amd64` - Linux x64
- `ppswitch-linux-arm64` - Linux ARM64
- `ppswitch-windows-amd64.exe` - Windows x64

## Usage

```bash
ppswitch                    # Show status of all agents
ppswitch <agent>            # Show status of specific agent
ppswitch <agent> <mode>     # Switch agent to mode
ppswitch -h, --help         # Show help
ppswitch -v, --version      # Show version
```

## Supported Agents

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

*VS Code extensions require manual configuration - ppswitch will display instructions.

## Modes

| Mode | Description |
|------|-------------|
| `proxy` | Route through ProxyPilot (`http://127.0.0.1:8317`) |
| `native` | Use direct API access (restore original config) |

## Examples

### Check all agent statuses

```bash
$ ppswitch

Agent Status:
  claude    proxy     ~/.claude/settings.json
  gemini    native    ~/.gemini/settings.json
  codex     native    ~/.codex/config.toml
  droid     not found
```

### Switch Claude to proxy mode

```bash
$ ppswitch claude proxy
Switched Claude Code to proxy mode
Config: ~/.claude/settings.json
Backup: ~/.claude/settings.json.native.json
```

### Switch Gemini back to native mode

```bash
$ ppswitch gemini native
Switched Gemini CLI to native mode
Restored from: ~/.gemini/settings.json.native.json
```

### Check single agent status

```bash
$ ppswitch claude
Claude Code: proxy
Config: ~/.claude/settings.json
```

## How It Works

### Switching to proxy mode

1. Backs up current config to `<config>.native.json`
2. Updates config to point to `http://127.0.0.1:8317`
3. Preserves all other settings (API keys, preferences, etc.)

### Switching to native mode

1. Restores config from `<config>.native.json` backup
2. Removes backup file after successful restore
3. Original API keys and settings are restored

### Config Detection

ppswitch automatically detects which mode an agent is in by checking:
- If config contains `127.0.0.1:8317` → proxy mode
- If backup file exists → was previously in proxy mode
- Otherwise → native mode

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

ppswitch will display these instructions when you run:
```bash
ppswitch kilo proxy
ppswitch roocode proxy
```

## Troubleshooting

### "Config not found"

The agent's config file doesn't exist. Run the agent once to create it, or create the config manually.

### "Failed to backup config"

Check file permissions on the config directory.

### "Already in proxy/native mode"

The agent is already configured for the requested mode. No changes made.

## CI/CD

ppswitch is published automatically:

1. **Release workflow** (`.github/workflows/ppswitch-release.yml`)
   - Triggered by `ppswitch-v*` tags
   - Builds binaries for all platforms
   - Creates GitHub release with assets

2. **Publish workflow** (`.github/workflows/ppswitch-bun.yml`)
   - Triggered after successful release build
   - Publishes to npm registry via bun

### Creating a new release

```bash
# Update version in cmd/ppswitch/main.go and bun/ppswitch/package.json
git tag ppswitch-v0.2.0
git push origin ppswitch-v0.2.0
```

## Links

- [npm package](https://www.npmjs.com/package/ppswitch)
- [GitHub Releases](https://github.com/Finesssee/ProxyPilot/releases?q=ppswitch)
- [Source Code](https://github.com/Finesssee/ProxyPilot/tree/main/cmd/ppswitch)
