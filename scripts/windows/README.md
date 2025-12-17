# Droid CLI + CLIProxyAPI (Windows)

This repo can run a local OpenAI-compatible proxy (`CLIProxyAPI`) and configure Factory's `droid` CLI to use it via BYOK custom models.

## One-time setup

1. From the repo root, run:
   - `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\setup-droid-cliproxy.ps1 -ProxyApiKey "<your-proxy-api-key>"`

This will:
- Build `bin\cliproxyapi.exe`
- Write `C:\Users\<you>\.factory\config.json` with a `custom_models` entry pointing at `http://127.0.0.1:8317/v1`

## Start / stop the proxy

- Build (recommended after pulling updates): `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-cliproxyapi.ps1`
- Start: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\start-cliproxy.ps1`
- Stop: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\stop-cliproxy.ps1`
- Restart: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\restart-cliproxy.ps1`

Logs:
- `logs\cliproxyapi.out.log`
- `logs\cliproxyapi.err.log`

## Auto-start at logon (Windows)

Recommended: install a per-user startup entry (no admin required):

- Install: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\install-cliproxyapi-startup-run.ps1`
- Uninstall: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\uninstall-cliproxyapi-startup-run.ps1`

Optional: install a scheduled task (may require admin depending on your system policy):

- Install: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\install-cliproxyapi-startup-task.ps1`
- Uninstall: `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\uninstall-cliproxyapi-startup-task.ps1`

This uses `scripts\run-cliproxyapi-hidden.vbs` to start the server hidden and write logs to `logs\`.

### One-time cleanup (if stop says "Access is denied")

That means CLIProxyAPI was started **elevated** (commonly via a scheduled task with `RunLevel=HighestAvailable`), and a normal restart can't kill it.

Run once in an **elevated PowerShell**:

- `taskkill /IM cliproxyapi.exe /F`
- `schtasks /Delete /TN "CLIProxyAPI-Logon" /F` (if it exists)

## Use it in Droid

- In `droid`, run `/model` and pick `CLIProxy (local)`
- In non-interactive mode, use: `droid exec --model custom:gpt-5.1 "..."` (replace model as needed)
- `scripts\setup-droid-cliproxy.ps1` adds several `custom:<model>` entries (e.g. `custom:gpt-5.2`, `custom:gpt-5.1-codex-max`, `custom:gemini-3-pro-preview`)
- For reasoning variants, pick a model like `CLIProxy (local): gpt-5.2 (reasoning: high)` (this uses `gpt-5.2(high)` under the hood).
- `gpt-5.1-codex-max` also has reasoning variants (including `xhigh`) like `custom:gpt-5.1-codex-max(xhigh)`.
