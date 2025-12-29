# ppswitch

ProxyPilot Configuration Switcher - Switch AI CLI agents between proxy and native modes.

## Install

```bash
bun install -g ppswitch
```

Or run without installing:

```bash
bunx ppswitch
```

## Usage

```bash
# Show status of all agents
ppswitch

# Switch Claude to proxy mode
ppswitch claude proxy

# Switch Gemini to native mode
ppswitch gemini native

# Show help
ppswitch --help
```

## Supported Agents

| Agent | Command |
|-------|---------|
| Claude Code | `ppswitch claude proxy` |
| Gemini CLI | `ppswitch gemini proxy` |
| Codex CLI | `ppswitch codex proxy` |
| OpenCode | `ppswitch opencode proxy` |
| Factory Droid | `ppswitch droid proxy` |
| Cursor | `ppswitch cursor proxy` |
| Kilo Code* | `ppswitch kilo proxy` |
| RooCode* | `ppswitch roocode proxy` |

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
