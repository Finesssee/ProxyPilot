# Environment

Environment variables, external dependencies, and setup notes.

**What belongs here:** required env vars, local setup notes, external dependency expectations, and fixture guidance.
**What does not belong here:** service start/stop commands or fixed ports outside explanatory notes.

---

## External Dependencies

- No external database, cache, or queue is required for this mission.
- No live Codex/OpenAI credentials are required for baseline automated validation.
- Use mocked local HTTP fixtures for upstream proxy and refresh-token flows unless the user explicitly expands scope to live credentials.

## Local Config Notes

- The Rust slice uses a TOML config and a separate state file.
- Workers may generate temporary local configs for smoke validation.
- Relative state-file paths resolve relative to the chosen config file path.

## Local Runtime Notes

- The default Rust bind is `127.0.0.1:8318`.
- Keep runtime observability surfaces on the same local Rust proxy process.
- Avoid unrelated busy ports already present on the machine.
