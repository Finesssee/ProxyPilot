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
- a TOML config model focused on one provider path: Codex
- an HTTP proxy path for `/v1/*` so the Rust line can forward real Codex/OpenAI-style traffic
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
cargo run -p proxypilot-rs -- run --config proxypilot-rs.toml
```

Open the TUI with:

```bash
cd rust
cargo run -p proxypilot-rs -- tui --config proxypilot-rs.toml
```
