# Long-Session Memory (ProxyPilot / CLIProxyAPI)

CLIProxyAPI can keep long agentic CLI sessions running without manual `/compact` by:

- Persisting trimmed/dropped history to disk (external memory)
- Retrieving relevant snippets and injecting them back into the prompt when the request must be trimmed

This is **not** a 7M-token context window. It is a store + retrieval loop.

## When it activates

The memory flow runs inside `internal/api/middleware/codex_prompt_budget.go` for agentic CLIs:

- `OpenAI Codex`
- `factory-cli` / `droid`
- `warp`
- Stainless SDK clients (`X-Stainless-*` headers)

It only injects memory when a request is oversized and must be trimmed.

## Where memory is stored

Default directory:

- If `util.WritablePath()` is set: `<WritablePath>/.proxypilot/memory`
- Otherwise: `./.proxypilot/memory`

Per-session logs:

- `.proxypilot/memory/sessions/<sessionKey>/events.jsonl`

The session key is chosen in this order:

1. `X-CLIProxyAPI-Session` header (preferred)
2. `X-Session-Id` header
3. JSON `prompt_cache_key`
4. JSON `metadata.session_id` / `session_id`
5. Fallback hash of `Authorization + User-Agent` (hashed; raw values are not stored)

## Retrieval / injection

- Dropped items are appended as JSONL events (`dropped_chat` / `dropped_responses`)
- Retrieval is a lightweight keyword scorer over the last ~2MB of stored events
- Up to 8 snippets (~6k chars) are injected:
  - `/v1/responses`: appended to `instructions`
  - `/v1/chat/completions`: appended to the first system message, or prepended as a new system message

## Environment variables

- `CLIPROXY_MEMORY_ENABLED`:
  - default: enabled
  - set to `0`, `false`, `off`, or `no` to disable persistence + retrieval
- `CLIPROXY_MEMORY_DIR`: override the base directory used to store memory

