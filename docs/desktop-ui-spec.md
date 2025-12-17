# Desktop UI for CLIProxyAPI — Shared Spec (Windows first)

This doc captures shared architecture and product decisions for a desktop “controller app” that manages a local `CLIProxyAPI` instance. OS-specific details live in:

- Windows: `docs/windows-ui-spec.md`
- macOS (future): `docs/macos-ui-spec.md` (not yet)

## What we’re building

A small tray-first desktop app that can:

- start/stop/restart the local proxy
- show health + port + “copy base URL” snippets for IDE/agentic CLIs
- surface logs/request diagnostics to debug issues like 400/429, “prompt too long”, streaming quirks

It is not a full replacement for the server’s Management UI; it’s a launcher + diagnostics tool.

## Two-shell strategy (Wails + Tauri)

We can support both Wails and Tauri without forking business logic by standardizing on a single control interface.

### Primary shell: Wails (Go)

- Pros: simplest integration with existing Go codebase, easiest to ship v1 fast.
- Architecture: Wails UI ⇄ Go control layer (in-process).

### Secondary shell: Tauri (Rust)

- Pros: smaller footprint, strong native tray/window tooling, good distribution story.
- Architecture (recommended): Tauri UI ⇄ helper CLI ⇄ Go control layer.
  - Tauri calls `cliproxyctl` (or similar) as a subprocess.
  - Benefits: no Rust↔Go FFI, no duplicated process logic, easy testing.

## Shared control interface

### Proposed Go package

- `internal/desktopctl`
  - Pure Go logic for process management, config reading/writing, health, logs, autostart primitives.
  - No UI dependencies.

### Suggested helper CLI (used by Tauri and power users)

- `cmd/cliproxyctl`
  - `cliproxyctl status --config <path>`
  - `cliproxyctl start --config <path>`
  - `cliproxyctl stop`
  - `cliproxyctl restart --config <path>`
  - `cliproxyctl autostart on|off`
  - `cliproxyctl diag --config <path> --out <file|clipboard>`

Wails can call `internal/desktopctl` directly; Tauri can call `cliproxyctl`.

## Feature scope (v1)

### Core controls

- Start/Stop/Restart `CLIProxyAPI`.
- Detect if a proxy is already running:
  - If owned by UI: show as “Managed”.
  - If not owned: show as “External” and offer “Attach” (read-only) vs “Stop anyway” behind a confirmation.

### Health

- Health check: `GET http://127.0.0.1:<port>/v1/models` (or add `/healthz` later).
- Show last successful check time and last error message.

### Config

- Config path chooser + remember last choice.
- “Open config” action (external editor).
- Validate YAML on save (or show parse error).

### Logs / request diagnostics

- Open logs folder.
- Tail last N lines of `cliproxyapi.out.log` and `cliproxyapi.err.log`.
- “Copy diagnostics” button that includes:
  - app version, proxy version
  - config path, port, pid
  - last error + last N log lines
  - pointers to recent request logs

### Quick-copy snippets

- `OPENAI_BASE_URL=http://127.0.0.1:<port>/v1`
- `OPENAI_API_KEY=<your local key>`
- Minimal curl sanity checks.

## Nice-to-haves (v1.1+)

- “Recent requests” viewer (parse `logs/v1-*.log`):
  - list request IDs, status, provider, model, auth label, latency
  - click to open the log file at the relevant section
- Surface auth status:
  - list loaded auth files + provider + account email (when available)
  - show “cooling down” vs “active”
  - optionally display primary/backup preference (e.g. `antigravity`)
- “Safe config wizard”:
  - toggle localhost-only management
  - add/generate local dev API key
- Update mechanism (self-update) and server binary update strategy.

## Security / safety rules

- Default to localhost-only assumptions.
- Don’t display or log API keys unless user explicitly reveals/copies them.
- Autostart toggles must show exactly what command will run.
- Avoid writing outside user profile by default.

## Cross-platform notes

We should split OS-specific operations behind small interfaces in `internal/desktopctl`:

- Process operations (spawn, signal/terminate, pid discovery)
- Autostart
- “Open folder” / “Open URL”

Windows and macOS implementations can live in separate files via Go build tags:

- `*_windows.go`
- `*_darwin.go`

## Open questions

- Should the desktop app bundle `cliproxyapi-latest` or manage an existing install?
- Do we add a stable `/healthz` endpoint to make health checks cheaper and consistent?
- Do we want a dedicated “desktop control” HTTP endpoint in the proxy (vs local process control only)?

