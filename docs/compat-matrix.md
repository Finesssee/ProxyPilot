# Compatibility Matrix (Quick Reference)

This project proxies multiple “OpenAI-compatible” clients to multiple upstreams. Small schema differences matter.

## Clients

### Codex CLI (`codex_cli_rs/...`)

- Endpoint: `POST /v1/responses`
- Streaming: SSE (`text/event-stream`)
- Notes:
  - `data: [DONE]` sentinel is tolerated/expected by many Codex/OpenAI clients.
  - Response output ordering and `output_text` fields must be stable.

### Droid / Factory CLI (`factory-cli/...`, Stainless headers)

- Endpoint: `POST /v1/responses`
- Streaming behavior:
  - Sends `stream: true` but typically uses `Accept: application/json`
  - CLIProxyAPI uses a non-stream upstream call and **synthesizes SSE** for reliability.
- Notes:
  - Droid attempts to JSON-parse each SSE `data:` line, so `data: [DONE]` must **not** be emitted for this client.

## Upstreams

### Antigravity (`cloudcode-pa.googleapis.com`)

- Endpoints:
  - `v1internal:generateContent` (non-stream)
  - `v1internal:streamGenerateContent?alt=sse` (stream)
- Notes:
  - `request.contents[].parts[].text` must be a **string** (`"text":"..."`), not an object.

## Model routing rules

### `gemini-3-*` provider preference

- Primary: `antigravity`
- Fallback: `gemini-cli` (only used if antigravity is unavailable/cooling)
- Rotation: disabled between these two providers to keep the primary stable.

## Regression tests

- Droid SSE parsing: `sdk/api/handlers/openai/openai_responses_handlers_sse_done_test.go`
- Codex SSE done sentinel: `sdk/api/handlers/openai/openai_responses_handlers_sse_json_compat_test.go`
- Antigravity request shape: `internal/runtime/executor/antigravity_executor_shape_test.go`
- Gemini-3-* provider ordering: `internal/util/provider_test.go` and `sdk/cliproxy/auth/provider_rotation_test.go`
