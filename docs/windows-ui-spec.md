# Windows UI for CLIProxyAPI — Spec (Wails + Tauri)

## Goals

- Provide a Windows-friendly way to run and manage `CLIProxyAPI` without PowerShell.
- Avoid “prompt too long” / runaway context issues by surfacing request/log diagnostics.
- Support two UI implementations over time:
  - Primary: Wails (Go backend + web UI)
  - Secondary/experimental: Tauri (Rust + web UI)
- Share as much logic as possible via a Go control layer so features don’t fork.

## Non-goals (v1)

- Replacing the existing Management API UI shipped with the server.
- Full cross-platform parity (macOS/Linux can come later).
- Full OAuth flows inside the desktop UI (v1 can “open browser” and show status).
- Becoming an agent/IDE: this is a small controller + diagnostics utility.

## Product decisions (to avoid bikeshedding)

- v1 ships as Wails.
- Tauri is optional later; it should reuse the same control layer via a helper CLI.
- The UI controls an existing `cliproxyapi-latest.exe` (built by the UI or already present).
- The UI should not require elevated privileges and should default to localhost-only behavior.

## Personas / Use-cases

- “I just want it running”: start/stop, see port, see health.
- “I need to debug”: view logs, open request logs, copy diagnostics.
- “I’m configuring tools”: show local base URL, API key hints, and quick-copy snippets.
- “I want it to launch on login”: enable/disable autostart (per-user).

## UX requirements

- Tray-first: app sits in the system tray with minimal friction.
- Single click status: Running/Stopped + port + quick actions.
- No admin required for normal operation.
- Clear failure reasons: missing config, port in use, binary missing, auth dir missing.

## Functional requirements (v1)

### Process control

- Build (optional): build `cmd/server` into `bin/cliproxyapi-latest.exe`.
- Start:
  - Uses a selectable config path (default: repo `config.yaml` if present).
  - Writes logs to `logs/cliproxyapi.out.log` and `logs/cliproxyapi.err.log`.
- Stop:
  - Prefer graceful stop; fallback to kill process if needed.
- Restart.
- Detect running instance:
  - PID-based if started by UI, plus best-effort discovery by listening port.
  - If an instance is running but not owned by the UI, show “External” and offer read-only “Attach” (don’t blindly kill it).

### Status & health

- Show:
  - port (from config or detection)
  - reachable status via `GET http://127.0.0.1:<port>/v1/models` (or a future `/healthz`)
  - last start time and PID (when known)
- Buttons:
  - Open Management UI (browser)
  - Open logs folder
  - Copy base URL (`http://127.0.0.1:<port>` and optionally `/v1`)

### Config

- Config path selector (browse + remember last choice).
- Validate YAML on save (or warn on parse failure).
- Minimal editor:
  - open in external editor, and/or inline text editor with Save.
- Safe defaults:
  - warn if management routes are remotely accessible.

### Autostart

- Enable/disable per-user startup on Windows.
- Backend supports:
  - registry `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
  - scheduled task fallback (optional; clearly labeled)

### Quick-copy snippets (v1)

- Show ready-to-paste snippets:
  - `OPENAI_BASE_URL=http://127.0.0.1:<port>/v1`
  - `OPENAI_API_KEY=<your local key>`
  - curl sanity check for `GET /v1/models`

### Diagnostics / support bundle (v1.1)

- One-click “Copy diagnostics”:
  - version, config path, port, PID, last 200 lines of logs
  - recent request logs list (or last N request IDs)

## Shared Go control layer

### Why

Wails and Tauri can both use the same battle-tested operations: process management, config, logs, autostart, health checks.

### Proposed module (new)

- Package: `internal/desktopctl` (or `sdk/desktopctl` if you want it reusable externally)
- Responsibilities:
  - `Build(repoRoot) error`
  - `Start(configPath) (pid int, err error)`
  - `Stop(pidOrDiscovery) error`
  - `Restart(configPath) (pid int, err error)`
  - `Status(configPath) (Status, error)`
  - `TailLog(path, nLines) (string, error)`
  - `EnableAutostart(mode) error` / `DisableAutostart(mode) error`
  - `OpenBrowser(url) error`
- Notes:
  - Prefer discover-by-port over discover-by-process-name.
  - Persist UI-owned PID/state in `%LOCALAPPDATA%/CLIProxyAPI/ui-state.json`.

## Suggested helper CLI (for Tauri and scripting)

Even if Wails is primary, a tiny CLI is useful:

- `cmd/cliproxyctl` (new)
  - `cliproxyctl status --config <path>`
  - `cliproxyctl start --config <path>`
  - `cliproxyctl stop`
  - `cliproxyctl restart --config <path>`
  - `cliproxyctl autostart on|off`

This should call the same `internal/desktopctl` code.

## Wails app (primary) — MVP

### Packaging

- Ship `CLIProxyAPI Manager.exe` with embedded web UI assets.
- Bundle or download `cliproxyapi-latest.exe` (open decision).

### Screens

- Tray menu:
  - Start / Stop / Restart
  - Open UI
  - Open logs
  - Quit
- Main window:
  - Status card (Running/Stopped, port, last error)
  - Buttons: Start/Stop/Restart, Open Management UI, Open Logs
  - Config path selector + Open config
  - Autostart toggle
  - Copy snippets section (base URL + key + curl check)

### First-run UX

- If no config path is set:
  - Detect `config.yaml` in common locations (repo root, `%LOCALAPPDATA%/CLIProxyAPI/`, user home).
  - Offer “Use detected config” or “Browse…”.
- If no API key is configured:
  - Offer “Generate local key” (writes to config) or “Open config” guidance.

## Tauri app (secondary) — MVP

### Purpose

- Evaluate smaller footprint distribution vs Wails.
- Prototype a more native-feeling tray + notifications.

### Integration options

- Option A (recommended): Tauri calls a helper CLI (`cliproxyctl.exe`).
- Option B: Tauri starts a small local HTTP control server.

## Security considerations

- Default to localhost-only assumptions.
- Don’t display or log API keys unless explicitly requested.
- When enabling autostart, show what command will run.
- Avoid writing outside the user profile unless configured.

## Versioning / releases

- v0: tray app + start/stop/restart + open logs + open management UI.
- v1: config selection + autostart toggle + basic health check + copy snippets.
- v1.1: diagnostics bundle and log tail UI.

## Open decisions

- Where config lives by default (repo-local vs user config dir).
- Whether UI bundles the server binary or manages an existing install.
- Whether to add a stable `/healthz` endpoint for fast health checks.
- Whether “Generate local key” should mutate `config.yaml` or use a UI-only key store.

## Acceptance criteria (v1)

- Installing and launching the app shows a tray icon and a status window.
- Start/Stop/Restart works reliably across app restarts (PID state persisted).
- Health check distinguishes:
  - process running but endpoint not reachable
  - endpoint reachable with `GET /v1/models` returning 200
- Open logs opens the correct folder.
- Copy base URL copies `http://127.0.0.1:<port>` (and optionally `/v1`).
