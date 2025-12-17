# Cursor IDE Integration

CLIProxyAPI can be used as a local OpenAI-compatible endpoint for Cursor.

## 1) Start CLIProxyAPI

Run the server and note the listening address (default: `http://localhost:8318`).

## 2) Configure Cursor

In Cursor settings:

- **API base URL**: `http://localhost:8318` (recommended)  
  - Alternatively: `http://localhost:8318/v1` (CLIProxyAPI also provides non-`/v1` aliases for this case)
- **API key**: use one of the keys from `config.yaml` under `api-keys`  
  - Example: `your-cli-proxy-api-key`

## 3) Model selection

Cursor will query `GET /v1/models` (or `/models` when base URL ends with `/v1`).
Pick any model exposed by your configured providers.

## Notes

- `POST /v1/embeddings` is implemented and returns deterministic synthetic embeddings to keep IDE features working even when an embeddings backend is not configured.
