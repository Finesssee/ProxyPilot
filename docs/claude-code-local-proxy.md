# Claude Code via CLIProxyAPI (local proxy)

This documents how to route Claude Code through CLIProxyAPI, and the related server changes made to improve model aliasing and auth error reporting.

## What we configured (Windows/macOS)

1. Run CLIProxyAPI locally (default in this repo uses `127.0.0.1:8318`).
2. Point Claude Code at the local proxy by setting Anthropic environment variables in your Claude Code user settings.

Claude Code reads settings from:

- Windows: `C:\Users\<you>\.claude\settings.json`
- macOS/Linux: `~/.claude/settings.json`

Example:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8318",
    "ANTHROPIC_AUTH_TOKEN": "local-dev-key",

    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "gemini-3-pro-preview",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "antigravity-claude-sonnet-4-5-thinking",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "antigravity-claude-opus-4-5-thinking"
  },
  "model": "opus"
}
```

Important:
- Avoid Claude Code “Auth conflict”: do **not** set both `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_API_KEY` at the same time.
- Restart Claude Code after editing settings so the new env vars take effect.

## Account risk warning

Claude/Anthropic OAuth, Google/Gemini OAuth, Gemini CLI OAuth, and Antigravity OAuth are unofficial compatibility paths. Providers may treat proxy, automation, shared-token, or third-party OAuth traffic as abnormal usage or a policy violation. These flows can risk quota loss, account restriction, or account suspension.

Do not use production, billing-critical, school/work, or irreplaceable personal accounts with these OAuth paths unless you accept that risk. Prefer official API keys or officially supported developer access for important accounts.

## Claude OAuth quota limitation

Anthropic currently treats third-party app traffic differently from regular Claude.ai or Claude Code subscription usage. If a Claude OAuth request fails with:

```text
Third-party apps now draw from your extra usage, not your plan limits.
Add more at claude.ai/settings/usage and keep going.
```

that response is coming from Anthropic, not ProxyPilot. It means the OAuth credential is valid, but Anthropic is requiring paid extra usage for this third-party API path. ProxyPilot cannot make those requests draw from normal Claude.ai subscription quota.

For Claude through ProxyPilot, either add extra usage in Anthropic settings, use a supported API-key billing path, or route the client to another provider/model alias such as Antigravity, Codex, Gemini, or Kiro.

## Server-side model aliasing (Claude handler)

CLIProxyAPI rewrites common Claude Code model aliases to locally-available models before routing:

- `opus` / `claude-opus-*` → `antigravity-claude-opus-4-5-thinking`
- `sonnet` / `claude-sonnet-*` → `antigravity-claude-sonnet-4-5-thinking`
- `haiku` / `claude-haiku-*` → `gemini-3-pro-preview`

Implementation: `sdk/api/handlers/claude/code_handlers.go` (`mapClaudeCodeModel`).

## Auth error reporting improvements

When all credentials are blocked for a model, CLIProxyAPI now reports more informative HTTP statuses:

- Quota/rate cooldown: `429` with `code: model_cooldown` and `Retry-After`
- Temporarily blocked credentials (non-quota): `503` with `code: auth_unavailable`, `Retry-After`, and a JSON error body including blocked counts and recent upstream HTTP statuses (when known)

Implementation: `sdk/cliproxy/auth/selector.go`, `sdk/cliproxy/auth/manager.go`.

## Quick sanity checks

List models as Claude Code would see them:

```powershell
$hdr=@{ 'User-Agent'='claude-cli'; 'Authorization'='Bearer local-dev-key' }
Invoke-RestMethod -Headers $hdr -Uri 'http://127.0.0.1:8318/v1/models' -Method Get
```

Or with `curl` (macOS/Linux):

```bash
curl -s \
  -H 'User-Agent: claude-cli' \
  -H 'Authorization: Bearer local-dev-key' \
  'http://127.0.0.1:8318/v1/models'
```

Send a minimal Claude Messages request through the proxy:

```powershell
$hdr=@{ 'User-Agent'='claude-cli'; 'Authorization'='Bearer local-dev-key'; 'Content-Type'='application/json' }
$body=@{ model='opus'; max_tokens=16; stream=$false; messages=@(@{ role='user'; content='hi' }) } | ConvertTo-Json -Depth 6
Invoke-RestMethod -Headers $hdr -Uri 'http://127.0.0.1:8318/v1/messages' -Method Post -Body $body
```

Or with `curl` (macOS/Linux):

```bash
curl -s \
  -H 'User-Agent: claude-cli' \
  -H 'Authorization: Bearer local-dev-key' \
  -H 'Content-Type: application/json' \
  -d '{"model":"opus","max_tokens":16,"stream":false,"messages":[{"role":"user","content":"hi"}]}' \
  'http://127.0.0.1:8318/v1/messages'
```
