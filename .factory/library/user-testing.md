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

## Flow Validator Guidance: shell
- Use only `.factory/validation/terminal-operator-console/user-testing/fixtures/cli-shell/` plus `/home/fsos/.factory/missions/7e6968f8-b3d3-4772-9518-ca3a93ea1374/evidence/terminal-operator-console/cli-shell/`.
- Use temporary config/state files inside that fixture directory; do not modify repo-root default configs.
- Stay on CLI/account flows only; do not start long-lived proxy services from this fixture.
- Do not use live device-login. Prefer `init`, `account add-codex`, `account list`, `account activate`, `account remove`, and negative-path `account refresh-codex` checks against local fixture state.
- If refresh-success validation cannot be isolated without live OpenAI OAuth, record it as blocked with concrete evidence rather than using private credentials.


## Flow Validator Guidance: curl
- Use only `.factory/validation/terminal-operator-console/user-testing/fixtures/http-runtime/` plus `/home/fsos/.factory/missions/7e6968f8-b3d3-4772-9518-ca3a93ea1374/evidence/terminal-operator-console/http-runtime/`.
- Own ports `18318-18339` for this validator only. Keep the proxy on `127.0.0.1:18318` unless a collision forces another port in that range.
- Mock the Codex upstream locally; never send `/v1/*` validation traffic to the real OpenAI upstream.
- Auth-refresh success paths may be blocked if the running binary cannot be pointed at a local OAuth token stub. Failure-path validation with intentionally invalid refresh tokens is allowed if it stays within the assigned fixture.
- Capture before/after `curl` output for stats-changing requests so counter deltas are explicit.


## Flow Validator Guidance: tuistory
- Use only `.factory/validation/terminal-operator-console/user-testing/fixtures/tui-console/` plus `/home/fsos/.factory/missions/7e6968f8-b3d3-4772-9518-ca3a93ea1374/evidence/terminal-operator-console/tui-console/`.
- Run only one TUI session at a time.
- Own ports `18418-18439` for proxy/mock services used by this validator. Keep runtime-down scenarios fully local by stopping those fixture services instead of touching shared ports.
- Keep disk-backed state isolated to the assigned fixture config/state files so reload, activate, delete, and refresh actions are attributable to this validator alone.
- Capture snapshots before and after each key flow (`r`, `a`, `d`, `f`, `R`, `c`) and note whether feedback came from local disk state or live runtime stats.

## Discovered Validation Quirks

- CLI `account refresh-codex` and runtime-triggered refresh paths currently exchange refresh tokens against the built-in OpenAI OAuth endpoint. From the real user surface, validators can reliably prove refresh-attempt visibility and failure handling, but cannot point refresh success to a local stub without code-level test hooks.
- `tuistory` sessions should be closed explicitly after the TUI exits with `q`; the PTY can remain allocated even after the binary terminates.
- In runtime-down TUI scenarios, selected-account refresh (`f`) still attempts the hardwired OAuth refresh exchange for refreshable saved accounts and surfaces that remote failure text. Treat that as truthful runtime-dependent behavior, not as loss of local disk state.
