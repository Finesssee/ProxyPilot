# Droid CLI + ProxyPilot (Windows)

This repo can run a local OpenAI-compatible proxy (ProxyPilot) and configure Factory's `droid` CLI to use it via BYOK custom models.

## One-time setup

1. From the repo root, run:
   - `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-droid-cliproxy.ps1 -ProxyApiKey "<your-proxy-api-key>"`

This will:
- Build `bin\proxypilot-engine.exe` (and also writes a back-compat copy as `bin\cliproxyapi-latest.exe`)
- Write `C:\Users\<you>\.factory\config.json` with a `custom_models` entry pointing at `http://127.0.0.1:8317/v1`

## Start / stop the proxy

- Build (recommended after pulling updates): `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-cliproxyapi.ps1`
- Start: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\start-cliproxy.ps1`
- Stop: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\stop-cliproxy.ps1`
- Restart: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\restart-cliproxy.ps1`

Logs:
- `logs\proxypilot-engine.out.log`
- `logs\proxypilot-engine.err.log`

## Auto-start at logon (Windows)

Prefer using **ProxyPilot** for autostart (it can toggle “Launch on login” in the tray menu), and start/stop the proxy from there.

### One-time cleanup (if stop says "Access is denied")

That means the engine was started **elevated** (commonly via a scheduled task with `RunLevel=HighestAvailable`), and a normal restart can't kill it.

Run once in an **elevated PowerShell**:

- `taskkill /IM proxypilot-engine.exe /F`
- `taskkill /IM cliproxyapi.exe /F` (back-compat)
- `schtasks /Delete /TN "CLIProxyAPI-Logon" /F` (if it exists; legacy autostart)

## Use it in Droid

- In `droid`, run `/model` and pick `CLIProxy (local)`
- In non-interactive mode, use: `droid exec --model custom:gpt-5.1 "..."` (replace model as needed)
- `scripts\setup-droid-cliproxy.ps1` adds several `custom:<model>` entries (e.g. `custom:gpt-5.2`, `custom:gpt-5.1-codex-max`, `custom:gemini-3-pro-preview`)
- For reasoning variants, pick a model like `CLIProxy (local): gpt-5.2 (reasoning: high)` (this uses `gpt-5.2(high)` under the hood).
- `gpt-5.1-codex-max` also has reasoning variants (including `xhigh`) like `custom:gpt-5.1-codex-max(xhigh)`.
