# ProxyPilot Engine (macOS)

This folder contains macOS helpers for building, running, and auto-starting the ProxyPilot engine.

## One-time setup

From the repo root:

- Build: `./scripts/build.sh`

This builds `bin/proxypilot-engine` (and also writes a back-compat copy as `bin/cliproxyapi-latest`).

## Start / stop the proxy

- Start: `./scripts/start.sh`
- Stop: `./scripts/stop.sh`
- Restart: `./scripts/restart.sh`

Logs:
- `logs/proxypilot-engine.out.log`
- `logs/proxypilot-engine.err.log`

## Auto-start at login (launchd)

Install a per-user LaunchAgent (no admin required):

- Install: `./scripts/install-autostart.sh`
- Uninstall: `./scripts/uninstall-autostart.sh`

The LaunchAgent plist is written to `~/Library/LaunchAgents/com.cliproxyapi.plist`.
