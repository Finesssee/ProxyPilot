# ProxyPilot (Desktop wrapper for CLIProxyAPI)

ProxyPilot is the desktop-facing packaging/branding for running and managing a local `CLIProxyAPI` instance, with extra compatibility features for agentic CLIs (Droid/Factory, Codex CLI, Warp).

ProxyPilot is a fork/rebrand of CLIProxyAPI: the engine is still `CLIProxyAPI`, and ProxyPilot is the Windows tray app + installer + updater UX around it.

## What’s included

- ProxyPilot tray app (Windows) to start/stop/restart the proxy engine, open the dashboard, open logs, and toggle autostart.
- Prompt-budget middleware to reduce “prompt too long” failures in strict CLIs.
- Long-session memory with pinned state + persistent TODO (local disk).
- Optional auth debugging and cooldown reset endpoints for local troubleshooting.

## Install / uninstall (Windows)

- Install: run `dist\\ProxyPilot-Setup.exe`
- Launch: Start Menu → ProxyPilot
- Uninstall: Settings → Apps → Installed apps → ProxyPilot → Uninstall

## Build / package

If you have Go installed:

- Build binaries: `go run .\\cmd\\proxypilotpack build`
- Package zip: `go run .\\cmd\\proxypilotpack package-zip` → `dist\\ProxyPilot.zip`
- Package installer (recommended): `go run .\\cmd\\proxypilotpack package-inno` (requires Inno Setup `ISCC.exe`) → `dist\\ProxyPilot-Setup.exe`

## Endpoints / health

- `GET /healthz` (no auth)
- `GET /v1/models` (requires API key)

Example:

- `curl -H "Authorization: Bearer local-dev-key" http://127.0.0.1:<port>/v1/models`

## Autostart toggles (tray)

ProxyPilot tray app can toggle:

- `Launch on login` (starts ProxyPilot)
- `Auto-start proxy` (starts the proxy engine when ProxyPilot launches)

On Windows this uses a per-user Run entry (no admin required).

## Logs / diagnostics

Useful paths:

- `logs/cliproxyapi.out.log`
- `logs/cliproxyapi.err.log`
- `logs/v1-responses-*.log` / `logs/v1-chat-completions-*.log` (request logs, if enabled)

