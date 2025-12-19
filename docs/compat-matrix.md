# Compatibility Matrix (Quick Reference)

This project proxies multiple “OpenAI-compatible” clients to multiple upstreams. Small schema differences matter.

## Clients

### Codex CLI (`codex_cli_rs/...`)

- Endpoint: `POST /v1/responses`
- Streaming: SSE (`text/event-stream`)
- Notes:
  - `data: [DONE]` sentinel is tolerated/expected by many Codex/OpenAI clients.
  - Response output ordering and `output_text` fields must be stable.
  - Routing transparency:
    - Responses from localhost include:
      - `X-ProxyPilot-Requested-Model`
      - `X-ProxyPilot-Resolved-Model`
      - `X-ProxyPilot-Provider-Candidates`
      - `X-ProxyPilot-Resolved-Provider`
    - Add `X-CLIProxyAPI-Debug-Auth: 1` (localhost only) to also receive:
      - `X-CLIProxyAPI-Selected-Provider`, `X-CLIProxyAPI-Selected-Auth`, `X-CLIProxyAPI-Selected-Label`, `X-CLIProxyAPI-Selected-Account`.

### Droid / Factory CLI (`factory-cli/...`, Stainless headers)

- Endpoint: `POST /v1/responses`
- Streaming behavior:
  - Sends `stream: true` but typically uses `Accept: application/json`
  - CLIProxyAPI uses a non-stream upstream call and **synthesizes SSE** for reliability.
- Notes:
  - Droid attempts to JSON-parse each SSE `data:` line, so `data: [DONE]` must **not** be emitted for this client.
  - Routing transparency:
    - For localhost requests, the same headers as Codex apply for underlying routing.
    - The `/v0/management/routing/preview?model=...` endpoint is the easiest way to debug which provider a `model` will map to before sending a turn from Droid/Factory.

### OpenAI-compatible SDKs (`openai`, `azure-openai`, etc.)

- Endpoints: `POST /v1/chat/completions`, `POST /v1/completions`.
- Streaming behavior:
  - If body has `"stream": true` and `Accept` includes `text/event-stream`, ProxyPilot streams SSE.
  - If `Accept: application/json` **without** `text/event-stream`, ProxyPilot automatically downgrades to non-streaming and returns JSON, even when `stream: true`.
- Notes:
  - Responses from localhost include routing headers as above.
  - For debugging, you can issue a test request and inspect:
    - `X-ProxyPilot-Requested-Model` vs `X-ProxyPilot-Resolved-Model` to see alias/`auto`/thinking normalisation.
    - `X-ProxyPilot-Provider-Candidates` and `X-ProxyPilot-Resolved-Provider` to see which upstream(s) and which provider actually ran.

## Upstreams

### Antigravity (`cloudcode-pa.googleapis.com`)

- Endpoints:
  - `v1internal:generateContent` (non-stream)
  - `v1internal:streamGenerateContent?alt=sse` (stream)
- Notes:
  - `request.contents[].parts[].text` must be a **string** (`"text":"..."`), not an object.

## Model routing rules

- Quick debugging helpers:
  - `GET /v0/management/routing/preview?model=<name>`
    - Returns JSON describing how a given model name will be normalised and which provider candidates will be used.
  - `POST /v0/management/routing/preview`
    - Accepts a full request JSON body (OpenAI/OpenAIResponses-compatible), extracts the `model` field, and returns the same routing preview for that model.
  - `/v0/management/routing/recent`
    - Returns a small in-memory list of the last few routed requests (timestamp, path, requested/resolved model, provider + candidates).
  - Both endpoints require a management key and are intended for localhost use.

### `gemini-3-*` provider preference

- Primary: `antigravity`
- Fallback: `gemini-cli` (only used if antigravity is unavailable/cooling)
- Rotation: disabled between these two providers to keep the primary stable.

## Regression tests

- Droid SSE parsing: `sdk/api/handlers/openai/openai_responses_handlers_sse_done_test.go`
- Codex SSE done sentinel: `sdk/api/handlers/openai/openai_responses_handlers_sse_json_compat_test.go`
- Antigravity request shape: `internal/runtime/executor/antigravity_executor_shape_test.go`
- Gemini-3-* provider ordering: `internal/util/provider_test.go` and `sdk/cliproxy/auth/provider_rotation_test.go`
