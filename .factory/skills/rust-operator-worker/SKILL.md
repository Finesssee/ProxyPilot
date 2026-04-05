---
name: rust-operator-worker
description: Implement Rust terminal-operator-console features for proxypilot-rs with test-first discipline and truthful runtime/TUI verification.
---

# Rust Operator Worker

NOTE: Startup and cleanup are handled by `worker-base`. This skill defines the WORK PROCEDURE.

## When to Use This Skill

Use for features in the Rust replatform slice that touch `proxypilot-rs` runtime/auth logic, local HTTP observability, CLI/account coherence, or the terminal operator console.

## Required Skills

- `tdd` — invoke before implementation to drive failing tests first for new runtime/auth/TUI helper behavior.
- `tuistory` — invoke when manually validating terminal operator-console behavior.
- `verification-before-completion` — invoke before claiming the feature is complete so evidence matches the actual commands and outputs.

## Work Procedure

1. Read `mission.md`, `validation-contract.md`, mission `AGENTS.md`, and `.factory/library/*.md` relevant to the feature before editing.
2. Confirm which validation assertions the feature fulfills and restate them in your notes.
3. Write or update failing Rust tests first (red) for the feature's intended behavior before changing implementation.
4. Implement the smallest Rust changes needed to make those tests pass while preserving the mission boundaries:
   - keep the work inside the Rust replatform slice unless docs are explicitly part of the feature
   - prefer shared auth/runtime helpers over duplicating logic
   - keep runtime-memory state distinct from disk-backed saved state
5. Run focused checks during iteration, then the required final validators:
   - `cargo fmt --all --manifest-path rust/Cargo.toml`
   - `cargo test --manifest-path rust/Cargo.toml`
6. If the feature affects the TUI or runtime observability surface, perform manual verification with `tuistory` or equivalent local smoke and capture what changed on screen or via local HTTP output.
7. Review the changed files for truthfulness of operator-visible wording, especially where runtime state and disk state can diverge.
8. Produce a handoff that cites exact commands, tests added, interactive checks, and any discovered gaps.

## Example Handoff

```json
{
  "salientSummary": "Added a shared auth-health/runtime model plus `/v0/runtime/stats`, then wired the TUI runtime panel to the new stats surface. Ran fmt/tests successfully and manually verified runtime counters plus active-account refresh feedback in the terminal UI.",
  "whatWasImplemented": "Created a shared runtime/auth module for auth-health classification and refresh status, extended proxy state with in-memory counters and last refresh metadata, added GET /v0/runtime/stats with stable fields, and updated the TUI to render runtime stats while keeping disk-backed account controls separate.",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {
        "command": "cargo fmt --all --manifest-path rust/Cargo.toml",
        "exitCode": 0,
        "observation": "Formatting completed without changes needed after final pass."
      },
      {
        "command": "cargo test --manifest-path rust/Cargo.toml",
        "exitCode": 0,
        "observation": "Workspace tests passed, including new runtime stats and auth-health cases."
      }
    ],
    "interactiveChecks": [
      {
        "action": "Launched proxypilot-rs TUI against a local fixture config, pressed `r`, `R`, and `c`, and inspected the runtime/auth panel.",
        "observed": "Runtime counters and last refresh outcome became visible, active-account refresh targeted the active account even when another row was selected, and clear-feedback only cleared the transient status line."
      }
    ]
  },
  "tests": {
    "added": [
      {
        "file": "rust/proxypilot-rs/src/proxy.rs",
        "cases": [
          {
            "name": "runtime_stats_reset_after_restart",
            "verifies": "In-memory runtime stats start empty on fresh process start and do not persist across restarts."
          },
          {
            "name": "runtime_stats_reports_401_refresh_path_once",
            "verifies": "A single client request that triggers 401 and retry increments upstream 401 and refresh counters without double-counting total proxied requests."
          }
        ]
      }
    ]
  },
  "discoveredIssues": []
}
```

## When to Return to Orchestrator

- The feature requires changing mission boundaries or touching non-Rust product surfaces outside the approved scope.
- The intended behavior cannot be implemented truthfully without clarifying runtime-vs-disk ownership semantics.
- A required local validation path cannot be exercised because config/fixtures/runtime setup is missing or broken.
- The feature reveals a larger refactor is needed than fits a single worker session.
