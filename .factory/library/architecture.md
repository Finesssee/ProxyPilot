# Architecture

High-level system guidance for the Rust terminal operator-console slice.

**What belongs here:** module boundaries, state ownership, data flow, invariants, and where workers should add new behavior.
**What does not belong here:** low-level implementation diffs or line-by-line rewrite plans.

---

## Current Rust Entry Points

- `rust/proxypilot-rs/src/main.rs` wires CLI entrypoints.
- `cli.rs` defines command surfaces.
- `accounts.rs` handles CLI account operations against disk-backed account state.
- `config.rs` loads TOML config and resolves the state-file path.
- `state.rs` owns persisted local account state.
- `codex.rs` owns Codex device login and refresh-token exchange logic.
- `proxy.rs` owns the local HTTP proxy process and current in-memory proxy state.
- `tui.rs` owns the terminal operator console.

## State Layers

There are three distinct state layers in this mission. Workers must keep them separate and truthful:

1. **Config state**
   - Loaded from TOML via `config.rs`
   - Includes bind address, state-file path, upstream base URL, and fallback API key

2. **Saved local account state**
   - Loaded and saved through `state.rs`
   - Used by CLI account commands and by the TUI for disk-backed account visibility and local actions

3. **Live runtime state**
   - Lives inside the running proxy process
   - Includes the adopted active credential, request counters, refresh attempts/results, and any runtime-auth summary shown by `/v0/runtime/stats`
   - Must not silently re-read disk on every stats request and pretend disk changes already altered runtime memory

## Desired Shared Domain for This Mission

This milestone should create a small shared auth/runtime domain used by both proxy and TUI-facing logic. It should own:

- RFC3339 parsing / clock helpers
- auth-health classification with the shared vocabulary:
  - `valid`
  - `expiring_soon`
  - `expired`
  - `static`
  - `unknown`
- refresh outcome/status types
- runtime stats snapshot types for `/v0/runtime/stats`

The goal is to remove duplicated auth/time decision logic currently spread across `proxy.rs` and `tui.rs`.

## Runtime Stats Surface

`GET /v0/runtime/stats` is a read-only local operator endpoint.

It should expose runtime-memory state for:
- bind address
- upstream base URL
- active account name
- account count
- auth-health summary
- request counters
- last runtime refresh status
- last runtime refresh timestamp

This endpoint is the runtime source of truth for the TUI's operator-health panels.

## TUI Composition Model

The TUI should combine two inputs:

- **Disk-backed local state** for account list, selected-account details, and local account actions
- **Runtime stats HTTP data** for live auth/runtime visibility

This split is intentional. The UI must stay truthful if local saved state and running runtime state diverge.

## Mission Invariants

- The branch remains Codex-only for this mission.
- The TUI remains terminal-first and should become more operator-console-like, not more dashboard-like.
- Runtime counters are in-memory only and reset when the proxy process restarts.
- `R` targets the active account runtime action; `f` targets the selected saved account action.
- `r` reloads local/disk and runtime surfaces; `c` clears transient feedback only.
- Empty states and failures must stay readable and operator-meaningful.
