# proxypilot

ProxyPilot CLI - Includes the ProxyPilot server and configuration switcher for AI CLI agents.

## Features

- **Server**: Run the ProxyPilot proxy server locally
- **Switch**: Switch AI CLI agents between proxy and native modes

## Install

```bash
bun install -g proxypilot
```

Or run without installing:

```bash
bunx proxypilot
```

## Usage

```bash
# Start the ProxyPilot server
proxypilot server

# Show status of all agents
proxypilot switch

# Switch Claude to proxy mode
proxypilot switch claude proxy

# Switch Gemini to native mode
proxypilot switch gemini native

# Show help
proxypilot --help
```

## Supported Agents

| Agent | Command |
|-------|---------|
| Claude Code | `proxypilot switch claude proxy` |
| Gemini CLI | `proxypilot switch gemini proxy` |
| Codex CLI | `proxypilot switch codex proxy` |
| OpenCode | `proxypilot switch opencode proxy` |
| Factory Droid | `proxypilot switch droid proxy` |
| Cursor | `proxypilot switch cursor proxy` |
| Kilo Code* | `proxypilot switch kilo proxy` |
| RooCode* | `proxypilot switch roocode proxy` |

\* VS Code extensions require manual configuration

## Modes

- `proxy` - Route through ProxyPilot (http://127.0.0.1:8317)
- `native` - Use direct API access (restore original config)

## How It Works

The switcher manages config files for each agent:

```
~/.claude/
├── settings.json           # Active config
├── settings.native.json    # Original/backup config
└── settings.proxy.json     # ProxyPilot config
```

## License

MIT
