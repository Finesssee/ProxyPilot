# ProxyPilot Rust Replatform

This branch carries the Rust rewrite line for ProxyPilot.

## Branch contract

- Go `main` remains the shipping line while Rust grows.
- Rust lives under [`rust/`](/home/fsos/code-lean/ai-tools/ProxyPilot/rust).
- The Rust line is terminal-first and does not treat the browser dashboard as a required early target.
- The first real milestone is a working Rust executable that can proxy a Codex-compatible request end-to-end.

## Current milestone

`proxypilot-rs` is the first Rust binary in the repo. It currently provides:

- a `run` command that starts a local proxy server
- a `tui` command that opens a minimal terminal operator view
- a TOML config model plus a separate local state file for saved accounts
- explicit Codex-facing routes for `/v1/models`, `/v1/chat/completions`, `/v1/responses`, `/v1/responses/compact`, and Codex direct `/backend-api/codex/responses` aliases
- a local `/v0/runtime/stats` operator surface for runtime-memory counters, auth-health, and last refresh status
- read-only `/v0/management/status`, `/v0/management/config`, and `/v0/management/accounts` endpoints with API key and token material redacted
- CLI account commands for adding, importing, exporting, device-login, refresh, listing, activating, and removing accounts; these manage saved disk state only and do not pretend a running proxy has already adopted the change
- optional `codex.refresh_token_url` config support for redirecting Codex refresh exchanges to a local stub during validation, while preserving the live OpenAI default when unset
- a TUI operator console that shows live runtime stats, runtime-unavailable copy, local accounts, selected-account details, token state labels, terminal actions for activate/refresh/delete, separate selected-account vs active-account refresh handling, and the `r` / `c` reload-clear controls
- runtime credential resolution that prefers the active saved account over the config fallback key
- proactive Codex token refresh when the active saved account is already expired or within the refresh window, plus the existing 401-retry refresh fallback
- end-to-end tests against a mocked upstream server

## Design defaults

- Go is the behavioral reference, not a line-by-line port target.
- The Rust branch is allowed to redesign config and management surfaces.
- The product identity does not change: ProxyPilot still exists to be a local proxy for coding tools.

## Local workflow

From the repo root:

```bash
cd rust
cargo run -p proxypilot-rs -- init
cargo run -p proxypilot-rs -- account add-codex --name primary --api-key sk-...
cargo run -p proxypilot-rs -- account import-codex --file ../auths/codex-example.json --activate
cargo run -p proxypilot-rs -- account export --file proxypilot-rs.accounts.toml
cargo run -p proxypilot-rs -- account import --file proxypilot-rs.accounts.toml
cargo run -p proxypilot-rs -- account import --file proxypilot-rs.accounts.toml --replace
cargo run -p proxypilot-rs -- account login-codex-device --activate
cargo run -p proxypilot-rs -- account refresh-codex
cargo run -p proxypilot-rs -- account remove --name old-account
cargo run -p proxypilot-rs -- run --config proxypilot-rs.toml
```

Run the local Codex-compatible smoke check without touching real provider
accounts:

```bash
cd rust
./scripts/codex-smoke.sh
```

The smoke check starts a local mock upstream, saves a temporary Codex account,
starts the Rust proxy, and verifies `/healthz`, `/v0/runtime/stats`,
`/v1/models`, `/v1/chat/completions`, and `/v1/responses`.

Run the local Claude API-key smoke check:

```bash
cd rust
./scripts/claude-smoke.sh
```

The Claude smoke starts a local mock Anthropic-compatible upstream, starts the
Rust proxy with `providers.active = "claude"`, and verifies Claude API-key
headers on `/v1/models` and `/v1/messages`.

## Current Codex Gaps

The Rust line is not ready to replace the Go Codex surface yet. Current status:

- `/health` and `/healthz` both return Rust health status.
- `/v1/responses/compact` forwards to the active provider upstream.
- `/backend-api/codex/responses` and `/backend-api/codex/responses/compact`
  forward through the Codex-compatible `/v1/responses` paths when the active
  provider is `codex`; they reject non-Codex active providers instead of
  silently routing a Codex direct alias to Claude or another provider.
- `GET /v1/responses` and `GET /backend-api/codex/responses` return explicit
  `501 Not Implemented` responses because Rust websocket response proxying is
  still deferred.
- Codex websocket streaming
- Codex image generation and image edit handling
- Browser dashboard and broad `/v0/management/**` parity beyond status, config,
  and accounts
- Go model registry and model-mapping behavior

Open the TUI with:

```bash
cd rust
cargo run -p proxypilot-rs -- tui --config proxypilot-rs.toml
```
