# Droid CLI + CLIProxyAPI (Windows)

This repo can run a local OpenAI-compatible proxy (`CLIProxyAPI`) and configure Factory's `droid` CLI to use it via BYOK custom models.

## One-time setup

1. From the repo root, run:
   - `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-droid-cliproxy.ps1 -ProxyApiKey "<your-proxy-api-key>"`

This will:
- Build `bin\cliproxyapi.exe`
- Write `C:\Users\<you>\.factory\config.json` with a `custom_models` entry pointing at `http://127.0.0.1:8317/v1`

## Start / stop the proxy

- Start: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\start-cliproxy.ps1`
- Stop: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\stop-cliproxy.ps1`

Logs:
- `logs\cliproxyapi.out.log`
- `logs\cliproxyapi.err.log`

## Use it in Droid

- In `droid`, run `/model` and pick `CLIProxy (local)`
- In non-interactive mode, use: `droid exec --model custom:gpt-5.1 "..."` (replace model as needed)
- `scripts\setup-droid-cliproxy.ps1` adds several `custom:<model>` entries (e.g. `custom:gpt-5.2`, `custom:gpt-5.1-codex`, `custom:gemini-3-pro-preview`)
- For reasoning variants, pick a model like `CLIProxy (local): gpt-5.2 (reasoning: high)` (this uses `gpt-5.2(high)` under the hood).
- `gpt-5.1-codex-max` also has reasoning variants (including `xhigh`) like `custom:gpt-5.1-codex-max(xhigh)`.
