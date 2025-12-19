# WARP.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Common commands

### Go (engine + desktop binaries)

Build the main deliverables into `bin/`:

```pwsh
# Builds:
# - bin\proxypilot-engine.exe   (engine)
# - bin\ProxyPilot.exe          (tray app)
# - bin\ProxyPilotUI.exe        (desktop dashboard)
go run .\cmd\proxypilotpack build
```

Run the engine directly:

```pwsh
# Uses config.yaml in the repo root by default
# (or pass -config <path>)
go run .\cmd\server -config .\config.yaml
```

Windows helpers (used for local development and Droid/Factory CLI integration):

```pwsh
# build engine (writes bin\proxypilot-engine.exe)
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-cliproxyapi.ps1

# start/stop/restart engine (writes logs\proxypilot-engine.{out,err}.log)
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\start-cliproxy.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\stop-cliproxy.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\restart-cliproxy.ps1
```

Package artifacts:

```pwsh
# Zip bundle -> dist\ProxyPilot.zip
# (includes tray, UI, engine, and config.example.yaml)
go run .\cmd\proxypilotpack package-zip

# Windows installer (Inno Setup) -> dist\ProxyPilot-Setup.exe
# Requires ISCC.exe (Inno Setup 6) available on PATH.
go run .\cmd\proxypilotpack package-inno
```

Docker (engine in a container):

```pwsh
docker compose up --build
```

### Web UI (Vite)

The web UI lives in `webui/`:

```pwsh
cd .\webui
npm install
npm run dev      # local dev server
npm run build    # typecheck + build
npm run lint
```

### Tests

Run all Go tests:

```pwsh
go test ./...
```

Run a single package:

```pwsh
go test .\internal\api -run TestName
```

Run a single test across all packages:

```pwsh
go test ./... -run TestName
```

Regression tests called out in docs:

- Droid SSE parsing: `sdk/api/handlers/openai/openai_responses_handlers_sse_done_test.go`
- Codex SSE done sentinel: `sdk/api/handlers/openai/openai_responses_handlers_sse_json_compat_test.go`
- Antigravity request shape: `internal/runtime/executor/antigravity_executor_shape_test.go`

## High-level architecture

### Binaries (`cmd/`)

This repo produces multiple Go binaries:

- `cmd/server` (engine): HTTP proxy server exposing OpenAI/Gemini/Claude-compatible endpoints; also hosts OAuth callback endpoints.
- `cmd/cliproxytray` (Windows tray app): starts/stops the engine, opens UI, toggles autostart, checks for updates.
- `cmd/proxypilotui` (Windows desktop UI): WebView2 desktop window for the Control Center; talks to the running engine and uses management endpoints.
- `cmd/cliproxyctl`: small CLI to start/stop/status/open UI via `internal/desktopctl`.
- `cmd/cliproxymgr`: local web manager UI that wraps `internal/desktopctl`.
- `cmd/proxypilotpack`: build/packaging orchestrator (build binaries into `bin/`, create `dist/` artifacts).

### Engine request path (big picture)

1. `cmd/server/main.go` loads config (`config.yaml` or a store-backed config), decides whether to run login flows or start the server.
2. `internal/cmd/StartService` builds the service via the exported SDK (`sdk/cliproxy.NewBuilder()`) and runs it.
3. The HTTP server is implemented in `internal/api/server.go` using Gin:
   - `/v1/*` OpenAI-compatible endpoints (plus non-`/v1` aliases for clients that set base URL to `/v1`).
   - `/v1/responses` has special streaming/SSE behavior for strict CLIs (see `docs/compat-matrix.md`).
   - `/v1beta/*` Gemini-compatible endpoints.
   - `/v1internal:method` Gemini CLI compatibility endpoint.
   - OAuth callbacks like `/codex/callback`, `/google/callback`, etc. persist short-lived state for the login goroutines.

Core handler layer:

- `sdk/api/handlers/*` contains provider-specific API adapters (OpenAI/Gemini/Claude) and schema/streaming shims.
- `internal/api/handlers/management/*` contains management endpoints mounted under `/v0/management/*`.

### Management + desktop UX

- Management endpoints are only enabled when a secret is configured:
  - `remote-management.secret-key` in `config.yaml`, or
  - environment `MANAGEMENT_PASSWORD`.
- The tray app starts the engine via `internal/desktopctl.Start()` and injects a per-user `MANAGEMENT_PASSWORD` so the desktop UX can use `/v0/management/*` locally.
- The ProxyPilot dashboard page (`/proxypilot.html`) is served **localhost-only** and injects the management key into the HTML so users don’t paste keys manually.

Key components:

- `internal/desktopctl`: process management (start/stop/status), log file locations, and per-user UI state.
- `internal/api/proxypilot_dashboard.go`: local-only ProxyPilot dashboard HTML.
- `internal/managementasset`: downloads/serves the upstream management UI bundle and applies runtime text replacements for ProxyPilot branding.

### Config + persistence

- Default config is `config.yaml` (template: `config.example.yaml`).
- The engine can optionally use different config/auth persistence backends based on env vars (selected in `cmd/server/main.go`):
  - Postgres-backed store (`PGSTORE_*`)
  - Git-backed token store (`GITSTORE_*`)
  - Object store (`OBJECTSTORE_*`)

### SDK embedding

The proxy can be embedded as a Go library:

- `sdk/cliproxy` exposes a builder (`NewBuilder`) that wires config watching, auth, translators, and the HTTP server.
- `docs/sdk-usage.md` shows the supported server options (middleware/router hooks, request logging overrides, management enablement).

## Docs worth reading first

- `docs/proxypilot.md` (Windows packaging + local management key behavior)
- `docs/compat-matrix.md` (client/upstream quirks; streaming/SSE gotchas; tests to protect)
- `docs/cursor-ide.md` (how Cursor expects base URL and API key behavior)
- `docs/sdk-usage.md` (how the engine is composed when embedded)
