# User Testing

Validation guidance for the Rust terminal operator-console mission.

**What belongs here:** validation surfaces, tools, setup notes, and concurrency guidance.
**What does not belong here:** service command definitions (use `.factory/services.yaml`).

---

## Validation Surface

### HTTP runtime surface
- Endpoints:
  - `GET /healthz`
  - `GET /v0/runtime/stats`
  - supported local Codex proxy routes touched by runtime tracking
- Primary tools:
  - `curl`
  - Rust mocked endpoint tests
- Validation focus:
  - local health identity
  - runtime stats schema and semantics
  - request counters
  - proactive refresh
  - 401 retry refresh
  - refresh failure tracking

### TUI surface
- Command:
  - `cargo run -p proxypilot-rs -- tui --config <path>`
- Primary tools:
  - `tuistory`
  - manual local smoke when needed
- Validation focus:
  - runtime/auth visibility
  - truthful footer/help text
  - local account visibility when runtime is down
  - reload, clear feedback, selected refresh, active refresh-now
  - readable empty/error states

### CLI/account surface
- Commands:
  - `init`
  - `account add-codex`
  - `account list`
  - `account activate`
  - `account remove`
  - `account refresh-codex`
- Primary tools:
  - shell
- Validation focus:
  - config creation semantics
  - disk-backed account lifecycle semantics
  - refreshable vs static-account behavior

## Validation Concurrency

- Automated Rust validation activities: max `2` concurrent
  - rationale: machine has ample memory/CPU, but Rust tests plus local mocked listener flows should stay conservative
- Manual TUI validation: max `1` at a time
  - rationale: interactive terminal verification is not realistically parallel and should not overlap with another heavy manual runtime smoke

## Setup Notes

- Baseline automated validation does not require live external credentials.
- Prefer mocked local upstream and refresh fixtures for HTTP/runtime tests.
- TUI validation requires a local config file and usually a running local Rust proxy for runtime-visible scenarios.
- Proxy-down scenarios are also required: validators should verify local disk-backed account visibility when runtime is unreachable.
