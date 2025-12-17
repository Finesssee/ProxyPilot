# ProxyPilot (CLIProxyAPI Desktop + Agentic CLI Hardening)

ProxyPilot is the desktop-facing packaging/branding for running and managing a local `CLIProxyAPI` instance, with extra compatibility features for agentic CLIs (Factory Droid, Codex CLI, Warp).

This doc summarizes the current behavior and the key “quality of life” features added recently.

## What’s included

- **ProxyPilot tray app (Windows)** to start/stop/restart the proxy, open UI, open logs, and toggle autostart.
- **Droid/Factory reliability fixes** so tool calls are emitted consistently and errors show up even for strict streaming clients.
- **Agentic prompt-budget middleware** to prevent upstream “prompt too long” failures.
- **Long-session memory** with anchored summaries + persistent TODO + pinned state (local disk).

## Quickstart (Windows)

## Build/Package without PowerShell

If you have Go installed, you can build/package from a single command (no `.ps1` scripts):

- Build: `go run .\cmd\proxypilotpack build`
- Zip: `go run .\cmd\proxypilotpack package-zip`
- Installer: `go run .\cmd\proxypilotpack package-setup`

### Build the proxy

- `.\scripts\build-cliproxyapi.ps1`

### Build ProxyPilot tray app

- `.\scripts\build-cliproxytray.ps1`

Output:

- `bin\ProxyPilot.exe`

### Package (zip)

- `.\scripts\package-cliproxytray.ps1`

Output:

- `dist\ProxyPilot.zip`

### Package (self-extracting installer)

- `.\scripts\package-cliproxytray-installer.ps1`

Output:

- `dist\ProxyPilot-Setup.exe`

## Endpoints / health

- `GET /healthz` (no auth)
- `GET /v1/models` (requires API key)

Example:

- `curl -H "Authorization: Bearer local-dev-key" http://127.0.0.1:<port>/v1/models`

## Agentic CLI compatibility (Droid / Codex / Warp)

### Tool-call correctness

For `/v1/responses` traffic from agentic CLIs, the proxy applies:

- Tool schema tightening (`additionalProperties:false`) to reduce invalid tool argument payloads.
- Tool argument sanitization (strip unknown fields from tool call args).
- Conversion of plain-text `<tool_call>{...}</tool_call>` blocks into structured Responses `function_call` items (when present).
- If a Factory/Stainless client omits `tools`, the proxy injects a minimal `tools` set and forces `tool_choice:"auto"` so the model can emit tool calls.

### Streaming behavior (Factory/Stainless)

Some strict clients request `stream:true` but don’t reliably render JSON error bodies.
For `factory-cli`/Stainless user agents, CLIProxyAPI may:

- Call upstream non-streaming and synthesize SSE so the client gets usable output/error text.

## Prompt budget (prevent “prompt too long”)

The `CodexPromptBudgetMiddleware` applies to agentic CLIs and will trim oversized requests:

- Keeps recent messages/input items
- Truncates very large text blocks
- Preserves tool call/result pairing to avoid “tool_use/tool_result mismatch” errors
- Avoids disabling tools for Droid/Factory

Model-aware budget:

- If the chosen model has a small `context_length`, the middleware uses a smaller max body budget to reduce upstream failures.

Override:

- `CLIPROXY_AGENTIC_MAX_BODY_BYTES` (bytes, clamped)

## Long-session memory (anchored compression + retrieval)

See `docs/memory.md` for details.

Key points:

- Events are stored to `.proxypilot/memory/sessions/<sessionKey>/events.jsonl`
- Anchored summary: `summary.md`
- Pinned always-on state: `pinned.md`
- Persistent TODO: `todo.md`

Session key precedence:

1. `X-CLIProxyAPI-Session`
2. `X-Session-Id`
3. JSON `prompt_cache_key`
4. JSON `metadata.session_id` / `session_id`
5. Hash fallback (auth+UA hashed; raw values not stored)

### Persistent TODO updates

You can update the TODO without touching files by sending:

- `X-CLIProxyAPI-Todo: <markdown>`

The proxy saves it to `todo.md` and strips the header before forwarding upstream.

## Autostart

ProxyPilot tray app can toggle “Launch on login”.
On Windows this uses a per-user autostart mechanism (no admin required).

## Logs / diagnostics

Useful paths:

- `logs/cliproxyapi.out.log`
- `logs/cliproxyapi.err.log`
- `logs/v1-responses-*.log` / `logs/v1-chat-completions-*.log` (request logs, if enabled)

## Repo safety / public readiness

- `config.yaml`, auth files, logs, binaries, and memory state are gitignored.
- Use `local-dev-key` only as an example; real keys should stay out of git.
